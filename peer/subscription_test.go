// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"context"
	"io/ioutil"
	"os"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"perun.network/go-perun/backend/sim/wallet"
	wire "perun.network/go-perun/wire/msg"
)


type DummyDialer struct {
}

func (DummyDialer) Dial(ctx context.Context, address Address) (Conn, error) {
	panic("Not implemented")
}

func (DummyDialer) Close() error {
	panic("Not implemented")
}


func TestSubscriptions(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	addr := wallet.NewRandomAddress(rng)
	tmpFile, _ := ioutil.TempFile("", "peer-subscription-test")
	defer os.Remove(tmpFile.Name())
	conn := NewConn(tmpFile)
	onClose := func(*Peer) {}
	dialer := new(DummyDialer)
	peer := newPeer(addr, conn, onClose, dialer)

	r0 := NewReceiver()
	r1 := NewReceiver()
	r2 := NewReceiver()
	s := makeSubscriptions(peer)

	assert.True(t, s.isEmpty())
	assert.NoError(t, s.add(wire.Peer, r0))
	assert.False(t, s.isEmpty())
	assert.NoError(t, s.add(wire.Peer, r1))
	assert.NoError(t, s.add(wire.Peer, r2))
	assert.Equal(t, len(s.subs[wire.Peer]), 3)
	s.delete(wire.Peer, r0)
	assert.Equal(t, len(s.subs[wire.Peer]), 2)
	assert.False(t, s.isEmpty())
}
