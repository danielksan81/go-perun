// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package channel // import "perun.network/go-perun/backend/sim/channel"

import (
	"fmt"
	"io"
	"math/rand"

	"perun.network/go-perun/log"
	"perun.network/go-perun/wire"
)

// Asset simulates a `perunchannel.Asset` by only containing a `string` as Name
type Asset struct {
	Name string
}

// NewRandomAsset returns a new random Asset
func NewRandomAsset(rng *rand.Rand) *Asset {
	return &Asset{Name: fmt.Sprintf("Asset #%d", rng.Int63())}
}

// Encode encodes an Asset into the io.Writer `w`
func (a *Asset) Encode(w io.Writer) error {
	return wire.ByteSlice(a.Name).Encode(w)
}

// Decode is not implemented in this simulation
func (a *Asset) Decode(r io.Reader) error {
	log.Panic("Asset.Decode is not implemented")
	return nil
}
