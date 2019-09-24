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
	t.Parallel()

	assert.True(t, NewReceiver().isEmpty(),
		"fresh receivers must be empty")
}

func TestReceiver_Subscribe(t *testing.T) {
	t.Parallel()

	r := NewReceiver()
	p := newPeer(nil, nil, nil)

	assert.True(t, r.isEmpty(), "receiver must be empty")
	assert.NoError(t, r.Subscribe(p, msg.Control), "first subscribe must not fail")
	assert.False(t, r.isEmpty(), "receiver must not be empty")
	assert.Panics(t, func() { r.Subscribe(p, msg.Control) }, "double subscription must panic")
	assert.NotPanics(t, func() { r.Unsubscribe(p, msg.Control) }, "first unsubscribe must not panic")
	assert.Panics(t, func() { r.Unsubscribe(p, msg.Control) }, "double unsubscribe must panic")
	assert.True(t, r.isEmpty(), "receiver must be empty")
	assert.NoError(t, r.Subscribe(p, msg.Control), "subscribe on empty must not fail")
	assert.False(t, r.isEmpty(), "receiver must not be empty")
	r.UnsubscribeAll()
	assert.True(t, r.isEmpty(), "receiver must be empty")

	close(p.closed) // Manual close needed here.
	assert.Error(t, r.Subscribe(p, msg.Control), "subscription on closed peer must fail")
	r.Close()
	assert.Error(t, r.Subscribe(p, msg.Control), "subscription on closed receiver must fail")
}

func TestReceiver_Next(t *testing.T) {
	t.Parallel()

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

	// This will trigger in the middle of the next test.
	go func() {
		<-time.NewTimer(50 * time.Millisecond).C
		r.Close()
	}()

	select {
	case _, ok := <-r.NextWait():
		assert.False(t, ok, "NextWait() must be closed")
	case <-time.NewTimer(100 * time.Millisecond).C:
		t.Fatal("NextWait() was not closed in time")
	}

	select {
	case _, ok := <-r.Next():
		assert.False(t, ok, "Next() must be closed")
	case <-time.NewTimer(200 * time.Millisecond).C:
		t.Fatal("Next() was not closed in time")
	}
}

func TestReceiver_NextWait(t *testing.T) {
	t.Parallel()

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

// TestReceiver_NextWait_ClosedRace forces that all branches in NextWait() get
// executed.
func TestReceiver_NextWait_ClosedRace(t *testing.T) {
	t.Parallel()

	for i := 0; i < 256; i++ {
		in, _ := newPipeConnPair()

		p := newPeer(nil, in, nil)
		go p.recvLoop()
		r := NewReceiver()

		next := r.NextWait()
		r.Close()

		select {
		case _, ok := <-next:
			assert.False(t, ok, "message channel must be closed")
		case <-time.NewTimer(100 * time.Millisecond).C:
			t.Fatal("did not close the message channel.")
		}
	}
}

// TestReceiver_renewChannel_transfer tests that a receiver will transfer the
// buffered messages to the next channel when renewing the channel.
func TestReceiver_renewChannel_transfer(t *testing.T) {
	t.Parallel()

	in, out := newPipeConnPair()

	p := newPeer(nil, in, nil)
	go p.recvLoop()
	r := NewReceiver()
	r.Subscribe(p, msg.Control)

	out.Send(msg.NewPingMsg())
	<-time.NewTimer(100 * time.Millisecond).C

	r.UnsubscribeAll()
	assert.True(t, r.isEmpty(), "receiver must be empty")
	r.Subscribe(p, msg.Control)
	assert.False(t, r.isEmpty(), "receiver must not be empty")

	select {
	case _, ok := <-r.Next():
		assert.True(t, ok, "message channel must contain the first message")
	case <-time.NewTimer(100 * time.Millisecond).C:
		t.Fatal("did not contain the message.")
	}
}
