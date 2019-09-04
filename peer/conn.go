// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"perun.network/go-perun/wire/msg"
)

// Conn is a connection to a peer, and can send wire messages.
// The Send and Recv methods do not have to be re-entrancy-safe, but calls to Close
// that happen in other threads must interrupt ongoing Send and Recv calls.
type Conn interface {
	// Recv receives a message from the peer.
	// If an error occurs, the connection must close itself.
	Recv() (msg.Msg, error)
	// Send sends a message to the peer.
	// If an error occurs, the connection must close itself.
	Send(msg msg.Msg) error
	// Close closes the connection.
	Close() error
}
