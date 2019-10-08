// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

// Package peer contains the peer connection related code.
package peer // import "perun.network/go-perun/peer"

import (
	"context"

	"github.com/pkg/errors"

	"perun.network/go-perun/log"
	"perun.network/go-perun/pkg/sync"
	wire "perun.network/go-perun/wire/msg"
)

// Peer is an authenticated and self-repairing connection to a Perun peer.
// It contains the peer's identity. Peers are thread-safe.
// Peers must not be created manually. The creation of peers is handled by the
// Registry, which tracks all existing peers. The registry, in turn, is used by
// the Client.
//
// Sending messages to a peer is done via the Send() method, or via the
// Broadcaster helper type. Receiving messages can only be done via the
// Receiver helper type (by subscribing).
//
// If a peer's connection fails, it is automatically repaired as soon as a new
// connection with the peer is established. This process is hidden, so the user
// will not notice when a connection breaks and is repaired again. This
// simplifies the communication code greatly.
type Peer struct {
	PerunAddress Address // The peer's perun address.

	conn Conn          // The peer's connection.
	subs subscriptions // The receivers that are subscribed to the peer.

	// repairing is a chan instead of a mutex because we need to wait for
	// Lock() in a select statement in repair() together with other cases.
	replacing sync.Mutex // Protects conn from replaceConn and Close.
	sending   sync.Mutex // Blocks multiple Send calls.
	connMutex sync.Mutex // Protect against data race when updating conn.

	closed    chan struct{} // Indicates whether the peer is closed.
	retrySend chan struct{} // Signals that Send calls can retry. Capacity 1.

	closeWork func(*Peer) // Work to be done when the peer is closed.
	repairer  Dialer      // The dialer that is used to repair the connection.
}

// recv receives a single message from a peer.
// If the transmission fails, blocks until the connection is repaired and
// retries to receive the message. Fails if the peer is closed via Close().
func (p *Peer) recv() (wire.Msg, error) {
	// Repeatedly attempt to receive a message.
	for {
		// Protect against race with concurrent replaceConn().
		p.connMutex.Lock()
		conn := p.conn
		p.connMutex.Unlock()

		if m, err := conn.Recv(); err == nil {
			return m, nil
		} else {
			log.Debugf("failed recv from peer %v\n", p.PerunAddress)
			if !p.repair() {
				return nil, errors.WithMessage(err, "peer closed manually")
			}
		}
	}
}

