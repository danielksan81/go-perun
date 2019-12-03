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
	"github.com/stretchr/testify/require"

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
	testClient_Multiplexing(t, 1, 1)
	testClient_Multiplexing(t, 1, 1024)
	testClient_Multiplexing(t, 1024, 1)
	testClient_Multiplexing(t, 32, 32)
}

func testClient_Multiplexing(
	t *testing.T, numListeningClients, numDialingClients int) {
	assert := assert.New(t)
	require := require.New(t)

	require.Less(0, numListeningClients)
	require.Less(0, numDialingClients)

	// the random sleep times are needed to make concurrency-related issues
	// appear more frequently
	// Consequently, the RNG must be seeded externally.

	isDialer := func(i int) bool { return i < numDialingClients }
	numClients := numListeningClients + numDialingClients
	rng := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	connHub := new(peertest.ConnHub)
	identities := make([]peer.Identity, numClients)
	dialers := make([]peer.Dialer, numClients)
	listeners := make([]peer.Listener, numClients)
	clients := make([]*Client, numClients)

	for i := 0; i < numClients; i++ {
		i := i
		id := wallet.NewRandomAccount(rng)
		dialer, listener, err := connHub.Create(id)
		require.NoError(err)

		identities[i] = id
		dialers[i] = dialer
		listeners[i] = listener
		clients[i] = New(id, dialer, new(DummyProposalHandler))

		if i >= numDialingClients {
			go clients[i].Listen(listeners[i])
		}
	}

	hostBarrier := new(sync.WaitGroup)
	peerBarrier := make(chan struct{})
	// every dialing client connects to every listening client
	numConnections := numListeningClients * numDialingClients
	hostBarrier.Add(numConnections)

	// create connections
	for i := 0; i < numDialingClients; i++ {
		for y := 0; y < numListeningClients; y++ {
			j := numDialingClients + y
			addr := identities[j].Address()
			sleepTime := time.Duration(rand.Int63n(10) + 1)

			require.True(isDialer(i))
			require.False(isDialer(j))

			go func(i int) {
				defer hostBarrier.Done()

				<-peerBarrier
				time.Sleep(sleepTime * time.Millisecond)

				// perform this test inside closure to detect errors involving
				// the loop variable
				require.False(clients[i].id.Address().Equals(addr))

				// trigger dialing
				_ = clients[i].peers.Get(addr)
			}(i)
		}
	}

	close(peerBarrier)
	hostBarrier.Wait()
	// race tests fail with lower sleep because not all Client.Listen routines
	// have added the peer to their registry yet.
	time.Sleep(200 * time.Millisecond)

	for i := 0; i < numDialingClients; i++ {
		assert.Equal(numListeningClients, clients[i].peers.NumPeers())
	}

	for i := numDialingClients; i < numDialingClients+numListeningClients; i++ {
		assert.Equal(numDialingClients, clients[i].peers.NumPeers())
	}

	for i := 0; i < numDialingClients; i++ {
		for j := numDialingClients; j < numDialingClients+numListeningClients; j++ {
			assert.True(clients[i].peers.Has(identities[j].Address()))
			assert.True(clients[j].peers.Has(identities[i].Address()))
		}
	}

	// close connections
	peerBarrier = make(chan struct{})
	hostBarrier.Add(numConnections)

	for i := 0; i < numDialingClients; i++ {
		i := i

		// disconnect numListeningClients/2 connections from dialer side
		// disconnect numListeningClients/2 connections from listener side
		xs := rng.Perm(numListeningClients)

		for k := 0; k < numListeningClients; k++ {
			j := numDialingClients + xs[k]
			sleepTime := time.Duration(rand.Int63n(10) + 1)

			require.True(isDialer(i))
			require.False(isDialer(j))

			go func(k int) {
				defer hostBarrier.Done()

				<-peerBarrier
				time.Sleep(sleepTime * time.Millisecond)

				var peers *peer.Registry
				var addr peer.Address
				if k < numListeningClients/2 {
					peers = clients[i].peers
					addr = identities[j].Address()
				} else {
					peers = clients[j].peers
					addr = identities[i].Address()
				}

				assert.True(peers.Has(addr))
				p := peers.Get(addr)
				assert.NoError(p.Close())
			}(k)
		}
	}

	close(peerBarrier)
	hostBarrier.Wait()
	time.Sleep(10 * time.Millisecond)

	for i, c := range clients {
		np := c.peers.NumPeers()
		assert.Zero(np, "Client %d has an unexpected number of peers: %d", i, np)
		assert.NoErrorf(c.Close(), "closing client[%d]", i)
	}
}

