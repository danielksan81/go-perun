// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"math/rand"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	sim "perun.network/go-perun/backend/sim/wallet"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wire/msg"
)

func init() {
	wallet.SetBackend(new(sim.Backend))
}

func TestAuthResponseMsg(t *testing.T) {
	rng := rand.New(rand.NewSource(1337))
	msg.TestMsg(t, NewAuthResponseMsg(sim.NewRandomAccount(rng)))
}

func TestAuthenticate_NilParams(t *testing.T) {
	rnd := rand.New(rand.NewSource(0xb0ba))
	assert.Panics(t, func() { Authenticate(nil, nil) })
	assert.Panics(t, func() { Authenticate(nil, newMockConn(nil)) })
	assert.Panics(t, func() {
		Authenticate(sim.NewRandomAccount(rnd), nil)
	})
}

type exchangeAddressesConn struct {
	net.Conn
}

func (e *exchangeAddressesConn) Send(m msg.Msg) error {
	return msg.Encode(m, e.Conn)
}

func (e *exchangeAddressesConn) Recv() (msg.Msg, error) {
	return msg.Decode(e.Conn)
}

func (e *exchangeAddressesConn) Close() error {
	return e.Conn.Close()
}

func TestExchangeAddresses_Success(t *testing.T) {
	rng := rand.New(rand.NewSource(0xfedd))
	conn0, conn1 := net.Pipe()
	account0 := sim.NewRandomAccount(rng)
	account1 := sim.NewRandomAccount(rng)
	waitGroup := new(sync.WaitGroup)

	defer conn0.Close()

	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()
		defer conn1.Close()

		receivedAddr, err := Authenticate(account1, &exchangeAddressesConn{conn1})
		assert.NoError(t, err)
		assert.Equal(t, receivedAddr, account0.Address())
	}()

	receivedAddr, err := Authenticate(account0, &exchangeAddressesConn{conn0})
	assert.NoError(t, err)
	assert.Equal(t, receivedAddr, account1.Address())

	waitGroup.Wait()
}
