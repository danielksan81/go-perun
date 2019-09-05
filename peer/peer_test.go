// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

// Package peer contains the peer connection related code.
package peer

import (
	"io"
	"math/rand"
	"testing"

	"perun.network/go-perun/backend/sim"
	"perun.network/go-perun/wire/msg"
)

// Setup is a test setup consisting of two connected peers.
type Setup struct {
	alice *Client
	bob   *Client
}

// MakeSetup creates a test setup.
func MakeSetup() Setup {
	a, b := newPipeConnPair()
	rng := rand.New(rand.NewSource(0x5D0))
	return Setup{
		alice: MakeClient(a, rng),
		bob:   MakeClient(b, rng),
	}
}

// ReplaceConnections replaces the connections of a test setup's peers.
func (s *Setup) ReplaceConnections() {
	a, b := newPipeConnPair()

	s.alice.Peer.replaceConn(a)
	s.bob.Peer.replaceConn(b)
}

// Client is a simulated client in the test setup.
// All of the client's incoming messages can be read from its receiver.
type Client struct {
	*Peer
	*Registry
	*Receiver
}

// MakeClient creates a simulated test client.
func MakeClient(conn Conn, rng io.Reader) *Client {
	var receiver = NewReceiver()
	var registry = NewRegistry(func(p *Peer) {
		receiver.Subscribe(p, msg.Control)
		receiver.Subscribe(p, msg.Peer)
		receiver.Subscribe(p, msg.Channel)
	})

	return &Client{
		Peer:     registry.Register(sim.NewRandomAccount(rng).Address(), conn),
		Registry: registry,
		Receiver: receiver,
	}
}

// TestConnectionRepair verifies that when sending messages over broken
// connections, the message is sent as soon as the connection is repaired.
// Tests both recovery in send as well as in receive.
func TestConnectionRepair(t *testing.T) {
	// Create a setup with two connected nodes.
	setup := MakeSetup()
	// Close both connections.
	setup.alice.Peer.conn.Close()
	setup.bob.Peer.conn.Close()

	done := make(chan struct{})
	// Send the message in the background.
	go func() {
		if nil != setup.alice.Peer.Send(msg.NewPingMsg()) {
			t.Error("Failed to send")
		}
		close(done)
	}()

	// Repair the connections.
	setup.ReplaceConnections()

	// Receive the message.
	if _, ok := <-setup.bob.Receiver.Next(); !ok {
		t.Error("failed to repair a connection and receive message.")
	}
	// Ensure the sender is done, so that if it fails, the error is not lost.
	<-done
}
