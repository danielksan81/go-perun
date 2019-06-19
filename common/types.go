// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

// Package common provides type abstractions that are used throughout go-perun.
package common

import (
	"encoding/hex"
)

// Address represents a identifier used in a cryptocurrency.
// It is dependent on the currency and needs to be implemented for every blockchain.
type Address []byte

// Hex returns a hexidecimal representation of an address
func (a Address) Hex() string {
	return hex.EncodeToString(a)
}

// HexToAddress converts a Hex string to an address
// It does not do any type checking
func HexToAddress(src string) Address {
	decoded, err := hex.DecodeString(src)
	if err != nil {
		return nil
	}
	return Address(decoded)
}
