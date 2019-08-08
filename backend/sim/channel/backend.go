// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package channel // import "perun.network/go-perun/backend/sim/channel"

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"io"

	"github.com/pkg/errors"

	"perun.network/go-perun/channel"
	"perun.network/go-perun/log"
	"perun.network/go-perun/wallet"
	"perun.network/go-perun/wire"
)

// Backend implements the utility interface defined in the wallet package.
type Backend struct{}

var _ channel.Backend = new(Backend)

// ChannelID Calculates a channels ID by hashing all its members deterministically
func (*Backend) ChannelID(p *channel.Params) channel.ID {
	buff := new(bytes.Buffer)
	w := bufio.NewWriter(buff)

	// Write ChallengeDuration
	if err := wire.Encode(w, p.ChallengeDuration); err != nil {
		log.Panic("Could not serialize to buffer")
	}
	// Write Parts
	for _, addr := range p.Parts {
		if err := addr.Encode(w); err != nil {
			log.Panic("Could not write to sha256 hasher")
		}
	}
	// Write App Address
	if err := p.App.Def().Encode(w); err != nil {
		log.Panic("Could not write to sha256 hasher")
	}
	// Write Nonce
	if err := wire.Encode(w, p.Nonce); err != nil {
		log.Panic("Could not write to sha256 hasher")
	}
	// Finalize
	if err := w.Flush(); err != nil {
		log.Panic("bufio flush")
	}

	return sha256.Sum256(buff.Bytes())
}

// writeSubAlloc Writes all fields of `a` to `w`
func (*Backend) writeSubAlloc(w io.Writer, a channel.SubAlloc) error {
	// Write ID
	if err := wire.ByteSlice(a.ID[:]).Encode(w); err != nil {
		return errors.WithMessage(err, "ID encode")
	}
	// Write Bals
	for _, bal := range a.Bals {
		if err := wire.Encode(w, bal); err != nil {
			return errors.WithMessage(err, "Bal encode")
		}
	}

	return nil
}

// writeAllocation Writes all fields of `a` to `w`
func (b *Backend) writeAllocation(w io.Writer, a channel.Allocation) error {
	// Write Assets
	for _, asset := range a.Assets {
		if err := asset.Encode(w); err != nil {
			return errors.WithMessage(err, "asset.Encode")
		}
	}
	// Write OfParts
	for _, ofpart := range a.OfParts {
		for _, bal := range ofpart {
			if err := wire.Encode(w, bal); err != nil {
				return errors.WithMessage(err, "big.Int encode")
			}
		}
	}
	// Write Locked
	for _, locked := range a.Locked {
		if err := b.writeSubAlloc(w, locked); err != nil {
			return errors.WithMessage(err, "Alloc.Encode")
		}
	}

	return nil
}

// packState packs all fields of a State into a []byte
func (b *Backend) packState(s channel.State) ([]byte, error) {
	buff := new(bytes.Buffer)
	w := bufio.NewWriter(buff)

	// Write ID
	if err := wire.ByteSlice(s.ID[:]).Encode(w); err != nil {
		return nil, errors.WithMessage(err, "state id encode")
	}
	// Write Version
	if err := wire.Encode(w, s.Version); err != nil {
		return nil, errors.WithMessage(err, "state version encode")
	}
	// Write Allocation
	if err := b.writeAllocation(w, s.Allocation); err != nil {
		return nil, errors.WithMessage(err, "state allocation encode")
	}
	// Write Data
	if err := s.Data.Encode(w); err != nil {
		return nil, errors.WithMessage(err, "state data encode")
	}
	// Write IsFinal
	if err := wire.Encode(w, s.IsFinal); err != nil {
		return nil, errors.WithMessage(err, "state isfinal encode")
	}
	// Finalize
	if err := w.Flush(); err != nil {
		return nil, errors.Wrap(err, "buffio flush")
	}

	return buff.Bytes(), nil
}

// Sign Signs `state`
func (b *Backend) Sign(addr wallet.Account, params *channel.Params, state *channel.State) ([]byte, error) {
	if addr == nil || params == nil || state == nil {
		return nil, errors.New("argument nil")
	}
	log.Tracef("Signing state %s version %d", string(state.ID[:]), state.Version)

	stateData, err := b.packState(*state)
	if err != nil {
		return nil, errors.WithMessage(err, "pack state")
	}

	// We need to wrap SignData because the interface of Backend says that error returns must return a nil signature
	sig, err := addr.SignData(stateData)
	if err != nil {
		return nil, errors.WithMessage(err, "sign data")
	}

	return sig, nil
}

// Verify Verifies the signature for `state`
func (b *Backend) Verify(addr wallet.Address, params *channel.Params, state *channel.State, sig []byte) (bool, error) {
	if addr == nil || params == nil || state == nil {
		return false, errors.New("argument nil")
	}
	log.Tracef("Verifying state %s version %d", string(state.ID[:]), state.Version)

	stateData, err := b.packState(*state)
	if err != nil {
		return false, errors.WithMessage(err, "pack state")
	}

	if b.ChannelID(params) != state.ID {
		return false, nil
	}

	return wallet.VerifySignature(stateData, sig, addr)
}