type Tuple struct {
	r int
	s int
}

func testClient_RandomMultiplexing(t *testing.T) {
	assert := assert.New(t)

	// the random sleep times are needed to make concurrency-related issues
	// appear more frequently
	// Consequently, the RNG must be seeded externally.

	const numClients = 128
	rng := rand.New(rand.NewSource(time.Now().UTC().UnixNano()))
	connHub := new(peertest.ConnHub)
	identities := make([]peer.Identity, numClients)
	dialers := make([]peer.Dialer, numClients)
	listeners := make([]peer.Listener, numClients)
	clients := make([]*Client, numClients)
	numPeers := make([]int, numClients)

	for i := 0; i < numClients; i++ {
		i := i
		id := wallet.NewRandomAccount(rng)
		dialer, listener, err := connHub.Create(id)
		assert.NoError(err)

		identities[i] = id
		dialers[i] = dialer
		listeners[i] = listener
		clients[i] = New(id, dialer, new(DummyProposalHandler))

		go clients[i].Listen(listeners[i])
	}

	const maxNumConnections = numClients * (numClients - 1) / 2
	numConnections := maxNumConnections / 10
	if numConnections < 1 {
		numConnections = (numClients) / 2
	}

	hostBarrier := new(sync.WaitGroup)
	peerBarrier := new(sync.WaitGroup)
	hostBarrier.Add(numConnections)
	peerBarrier.Add(numConnections)
	connectionList := map[Tuple]int{}

	for n := 0; n < numConnections; n++ {
		i := rng.Intn(numClients)
		j := rng.Intn(numClients - 1)
		r := j
		s := i
		if j >= i {
			j = j + 1
			r = i
			s = j
		}

		assert.Less(r, s)

		if _, ok := connectionList[Tuple{r, s}]; ok {
			n--
			continue
		}

		connectionList[Tuple{r, s}] = 1
		registry := clients[i].peers
		peerAddr := identities[j].Address()

		numPeers[i] += 1
		numPeers[j] += 1
		sleepTime := time.Duration(rand.Int63n(10) + 1)

		go func() {
			defer hostBarrier.Done()

			peerBarrier.Done()
			peerBarrier.Wait()
			time.Sleep(sleepTime * time.Millisecond)

			_ = registry.Get(peerAddr)
		}()
	}

	hostBarrier.Wait()
	time.Sleep(10 * time.Millisecond)

	for i := 0; i < numClients; i++ {
		assert.Equal(numPeers[i], clients[i].peers.NumPeers())
	}

	// close connections
	hostBarrier.Add(numClients)
	peerBarrier.Add(numClients)

	for _, c := range clients {
		c := c
		sleepTime := time.Duration(rand.Int63n(10) + 1)
		go func() {
			defer hostBarrier.Done()

			peerBarrier.Done()
			peerBarrier.Wait()
			time.Sleep(sleepTime * time.Millisecond)

			assert.NoError(c.Close())
		}()
	}

	hostBarrier.Wait()
	time.Sleep(10 * time.Millisecond)

	for i, c := range clients {
		assert.Equal(
			0, c.peers.NumPeers(),
			"Client %d has an unexpected number of peers", i)
		//assert.NoError(c.Close())
	}
}
