package service

import (
	"math/big"

	"github.com/icon-project/goloop/module"
)

const (
	dataTypeNone    = ""
	dataTypeMessage = "message"
	dataTypeCall    = "call"
	dataTypeDeploy  = "deploy"
)

type TransactionHandler interface {
	Prepare(wc WorldContext) (WorldContext, error)
	Execute(wc WorldContext) (Receipt, error)
	Dispose()
}

type transactionHandler struct {
	from      module.Address
	to        module.Address
	value     *big.Int
	stepLimit *big.Int
	dataType  string
	data      []byte

	handler ContractHandler
	cc      CallContext
	receipt Receipt
}

func NewTransactionHandler(cm ContractManager, from, to module.Address,
	value, stepLimit *big.Int, dataType string, data []byte,
) TransactionHandler {
	tc := &transactionHandler{
		from:      from,
		to:        to,
		value:     value,
		stepLimit: stepLimit,
		dataType:  dataType,
		data:      data,
	}
	ctype := ctypeNone // invalid contract type
	switch dataType {
	case dataTypeNone:
		ctype = ctypeTransfer
	case dataTypeMessage:
		ctype = ctypeTransferAndMessage
	case dataTypeDeploy:
		ctype = ctypeTransferAndDeploy
	case dataTypeCall:
		ctype = ctypeTransferAndCall
	}

	tc.receipt = NewReceipt(to)
	tc.cc = newCallContext(tc.receipt)
	tc.handler = cm.GetHandler(tc.cc, from, to, value, stepLimit, ctype, data)
	if tc.handler == nil {
		return nil
	}
	return tc
}

func (th *transactionHandler) Prepare(wc WorldContext) (WorldContext, error) {
	return th.handler.Prepare(wc)
}

func (th *transactionHandler) Execute(wc WorldContext) (Receipt, error) {
	th.cc.Setup(wc)
	status, stepUsed, _, addr := th.cc.Call(th.handler)
	// TODO 확인 필요.
	if status != module.StatusSuccess {
		stepUsed = th.stepLimit
	}
	th.receipt.SetResult(status, stepUsed, wc.StepPrice(), addr)
	return th.receipt, nil
}

func (th *transactionHandler) Dispose() {
	th.cc.Dispose()
}
