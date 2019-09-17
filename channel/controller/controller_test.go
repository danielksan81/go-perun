// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package channel // import "perun.network/go-perun/channel"

import "testing"

/*"testing"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"

sim "perun.network/go-perun/backend/sim/channel"
"perun.network/go-perun/backend/sim/wallet"
"perun.network/go-perun/channel"*/

func TestController(t *testing.T) {
	/*rng := rand.New(rand.NewSource(1337))
	app := sim.NewStateApp1(*wallet.NewRandomAddress(rng))

	randParams := newRandomParams(rng, app)
	params, err := channel.NewParams(sim.StateApp1ChDuration, randParams.Parts, app, randParams.Nonce)
	require.NoError(t, err)
	alloc := newRandomAllocation(rng, params)
	initData := &stateData{stateApp1InitValue}

	acc := wallet.NewRandomAccount(rng)
	params.Parts[0] = acc.Address()

	machine, err := channel.NewStateMachine(acc, *params)
	assert.NoError(t, err)

	err = machine.Init(*alloc, initData)
	assert.NoError(t, err)*/
}
