// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

// Package peer contains the peer connection related code.
package peer

import (
	"context"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"perun.network/go-perun/backend/sim/wallet"
	wire "perun.network/go-perun/wire/msg"
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
	rng := rand.New(rand.NewSource(0xb0baFEDD))
	// We need the setup adress when constructing the clients.
	setup := new(Setup)
	*setup = Setup{
		alice: MakeClient(a, rng, setup),
		bob:   MakeClient(b, rng, setup),
	}
	return setup
}

// Dial simulates creating a connection to a peer.
func (s *Setup) Dial(ctx context.Context, addr Address) (Conn, error) {
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
		_ = receiver.Subscribe(p, func(wire.Msg) bool { return true })
	}, dialer)

	return &Client{
		partner:  registry.Register(wallet.NewRandomAddress(rng), conn),
		Registry: registry,
		Receiver: receiver,
	}
}

// TestPeer_Close tests that closing a peer will make the peer object unusable,
// and that the remote end will try to re-establish the connection, and that
// this results in a new peer object.
func TestPeer_Close(t *testing.T) {
	t.Parallel()
	t.Helper()

	N := 1000

	done := make(chan struct{}, N)
	// Test it often to detect races.
	for i := 0; i < N; i++ {
		go testPeer_Close(t, done)
	}

	for i := 0; i < N; i++ {
		<-done
	}
}

func testPeer_Close(t *testing.T, done chan struct{}) {
	defer func() { done <- struct{}{} }()
	setup := MakeSetup()
	// Remember bob's address for later, we will need it for a registry lookup.
	bobAddress := setup.alice.partner.PerunAddress
	// The lookup needs to work because the test relies on it.
	found, _ := setup.alice.Registry.find(bobAddress)
	assert.Equal(t, setup.alice.partner, found)
	// Close Alice's connection to Bob.
	assert.Nil(t, setup.alice.partner.Close(), "closing a peer once must succeed")
	assert.NotNil(t, setup.alice.partner.Close(), "closing peers twice must fail")

	// Sending over closed peers (not connections) must fail.
	err := setup.alice.partner.Send(context.Background(), wire.NewPingMsg())
	assert.NotNil(t, err, "sending to bob must fail", err)
}

func TestPeer_Send_ImmediateAbort(t *testing.T) {
	t.Parallel()

	setup := MakeSetup()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// This operation should abort immediately.
	assert.Error(t, setup.alice.partner.Send(ctx, wire.NewPingMsg()))

	assert.True(t, setup.alice.partner.isClosed(), "peer must be closed after failed sending")
}

func TestPeer_Send_Timeout(t *testing.T) {
	t.Parallel()

	conn, _ := newPipeConnPair()
	p := newPeer(nil, conn, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()

	assert.Error(t, p.Send(ctx, wire.NewPingMsg()),
		"Send() must timeout on blocked connection")
	assert.True(t, p.isClosed(), "peer must be closed after failed Send()")
}

func TestPeer_Send_Close(t *testing.T) {
	conn, _ := newPipeConnPair()
	p := newPeer(nil, conn, nil, nil)

	go func() {
		<-time.NewTimer(Timeout).C
		p.Close()
	}()
	assert.Error(t, p.Send(context.Background(), wire.NewPingMsg()),
		"Send() must be aborted by Close()")
}

func TestPeer_isClosed(t *testing.T) {
	setup := MakeSetup()
	assert.False(t, setup.alice.partner.isClosed(), "fresh peer must be open")
	assert.NoError(t, setup.alice.partner.Close(), "closing must succeed")
	assert.True(t, setup.alice.partner.isClosed(), "closed peer must be closed")
}

func TestPeer_create(t *testing.T) {
	p := newPeer(nil, nil, nil, nil)
	select {
	case <-p.exists:
		t.Fatal("peer must not yet exist")
	case <-time.NewTimer(Timeout).C:
	}

	conn, _ := newPipeConnPair()
	p.create(conn)

	select {
	case <-p.exists:
	default:
		t.Fatal("peer must exist")
	}

	assert.NoError(t, conn.Close(),
		"Peer.create() on nonexisting peers must not close the new connection")

	conn2, _ := newPipeConnPair()
	p.create(conn2)
	assert.Error(t, conn2.Close(),
		"Peer.create() on existing peers must close the new connection")
}
