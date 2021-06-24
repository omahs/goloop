/*
 * Copyright 2020 ICON Foundation
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

package icstate

import (
	"fmt"
	"math/big"

	"github.com/icon-project/goloop/common/codec"
	"github.com/icon-project/goloop/icon/iiss/icobject"
)

const (
	issueVersion1 = iota + 1
	issueVersion  = issueVersion1
)

type Issue struct {
	icobject.NoDatabase

	totalReward     *big.Int // amount of reward calculated by Issuer in current term
	prevTotalReward *big.Int
	overIssued      *big.Int // prevTotalReward - reward calculated by calculator
	iScoreRemains   *big.Int // not issued IScore
	prevBlockFee    *big.Int
}

func newIssue(_ icobject.Tag) *Issue {
	return new(Issue)
}

func NewIssue() *Issue {
	return &Issue{
		totalReward:     new(big.Int),
		prevTotalReward: new(big.Int),
		overIssued:      new(big.Int),
		iScoreRemains:   new(big.Int),
		prevBlockFee:    new(big.Int),
	}
}

func (i *Issue) Version() int {
	return issueVersion
}

func (i *Issue) RLPDecodeFields(decoder codec.Decoder) error {
	return decoder.DecodeAll(
		&i.totalReward,
		&i.prevTotalReward,
		&i.overIssued,
		&i.iScoreRemains,
		&i.prevBlockFee,
	)
}

func (i *Issue) RLPEncodeFields(encoder codec.Encoder) error {
	return encoder.EncodeMulti(
		i.totalReward,
		i.prevTotalReward,
		i.overIssued,
		i.iScoreRemains,
		i.prevBlockFee,
	)
}

func (i *Issue) Equal(o icobject.Impl) bool {
	if i2, ok := o.(*Issue); ok {
		return i.totalReward.Cmp(i2.totalReward) == 0 &&
			i.prevTotalReward.Cmp(i2.prevTotalReward) == 0 &&
			i.overIssued.Cmp(i2.overIssued) == 0 &&
			i.iScoreRemains.Cmp(i2.iScoreRemains) == 0 &&
			i.prevBlockFee.Cmp(i2.prevBlockFee) == 0
	} else {
		return false
	}
}

func (i *Issue) Clone() *Issue {
	ni := NewIssue()
	ni.totalReward = i.totalReward
	ni.prevTotalReward = i.prevTotalReward
	ni.overIssued = i.overIssued
	ni.iScoreRemains = i.iScoreRemains
	ni.prevBlockFee = i.prevBlockFee
	return ni
}

func (i *Issue) TotalReward() *big.Int {
	return i.totalReward
}

func (i *Issue) SetTotalReward(v *big.Int) {
	i.totalReward = v
}

func (i *Issue) PrevTotalReward() *big.Int {
	return i.prevTotalReward
}

func (i *Issue) SetPrevTotalReward(v *big.Int) {
	i.prevTotalReward = v
}

func (i *Issue) OverIssued() *big.Int {
	return i.overIssued
}

func (i *Issue) SetOverIssued(v *big.Int) {
	i.overIssued = v
}

func (i *Issue) IScoreRemains() *big.Int {
	return i.iScoreRemains
}

func (i *Issue) SetIScoreRemains(v *big.Int) {
	i.iScoreRemains = v
}

func (i *Issue) PrevBlockFee() *big.Int {
	return i.prevBlockFee
}

func (i *Issue) SetPrevBlockFee(v *big.Int) {
	i.prevBlockFee = v
}

func (i *Issue) Update(totalReward *big.Int, byFee *big.Int, byOverIssued *big.Int) *Issue {
	issue := i.Clone()
	issue.totalReward = new(big.Int).Add(issue.totalReward, totalReward)
	if byFee.Sign() != 0 {
		issue.prevBlockFee = new(big.Int).Sub(issue.prevBlockFee, byFee)
	}
	if byOverIssued.Sign() != 0 {
		issue.overIssued = new(big.Int).Sub(issue.overIssued, byOverIssued)
	}
	return issue
}

func (i *Issue) ResetTotalReward() {
	i.prevTotalReward = i.totalReward
	i.totalReward = new(big.Int)
}

func (i *Issue) Format(f fmt.State, c rune) {
	switch c {
	case 'v':
		if f.Flag('+') {
			fmt.Fprintf(f, "Issue{totalReward=%s prevTotalReward=%s overIssued=%s iscoreRemains=%s prevBlockFee=%s}",
				i.totalReward, i.prevTotalReward, i.overIssued, i.iScoreRemains, i.prevBlockFee)
		} else {
			fmt.Fprintf(f, "Issue{%s %s %s %s %s}",
				i.totalReward, i.prevTotalReward, i.overIssued, i.iScoreRemains, i.prevBlockFee)
		}
	}
}
