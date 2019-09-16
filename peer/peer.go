// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

// Package peer contains the peer connection related code.
package peer // import "perun.network/go-perun/peer"

import (
	"sync"

	"github.com/pkg/errors"

	"perun.network/go-perun/log"
	"perun.network/go-perun/wire/msg"
)

// Peer is an authenticated and self-repairing connection to a Perun peer.
// It contains the peer's identity.
// Peers must not be created manually. Peers are thread-safe.
//
// If a peer's connection fails, it is automatically repaired as soon as a new
// connection with the peer is established. This process is hidden, so the user
// will not notice when a connection breaks and is repaired again. This
// simplifies the communication code greatly.
type Peer struct {
	PerunAddress Address // The peer's perun address.

	conn     Conn          // The peer's connection.
	subs     subscriptions // The receivers that are subscribed to the peer.
	registry *Registry     // The registry that holds the peer.

	// repairing is a chan instead of a mutex because we need to wait for
	// Lock() in a select statement in repair() together with other cases.
	repairing chan struct{} // Protects against concurrent repair() calls.
	replacing sync.Mutex    // Protects conn from replaceConn and Close.
	sending   chan struct{} // Blocks multiple Send calls.
	connMutex sync.Mutex    // Protect against data race when updating conn.

	closed    chan struct{} // Indicates whether the peer is closed.
	retrySend chan struct{} // Signals that Send calls can retry. Capacity 1.
	retryRecv chan struct{} // Signals that Recv calls can retry. Capacity 1.
}

// Recv receives a single message from a peer.
// If the transmission fails, blocks until the connection is repaired and
// retries to receive the message. Fails if the peer is closed via Close().
func (p *Peer) recv() (msg.Msg, error) {
	// Repeatedly attempt to receive a message.
	for {
		// Protect against race with concurrent replaceConn().
		p.connMutex.Lock()
		conn := p.conn
		p.connMutex.Unlock()

		if m, err := conn.Recv(); err == nil {
			log.Debugf("failed recv from peer %v\n", p.PerunAddress)
			return m, nil
		} else {
			if !p.repair(p.retryRecv) {
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
// The additional 'abort' channel is used to timeout the send operation.
// However, the peer's internal connection's Send() call cannot be aborted.
func (p *Peer) Send(m msg.Msg, abort <-chan struct{}) error {
	select {
	case <-p.closed: // Wait for closing of the peer.
		return errors.New("peer closed")
	case <-abort: // Wait for manual abort.
		return errors.New("aborted manually")
	case p.sending <- struct{}{}: // Wait for sending to be unblocked.
		// Repeatedly attempt to send the message.
		for {
			// Protect against torn reads from concurrent replaceConn().
			p.connMutex.Lock()
			conn := p.conn
			p.connMutex.Unlock()

			if err := conn.Send(m); err == nil {
				// Unlock the mutex.
				<-p.sending
				return nil
			} else {
				log.Debugf("failed Send to peer %v", p.PerunAddress)

				repair := make(chan bool, 1)
				// Asynchronously repair the connection.
				go func() { repair <- p.repair(p.retrySend) }()
				// Wait for repair to retry, or abort if requested.
				select {
				case success := <-repair: // Repairing done?
					if !success {
						// Unlock the mutex before failing.
						<-p.sending
						return errors.New("peer closed")
					} else {
						// Retry sending.
						continue
					}
				case <-abort: // Aborted by the user?
					// Only release mutex after repair is done. This is needed
					// as repair() must not be called twice by Send().
					go func() { <-repair; <-p.sending }()
					return errors.New("aborted manually")
				}
			}
		}
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
func (p *Peer) repair(repaired <-chan struct{}) bool {
	// Do not repair as a consequence of the connection being closed in
	// replaceConn(). Wait until replaceConn() is done, then check whether we
	// still need to repair anything.
	p.replacing.Lock()
	p.replacing.Unlock()

	// Wait until the 'repairing' mutex is locked or the connection is repaired
	// or the peer is closed.
	select {
	case <-repaired: // Success if the peer's connection is already repaired.
		return true
	case <-p.closed: // Fail if the peer is closed.
		return false
	case p.repairing <- struct{}{}: // Try repairing if the mutex is acquired.
		// Automatically release the mutex lock again.
		defer func() { <-p.repairing }()

		// This is used to abort the dialing process.
		abort := make(chan struct{})
		// Automatically abort the dialing upon concurrent success/failure.
		defer close(abort)

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
				if conn, err := p.registry.repairer.Dial(p.PerunAddress, abort); err == nil {
					dialed <- conn
				}
			}()

			// Wait for dialing to return, external repair, or closing.
			select {
			case <-repaired: // Succeed if the connection was repaired.
				// Do not let the dialed connection rot.
				go func() {
					if conn := <-dialed; conn != nil {
						p.replaceConn(conn)
					}
				}()
				return true
			case <-p.closed: // Fail if the peer is closed.
				go func() {
					if conn := <-dialed; conn != nil {
						conn.Close()
					}
				}()
				return false
			case conn := <-dialed: // If dialing returned, check for success.
				if conn != nil {
					// This also unblocks the other instance of repair().
					p.replaceConn(conn)
					return true
				}
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
	clear(p.retryRecv)

	// Close the old connection to fail all send and receive calls.
	p.conn.Close()
	p.connMutex.Lock() // Protect against torn reads in recv() and Send().
	p.conn = conn
	p.connMutex.Unlock()

	// Send the retry signal to both send and recv.
	p.retrySend <- struct{}{}
	p.retryRecv <- struct{}{}

	log.Debugf("replaced connection for peer %v", p.PerunAddress)

	return true
}

func clear(event chan struct{}) {
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
	p.registry.delete(p)
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
func newPeer(addr Address, conn Conn, registry *Registry) *Peer {
	p := new(Peer)
	*p = Peer{
		PerunAddress: addr,

		conn:     conn,
		subs:     makeSubscriptions(p),
		registry: registry,

		// Simulated mutex needs capacity 1.
		repairing: make(chan struct{}, 1),
		sending:   make(chan struct{}, 1),

		closed:    make(chan struct{}),
		retrySend: make(chan struct{}, 1),
		retryRecv: make(chan struct{}, 1),
	}
	return p
}
