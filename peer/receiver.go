// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"sync"

	"github.com/pkg/errors"

	"perun.network/go-perun/log"
	"perun.network/go-perun/wire/msg"
)

const (
	// receiverBufferSize controls how many messages can be queued in a
	// receiver before blocking.
	receiverBufferSize = 16
)

var closedMsgTupleChan chan MsgTuple

func init() {
	closedMsgTupleChan = make(chan MsgTuple)
	close(closedMsgTupleChan)
}

// MsgTuple is a helper type, because channels cannot have tuple types.
type MsgTuple struct {
	*Peer
	msg.Msg
}

// Receiver is a helper object that can subscribe to different message
// categories from multiple peers. Receivers must only be used by a single
// execution context at a time. If multiple contexts need to access a peer's
// messages, then multiple receivers have to be created.
//
// Receivers have two ways of accessing peer messages: Next() and NextWait().
// Both will return a channel to the next message, but the difference is that
// Next() will fail when the receiver is not subscribed to any peers, while
// NextWait() will only fail if the receiver is manually closed via Close().
type Receiver struct {
	mutex       sync.Mutex    // Protects all fields.
	receiving   sync.Mutex    // Only one receive call can run at a time.
	renewedMsgs chan struct{} // Whether the message channel has been renewed.
	msgs        chan MsgTuple // Queued messages. Closed when not subscribed.
	closed      chan struct{}
	subs        map[msg.Category][]*Peer // The receiver's subscription list.
}

// Subscribe subscribes a receiver to all of a peer's messages of the requested
// message category. Returns an error if the receiver is closed.
func (r *Receiver) Subscribe(p *Peer, c msg.Category) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	select {
	case <-r.closed:
		return errors.New("receiver is closed")
	default:
	}

	empty := r.isEmpty()

	if err := p.subs.add(c, r); err != nil {
		return err
	}

	r.subs[c] = append(r.subs[c], p)

	if empty {
		r.renewChannel()
	}

	return nil
}

func (r *Receiver) renewChannel() {
	old := r.msgs
	r.msgs = make(chan MsgTuple, receiverBufferSize)

	// Transfer all remaining messages from the old channel.
	for m := range old {
		r.msgs <- m
	}

	// Try to notify the renewed message channel event.
	select {
	case r.renewedMsgs <- struct{}{}:
	default:
	}
}

func (r *Receiver) safelyGetMsgs() chan MsgTuple {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.msgs
}

// Unsubscribe removes a receiver's subscription to a peer's messages of the
// requested category. Returns an error if the receiver was not subscribed to
// the requested peer and message category.
func (r *Receiver) Unsubscribe(p *Peer, c msg.Category) {
	r.unsubscribe(p, c, true)
}

func (r *Receiver) unsubscribe(p *Peer, c msg.Category, delete bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if delete {
		p.subs.delete(c, r)
	}
	for i, _p := range r.subs[c] {
		if _p == p {
			r.subs[c][i] = r.subs[c][len(r.subs[c])-1]
			r.subs[c] = r.subs[c][:len(r.subs[c])-1]

			if r.isEmpty() {
				close(r.msgs)
			}
			return
		}
	}
	log.Panic("unsubscribe called on not-subscribed source")
}

// unsubscribeAll removes all of a receiver's subscriptions.
func (r *Receiver) UnsubscribeAll() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.unsubscribeAll()
}

func (r *Receiver) unsubscribeAll() {
	for cat, subs := range r.subs {
		for _, p := range subs {
			p.subs.delete(cat, r)
		}
		r.subs[cat] = nil
	}

	close(r.msgs)
}

// Next returns a channel to the next message.
// If, before a new message arrives, the receiver is no longer subscribed to
// any peers, then the returned channel is closed.
//
// The returned channel has to be read. Until the message is read from the
// returned channel, no new messages can be read from the receiver.
func (r *Receiver) Next() <-chan MsgTuple {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.receiving.Lock()

	msgs := r.msgs // Read it while we still have the mutex.
	next := make(chan MsgTuple, 1)
	go func() {
		defer r.receiving.Unlock()
		if m, ok := <-msgs; !ok {
			close(next)
		} else {
			next <- m
		}
	}()

	return next
}

// NextWait returns a channel that will hold the next received message.
// Unlike Next(), NextWait() will not abort when the receiver is no longer
// subscribed to anything. NextWait() will abort when Close() is called.
//
// The returned channel has to be read. Until the message is read from the
// returned channel, no new messages can be read from the receiver.
func (r *Receiver) NextWait() <-chan MsgTuple {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.receiving.Lock()

	next := make(chan MsgTuple, 1)
	go func() {
		defer r.receiving.Unlock()
		for {
			select {
			case m, ok := <-r.safelyGetMsgs():
				if ok { // We got a message.
					next <- m
					return
				} else { // We just unsubscribed from all peers.
					select {
					case <-r.closed: // The receiver is closed.
						close(next)
						return
					case <-r.renewedMsgs: // We subscribed to something again.
						continue
					}
				}
			case <-r.closed: // Receiver is closed.
				close(next)
				return
			}
		}
	}()

	return next
}

// Close closes a receiver.
// Any ongoing receiver operations will be aborted (if there are no messages in backlog).
func (r *Receiver) Close() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	// Safely close the channel.
	func() { defer recover(); close(r.closed) }()
	// Remove all subscriptions; this also closes r.msgs.
	r.unsubscribeAll()
}

func (r *Receiver) isEmpty() bool {
	for _, cats := range r.subs {
		for _ = range cats {
			return false
		}
	}
	return true
}

// NewReceiver creates a new receiver.
func NewReceiver() *Receiver {
	return &Receiver{
		msgs:        closedMsgTupleChan,
		subs:        make(map[msg.Category][]*Peer),
		closed:      make(chan struct{}),
		renewedMsgs: make(chan struct{}, 1),
	}
}
