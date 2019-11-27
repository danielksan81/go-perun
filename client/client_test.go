// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package client

import (
	"context"
	"math/rand"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	_ "perun.network/go-perun/backend/sim/channel" // backend init
	"perun.network/go-perun/backend/sim/wallet"
	"perun.network/go-perun/peer"
	peertest "perun.network/go-perun/peer/test"
	wire "perun.network/go-perun/wire/msg"
)

type DummyDialer struct {
}

func (DummyDialer) Dial(ctx context.Context, addr peer.Address) (peer.Conn, error) {
	panic("BUG: DummyDialer.Dial called")
}

func (DummyDialer) Close() error {
	panic("BUG: DummyDialer.Close called")
}

type DummyProposalHandler struct {
}

func (DummyProposalHandler) Handle(_ *ChannelProposal, _ *ProposalResponder) {
	panic("BUG: DummyProposalHandler called")
}

type DummyListener struct {
	done chan struct{}
}

func NewDummyListener() *DummyListener {
	return &DummyListener{make(chan struct{})}
}

func (d *DummyListener) Accept() (peer.Conn, error) {
	<-d.done
	return nil, errors.New("EOF")
}

func (d *DummyListener) Close() error {
	select {
	case <-d.done:
		panic("DummyListener already closed")
	default:
		close(d.done)
	}

	return nil
}

func TestClient_New(t *testing.T) {
	rng := rand.New(rand.NewSource(0x1a2b3c))
	id := wallet.NewRandomAccount(rng)
	dialer := new(DummyDialer)
	proposalHandler := new(DummyProposalHandler)
	c := New(id, dialer, proposalHandler)

	assert.NotNil(t, c)
	assert.NotNil(t, c.peers)
	assert.Equal(t, c.propHandler, proposalHandler)
}

func TestClient_NewAndListen(t *testing.T) {
	rng := rand.New(rand.NewSource(0x1a2b3c))
	id := wallet.NewRandomAccount(rng)
	dialer := new(DummyDialer)
	proposalHandler := new(DummyProposalHandler)
	c := New(id, dialer, proposalHandler)

	assert.NotNil(t, c)
	assert.NotNil(t, c.peers)
	assert.Equal(t, c.propHandler, proposalHandler)

	done := make(chan struct{})
	listener := NewDummyListener()
	numGoroutines := runtime.NumGoroutine()

	go func() {
		defer close(done)
		c.Listen(listener)
	}()

	assert.Nil(t, listener.Close())

	select {
	case <-done:
		break
	case <-time.After(1 * time.Second):
		t.Error("Listener apparently not stopped")
	}

	// yield processor to give the goroutine above time to terminate itself
	// (it may be put to sleep by the scheduler after closing the channel)
	runtime.Gosched()

	assert.Equal(t, numGoroutines, runtime.NumGoroutine())
}

type OneTimeListener struct {
	Conn net.Conn
	Done chan struct{}
}

var _ peer.Listener = (*OneTimeListener)(nil)

func NewOneTimeListener(c net.Conn) *OneTimeListener {
	return &OneTimeListener{c, make(chan struct{})}
}

// Return one connection, wait for channel close afterwards
func (o *OneTimeListener) Accept() (peer.Conn, error) {
	e := errors.New("Closed accept")
	select {
	case <-o.Done:
		return nil, e
	default:
	}

	c := o.Conn
	o.Conn = nil

	if c != nil {
		return peer.NewIoConn(c), nil
	}

	<-o.Done
	return nil, e
}

func (o *OneTimeListener) Close() error {
	close(o.Done)
	return nil
}

func TestClient_NoAuthResponseMsg(t *testing.T) {
	assert := assert.New(t)

	rng := rand.New(rand.NewSource(0x1a2b3c))
	id := wallet.NewRandomAccount(rng)
	dialer := new(DummyDialer)
	proposalHandler := new(DummyProposalHandler)
	c := New(id, dialer, proposalHandler)
	conn0, conn1 := net.Pipe()

	assert.NotNil(c)
	assert.NotNil(c.peers)
	assert.Equal(0, c.peers.NumPeers())
	assert.Equal(c.propHandler, proposalHandler)

	done := make(chan string, 1)
	listener := NewOneTimeListener(conn1)

	go func() {
		defer close(done)
		c.Listen(listener)
	}()

	if err := wire.Encode(wire.NewPingMsg(), conn0); err != nil {
		assert.NoError(err)
	}

	assert.Equal(0, c.peers.NumPeers())
	assert.NoError(conn0.Close())

	// heuristically time.Sleep works better here than runtime.Gosched()
	time.Sleep(time.Millisecond)

	select {
	case <-done:
		t.Error("client.Listen goroutine terminated already when it should not")
	default:
	}

	listener.Close()
	<-done
	time.Sleep(time.Millisecond)
}

