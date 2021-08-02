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

package blockv1_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/icon-project/goloop/common/codec"
	"github.com/icon-project/goloop/icon/blockv0"
	"github.com/icon-project/goloop/icon/blockv1"
	"github.com/icon-project/goloop/icon/ictest"
	"github.com/icon-project/goloop/module"
	"github.com/icon-project/goloop/test"
)

func newFixture(t *testing.T) *test.Node {
	return test.NewNode(t, ictest.UseBMForBlockV1)
}

func TestHandler_Basics(t_ *testing.T) {
	t := newFixture(t_)
	defer t.Close()

	t.AssertLastBlock(nil, module.BlockVersion1)
	t.ProposeImportFinalizeBlock((*blockv0.BlockVoteList)(nil))
	t.AssertLastBlock(t.PrevBlock, module.BlockVersion1)
	t.ProposeImportFinalizeBlock((*blockv0.BlockVoteList)(nil))
	t.AssertLastBlock(t.PrevBlock, module.BlockVersion1)
}

func TestHandler_BlockV13(t_ *testing.T) {
	t := newFixture(t_)
	defer t.Close()

	t.AssertLastBlock(nil, module.BlockVersion1)
	bc := t.ProposeBlock((*blockv0.BlockVoteList)(nil))

	var b blockv1.Format
	b.Version = module.BlockVersion1
	b.Height = t.LastBlock.Height()+1
	b.PrevHash = t.LastBlock.Hash()
	b.PrevID = t.LastBlock.ID()
	b.Result = bc.Result()
	b.VersionV0 = blockv0.Version03
	bs := codec.MustMarshalToBytes(&b)
	t.ImportFinalizeBlockByReader(bytes.NewReader(bs))
	t.AssertLastBlock(t.PrevBlock, module.BlockVersion1)
}

func NewVoteListV1ForLastBlock(t *test.Node) *blockv0.BlockVoteList {
	bv := blockv0.NewBlockVote(
		t.Chain.Wallet(),
		t.LastBlock.Height(),
		0,
		t.LastBlock.ID(),
		t.LastBlock.Timestamp() + 1,
	)
	return blockv0.NewBlockVoteList(bv)
}

func TestHandler_ValidatorChange(t_ *testing.T) {
	t := newFixture(t_)
	defer t.Close()

	t.AssertLastBlock(nil, module.BlockVersion1)

	nilVotes := (*blockv0.BlockVoteList)(nil)
	t.ProposeImportFinalizeBlockWithTX(nilVotes, fmt.Sprintf(`{
		"type": "test",
		"timestamp": "0x0",
		"validators": [ "%s" ]
	}`, t.Chain.Wallet().Address()))
	assert.Nil(t, t.LastBlock.NextValidatorsHash())

	t.ProposeImportFinalizeBlock(nilVotes)
	// now the tx is applied in the tx
	assert.NotNil(t, t.LastBlock.NextValidatorsHash())
	// still no votes
	assert.Nil(t, t.LastBlock.Votes())

	t.ProposeImportFinalizeBlock(NewVoteListV1ForLastBlock(t))
	// now there is some votes
	assert.NotNil(t, t.LastBlock.Votes())
}

func TestHandler_BlockVersionChange(t_ *testing.T) {
	t := newFixture(t_)
	defer t.Close()

	t.AssertLastBlock(nil, module.BlockVersion1)

	nilVotes := (*blockv0.BlockVoteList)(nil)
	t.ProposeImportFinalizeBlockWithTX(nilVotes, fmt.Sprintf(`{
		"type": "test",
		"timestamp": "0x0",
		"validators": [ "%s" ]
	}`, t.Chain.Wallet().Address()))
	assert.Nil(t, t.LastBlock.NextValidatorsHash())

	t.ProposeImportFinalizeBlock(nilVotes)
	// now the tx is applied in the tx
	assert.NotNil(t, t.LastBlock.NextValidatorsHash())
	// still no votes
	assert.Nil(t, t.LastBlock.Votes())

	t.ProposeImportFinalizeBlock(NewVoteListV1ForLastBlock(t))
	// now there is some votes
	assert.NotNil(t, t.LastBlock.Votes())

	t.ProposeImportFinalizeBlockWithTX(
		NewVoteListV1ForLastBlock(t),
		`{
			"type": "test",
			"timestamp": "0x0",
			"nextBlockVersion": "0x2"
		}`,
	)
	assert.EqualValues(
		t, module.BlockVersion1, t.SM.GetNextBlockVersion(t.LastBlock.Result()),
	)

	t.ProposeImportFinalizeBlock(NewVoteListV1ForLastBlock(t))
	t.AssertLastBlock(t.PrevBlock, module.BlockVersion1)
	// now the tx is applied in the tx
	assert.EqualValues(
		t, module.BlockVersion2, t.SM.GetNextBlockVersion(t.LastBlock.Result()),
	)

	t.ProposeImportFinalizeBlock(t.NewVoteListForLastBlock())
	t.AssertLastBlock(t.PrevBlock, module.BlockVersion2)

	t.ProposeImportFinalizeBlock(t.NewVoteListForLastBlock())
	t.AssertLastBlock(t.PrevBlock, module.BlockVersion2)
}
