// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"perun.network/go-perun/backend/sim/wallet"
)

var _ Dialer = (*MockDialer)(nil)

type MockDialer struct {
	dial chan Conn
}

func (d *MockDialer) Close() error {
	close(d.dial)
	return nil
}

func (d *MockDialer) Dial(ctx context.Context, addr Address) (Conn, error) {
	select {
	case <-ctx.Done():
		return nil, errors.New("aborted manually")
	case conn := <-d.dial:
		if conn != nil {
			return conn, nil
		} else {
			return nil, errors.New("dialer closed")
		}
	}
}

func TestRegistry_Get(t *testing.T) {
	t.Parallel()

	for i := 0; i < 2; i++ {
		rng := rand.New(rand.NewSource(0xb0baFEDD))
		d := &MockDialer{make(chan Conn)}
		r := NewRegistry(func(*Peer) {}, d)

		addr := wallet.NewRandomAddress(rng)
		p := r.Get(addr)
		assert.NotNil(t, p, "Get() must not return nil", i)
		assert.Equal(t, p, r.Get(addr), "Get must return the existing peer", i)
		assert.NotEqual(t, p, r.Get(wallet.NewRandomAddress(rng)),
			"Get() must return different peers for different addresses", i)

		select {
		case <-p.exists:
			t.Fatal("Peer that is still being dialed must not exist", i)
		default:
		}

		conn, _ := newPipeConnPair()
		if i == 0 {
			d.dial <- conn
		} else {
			r.Register(addr, conn)
		}

		<-time.NewTimer(Timeout).C

		select {
		case <-p.exists:
		default:
			t.Fatal("Peer that is successfully dialed must exist", i)
		}

		assert.False(t, p.isClosed(), "Dialed peer must not be closed", i)
	}
}

func TestRegistry_delete(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(0xb0baFEDD))
	d := &MockDialer{make(chan Conn)}
	r := NewRegistry(func(*Peer) {}, d)

	addr := wallet.NewRandomAddress(rng)
	p := r.Get(addr)
	p2, _ := r.find(addr)
	assert.Equal(t, p, p2)

	r.delete(p2)
	p2, _ = r.find(addr)
	assert.Nil(t, p2)

	assert.Panics(t, func() { r.delete(p) }, "double delete must panic")
}