// recvLoop continuously receives messages from a peer until it is closed.
// Received messages are relayed via the peer's subscription system. This is
// called by the registry when the peer is registered.
func (p *Peer) recvLoop() {
	for {
		if m, err := p.recv(); err != nil {
			log.Debugf("ending recvLoop of closed peer %v", p.PerunAddress)
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
//
// The passed context is used to timeout the send operation.
// However, the peer's internal connection's Send() call cannot be aborted.
func (p *Peer) Send(ctx context.Context, m wire.Msg) error {
	select {
	case <-ctx.Done():
		return errors.New("aborted manually")
	default:
	}

	if !p.sending.TryLock(ctx) {
		return errors.New("aborted manually")
	}

	sent := make(chan struct{}, 1)
	// Asynchronously send, because we cannot abort Conn.Send().
	go func() {
		// Unlock the mutex only after sending is done.
		defer p.sending.Unlock()

		// Repeatedly attempt to send the message.
		for {
			// Protect against torn reads from concurrent replaceConn().
			p.connMutex.Lock()
			conn := p.conn
			p.connMutex.Unlock()

			if err := conn.Send(m); err == nil {
				sent <- struct{}{}
				return
			} else {
				log.Debugf("failed Send to peer %v", p.PerunAddress)

				// Wait for repair to retry, or abort on timeout / closed peer.
				select {
				case <-p.closed:
					return
				case <-ctx.Done():
					return
				case <-p.retrySend:
					continue
				}
			}
		}
	}()

	// Return as soon as the sending finishes, times out, or peer is closed.
	select {
	case <-sent:
		return nil
	case <-p.closed:
		return errors.New("peer closed")
	case <-ctx.Done():
		return errors.New("aborted manually")
	}
}

// repair attempts to repair the peer's connection.
// It is passed a either retrySend or retryRecv, which are used to detect
// external repairing of the connection, in which case it reads the signal from
// the passed channel.
//
// repair() blocks until the channel is closed or repaired, and returns true if
// the channel was successfully repaired, and false otherwise. The function can
// be called concurrently, but must only be called from within Send() and
// recv(). Send() must pass retrySend, and recv() must pass retryRecv. When
// called concurrently, only one instance of the function tries to repair the
// connection, and the other instance is blocked until the connection is
// repaired or the peer is closed.
//
// Internally, repair() tries to dial a new connection to the peer repeatedly,
// until dialing succeeds or the connection is replaced externally (through
// the client's listener, via the registry), or the peer is manually closed.
func (p *Peer) repair() bool {
	// Do not repair as a consequence of the connection being closed in
	// replaceConn(). Wait until replaceConn() is done, then check whether we
	// still need to repair anything.
	p.replacing.Lock()
	p.replacing.Unlock()

	if p.isClosed() {
		return false
	}

	// This is used to abort the dialing process.
	dialctx, dialcancel := context.WithCancel(context.Background())
	// Automatically abort the dialing upon concurrent success/failure.
	defer dialcancel()

	// Continuously retry to repair the connection, until successful or the
	// peer is closed.
	for {
		// This holds the dialed connection (or nil, if failed).
		dialed := make(chan Conn, 1)

		// Asynchronously dial the peer.
		// This helper function dials a connection, and returns it over a
		// channel, if successful, otherwise, closes the 'dialed' channel.
		go func() {
			defer close(dialed) // Ensure nil is written on failure.
			// Dial a connection using the registry's dialer.
			if conn, err := p.repairer.Dial(dialctx, p.PerunAddress); err == nil {
				dialed <- conn
			}
		}()

		// Wait for dialing to return, external repair, or closing.
		select {
		case <-p.closed: // Fail if the peer is closed.
			go func() {
				if conn := <-dialed; conn != nil {
					conn.Close()
				}
			}()
			return false
		case conn := <-dialed: // If dialing returned, check for success.
			if conn != nil {
				// This unblocks Send().
				p.replaceConn(conn)
				return true
			}
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
		// Abort and close the connection if the peer is closed.
		conn.Close()
		return false
	}

	clear(p.retrySend)

	// Close the old connection to fail all send and receive calls.
	p.conn.Close()
	p.connMutex.Lock() // Protect against torn reads in recv() and Send().
	p.conn = conn
	p.connMutex.Unlock()

	// Send the retry signal to both send and recv.
	p.retrySend <- struct{}{}

	log.Debugf("replaced connection for peer %v", p.PerunAddress)

	return true
}

// clear empties an event channel's unread events.
func clear(event <-chan struct{}) {
	select {
	case <-event:
	default:
	}
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

	// Unregister the peer.
	p.closeWork(p)
	// Mark the peer as closed.
	close(p.closed)
	// Close the peer's connection.
	p.conn.Close()
	// Delete this peer from all receivers.
	p.subs.mutex.Lock()
	defer p.subs.mutex.Unlock()
	for cat, recvs := range p.subs.subs {
		for _, recv := range recvs {
			recv.unsubscribe(p, cat, false)
		}
	}

	return nil
}

// newPeer creates a new peer from a peer address and connection.
func newPeer(addr Address, conn Conn, closeWork func(*Peer), repairer Dialer) *Peer {
	// In tests, it is useful to omit the function.
	if closeWork == nil {
		closeWork = func(*Peer) {}
	}

	p := new(Peer)
	*p = Peer{
		PerunAddress: addr,

		conn: conn,
		subs: makeSubscriptions(p),

		closed:    make(chan struct{}),
		retrySend: make(chan struct{}, 1),

		closeWork: closeWork,
		repairer:  repairer,
	}
	return p
}
