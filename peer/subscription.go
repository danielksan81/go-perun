// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"sync"

	"perun.network/go-perun/log"
	"perun.network/go-perun/wire/msg"
)

type subscriptions struct {
	mutex sync.RWMutex
	subs  map[msg.Category][]*Receiver
}

// add adds a receiver to the subscriptions.
// If the receiver was already subscribed, panics.
func (s *subscriptions) add(cat msg.Category, r *Receiver) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, rec := range s.subs[cat] {
		if rec == r {
			log.Panic("duplicate peer subscription")
		}
	}

	s.subs[cat] = append(s.subs[cat], r)
}

func (s *subscriptions) delete(cat msg.Category, r *Receiver) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	subs := s.subs[cat]
	for i, rec := range s.subs[cat] {
		if rec == r {
			subs[i] = subs[len(subs)-1]
			subs = subs[:len(subs)-1]
		}
	}
}

func (s *subscriptions) put(m msg.Msg, p *Peer) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	tuple := MsgTuple{p, m}
	subs := s.subs[m.Category()]
	for _, rec := range subs {
		rec.msgs <- tuple
	}
}

func makeSubscriptions() (s subscriptions) {
	s.subs = make(map[msg.Category][]*Receiver)
	return
}
