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

	"perun.network/go-perun/backend/sim"
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

func (s *Setup) Dial(addr Address, abort <-chan struct{}) (Conn, error) {
	select {
	case <-s.closed:
		return nil, errors.New("dialer closed")
	default:
	}

	// a: Alice's end, b: Bob's end.
	a, b := newPipeConnPair()

	if addr.Equals(s.alice.Peer.PerunAddress) { // Dialing Bob?
		s.bob.Registry.Register(addr, a) // Bob accepts connection.
		return b, nil
	} else if addr.Equals(s.bob.Peer.PerunAddress) { // Dialing Alice?
		s.alice.Registry.Register(addr, b) // Alice accepts connection.
		return a, nil
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
	*Peer
	*Registry
	*Receiver
}

// MakeClient creates a simulated test client.
func MakeClient(conn Conn, rng io.Reader, dialer Dialer) *Client {
	var receiver = NewReceiver()
	var registry = NewRegistry(func(p *Peer) {
		receiver.Subscribe(p, msg.Control)
		receiver.Subscribe(p, msg.Peer)
		receiver.Subscribe(p, msg.Channel)
	}, dialer)

	return &Client{
		Peer:     registry.Register(sim.NewRandomAccount(rng).Address(), conn),
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
	setup.alice.Peer.conn.Close()
	setup.bob.Peer.conn.Close()

	done := make(chan struct{})
	// Send the message in the background.
	go func() {
		if nil != setup.alice.Peer.Send(msg.NewPingMsg(), nil) {
			t.Error("Failed to send")
		}
		close(done)
	}()

	// Receive the message.
	if _, ok := <-setup.bob.Receiver.Next(); !ok {
		t.Error("failed to repair a connection and receive message.")
	}
	// Ensure the sender is done, so that if it fails, the error is not lost.
	<-done
}
