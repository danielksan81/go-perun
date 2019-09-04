// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"perun.network/go-perun/log"
	"perun.network/go-perun/wire/msg"
)

const (
	// receiverBufferSize controls how many messages can be queued in a
	// receiver before blocking.
	receiverBufferSize = 16
)

// MsgTuple is a helper type, because channels cannot have tuple types.
type MsgTuple struct {
	*Peer
	msg.Msg
}

// Receiver is a helper object that can subscribe to different message
// categories from multiple peers. Receivers are not thread-safe and must only
// be used by a single execution context at a time. If multiple contexts need
// to access a peer's messages, then multiple receivers have to be created.
type Receiver struct {
	msgs chan MsgTuple
	subs map[msg.Category][]*Peer
}

// Subscribe subscribes a receiver to all of a peer's messages of the requested
// message category.
func (r *Receiver) Subscribe(p *Peer, c msg.Category) {
	p.subs.add(c, r)
	r.subs[c] = append(r.subs[c], p)
}

// Unsubscribe removes a receiver's subscription to a peer's messages of the
// requested category.
func (r *Receiver) Unsubscribe(p *Peer, c msg.Category) {
	p.subs.delete(c, r)
	for i, _p := range r.subs[c] {
		if _p == p {
			r.subs[c][i] = r.subs[c][len(r.subs[c])-1]
			r.subs[c] = r.subs[c][:len(r.subs[c])-1]
			return
		}
	}
	log.Panic("unsubscribe called on not-subscribed source")
}

// UnsubscribeAll removes all of a receiver's subscriptions.
func (r *Receiver) UnsubscribeAll() {
	for cat, subs := range r.subs {
		for _, p := range subs {
			p.subs.delete(cat, r)
		}
	}

	r.subs = nil
}

// Next returns the channel to the receiver's messages.
func (r *Receiver) Next() <-chan MsgTuple {
	return r.msgs
}

// NewReceiver creates a new receiver.
func NewReceiver() *Receiver {
	rec := new(Receiver)
	rec.msgs = make(chan MsgTuple, receiverBufferSize)
	rec.subs = make(map[msg.Category][]*Peer)
	return rec
}
