// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"testing"

	"github.com/stretchr/testify/assert"

	wire "perun.network/go-perun/wire/msg"
)

func TestSubscriptions(t *testing.T) {
	peer := newPeer(nil, nil, nil, nil)

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
