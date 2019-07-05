// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package wallet_test

import (
	"testing"

	"github.com/perun-network/go-perun/wallet"
	"github.com/stretchr/testify/assert"
)

type TestingObject struct {
	//Testing object
	T *testing.T
	//Wallet tests
	Wallet   wallet.Wallet
	Path     string
	WalletPW string
	//Account/Address tests
	AccountPW  string
	SampleAddr string
	Helper     wallet.Helper
	// Signature tests
	DataToSign []byte
	SignedData []byte
}

func testUninitializedWallet(t *TestingObject) {

	assert.NotNil(t.T, t.Wallet, "Wallet should not be nil")

	assert.NotNil(t.T, t.Wallet.Connect("", ""), "Expected connect to fail")

	assert.Equal(t.T, "", t.Wallet.Path(), "Expected path to be not initialized")

	_, err := t.Wallet.Status()
	assert.NotNil(t.T, err, "Expected error on not connected wallet")

	assert.NotNil(t.T, t.Wallet.Disconnect(), "Disconnect of not connected wallet should return an error")

	assert.NotNil(t.T, t.Wallet.Accounts(), "Expected empty byteslice")

	assert.Equal(t.T, 0, len(t.Wallet.Accounts()), "Expected empty byteslice")

	assert.False(t.T, t.Wallet.Contains(*new(wallet.Account)), "Uninitalized wallet should not contain account")

	assert.NotNil(t.T, t.Wallet.Lock(), "Expected lock to fail")
}

func testInitializedWallet(t *TestingObject) {

	assert.Nil(t.T, t.Wallet.Connect(t.Path, t.WalletPW), "Expected connect to succeed")

	_, err := t.Wallet.Status()
	assert.Nil(t.T, err, "Unlocked wallet should not produce errors")

	assert.Equal(t.T, t.Path, t.Wallet.Path(), "Expected T.path to match init")

	assert.NotNil(t.T, t.Wallet.Accounts(), "Expected accounts")

	assert.False(t.T, t.Wallet.Contains(*new(wallet.Account)), "Expected wallet not to contain an empty account")

	assert.Equal(t.T, 1, len(t.Wallet.Accounts()), "Expected one account")

	acc := t.Wallet.Accounts()[0]
	assert.True(t.T, t.Wallet.Contains(acc), "Expected wallet to contain account")
	// Check unlock account
	{
		assert.False(t.T, acc.IsUnlocked(), "Account should be locked")

		assert.NotNil(t.T, acc.Unlock(""), "Unlock with wrong pw should fail")

		assert.Nil(t.T, acc.Unlock(t.AccountPW), "Expected unlock to work")

		assert.True(t.T, acc.IsUnlocked(), "Account should be unlocked")

		assert.Nil(t.T, acc.Lock(), "Expected lock to work")

		assert.False(t.T, acc.IsUnlocked(), "Account should be locked")
	}
	// Check lock all accounts
	{
		assert.Nil(t.T, acc.Unlock(t.AccountPW), "Expected unlock to work")

		assert.True(t.T, acc.IsUnlocked(), "Account should be unlocked")

		assert.Nil(t.T, t.Wallet.Lock(), "Expected lock to succeed")

		assert.False(t.T, acc.IsUnlocked(), "Account should be locked")
	}
	assert.Nil(t.T, t.Wallet.Disconnect(), "Expected disconnect to succeed")
}

func testSignature(t *TestingObject) {
	assert.Nil(t.T, t.Wallet.Connect(t.Path, t.WalletPW), "Expected connect to succeed")

	assert.Equal(t.T, 1, len(t.Wallet.Accounts()), "Expected one account")

	acc := t.Wallet.Accounts()[0]
	// Check locked account
	{
		_, err := acc.SignData(t.DataToSign)
		assert.NotNil(t.T, err, "Sign with locked account should fail")

		sign, err := acc.SignDataWithPW(t.WalletPW, t.DataToSign)
		assert.Nil(t.T, err, "SignPW with locked account should succeed")
		valid, err := t.Helper.VerifySignature(t.DataToSign, sign, acc.Address())
		assert.True(t.T, valid, "Verification should succeed")
		assert.Nil(t.T, err, "Verification should succeed")

		assert.False(t.T, acc.IsUnlocked(), "Account should not be unlocked")
	}
	assert.Nil(t.T, acc.Unlock(t.AccountPW), "Unlock should not fail")
	// Check unlocked account
	{
		sign, err := acc.SignData(t.DataToSign)
		assert.Nil(t.T, err, "Sign with unlocked account should succeed")
		valid, err := t.Helper.VerifySignature(t.DataToSign, sign, acc.Address())
		assert.True(t.T, valid, "Verification should succeed")
		assert.Nil(t.T, err, "Verification should not produce error")

		sign, err = acc.SignDataWithPW(t.WalletPW, t.DataToSign)
		assert.Nil(t.T, err, "SignPW with unlocked account should succeed")
		valid, err = t.Helper.VerifySignature(t.DataToSign, sign, acc.Address())
		assert.True(t.T, valid, "Verification should succeed")
		assert.Nil(t.T, err, "Verification should not produce error")

		addr, err := t.Helper.NewAddressFromString(t.SampleAddr)
		assert.Nil(t.T, err, "Byte deserialization of Address should work")
		valid, err = t.Helper.VerifySignature(t.DataToSign, sign, addr)
		assert.False(t.T, valid, "Verification with wrong address should fail")
		assert.Nil(t.T, err, "Verification of valid signature should not produce error")

		sign[0] = ^sign[0] // invalidate signature
		valid, err = t.Helper.VerifySignature(t.DataToSign, sign, acc.Address())
		assert.False(t.T, valid, "Verification should fail")
		assert.NotNil(t.T, err, "Verification of invalid signature should produce error")

		assert.True(t.T, acc.IsUnlocked(), "Account should be unlocked")
	}
	assert.Nil(t.T, t.Wallet.Disconnect(), "Expected disconnect to succeed")
}

func testAccounts(t *TestingObject) {

	init, err := t.Helper.NewAddressFromString(t.SampleAddr)
	assert.Nil(t.T, err, "Byte deserialization of Address should work")

	unInit, err := t.Helper.NewAddressFromBytes(make([]byte, len(init.Bytes()), len(init.Bytes())))
	assert.Nil(t.T, err, "Byte deserialization of Address should work")

	acc, err := t.Helper.NewAddressFromBytes(init.Bytes())
	assert.Nil(t.T, err, "Byte deserialization of Address should work")

	assert.Equal(t.T, init, acc, "Expected equality to serialized byte array")

	acc, err = t.Helper.NewAddressFromString(init.String())
	assert.Nil(t.T, err, "String deserialization of Address should work")

	assert.Equal(t.T, init, acc, "Expected equality to serialized string array")

	assert.True(t.T, init.Equals(init), "Expected equality to itself")

	assert.False(t.T, init.Equals(unInit), "Expected non-equality to other")

	assert.True(t.T, unInit.Equals(unInit), "Expected equality to itself")
}

// GenericWalletTest runs a test suite designed to test the general functionality of an implementation of wallet.
// This function should be called by every implementation of the wallet interface.
func GenericWalletTest(t *TestingObject) {
	testUninitializedWallet(t)
	testInitializedWallet(t)
	testUninitializedWallet(t)

	testAccounts(t)
	testSignature(t)
}
