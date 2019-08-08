// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package test // import "perun.network/go-perun/channel/test"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/pkg/io"
	"perun.network/go-perun/wallet"
)

type addressCreator = func() wallet.Address

// Setup provides all objects needed for the generic channel tests
type Setup struct {
	// Params are the random parameters of `State`
	Params *channel.Params
	// Params2 are the parameters of `State2` and must differ in all fields from `Params`
	Params2 *channel.Params

	// State is a random state with parameters `Params`
	State *channel.State
	// State2 is a random state with parameters `Params2` and must differ in all fields from `State`
	State2 *channel.State

	// Account is a random account
	Account wallet.Account
	// Account2 is a random account and must differ in all fields from `Account`
	Account2 wallet.Account
	// RandomAddress returns a new random address
	RandomAddress addressCreator
}

// GenericBackendTest tests the interface functions of the global channel.Backend with the passed test data.
// The global backend must be set prior to this with backend.Set{…} .
func GenericBackendTest(t *testing.T, s *Setup) {
	ID := channel.ChannelID(s.Params)
	require.Equal(t, ID, s.State.ID, "ChannelID(params) should match the States ID")
	require.Equal(t, ID, s.Params.ID(), "ChannelID(params) should match the Params ID")
	require.NotNil(t, s.State.Data, "State data can not be nil")
	require.NotNil(t, s.State2.Data, "State2 data can not be nil")

	t.Run("ChannelID", func(t *testing.T) {
		genericChannelIDTest(t, s)
	})

	t.Run("Sign", func(t *testing.T) {
		genericSignTest(t, s)
	})

	t.Run("Verify", func(t *testing.T) {
		genericVerifyTest(t, s)
	})
}

func genericVerifyTest(t *testing.T, s *Setup) {
	addr := s.Account.Address()
	require.Equal(t, channel.ChannelID(s.Params), s.Params.ID(), "Invalid test params")
	sig, err := channel.Sign(s.Account, s.Params, s.State)
	require.NoError(t, err, "Sign should work")

	ok, err := channel.Verify(addr, s.Params, s.State, sig)
	assert.NoError(t, err, "Verify should work")
	assert.True(t, ok, "Verify should not work")

	// Different state and same params
	ok, err = channel.Verify(addr, s.Params, s.State2, sig)
	assert.NoError(t, err, "Verify should work")
	assert.False(t, ok, "Verify should not work")

	// Different params and same state
	ok, err = channel.Verify(addr, s.Params2, s.State, sig)
	assert.NoError(t, err, "Verify should work")
	assert.False(t, ok, "Verify should not work")

	// Different params and different state
	for _, fakeParams := range buildModifiedParams(s.Params, s.Params2, s) {
		for _, fakeState := range buildModifiedStates(s.State, s.State2) {
			ok, err = channel.Verify(addr, &fakeParams, &fakeState, sig)
			assert.NoError(t, err, "Verify should not work")
			assert.False(t, ok, "Verify should not work")
		}
	}

	// Different address and same state and params
	for i := 0; i < 100; i++ {
		ok, err := channel.Verify(s.RandomAddress(), s.Params, s.State, sig)
		assert.NoError(t, err, "Verify should work")
		assert.False(t, ok, "Verify should not work")
	}
}

