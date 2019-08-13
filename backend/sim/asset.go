// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package sim

import (
	"fmt"
	"io"
	"math/rand"

	perunwire "perun.network/go-perun/wire"
)

type Asset struct {
	Name string
}

func NewRandomAsset(rng *rand.Rand) *Asset {
	return &Asset{Name: fmt.Sprintf("Asset #%d", rng.Int63())}
}

func (a *Asset) Encode(w io.Writer) error {
	return perunwire.ByteSlice(a.Name).Encode(w)
}

func (a *Asset) Decode(r io.Reader) error {
	var name perunwire.ByteSlice
	if err := name.Decode(r); err != nil {
		return err
	}
	a.Name = string(name)
	return nil
}
