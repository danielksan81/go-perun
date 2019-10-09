// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"sync"

	"github.com/pkg/errors"

	"perun.network/go-perun/log"
)

// Registry is a peer Registry.
// It should not be used manually, but only internally by the client.
type Registry struct {
	mutex sync.RWMutex
	peers []*Peer // The list of all of the registry's peers.

	repairer  Dialer      // The repairer is used by peers for repairing.
	subscribe func(*Peer) // Sets up peer subscriptions.
}

// NewRegistry creates a new registry.
// The provided callback is used to set up new peer's subscriptions and it is
// called before the peer starts receiving messages.
func NewRegistry(subscribe func(*Peer), repairer Dialer) *Registry {
	return &Registry{
		subscribe: subscribe,
		repairer:  repairer,
	}
}

// find looks up a peer via its Perun address.
// If found, returns the peer and its index, otherwise returns a nil peer.
func (r *Registry) find(addr Address) (*Peer, int) {
	for i, peer := range r.peers {
		if peer.PerunAddress.Equals(addr) {
			return peer, i
		}
	}

	return nil, -1
}

// Find looks up the peer via its perun address.
func (r *Registry) Find(addr Address) (*Peer, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if p, i := r.find(addr); i != -1 {
		return p, nil
	}
	return nil, errors.New("peer not found")
}

// Register registers a peer in the registry.
// If a peer with the same perun address already existed, replaces that peer's
// connection with the new peer's connection, but keeps the old peer object.
// Otherwise, enters the new peer into the registry.
func (r *Registry) Register(addr Address, conn Conn) (peer *Peer) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if peer, _ = r.find(addr); peer == nil {
		// Create and register a new peer.
		peer = newPeer(addr, conn, func(p *Peer) { r.delete(p) }, r.repairer)
		r.peers = append(r.peers, peer)
		// Setup the peer's subscriptions.
		r.subscribe(peer)
		// Start receiving messages.
		go peer.recvLoop()
	} else {
		peer.replaceConn(conn, true)
	}

	return
}

// delete deletes a peer from the registry.
// If the peer does not exist in the registry, panics. Does not close the peer.
func (r *Registry) delete(peer *Peer) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, i := r.find(peer.PerunAddress); i != -1 {
		// Delete the i-th entry.
		r.peers[i] = r.peers[len(r.peers)-1]
		r.peers = r.peers[:len(r.peers)-1]
	} else {
		log.Panic("tried to delete non-existent peer!")
	}
}
