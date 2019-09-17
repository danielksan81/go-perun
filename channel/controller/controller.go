// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package channel // import "perun.network/go-perun/channel"

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/log"
	"perun.network/go-perun/peer"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wire/msg"
)

// Controller represents the formerly called 'machine-controller' which wraps a `StateMachine`
// and makes it 'intelligent'.
// It reacts to real world events like disputes and timeouts. A user would only interact with a `Controller`,
// never with a `StateMachine` directly.
type Controller struct {
	machineLock sync.RWMutex
	machine     *channel.StateMachine

	chConn        ChannelConn
	updateChannel chan RemoteUpdate

	errorSubsLock sync.RWMutex
	errorSubs     map[<-chan struct{}]chan<- error

	logger log.Logger

	// quit shut down all goRoutines iff this chan is closed
	quit chan struct{}
}

// RemoteUpdate Update request from a participant.
// The user has to react by writing a `RemoteUpdateRes` into the `response` chan.
type RemoteUpdate struct {
	part     uint
	state    *channel.State
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
	alternative *channel.State
}

type ChannelConn struct {
	*peer.Receiver
	//*peer.Broadcaster
}

type UpdateReq struct {
	ctx   *context.Context
	state *channel.State
}

// NewController returns a new channel controller or error
func NewController(acc wallet.Account, params channel.Params, reg *peer.Registry, chConn ChannelConn) (*Controller, error) {
	machine, err := channel.NewStateMachine(acc, params)
	if err != nil {
		return nil, errors.WithMessage(err, "channel controller creation failed")
	}

	// This will be only buffered for testing
	transitionSub := make(chan channel.PhaseTransition, 20)
	machine.Subscribe(channel.InitActing, "controller", transitionSub)
	machine.Subscribe(channel.InitSigning, "controller", transitionSub)
	machine.Subscribe(channel.Funding, "controller", transitionSub)
	machine.Subscribe(channel.Acting, "controller", transitionSub)
	machine.Subscribe(channel.Signing, "controller", transitionSub)
	machine.Subscribe(channel.Final, "controller", transitionSub)
	machine.Subscribe(channel.Settled, "controller", transitionSub)

	c := &Controller{sync.RWMutex{}, machine, chConn, make(chan RemoteUpdate), sync.RWMutex{}, make(map[<-chan struct{}]chan<- error, 0), log.WithField("controller", nil), make(chan struct{})}

	// Receive all the phase updates from the StateMachine
	go func() {
		c.logger.Trace("Listening for PhaseTransitions")
	loop:
		for {
			select {
			case trans, ok := <-transitionSub:
				if !ok {
					c.logger.Panic("Read error for PhaseTransition from StateMachine")
				} else {
					c.logger.Tracef("Tracked PhaseTransition %s->%s", trans.From.String(), trans.To.String())

					if len(transitionSub) > 0 {
						c.logger.Panic("There should only be one element in transitionSub at all time")
					}
				}
			case <-c.quit:
				break loop
			}
		}
		c.logger.Trace("No longer listening for PhaseTransitions")
	}()

	// Listen on messages from the participants
	go func() {
		ch := chConn.Receiver.Next()
		c.logger.Trace("Listening for messages from participants")
	loop:
		for {
			select {
			case <-c.quit:
				break loop
			case msgTuple, ok := <-ch:
				if !ok {
					c.logger.Panic("Read error from Receiver.Next()")
				} else {
					cmsg, ok := msgTuple.Msg.(msg.ChannelMsg)
					if !ok {
						c.logger.Panic("Received wrong message type from Receiver")
					}
					c.handleMessage(cmsg, msgTuple.PerunAddress)
				}
			}
		}
		c.logger.Trace("No longer listening for messages from participants")
	}()

	return c, nil
}

func (c *Controller) handleMessage(msg msg.ChannelMsg, part peer.Address) {
	c.logger.Debugf("Received message of type: %s", msg.Category().String())
}

// Update request the update if the state based machine
func (c *Controller) Update(req UpdateReq) error {
	if err := c.machine.Update(req.state); err != nil {
		return errors.WithMessage(err, "can't update StateMachine")
	}

	_, err := c.machine.Sig()
	if err != nil {
		return errors.WithMessage(err, "could not sign the staged state")
	}

	// Send the new requested state together with the signature to all other participants

	return nil
}

// State returns the state of the underlying `machine`
/*func (c *Controller) State() State {
	// ERROR: Not every Phase has a state!
}*/

// RemoteUpdates 
func (c *Controller) RemoteUpdates() <-chan RemoteUpdate {
	return c.updateChannel
}

// Err returns a new channel that gets informed when an error occures within the controller
func (c *Controller) Err() (<-chan error, chan<- struct{}) {
	data := make(chan error, 10)
	quit := make(chan struct{})

	c.errorSubsLock.Lock()
	defer c.errorSubsLock.Unlock()
	c.errorSubs[quit] = data

	// subscription sub-clean-up go routine
	go func() {
		select {
		case <-quit:
			c.errorSubsLock.Lock()
			defer c.errorSubsLock.Unlock()
			close(c.errorSubs[quit])
			delete(c.errorSubs, quit)
		case <-c.quit: // Thanks go for no fallthrough cases
			c.errorSubsLock.Lock()
			defer c.errorSubsLock.Unlock()
			close(c.errorSubs[quit])
			delete(c.errorSubs, quit)
		}
	}()

	return data, quit
}

func (c *Controller) errorOccurred(e error) {
	c.errorSubsLock.RLock()
	defer c.errorSubsLock.RUnlock()
	c.logger.Tracef("Sending error to %d subscription(s)", len(c.errorSubs))

	for quit, data := range c.errorSubs {
		// Was the subscription closed in the meantime?
		select {
		case <-quit: // This will be handled by the sub-clean-up go routine
		case data <- e:
		}
	}
}

func (c *Controller) Quit() {
	c.logger.Warn("Controller shut down by Quit")
	close(c.quit)
}
