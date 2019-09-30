// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"context"
	"strconv"

	"perun.network/go-perun/wire/msg"
)

// Broadcaster is a communications object that allows sending a single message
// to multiple peers in a single operation.
type Broadcaster struct {
	peers  []*Peer
	gather chan sendError
}

// Send sends the requested message to all of the broadcaster's recipients.
// This call blocks until all messages have been sent (or failed to send). This
// function returns an error if the message could not be delivered to any of
// the broadcaster's recipients.
//
// The 'abort' channel can be used to manually abort the broadcast.
// The returned error is nil if the message was successfully sent to all peers.
// Otherwise, the returned error contains an array of all individual errors
// that occurred.
func (b *Broadcaster) Send(ctx context.Context, m msg.Msg) *BroadcastError {
	// Send all messages in parallel.
	for i, p := range b.peers {
		go func(i int, p *Peer, m msg.Msg) {
			err := sendError{index: i}
			defer func() { b.gather <- err }()

			err.err = p.Send(ctx, m)
		}(i, p, m)
	}

	// Gather results and collect errors.
	var error BroadcastError
	error.errors = make([]sendError, 0)
	for _, _ = range b.peers {
		err := <-b.gather
		if err.err != nil {
			error.errors = append(error.errors, err)
		}
	}

	if len(error.errors) == 0 {
		return nil
	} else {
		return &error
	}
}

type sendError struct {
	index int
	err   error
}

var _ error = &BroadcastError{}

// BroadcastError is a collection of errors that occurred during a broadcast
// operation.
type BroadcastError struct {
	errors []sendError
}

func (err *BroadcastError) Error() string {
	msg := "failed to send message:"
	for _, err := range err.errors {
		msg += "\npeer[" + strconv.Itoa(err.index) + "]: " + err.err.Error()
	}
	return msg
}

// NewBroadcaster creates a new broadcaster instance.
func NewBroadcaster(peers []*Peer) Broadcaster {
	return Broadcaster{
		peers:  peers,
		gather: make(chan sendError, len(peers)),
	}
}
