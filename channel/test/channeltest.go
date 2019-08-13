// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package test // import "perun.network/go-perun/channel/test"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
)

// Setup provides all objects needed for the generic tests
type Setup struct {
	Channel channel.Backend
	Wallet  wallet.Backend

	Params channel.Params
	// Params2 must differ in all fields from Params
	Params2 channel.Params

	State channel.State
	// State2 must differ in all fields from State
	State2 channel.State

	Account wallet.Account
	// Account2 must differ in all fields from Account
	Account2 wallet.Account
}

func GenericBackendTest(t *testing.T, s *Setup) {
	ID := s.Channel.ChannelID(&s.Params)
	require.Equal(t, ID, s.State.ID, "ChannelID(params) should match the States ID")
	require.Equal(t, ID, s.Params.ID(), "ChannelID(params) should match the Params ID")
	require.NotNil(t, s.State.Data, "State data cant be nil")
	require.NotNil(t, s.State2.Data, "State2 data cant be nil")

	t.Run("ChannelID", func(t *testing.T) {
		genericChannelIDTest(t, s)
	})

	t.Run("Sign", func(t *testing.T) {
		genericSignTest(t, s)
	})
}

func genericSignTest(t *testing.T, s *Setup) {
	{
		sig1, err1 := s.Channel.Sign(s.Account, &s.Params, &s.State)
		sig2, err2 := s.Channel.Sign(s.Account, &s.Params, &s.State2)

		assert.Nil(t, err1, "Signature should work")
		assert.Nil(t, err2, "Signature should work")
		assert.NotEqual(t, sig1, sig2, "Signatures for different states should differ")
	}

	{
		sig1, err1 := s.Channel.Sign(s.Account, &s.Params, &s.State)
		sig2, err2 := s.Channel.Sign(s.Account, &s.Params2, &s.State)

		assert.Nil(t, err1, "Signature should work")
		assert.Nil(t, err2, "Signature should work")
		assert.NotEqual(t, sig1, sig2, "Signatures for different states should differ")
	}

	{
		sig1, err1 := s.Channel.Sign(s.Account, &s.Params, &s.State)
		sig2, err2 := s.Channel.Sign(s.Account, &s.Params2, &s.State2)

		assert.Nil(t, err1, "Signature should work")
		assert.Nil(t, err2, "Signature should work")
		assert.NotEqual(t, sig1, sig2, "Signatures for different states should differ")
	}
}

func genericChannelIDTest(t *testing.T, s *Setup) {
	require.NotNil(t, s.Params.Parts, "params.Parts cant be nil")

	// Modify ID - should not change the hash
	{
		// Not possible right nows
	}

	// Modify ChallengeDuration - should change the hash
	{
		fakeParams := s.Params
		fakeParams.ChallengeDuration = s.Params2.ChallengeDuration
		ID := s.Channel.ChannelID(&fakeParams)
		assert.NotEqual(t, ID, s.State.ID, "ChannelID(fakeParams) should not match the States ID")
	}

	// Modify Parts - should change the hash
	{
		fakeParams := s.Params
		fakeParams.Parts = s.Params2.Parts
		ID := s.Channel.ChannelID(&fakeParams)
		assert.NotEqual(t, ID, s.State.ID, "ChannelID(fakeParams) should not match the States ID")
	}

	// Modify App - should change the hash
	{
		fakeParams := s.Params
		fakeParams.App = s.Params2.App
		ID := s.Channel.ChannelID(&fakeParams)
		assert.NotEqual(t, ID, s.State.ID, "ChannelID(fakeParams) should not match the States ID")
	}

	// Modify Nonce - should change the hash
	{
		fakeParams := s.Params
		fakeParams.Nonce = s.Params2.Nonce
		ID := s.Channel.ChannelID(&fakeParams)
		assert.NotEqual(t, ID, s.State.ID, "ChannelID(fakeParams) should not match the States ID")
	}
}
