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

package icconsensus_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/icon-project/goloop/icon/ictest"
	"github.com/icon-project/goloop/test"
)

func TestConsensus_WithAccumulatorBasics(t *testing.T) {
	gen := test.NewNode(t, ictest.UseBMForBlockV1, ictest.UseCSForBlockV1)
	defer gen.Close()

	const height = 10
	root, leaves := ictest.NodeGenerateBlocksAndFinalizeMerkle(gen, height)

	gen = test.NewNode(
		t, ictest.UseBMForBlockV1, ictest.UseCSForBlockV1,
		ictest.UseMerkle(root, leaves), test.UseDB(gen.Chain.Database()),
	)
	defer gen.Close()

	var err error
	for i:=0; i<height; i++ {
		_, err = gen.BM.GetBlockByHeight(int64(i))
		assert.NoError(t, err)
	}

	f := test.NewNode(
		t, ictest.UseBMForBlockV1, ictest.UseCSForBlockV1,
		ictest.UseMerkle(root, leaves),
	)
	defer f.Close()

	err = gen.CS.Start()
	assert.NoError(t, err)

	err = f.CS.Start()
	assert.NoError(t, err)

	f.NM.Connect(gen.NM)

	chn, err := f.BM.WaitForBlock(height-1)
	assert.NoError(t, err)
	blk := <-chn
	assert.EqualValues(t, height-1, blk.Height())
	assert.EqualValues(t, height, f.CS.GetStatus().Height)
}
