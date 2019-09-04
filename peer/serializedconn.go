// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"io"

	"perun.network/go-perun/wire/msg"
)

var _ Conn = (*serializedConn)(nil)

// serializedConn is a connection that communicates its messages over a stream.
type serializedConn struct {
	conn io.ReadWriteCloser
}

// NewConn creates a serialized connection from a stream.
func NewConn(conn io.ReadWriteCloser) Conn {
	return &serializedConn{
		conn: conn,
	}
}

func (c *serializedConn) Send(m msg.Msg) error {
	if err := msg.Encode(m, c.conn); err != nil {
		c.conn.Close()
		return err
	} else {
		return nil
	}
}

func (c *serializedConn) Recv() (msg.Msg, error) {
	if m, err := msg.Decode(c.conn); err != nil {
		c.conn.Close()
		return nil, err
	} else {
		return m, nil
	}
}

func (c *serializedConn) Close() error {
	return c.conn.Close()
}
