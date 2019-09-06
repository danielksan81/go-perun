// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

// Package peer contains the peer connection related code.
package peer // import "perun.network/go-perun/peer"

import (
	"sync"

	"github.com/pkg/errors"

	"perun.network/go-perun/wire/msg"
)

// Peer is a Perun peer. It contains the peer's identity, and also represents
// a communications channel with that peer. Peers must not be created manually.
// Peers are thread-safe.
//
// If a peer's connection fails, it is automatically repaired as soon as a new
// connection with the peer is established. This process is hidden.
type Peer struct {
	PerunAddress Address // The peer's perun address.

	conn     Conn
	subs     subscriptions
	registry *Registry

	replacing sync.Mutex // Protects conn from replaceConn and Close.
	sending   sync.Mutex // Blocks multiple Send calls.
	connMutex sync.Mutex // Protect against data race when updating conn.

	closed    chan struct{} // Indicates whether the peer is closed.
	retrySend chan struct{} // Signals that Send calls can retry. Capacity 1.
	retryRecv chan struct{} // Signals that Recv calls can retry. Capacity 1.
}

// Recv receives a single message from a peer.
// If the transmission fails, blocks until the connection is repaired and
// retries to receive the message. Fails if the peer is closed via Close().
func (p *Peer) recv() msg.Msg {
	// Repeatedly attempt to receive a message.
	for {
		// Protect against race with concurrent replaceConn().
		p.connMutex.Lock()
		conn := p.conn
		p.connMutex.Unlock()

		if m, err := conn.Recv(); err == nil {
			return m
		} else {
			select {
			case <-p.closed:
				// Fail when the peer is closed.
				return nil
			case <-p.retryRecv:
				// Retry when the connection is repaired.
				continue
			}
		}
	}
}

// recvLoop continuously receives messages from a peer until it is closed.
// Received messages are relayed via the peer's subscription system. This is
// called by the registry when the peer is registered.
func (p *Peer) recvLoop() {
	for {
		if m := p.recv(); m == nil {
			return
		} else {
			// Broadcast the received message to all interested subscribers.
			p.subs.put(m, p)
		}
	}
}

// Send sends a single message to a peer.
// If the transmission fails, blocks until the connection is repaired and
// retries to send the message. Fails if the peer is closed via Close().
func (p *Peer) Send(m msg.Msg) error {
	// Repeatedly attempt to send the message.
	for {
		// Protect against race with concurrent replaceConn().
		p.connMutex.Lock()
		conn := p.conn
		p.connMutex.Unlock()

		if conn.Send(m) != nil {
			select {
			case <-p.closed:
				// Fail when the peer is closed.
				return errors.New("connection closed")
			case <-p.retrySend:
				// Retry when the connection is repaired.
				continue
			}
		} else {
			return nil
		}
	}
}

// replaceConn replaces a peer's connection, if the peer itself is not closed.
// Returns whether the peer's connection has been replaced.
func (p *Peer) replaceConn(conn Conn) bool {
	// Replacing is not thread-safe.
	p.replacing.Lock()
	defer p.replacing.Unlock()

	if p.isClosed() {
		// Abort if the peer is closed.
		return false
	}

	// Clear the retrySend and retryRecv channels.
	select {
	case <-p.retrySend:
	default:
	}
	select {
	case <-p.retryRecv:
	default:
	}

	// Close the old connection to fail all send and receive calls.
	p.conn.Close()
	p.connMutex.Lock()
	p.conn = conn
	p.connMutex.Unlock()

	// Send the retry signal to both send and recv.
	p.retrySend <- struct{}{}
	p.retryRecv <- struct{}{}

	return true
}

// isClosed checks whether the peer is marked as closed.
// This is different from the peer's connection being closed, in that a closed
// peer cannot have its connection restored and is marked for deletion.
func (p *Peer) isClosed() bool {
	select {
	case <-p.closed:
		return true
	default:
		return false
	}
}

// Close closes a peer's connection and deletes it from the registry. If the
// peer was already closed, results in an error. A closed peer is no longer
// usable.
func (p *Peer) Close() error {
	// Must not interfere with ongoing replacement and closing attempts.
	p.replacing.Lock()
	defer p.replacing.Unlock()

	if p.isClosed() {
		return errors.New("already closed")
	}

	// Mark the peer as closed.
	close(p.closed)
	// Close the peer's connection.
	p.conn.Close()
	// Unregister the peer.
	p.registry.delete(p)

	return nil
}

// newPeer creates a new peer from a peer address and connection.
func newPeer(addr Address, conn Conn, registry *Registry) *Peer {
	return &Peer{
		PerunAddress: addr,

		conn:     conn,
		subs:     makeSubscriptions(),
		registry: registry,

		closed:    make(chan struct{}),
		retrySend: make(chan struct{}, 1),
		retryRecv: make(chan struct{}, 1),
	}
}