func TestClient_AuthResponseMsg(t *testing.T) {
	assert := assert.New(t)

	rng := rand.New(rand.NewSource(0xC0FFEE))
	hostId := wallet.NewRandomAccount(rng)
	peerId := wallet.NewRandomAccount(rng)
	dialer := new(DummyDialer)
	proposalHandler := new(DummyProposalHandler)
	c := New(hostId, dialer, proposalHandler)
	conn0, conn1 := net.Pipe()

	assert.NotNil(c)
	assert.NotNil(c.peers)
	assert.Equal(0, c.peers.NumPeers())
	assert.Equal(c.propHandler, proposalHandler)

	waitGroup := new(sync.WaitGroup)
	listener := NewOneTimeListener(conn0)

	waitGroup.Add(2)

	go func() {
		defer waitGroup.Done()
		c.Listen(listener)
	}()

	go func() {
		defer waitGroup.Done()

		addr, err := peer.ExchangeAddrs(peerId, peer.NewIoConn(conn1))

		assert.Equal(hostId.Address(), addr)
		assert.NoError(err)

		listener.Close()
	}()

	waitGroup.Wait()

	time.Sleep(10 * time.Millisecond)
	assert.Equal(1, c.peers.NumPeers())
	assert.True(c.peers.Has(peerId.Address()))

	p := c.peers.Get(peerId.Address())
	assert.NoError(p.Close())

	time.Sleep(10 * time.Millisecond)

	assert.Equal(0, c.peers.NumPeers())
}

func TestClient_Multiplexing(t *testing.T) {
	assert := assert.New(t)

	// the random sleep times are needed to make concurrency-related issues
	// appear more frequently
	// Consequently, the RNG must be seeded externally.

	const numClients = 2
	rng := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	connHub := new(peertest.ConnHub)
	identities := make([]peer.Identity, numClients)
	dialers := make([]peer.Dialer, numClients)
	listeners := make([]peer.Listener, numClients)
	clients := make([]*Client, numClients)
	hostBarrier := new(sync.WaitGroup)
	peerBarrier := new(sync.WaitGroup)

	hostBarrier.Add(numClients)
	peerBarrier.Add(numClients)

	for i := 0; i < numClients; i++ {
		index := i // avoid false sharing
		id := wallet.NewRandomAccount(rng)
		dialer, listener, err := connHub.Create(id)
		assert.NoError(err)

		identities[index] = id
		dialers[index] = dialer
		listeners[index] = listener
		clients[index] = New(id, dialer, new(DummyProposalHandler))

		sleepTime := time.Duration(rand.Int63n(10) + 1)

		go func() {
			defer hostBarrier.Done()

			peerBarrier.Done()
			peerBarrier.Wait()
			time.Sleep(sleepTime * time.Millisecond)

			go clients[index].Listen(listeners[index])

			if index == 0 {
				return
			}

			registry := clients[index].peers
			_ = registry.Get(identities[0].Address())
		}()
	}

	hostBarrier.Wait()

	for clients[0].peers.NumPeers() != numClients-1 {
		time.Sleep(10 * time.Millisecond)
	}

	for i, id := range identities[1:] {
		assert.True(
			clients[0].peers.Has(id.Address()),
			"Identity of client %d unknown to client 0", i+1)
		assert.True(
			clients[i+1].peers.Has(identities[0].Address()),
			"Client %d missing identity of client 0", i+1)
	}

	assert.Equal(clients[0].peers.NumPeers(), numClients-1)

	// close connections
	hostBarrier.Add(numClients)
	peerBarrier.Add(numClients)

	for i, c := range clients {
		// avoid false sharing
		index := i
		client := c
		sleepTime := time.Duration(rand.Int63n(10) + 1)
		go func() {
			defer hostBarrier.Done()

			peerBarrier.Done()
			peerBarrier.Wait()
			time.Sleep(sleepTime * time.Millisecond)

			if index == 0 {
				return
			}

			var p *peer.Peer
			if index < numClients/2 {
				p = clients[0].peers.Get(identities[index].Address())
			} else {
				p = client.peers.Get(identities[0].Address())
			}
			assert.NoError(p.Close())
		}()
	}

	hostBarrier.Wait()
	time.Sleep(10 * time.Millisecond)

	for i, c := range clients {
		assert.Equal(
			0, c.peers.NumPeers(),
			"Client %d has an unexpected number of peers", i)
		assert.NoError(c.Close())
	}
}