// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package client

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"perun.network/go-perun/channel/test"
)

func TestChanRegistry_Put(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDDDDdede))
	ch := &Channel{}
	id := test.NewRandomChannelID(rng)

	t.Run("single insert", func(t *testing.T) {
		r := makeChanRegistry()
		assert.True(t, r.Put(id, ch))
		c, ok := r.Get(id)
		require.True(t, ok)
		require.Same(t, c, ch)
	})

	t.Run("double insert", func(t *testing.T) {
		r := makeChanRegistry()
		require.True(t, r.Put(id, ch))
		assert.False(t, r.Put(id, ch))
	})
}

func TestChanRegistry_Has(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDDDDdede))
	ch := &Channel{}
	id := test.NewRandomChannelID(rng)

	t.Run("nonexistent has", func(t *testing.T) {
		r := makeChanRegistry()
		assert.False(t, r.Has(id))
	})

	t.Run("existing has", func(t *testing.T) {
		r := makeChanRegistry()
		require.True(t, r.Put(id, ch))
		assert.True(t, r.Has(id))
	})
}

func TestChanRegistry_Get(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDDDDdede))
	ch := &Channel{}
	id := test.NewRandomChannelID(rng)

	t.Run("nonexistent get", func(t *testing.T) {
		r := makeChanRegistry()
		c, ok := r.Get(id)
		assert.False(t, ok)
		assert.Nil(t, c)
	})

	t.Run("existing get", func(t *testing.T) {
		r := makeChanRegistry()
		require.True(t, r.Put(id, ch))
		c, ok := r.Get(id)
		assert.True(t, ok)
		assert.Same(t, c, ch)
	})
}

func TestChanRegistry_Delete(t *testing.T) {
	rng := rand.New(rand.NewSource(0xDDDDdede))
	ch := &Channel{}
	id := test.NewRandomChannelID(rng)

	t.Run("nonexistent delete", func(t *testing.T) {
		r := makeChanRegistry()
		assert.False(t, r.Delete(id))
		require.False(t, r.Has(id))
	})

	t.Run("existing delete", func(t *testing.T) {
		r := makeChanRegistry()
		require.True(t, r.Put(id, ch))
		assert.True(t, r.Delete(id))
		assert.False(t, r.Has(id))
	})

	t.Run("double delete", func(t *testing.T) {
		r := makeChanRegistry()
		require.True(t, r.Put(id, ch))
		require.True(t, r.Delete(id))
		assert.False(t, r.Delete(id))
		assert.False(t, r.Has(id))
	})
}
