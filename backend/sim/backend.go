// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package sim

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"io"

	"github.com/pkg/errors"

	perunchannel "perun.network/go-perun/channel"
	"perun.network/go-perun/log"
	"perun.network/go-perun/wallet"
	perunwallet "perun.network/go-perun/wallet"
	"perun.network/go-perun/wire"
)

var curve = elliptic.P256()

// Backend implements the utility interface defined in the wallet package.
type Backend struct{}

var _ perunwallet.Backend = new(Backend)
var _ perunchannel.Backend = new(Backend)

// ChannelID Calculates a channels ID by hashing all its members deterministically
func (h *Backend) ChannelID(p *perunchannel.Params) perunchannel.ID {
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

// writeAlloc Writes all fields of `a` to `w`
func (h *Backend) writeAlloc(w io.Writer, a perunchannel.Alloc) error {
	// Write AppID
	if err := wire.Encode(w, a.AppID); err != nil {
		return errors.WithMessage(err, "AppID encode")
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
func (h *Backend) writeAllocation(w io.Writer, a perunchannel.Allocation) error {
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
		if err := h.writeAlloc(w, locked); err != nil {
			return errors.WithMessage(err, "Alloc.Encode")
		}
	}

	return nil
}

// packState packs all fields of a State into a []byte
func (h *Backend) packState(s perunchannel.State) ([]byte, error) {
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
	if err := h.writeAllocation(w, s.Allocation); err != nil {
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
func (h *Backend) Sign(addr wallet.Account, params *perunchannel.Params, state *perunchannel.State) ([]byte, error) {
	log.Tracef("Signing state %s version %d", string(state.ID[:]), state.Version)

	stateData, err := h.packState(*state)
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
func (h *Backend) Verify(addr wallet.Address, params *perunchannel.Params, state *perunchannel.State, sig []byte) (bool, error) {
	log.Tracef("Verifying state %s version %d", string(state.ID[:]), state.Version)

	stateData, err := h.packState(*state)
	if err != nil {
		return false, errors.WithMessage(err, "pack state")
	}

	return h.VerifySignature(stateData, sig, addr)
}

// NewAddressFromString creates a new address from a string.
// DEPRECATED
func (h *Backend) NewAddressFromString(s string) (perunwallet.Address, error) {
	return h.NewAddressFromBytes([]byte(s))
}

// NewAddressFromBytes creates a new address from a byte array.
// DEPRECATED
func (h *Backend) NewAddressFromBytes(data []byte) (perunwallet.Address, error) {
	return h.DecodeAddress(bytes.NewReader(data))
}

// DecodeAddress decodes an address from the given Reader
func (h *Backend) DecodeAddress(r io.Reader) (perunwallet.Address, error) {
	var addr Address
	return &addr, addr.Decode(r)
}

// VerifySignature verifies if a signature was made by this account.
func (h *Backend) VerifySignature(msg, sig []byte, a perunwallet.Address) (bool, error) {
	addr, ok := a.(*Address)
	if !ok {
		log.Panic("Wrong address type passed to Backend.VerifySignature")
	}
	pk := (*ecdsa.PublicKey)(addr)

	r, s, err := deserializeSignature(sig)
	if err != nil {
		return false, errors.WithMessage(err, "could not deserialize signature")
	}

	// escda.Verify needs a digest as input
	// ref https://golang.org/pkg/crypto/ecdsa/#Verify
	return ecdsa.Verify(pk, digest(msg), r, s), nil
}

func digest(msg []byte) []byte {
	digest := sha256.Sum256(msg)
	return digest[:]
}
