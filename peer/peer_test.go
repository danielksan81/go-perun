// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

// Package peer contains the peer connection related code.
package peer

import (
	"io"
	"math/rand"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"perun.network/go-perun/backend/sim/wallet"
	"perun.network/go-perun/wire/msg"
)

// Setup is a test setup consisting of two connected peers.
// It is also a mock dialer.
type Setup struct {
	closed chan struct{}
	alice  *Client
	bob    *Client
}

// MakeSetup creates a test setup.
func MakeSetup() *Setup {
	a, b := newPipeConnPair()
	rng := rand.New(rand.NewSource(0x5D0))
	// We need the setup adress when constructing the clients.
	setup := new(Setup)
	*setup = Setup{
		alice: MakeClient(a, rng, setup),
		bob:   MakeClient(b, rng, setup),
	}
	return setup
}

// Dial simulates creating a connection to a peer.
func (s *Setup) Dial(addr Address, abort <-chan struct{}) (Conn, error) {
	select {
	case <-s.closed:
		return nil, errors.New("dialer closed")
	default:
	}

	// a: Alice's end, b: Bob's end.
	a, b := newPipeConnPair()

	if addr.Equals(s.alice.partner.PerunAddress) { // Dialing Bob?
		s.bob.Registry.Register(s.bob.partner.PerunAddress, b) // Bob accepts connection.
		return a, nil
	} else if addr.Equals(s.bob.partner.PerunAddress) { // Dialing Alice?
		s.alice.Registry.Register(s.alice.partner.PerunAddress, a) // Alice accepts connection.
		return b, nil
	} else {
		return nil, errors.New("unknown peer")
	}
}

func (s *Setup) Close() error {
	select {
	case <-s.closed:
		return errors.New("dialer closed")
	default:
		defer recover() // Recover if closing concurrently.
		close(s.closed)
		return nil
	}
}

// Client is a simulated client in the test setup.
// All of the client's incoming messages can be read from its receiver.
type Client struct {
	partner *Peer
	*Registry
	*Receiver
}

// MakeClient creates a simulated test client.
func MakeClient(conn Conn, rng io.Reader, dialer Dialer) *Client {
	var receiver = NewReceiver()
	var registry = NewRegistry(func(p *Peer) {
		_ = receiver.Subscribe(p, msg.Control)
		_ = receiver.Subscribe(p, msg.Peer)
		_ = receiver.Subscribe(p, msg.Channel)
	}, dialer)

	return &Client{
		partner:  registry.Register(wallet.NewRandomAddress(rng), conn),
		Registry: registry,
		Receiver: receiver,
	}
}

// TestConnectionRepair verifies that when sending messages over broken
// connections, the message is sent as soon as the connection is repaired.
// Tests recovery in both send as well as in receive.
func TestConnectionRepair(t *testing.T) {
	// Create a setup with two connected nodes.
	setup := MakeSetup()
	// Close both connections.
	setup.alice.partner.conn.Close()
	setup.bob.partner.conn.Close()

	done := make(chan struct{})
	// Send the message in the background.
	go func() {
		assert.Nil(t, setup.alice.partner.Send(msg.NewPingMsg(), nil), "failed to send")
		close(done)
	}()

	// Receive the message.
	if _, ok := <-setup.bob.Receiver.Next(); !ok {
		t.Error("failed to repair a connection and receive message.")
	}
	// Ensure the sender is done, so that if it fails, the error is not lost.
	<-done
}

// TestPeer_Close tests that closing a peer will make the peer object unusable,
// and that the remote end will try to re-establish the connection, and that
// this results in a new peer object.
func TestPeer_Close(t *testing.T) {
	setup := MakeSetup()
	// Remember bob's address for later, we will need it for a registry lookup.
	bobAddress := setup.alice.partner.PerunAddress
	// The lookup needs to work because the test relies on it.
	assert.Equal(t, setup.alice.partner, setup.alice.Registry.Find(bobAddress))
	setup.alice.partner.Close() // Close Alice's connection to Bob.
	// Sending over closed peers (not connections) must fail.
	assert.NotNil(t, setup.alice.partner.Send(msg.NewPingMsg(), nil), "sending to bob must fail")
	// Sending from the other side must succeed, as the remote will repair the
	// peer connection.
	assert.Nil(t, setup.bob.partner.Send(msg.NewPingMsg(), nil), "sending to alice must succeed (new socket)")
	// The receiver is subscribed automatically because it is registered in the
	// registry (see MakeClient()).
	assert.NotNil(t, (<-setup.alice.Receiver.Next()).Msg, "new alice must receive")
	// There must only be one peer in the registry (the closed peer deleted itself).
	assert.Equal(t, 1, len(setup.alice.Registry.peers))

	// Retrieve the new peer connection to bob and check that it is a new
	// instance (different address) than the old, closed peer.
	aliceNewPartner := setup.alice.Registry.Find(bobAddress)
	// The new peer must not be nil.
	assert.NotNil(t, aliceNewPartner)
	// The new peer must not be the old, closed peer.
	assert.True(t,
		setup.alice.partner != aliceNewPartner,
		"Alice needs to have been replaced")

	// Check that the new peer can communicate in both directions.
	assert.Nil(t, setup.bob.partner.Send(msg.NewPongMsg(), nil), "Sending to new Alice")
	assert.Nil(t, aliceNewPartner.Send(msg.NewPingMsg(), nil), "new alice must send to bob")
	assert.NotNil(t, (<-setup.alice.Receiver.Next()).Msg, "new alice must receive")
	assert.NotNil(t, (<-setup.bob.Receiver.Next()).Msg, "bob must receive")
}
