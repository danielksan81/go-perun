// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package client_test

import (
	"math/big"
	"math/rand"
	"testing"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
	"perun.network/go-perun/client"
	"perun.network/go-perun/wallet"
	wallettest "perun.network/go-perun/wallet/test"
	"perun.network/go-perun/wire/msg"
)

func init() {
	channel.SetAppBackend(new(test.NoAppBackend))
	test.SetBackend(new(test.TestBackend))
	wallet.SetBackend(new(wallettest.DefaultWalletBackend))
	wallettest.SetBackend(new(wallettest.DefaultBackend))
}

func TestChannelProposalSerialization(t *testing.T) {
	rng := rand.New(rand.NewSource(0xdeadbeef))
	inputs := []*client.ChannelProposal{
		&client.ChannelProposal{
			ChallengeDuration: 0,
			Nonce:             big.NewInt(1),
			ParticipantAddr:   wallettest.NewRandomAddress(rng),
			AppDef:            wallettest.NewRandomAddress(rng),
			InitData:          test.NewRandomData(rng),
			InitBals: &channel.Allocation{
				Assets: []channel.Asset{&test.Asset{ID: 7}},
				OfParts: [][]channel.Bal{
					[]channel.Bal{big.NewInt(8)},
					[]channel.Bal{big.NewInt(9)}},
				Locked: []channel.SubAlloc{},
			},
			Parts: []wallet.Address{wallettest.NewRandomAddress(rng), wallettest.NewRandomAddress(rng)},
		},
		&client.ChannelProposal{
			ChallengeDuration: 99,
			Nonce:             big.NewInt(100),
			ParticipantAddr:   wallettest.NewRandomAddress(rng),
			AppDef:            wallettest.NewRandomAddress(rng),
			InitData:          test.NewRandomData(rng),
			InitBals: &channel.Allocation{
				Assets: []channel.Asset{&test.Asset{ID: 8}, &test.Asset{ID: 255}},
				OfParts: [][]channel.Bal{
					[]channel.Bal{big.NewInt(9), big.NewInt(131)},
					[]channel.Bal{big.NewInt(1), big.NewInt(1024)}},
				Locked: []channel.SubAlloc{
					channel.SubAlloc{
						ID:   channel.ID{0xCA, 0xFE},
						Bals: []channel.Bal{big.NewInt(11), big.NewInt(12)}}},
			},
			Parts: []wallet.Address{wallettest.NewRandomAddress(rng), wallettest.NewRandomAddress(rng)},
		},
	}

	for _, m := range inputs {
		msg.TestMsg(t, m)
	}
}

func TestChannelProposalAccSerialization(t *testing.T) {
	rng := rand.New(rand.NewSource(0xcafecafe))
	for i := 0; i < 16; i++ {
		m := &client.ChannelProposalAcc{
			SessID:          NewRandomSessID(rng),
			ParticipantAddr: wallettest.NewRandomAddress(rng),
		}
		msg.TestMsg(t, m)
	}
}

func TestChannelProposalRejSerialization(t *testing.T) {
	rng := rand.New(rand.NewSource(0xcafecafe))
	for i := 0; i < 1; i++ {
		r := make([]byte, 16+rng.Intn(16)) // random string of length 16..32
		rng.Read(r)
		m := &client.ChannelProposalRej{
			SessID: NewRandomSessID(rng),
			Reason: string(r),
		}
		msg.TestMsg(t, m)
	}
}

func NewRandomSessID(rng *rand.Rand) (id client.SessionID) {
	if _, err := rng.Read(id[:]); err != nil {
		panic("could not read from rng")
	}
	return
}