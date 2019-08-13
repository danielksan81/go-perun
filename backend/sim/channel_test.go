// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package sim

import (
	"math/big"
	"math/rand"
	"testing"

	perunbackend "perun.network/go-perun/backend"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
	"perun.network/go-perun/log"
	perunio "perun.network/go-perun/pkg/io"
	"perun.network/go-perun/wallet"
	perunwire "perun.network/go-perun/wire"
)

func TestGenericChannelBackendTests(t *testing.T) {
	b := new(Backend)
	perunbackend.Set(perunbackend.Collection{Wallet: b, Channel: b})

	setup := newChannelSetup(b)
	test.GenericBackendTest(t, setup)
}

func newRandomAllocation(rng *rand.Rand, params *channel.Params) channel.Allocation {
	assets := make([]perunio.Serializable, 10)
	for i := 0; i < len(assets); i++ {
		assets[i] = NewRandomAsset(rng)
	}

	ofparts := make([][]channel.Bal, len(params.Parts))
	for i := 0; i < len(ofparts); i++ {
		ofparts[i] = make([]channel.Bal, len(assets))
		for j := 0; j < len(ofparts); j++ {
			ofparts[i][j] = channel.Bal(big.NewInt(rng.Int63()))
		}
	}

	// Stub
	var locked []channel.Alloc

	return channel.Allocation{Assets: assets, OfParts: ofparts, Locked: locked}
}

func newRandomParams(rng *rand.Rand) channel.Params {
	var challengeDuration = rng.Uint64()
	parts := make([]wallet.Address, 10)
	for i := 0; i < len(parts); i++ {
		parts[i] = NewRandomAddress(rng)
	}
	adjudicator := NewRandomAddress(rng)
	app := NewApp(*adjudicator)
	nonce := big.NewInt(rng.Int63())

	return *channel.NewParams(challengeDuration, parts, app, nonce)
}

func newRandomState(rng *rand.Rand, p *channel.Params) channel.State {
	var random perunwire.ByteSlice
	_, err := rng.Read(random)
	if err != nil {
		log.Panic("rng read failed")
	}

	return channel.State{
		ID:         p.ID(),
		Version:    rng.Uint64(),
		Allocation: newRandomAllocation(rng, p),
		Data:       &random,
		IsFinal:    false,
	}
}

func newChannelSetup(b *Backend) *test.Setup {
	rng := rand.New(rand.NewSource(1337))

	params := newRandomParams(rng)
	params2 := newRandomParams(rng)

	return &test.Setup{
		Channel:  b,
		Wallet:   b,
		Params:   params,
		Params2:  params2,
		State:    newRandomState(rng, &params),
		State2:   newRandomState(rng, &params2),
		Account:  NewRandomAccount(rng),
		Account2: NewRandomAccount(rng),
	}
}
