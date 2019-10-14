// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"perun.network/go-perun/backend/sim/wallet"
	wire "perun.network/go-perun/wire/msg"
)

var rng = rand.New(rand.NewSource(0xb0baFEDD))

func TestBroadcaster_Send(t *testing.T) {
	N := 5

	recvPeers := make([]*Peer, N)
	sendPeers := make([]*Peer, N)

	for i := 0; i < N; i++ {
		in, out := newPipeConnPair()
		sendPeers[i] = newPeer(nil, out, nil, nil)
		recvPeers[i] = newPeer(nil, in, nil, nil)
		go recvPeers[i].recvLoop()
	}

	b := NewBroadcaster(sendPeers)

	assert.Nil(t, b.Send(context.Background(), wire.NewPingMsg()), "broadcast must succeed")
}

func TestBroadcaster_Send_Error(t *testing.T) {
	N := 5

	reg := NewRegistry(func(*Peer) {}, nil)
	recvPeers := make([]*Peer, N)
	sendPeers := make([]*Peer, N)

	for i := 0; i < N; i++ {
		in, out := newPipeConnPair()
		sendPeers[i] = reg.Register(wallet.NewRandomAddress(rng), out)
		recvPeers[i] = newPeer(nil, in, nil, nil)
		go recvPeers[i].recvLoop()
	}

	sendPeers[1].Close()

	b := NewBroadcaster(sendPeers)

	err := b.Send(context.Background(), wire.NewPingMsg())
	require.NotNil(t, err, "broadcast must fail")
	assert.Equal(t, len(err.errors), 1)
	assert.Equal(t, err.errors[0].index, 1)
	assert.Equal(t, err.Error(), "failed to send message:\npeer[1]: "+err.errors[0].err.Error())
}

func TestNewBroadcaster(t *testing.T) {
	peers := []*Peer{newPeer(nil, nil, nil, nil), newPeer(nil, nil, nil, nil)}

	b := NewBroadcaster(peers)
	assert.Equal(t, peers, b.peers)
	assert.NotPanics(t, func() { close(b.gather) })
}
