// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

var alreadyClosedErr = errors.New("already closed")

// closeOnce is a mock stream object that is used to test that Close() returns
// an error if called multiple times. The standard pipe types never return an
// error when closing twice, so they are unsuited for this test.
type closeOnce struct {
	closed chan struct{}
}

func newCloseOnce() *closeOnce {
	return &closeOnce{make(chan struct{})}
}
func (c *closeOnce) Read([]byte) (int, error) {
	panic("dummy")
}

func (c *closeOnce) Write([]byte) (int, error) {
	panic("dummy")
}

func (c *closeOnce) Close() (err error) {
	defer func() { recover() }()

	err = alreadyClosedErr
	close(c.closed)
	err = nil

	return
}

// TestPipeConn_Close tests that the first call to Close() will not return an
// error, but the second call will.
func TestPipeConn_Close(t *testing.T) {
	t.Parallel()

	close := newCloseOnce()
	c := pipeConn{close, close, close}

	assert.NoError(t, c.Close())
	assert.Error(t, c.Close())
}
