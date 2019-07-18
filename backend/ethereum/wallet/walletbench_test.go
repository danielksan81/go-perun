// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package wallet

import (
	"testing"

	"perun.network/go-perun/wallet/test"
)

func BenchmarkGenericAccount(b *testing.B) {
	setup := newBenchSetup()
	test.GenericAccountBenchmark(b, setup)
}

func BenchmarkGenericWallet(b *testing.B) {
	setup := newBenchSetup()
	test.GenericWalletBenchmark(b, setup)
}

func BenchmarkGenericBackend(b *testing.B) {
	setup := newBenchSetup()
	test.GenericBackendBenchmark(b, setup)
}

func newBenchSetup() *test.Setup {
	// Filled with the same data as the testing
	return &test.Setup{
		Wallet:     new(Wallet),
		Path:       "./" + keyDir,
		WalletPW:   password,
		AccountPW:  password,
		Backend:    new(Backend),
		AddrString: sampleAddr,
		DataToSign: []byte(dataToSign),
	}
}
