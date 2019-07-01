// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

// Package wallet defines an abstraction to wallet providers.
// It provides an interface to connect other packages to a wallet provider.
// Wallet providers can be hardware, software remote or local wallets.
package wallet

// Helper is an empty interface that implements helper methods
type Helper interface {
	// NewAddressFromString creates a new address from a string
	NewAddressFromString(s string) (Address, error)
	// NewAddressFromBytes creates a new address from a byte array
	NewAddressFromBytes(data []byte) error
}

// Wallet represents single or multiple accounts on a hardware or software wallet.
type Wallet interface {

	// Path returns an identifier under which this wallet is located.
	Path() string

	// Connect establishes a connection to a wallet.
	// It does not decrypt the keys.
	Connect(path, password string) error

	// Disconnect closes a connection to a wallet and locks all accounts.
	Disconnect() error

	// Status returns the current status of the wallet.
	Status() (string, error)

	// Accounts returns all accounts associated with this wallet.
	Accounts() []Account

	// Contains checks whether this wallet contains this account.
	Contains(a Account) bool

	// Lock locks all
	Lock() error
}
