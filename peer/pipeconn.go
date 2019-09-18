// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"io"

	"github.com/pkg/errors"
)

// pipeConn is a connection that sends over a local pipe.
// It is probably only useful for simpler testing.
type pipeConn struct {
	io.ReadCloser
	io.WriteCloser
}

func (c *pipeConn) Close() error {
	r := c.ReadCloser.Close()
	w := c.WriteCloser.Close()
	if r != nil || w != nil {
		return errors.Errorf("error closing pipeConn: ReadCloser: %v WriteCloser: %v", r, w)
	}
	return nil
}

// newPipeConnPair creates endpoints that are connected via pipes.
func newPipeConnPair() (a Conn, b Conn) {
	ra, wa := io.Pipe()
	rb, wb := io.Pipe()
	return NewConn(&pipeConn{ra, wb}), NewConn(&pipeConn{rb, wa})
}
