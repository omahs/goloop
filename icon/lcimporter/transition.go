/*
 * Copyright 2021 ICON Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package lcimporter

import (
	"bytes"
	"sync"

	"github.com/icon-project/goloop/common/codec"
	"github.com/icon-project/goloop/common/containerdb"
	"github.com/icon-project/goloop/common/db"
	"github.com/icon-project/goloop/common/errors"
	"github.com/icon-project/goloop/common/log"
	"github.com/icon-project/goloop/common/trie"
	"github.com/icon-project/goloop/common/trie/trie_manager"
	"github.com/icon-project/goloop/icon/merkle/hexary"
	"github.com/icon-project/goloop/module"
	"github.com/icon-project/goloop/service/scoredb"
	"github.com/icon-project/goloop/service/transaction"
	"github.com/icon-project/goloop/service/txresult"
)

type transitionID struct{ int }

type transitionState int

const (
	stepInitial transitionState = iota
	stepProposed
	stepNeedSync
	stepExecuting
	stepComplete
	stepCanceled
	stepFailed
)

type transitionBase struct {
	sm  *ServiceManager
	ex  *Executor
	log log.Logger
}

type transitionResult struct {
	State   []byte
	Receipt []byte
}

func (r *transitionResult) SetBytes(bs []byte) *transitionResult {
	if len(bs) > 0 {
		if _, err := codec.BC.UnmarshalFromBytes(bs, r); err == nil {
			return r
		}
	}
	r.State = nil
	r.Receipt = nil
	return r
}

func (r *transitionResult) Bytes() []byte {
	return codec.BC.MustMarshalToBytes(r)
}

type transition struct {
	*transitionBase

	pid    *transitionID
	parent *transition

	lock  sync.Mutex
	chn   chan interface{}
	state transitionState
	bi    module.BlockInfo
	txs   module.TransactionList

	worldSnapshot  trie.Immutable
	nextValidators module.ValidatorList
	receipts       module.ReceiptList
}

func (t *transition) PatchTransactions() module.TransactionList {
	return t.sm.emptyTransactions
}

func (t *transition) NormalTransactions() module.TransactionList {
	return t.txs
}

func (t *transition) PatchReceipts() module.ReceiptList {
	return t.sm.emptyReceipts
}

func (t *transition) NormalReceipts() module.ReceiptList {
	return t.receipts
}

func newTransactionSliceFromList(txl module.TransactionList) ([]*BlockTransaction, error) {
	var txs []*BlockTransaction
	for itr := txl.Iterator(); itr.Has(); _ = itr.Next() {
		tx, _, err := itr.Get()
		if err != nil {
			return nil, err
		}
		btx := transaction.Unwrap(tx).(*BlockTransaction)
		txs = append(txs, btx)
	}
	return txs, nil
}

func (t *transition) transitState(target transitionState, from ...transitionState) bool {
	t.lock.Lock()
	defer t.lock.Unlock()

	if len(from) > 0 {
		for _, s := range from {
			if t.state == s {
				t.state = target
				return true
			}
		}
		return false
	}
	t.state = target
	return true
}

func makeReceiptList(dbase db.Database, size int, rct txresult.Receipt) module.ReceiptList {
	rcts := make([]txresult.Receipt, size)
	for i := 0; i < size; i++ {
		rcts[i] = rct
	}
	return txresult.NewReceiptListFromSlice(dbase, rcts)
}

func (t *transition) getNextHeight() int64 {
	bsn := containerdb.NewBytesStoreSnapshotFromRaw(t.worldSnapshot)
	store := containerdb.NewBytesStoreStateWithSnapshot(bsn)
	return scoredb.NewVarDB(store, VarNextBlockHeight).Int64()
}

func (t *transition) setResult(txs int, next int64, mh *hexary.MerkleHeader, vl module.ValidatorList) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if txs > 0 {
		ws := trie_manager.NewMutableFromImmutable(t.parent.worldSnapshot)
		store := containerdb.NewBytesStoreStateFromRaw(ws)
		scoredb.NewVarDB(store, VarNextBlockHeight).Set(next)
		mhBytes := codec.BC.MustMarshalToBytes(mh)
		scoredb.NewVarDB(store, VarCurrentMerkle).Set(mhBytes)
		t.worldSnapshot = ws.GetSnapshot()
		t.log.Warnf("T_%p.SetResult(next=%d,mh=%s)", t, next, mh)
	} else {
		t.worldSnapshot = t.parent.worldSnapshot
	}
	t.receipts = makeReceiptList(t.sm.db, txs, t.sm.defaultReceipt)
	if vl != nil {
		t.nextValidators = vl
	} else {
		t.nextValidators = t.parent.nextValidators
	}
	t.state = stepComplete
}

func (t *transition) doExecute(cb module.TransitionCallback, check bool) (ret error) {
	defer func() {
		if ret != nil {
			t.transitState(stepFailed, stepExecuting)
			cb.OnValidate(t, ret)
		} else {
			cb.OnValidate(t, nil)
			t.transitState(stepComplete)
			cb.OnExecute(t, nil)
		}
	}()

	vls := t.sm.getInitialValidators()
	if t.bi.Height() == 0 {
		mh := &hexary.MerkleHeader{}
		t.setResult(1, 0, mh, vls)
		return nil
	}

	txs, err := newTransactionSliceFromList(t.txs)
	if err != nil {
		return err
	}

	if len(txs)>0 {
		if check {
			if err := t.checkTransactions(txs); err != nil {
				return err
			}
		}
		from := txs[0].Height
		next := txs[len(txs)-1].Height+1
		mh, err := t.ex.GetMerkleHeader(from)
		if err != nil {
			return err
		}
		t.setResult(len(txs), next, mh, vls)
	} else {
		t.setResult(0, -1, nil, vls)
	}
	return nil
}

func (t *transition) onTransactions(txs []*BlockTransaction, err error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	if err != nil {
		if t.state == stepExecuting {
			t.chn <- err
		}
	} else {
		t.chn <- txs
	}
}

func (t *transition) checkTransactions(txs []*BlockTransaction) error {
	if tx := txs[0]; tx.IsLast() {
		mh, err := t.ex.GetMerkleHeader(tx.Height)
		if err != nil {
			return err
		}
		if !bytes.Equal(tx.Result, mh.RootHash) {
			return errors.InvalidStateError.Errorf("DifferentAccumulatorHash(%#x!=%#x",
				tx.Result, mh.RootHash)
		} else {
			return nil
		}
	}
	from := txs[0].Height
	to := txs[len(txs)-1].Height
	if logServiceManager {
		t.log.Warnf("T_%p.GetTransactions(%d,%d)", t, from, to)
	}
	cancel, err := t.ex.GetTransactions(from, to, t.onTransactions)
	if err != nil {
		t.log.Warnf("FailToGetTransaction(from=%d,to=%d,err=%+v)", from, to, err)
		return err
	}

	result := <-t.chn
	if err, ok := result.(error); ok {
		if err == ErrCanceled {
			cancel()
		}
		t.log.Warnf("FailToGetTransaction(from=%d,to=%d,err=%+v)", from, to, err)
		return err
	}
	rtxs := result.([]*BlockTransaction)

	// check length
	if len(rtxs) != len(txs) {
		return errors.InvalidStateError.Errorf("DifferentLength(rtxs=%d,txs=%d)", len(rtxs), len(txs))
	}

	// compare each transactions
	for idx, tx := range txs {
		rtx := rtxs[idx]
		if !tx.Equal(rtx) {
			return errors.InvalidStateError.Errorf("DifferentTx(idx=%d,exp=%+v,real=%+v)", idx, tx, rtx)
		}
	}
	return nil
}

func (t *transition) doSync(cb module.TransitionCallback) (ret error) {
	defer func() {
		if ret != nil {
			t.transitState(stepFailed, stepExecuting)
			cb.OnValidate(t, ret)
		} else {
			cb.OnValidate(t, nil)
			t.transitState(stepComplete)
			cb.OnExecute(t, nil)
		}
	}()

	txs, err := newTransactionSliceFromList(t.txs)
	if err != nil {
		return err
	}
	if len(txs) < 1 {
		return errors.CriticalFormatError.New("NoTransactions")
	}

	if logServiceManager {
		t.log.Warnf("T_%p.SyncTransactions(from=%d,to=%d)",
			t, txs[0].Height, txs[len(txs)-1].Height)
	}
	if err := t.ex.SyncTransactions(txs); err != nil {
		return err
	}

	if err := t.checkTransactions(txs); err != nil {
		return err
	}

	from := txs[0].Height
	mh, err := t.ex.getMerkleHeaderInLock(from)
	if err != nil {
		return err
	}
	t.setResult(len(txs), from, mh, t.sm.getInitialValidators())
	return nil
}

var ErrCanceled = errors.NewBase(errors.InterruptedError, "Canceled")

func (t *transition) cancel() bool {
	t.lock.Lock()
	defer t.lock.Unlock()
	switch t.state {
	case stepComplete:
		return false
	default:
		if t.state == stepExecuting {
			t.chn <- ErrCanceled
		}
		t.state = stepCanceled
		return true
	}
	return true
}

func (t *transition) Execute(cb module.TransitionCallback) (canceler func() bool, err error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	switch t.state {
	case stepInitial, stepProposed:
		check := t.state == stepInitial
		t.state = stepExecuting
		t.chn = make(chan interface{},2)
		go t.doExecute(cb, check)
		return t.cancel, nil
	case stepNeedSync:
		t.state = stepExecuting
		t.chn = make(chan interface{},2)
		go t.doSync(cb)
		return t.cancel, nil
	default:
		return nil, errors.ErrInvalidState
	}
}

func (t *transition) ExecuteForTrace(ti module.TraceInfo) (canceler func() bool, err error) {
	return nil, errors.ErrUnsupported
}

func (t *transition) Result() []byte {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.state != stepComplete {
		return nil
	}
	result := &transitionResult{
		t.worldSnapshot.Hash(),
		t.receipts.Hash(),
	}
	return result.Bytes()
}

func (t *transition) NextValidators() module.ValidatorList {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.state != stepComplete {
		return nil
	}
	return t.nextValidators
}

func (t *transition) LogsBloom() module.LogsBloom {
	return new(txresult.LogsBloom)
}

func (t *transition) BlockInfo() module.BlockInfo {
	return t.bi
}

func (t *transition) Equal(t2 module.Transition) bool {
	tr2, _ := t2.(*transition)
	if t == tr2 {
		return true
	}
	if t == nil || tr2 == nil {
		return false
	}
	if t.pid == tr2.pid {
		return true
	}
	// TODO implement
	return false
}

func (t *transition) finalizeTransactions() error {
	if err := t.txs.Flush(); err != nil {
		return err
	}
	// special case for genesis block
	if t.bi.Height() == 0 {
		return nil
	}
	// finalize transactions
	var ltx module.Transaction
	for itr := t.txs.Iterator(); itr.Has() ; itr.Next() {
		if tx, _, err := itr.Get() ; err != nil {
			return err
		} else {
			ltx = tx
		}
	}
	if ltx == nil {
		return nil
	}
	if btx, ok := transaction.Unwrap(ltx).(*BlockTransaction) ; ok {
		if btx.IsLast() {
			if logServiceManager {
				t.log.Warnf("T_%p.FinalizeAll(root=%#x,blocks=%d)",
					btx.Result, btx.Height)
			}
			mh, votes, err := t.ex.FinalizeBlocks(btx.Height)
			if err != nil {
				return err
			}
			if !bytes.Equal(btx.Result, mh.RootHash) || btx.Height != mh.Leaves {
				return errors.InvalidStateError.Errorf("DifferentFinalizeData(%#x!=%#x or %d!=%d)",
					btx.Result, mh.RootHash, btx.Height, mh.Leaves)
			}
			if err := t.sm.ms.SetBlockV1Proof(btx.Result, btx.Height, votes); err != nil {
				return err
			}
			return errors.Wrap(ErrAfterLastBlock, "Finalized")
		}
		return nil
	} else {
		return errors.CriticalFormatError.New("InvalidLastTransaction")
	}
}

func (t *transition) finalizeResult() error {
	if err := t.worldSnapshot.(trie.Snapshot).Flush(); err != nil {
		return err
	}
	if err := t.nextValidators.Flush(); err != nil {
		return err
	}
	if err := t.receipts.Flush(); err != nil {
		return err
	}
	next := t.getNextHeight()
	if logServiceManager {
		t.log.Warnf("T_%p.FinalizeResult(next=%d)", t, next)
	}
	return t.ex.FinalizeTransactions(next-1)
}

func createInitialTransition(dbase db.Database, result []byte, nvl module.ValidatorList, sm *ServiceManager, ex *Executor) *transition {
	r := new(transitionResult).SetBytes(result)
	return &transition{
		transitionBase: &transitionBase{
			sm:  sm,
			ex:  ex,
			log: sm.log,
		},

		pid: new(transitionID),

		state:          stepComplete,
		worldSnapshot:  trie_manager.NewImmutable(dbase, r.State),
		receipts:       txresult.NewReceiptListFromHash(dbase, r.Receipt),
		nextValidators: nvl,
	}
}

func createTransition(parent *transition, bi module.BlockInfo, txs module.TransactionList, executed bool) *transition {
	var state transitionState
	if executed {
		state = stepProposed
	} else {
		state = stepInitial
	}
	tr := &transition{
		transitionBase: parent.transitionBase,

		pid:    new(transitionID),
		parent: parent,

		state: state,
		bi:    bi,
		txs:   txs,
	}
	return tr
}

func createSyncTransition(tr *transition) *transition {
	return &transition{
		transitionBase: tr.transitionBase,

		pid:    new(transitionID),
		parent: tr.parent,

		state: stepNeedSync,
		bi:    tr.bi,
		txs:   tr.txs,
	}
}
