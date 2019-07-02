// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package wallet

import "fmt"

// Address represents an address in a blockchain network.
type Address interface {
	// String converts this address to a string.
	fmt.Stringer
	// Bytes returns the bytes representation of this address.
	Bytes() []byte
	// Equals checks the equality of two addresses.
	Equals(Address) bool
}