// buildModifiedStates returns a slice of States that are different from `s1` assuming that `s2` differs in
// every member from `s1`.
func buildModifiedStates(s1, s2 *channel.State) (ret []channel.State) {
	ret = append(ret, *s2)

	// Modify state
	{
		// Modify complete State
		{
			fakeState := *s2
			ret = append(ret, fakeState)
		}
		// Modify ID
		{
			fakeState := *s1
			fakeState.ID = s2.ID
			ret = append(ret, fakeState)
		}
		// Modify Version
		{
			fakeState := *s1
			fakeState.Version = s2.Version
			ret = append(ret, fakeState)
		}
		// Modify Allocation
		{
			// Modify complete Allocation
			{
				fakeState := *s1
				fakeState.Allocation = s2.Allocation
				ret = append(ret, fakeState)
			}
			// Modify Assets
			{
				// Modify complete Assets
				{
					fakeState := *s1
					fakeState.Allocation.Assets = s2.Allocation.Assets
					ret = append(ret, fakeState)
				}
				// Modify Assets[0]
				{
					fakeState := *s1
					fakeState.Assets = make([]io.Serializable, len(s1.Allocation.Assets))
					copy(fakeState.Allocation.Assets, s1.Allocation.Assets)
					fakeState.Allocation.Assets[0] = s2.Allocation.Assets[0]
					ret = append(ret, fakeState)
				}
			}
			// Modify OfParts
			{
				// Modify complete OfParts
				{
					fakeState := *s1
					fakeState.Allocation.OfParts = s2.Allocation.OfParts
					ret = append(ret, fakeState)
				}
				// Modify OfParts[0]
				{
					fakeState := *s1
					fakeState.Allocation.OfParts[0] = s2.Allocation.OfParts[0]
					ret = append(ret, fakeState)
				}
				// Modify OfParts[0][0]
				{
					fakeState := *s1
					fakeState.Allocation.OfParts[0][0] = s2.Allocation.OfParts[0][0]
					ret = append(ret, fakeState)
				}
			}
			// Modify Locked
			{
				// Modify complete Locked
				{
					fakeState := *s1
					fakeState.Allocation.Locked = s2.Allocation.Locked
					ret = append(ret, fakeState)
				}
				// Modify AppID
				{
					fakeState := *s1
					fakeState.Allocation.Locked[0].ID = s2.Allocation.Locked[0].ID
					ret = append(ret, fakeState)
				}
				// Modify Bals
				{
					fakeState := *s1
					fakeState.Allocation.Locked[0].Bals = s2.Allocation.Locked[0].Bals
					ret = append(ret, fakeState)
				}
				// Modify Bals[0]
				{
					fakeState := *s1
					fakeState.Allocation.Locked[0].Bals[0] = s2.Allocation.Locked[0].Bals[0]
					ret = append(ret, fakeState)
				}
			}
		}
		// Modify Data
		{
			fakeState := *s1
			fakeState.Data = s2.Data
			ret = append(ret, fakeState)
		}
		// Modify IsFinal
		{
			fakeState := *s1
			fakeState.IsFinal = s2.IsFinal
			ret = append(ret, fakeState)
		}
	}
	return
}

// buildModifiedParams returns a slice of Params that are different from `p1` assuming that `p2` differs in
// every member from `p1`.
func buildModifiedParams(p1, p2 *channel.Params, s *Setup) (ret []channel.Params) {
	ret = append(ret, *p2)

	// Modify params
	{
		// Modify ChallengeDuration
		{
			fakeParams := *p1
			fakeParams.ChallengeDuration = p2.ChallengeDuration
			ret = append(ret, fakeParams)
		}
		// Modify Parts
		{
			// Modify complete Parts
			{
				fakeParams := *p1
				fakeParams.Parts = p2.Parts
				ret = append(ret, fakeParams)
			}
			// Modify Parts[0]
			{
				fakeParams := *p1
				fakeParams.Parts = make([]wallet.Address, len(p1.Parts))
				copy(fakeParams.Parts, p1.Parts)
				fakeParams.Parts[0] = s.RandomAddress()
				ret = append(ret, fakeParams)
			}
		}
		// Modify App
		{
			fakeParams := *p1
			fakeParams.App = p2.App
			ret = append(ret, fakeParams)
		}
		// Modify Nonce
		{
			fakeParams := *p1
			fakeParams.Nonce = p2.Nonce
			ret = append(ret, fakeParams)
		}
	}

	return
}

func genericSignTest(t *testing.T, s *Setup) {
	_, err := channel.Sign(nil, s.Params, s.State)
	assert.Error(t, err, "Sign should fail")
	_, err = channel.Sign(s.Account, nil, s.State)
	assert.Error(t, err, "Sign should fail")
	_, err = channel.Sign(s.Account, s.Params, nil)
	assert.Error(t, err, "Sign should fail")

	// Test that different inputs produce different signatures
	sig1, err1 := channel.Sign(s.Account, s.Params, s.State)
	assert.Nil(t, err1, "Sign should work")

	for _, fakeParams := range buildModifiedParams(s.Params, s.Params2, s) {
		sig2, err2 := channel.Sign(s.Account, &fakeParams, s.State)

		assert.Nil(t, err2, "Sign should work")
		assert.NotEqual(t, sig1, sig2, "Sign for different params should differ")
	}

	{
		sig2, err2 := channel.Sign(s.Account, s.Params2, s.State2)

		assert.Nil(t, err2, "Sign should work")
		assert.NotEqual(t, sig1, sig2, "Sign for different state and params should differ")
	}
}

func genericChannelIDTest(t *testing.T, s *Setup) {
	require.NotNil(t, s.Params.Parts, "params.Parts can not be nil")
	assert.Panics(t, func() { channel.ChannelID(nil) }, "ChannelID(nil) should panic")

	// Check that modifying the state changes the id
	for _, fakeParams := range buildModifiedParams(s.Params, s.Params2, s) {
		ID := channel.ChannelID(&fakeParams)
		assert.NotEqual(t, ID, s.State.ID, "Channel ids should differ")
	}
}
