// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package channel // import "perun.network/go-perun/channel"

import (
	"github.com/pkg/errors"

	"perun.network/go-perun/log"
	"perun.network/go-perun/wallet"
)

// Controller represents the formerly called 'machine-controller' which wraps a `machine`
// and makes it 'intelligent'.
// It reacts to real world events like disputes and timeouts. A user would only interact with a `Controller`,
// never with a `machine` directly.
type Controller struct {
	*machine
	updateChannel chan RemoteUpdate
}

// RemoteUpdate Update request from a participant.
// The user has to react by writing a `RemoteUpdateRes` into the `response` chan.
type RemoteUpdate struct {
	part     uint
	state    *State
	response chan<- RemoteUpdateRes
}

// RemoteUpdateRes The users response to a `RemoteUpdate`.
type RemoteUpdateRes struct {
	// Whether or not the user agrees with the requested `RemoteUpdate` or not.
	// ok=false does not imply a dispute.
	ok bool
	// Reason for disagreement, only needed when ok=false
	reason string
	// Alternative requested state, only needed when ok=false
	alternative *State
}

// NewController returns a new channel controller or error
func NewController(acc wallet.Account, params Params) (*Controller, error) {
	machine, err := newMachine(acc, params)
	if err != nil {
		return nil, errors.WithMessage(err, "channel controller creation failed")
	}

	// This will be only buffered for testing
	transitionSub := make(chan PhaseTransition, 20)
	machine.Subscribe(InitActing, "controller", transitionSub)
	machine.Subscribe(InitSigning, "controller", transitionSub)
	machine.Subscribe(Funding, "controller", transitionSub)
	machine.Subscribe(Acting, "controller", transitionSub)
	machine.Subscribe(Signing, "controller", transitionSub)
	machine.Subscribe(Final, "controller", transitionSub)
	machine.Subscribe(Settled, "controller", transitionSub)

	go func() {
		trans := <-transitionSub
		log.Tracef("controller: tracked PhaseTransition %s->%s", trans.From.String(), trans.To.String())

		if len(transitionSub) > 0 {
			log.Panic("There should only be one element in transitionSub at all time")
		}
	}()

	return &Controller{machine, make(chan RemoteUpdate)}, nil
}

// State returns the state of the underlying `machine`
func (c *Controller) State() State {

}

// RemoteUpdates 
func (c *Controller) RemoteUpdates() <-chan RemoteUpdate {
	return c.updateChannel
}
