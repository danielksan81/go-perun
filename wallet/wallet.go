// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

// Package wallet defines an abstraction to wallet providers.
// It provides an interface to connect other packages to a wallet provider.
// Wallet providers can be hardware, software remote or local wallets.
package wallet

import (
	"time"

	"github.com/perun-network/go-perun/common"
)

// Account represents a single account
type Account interface {

	// Address used by this account
	Address() common.Address

	// Wallet returns the wallet this account belongs to.
	Wallet() Wallet

	// Path returns an optional resource locator within a backend
	Path() string

	// Unlocks this account with the given passphrase for a limited amount of time.
	// If no timeout is set (nil), the wallet will be unlocked indefinetly.
	// If a timeout is set, it overwrites a previously set timeout, even if it was unlocked indefinetly.
	Unlock(password string, timeout time.Duration) error

	// Locks this account.
	Lock() error

	// SignData requests a signature from this account.
	// It returns the signature or an error.
	SignData(data []byte) ([]byte, error)

	// SignDataWithPW requests a signature from this account.
	// It returns the signature or an error.
	// If the account is locked, it will unlock the account, sign the data and lock the account again.
	SignDataWithPW(password string, data []byte) ([]byte, error)
}

// Wallet represents single or multiple accounts on a hardware or software wallet.
type Wallet interface {

	// Path returns an identifier under which this wallet is located.
	Path() string

	// Connect establishes a connection to a wallet.
	// It does not decrypt the keys.
	Connect(password string) error

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
