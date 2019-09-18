// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"perun.network/go-perun/wire/msg"
)

func TestNewReceiver(t *testing.T) {
	assert.True(t, NewReceiver().isEmpty(),
		"fresh receivers must be empty")
}

func TestReceiver_Next(t *testing.T) {
	in, out := newPipeConnPair()
	p := newPeer(nil, in, nil)
	go p.recvLoop()
	r := NewReceiver()

	var closedMsgTupleChanAsRead <-chan MsgTuple = closedMsgTupleChan
	assert.Equal(t, r.Next(), closedMsgTupleChanAsRead,
		"fresh receivers must return closed channel in Next()")

	r.Subscribe(p, msg.Control)
	next := r.Next()
	assert.NotEqual(t, next, closedMsgTupleChan,
		"subscribed receivers must not return closed Next() channel")
	select {
	case <-next:
		t.Fatal("subscribed receiver closed Next() channel")
	default:
	}
	r.Unsubscribe(p, msg.Control)

	select {
	case _, ok := <-next:
		assert.False(t, ok, "unsubscribed receiver must close Next() channel")
	default:
		t.Fatal("unsubscribed receiver did not close Next() channel")
	}

	out.Send(msg.NewPingMsg())

	// Ensure that the peer received the message.
	<-time.NewTimer(100 * time.Millisecond).C

	r.Subscribe(p, msg.Control)
	// Get a new Next() channel.
	next = r.Next()
	// The previously sent message must not appear.
	select {
	case _, ok := <-next:
		if ok {
			t.Fatal("messages received before subscribing must not appear.")
		} else {
			t.Fatal("subscribed receivers must not close the NextWait() channel")
		}
	case <-time.NewTimer(100 * time.Millisecond).C:
	}

	out.Send(msg.NewPongMsg())
	// The new message must appear.
	select {
	case m, ok := <-next:
		if ok {
			assert.Equal(t, m.Peer, p, "message must come from the subscribed peer")
			assert.NotNil(t, m.Msg.(*msg.PongMsg), "received message must be PongMsg")
		} else {
			t.Fatal("subscribed receivers must not close the NextWait() channel")
		}
	case <-time.NewTimer(100 * time.Millisecond).C:
		t.Fatal("failed to receive message")
	}
}

func TestReceiver_NextWait(t *testing.T) {
	in, out := newPipeConnPair()

	p := newPeer(nil, in, nil)
	go p.recvLoop()
	r := NewReceiver()

	next := r.NextWait()

	out.Send(msg.NewPingMsg())
	// The receiver is not yet subscribed and must not see any messages.
	select {
	case _, ok := <-next:
		if ok {
			t.Fatal("unsubscribed fresh receivers must not contain messages")
		} else {
			t.Fatal("unsubscribed fresh receivers must not close the NextWait() channel")
		}
	case <-time.NewTimer(100 * time.Millisecond).C:
	}

	r.Subscribe(p, msg.Control)
	// The previously sent message must not appear.
	select {
	case _, ok := <-next:
		if ok {
			t.Fatal("messages received before subscribing must not appear.")
		} else {
			t.Fatal("subscribed receivers must not close the NextWait() channel")
		}
	case <-time.NewTimer(100 * time.Millisecond).C:
	}

	out.Send(msg.NewPongMsg())
	// The new message must appear.
	select {
	case m, ok := <-next:
		if ok {
			assert.Equal(t, m.Peer, p, "message must come from the subscribed peer")
			assert.NotNil(t, m.Msg.(*msg.PongMsg), "received message must be PongMsg")
		} else {
			t.Fatal("subscribed receivers must not close the NextWait() channel")
		}
	case <-time.NewTimer(100 * time.Millisecond).C:
		t.Fatal("failed to receive message")
	}
}
