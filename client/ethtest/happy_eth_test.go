// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package client_test

import (
	"context"
	"io/ioutil"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"perun.network/go-perun/backend/ethereum/channel"      // backend init
	"perun.network/go-perun/backend/ethereum/channel/test" // backend init
	"perun.network/go-perun/backend/ethereum/wallet"
	clienttest "perun.network/go-perun/client/test"
	"perun.network/go-perun/log"
	plogrus "perun.network/go-perun/log/logrus"
	"perun.network/go-perun/peer"
	peertest "perun.network/go-perun/peer/test"
)

func init() {
	logger := logrus.New()
	logger.SetLevel(logrus.TraceLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors: true,
	})
	log.Set(plogrus.FromLogrus(logger))
}

var defaultTimeout = 5 * time.Second

func TestHappyAliceBobETH(t *testing.T) {
	log.Info("Starting happy test")
	var hub peertest.ConnHub

	// Create a new KeyStore
	tmpDir, err := ioutil.TempDir("", "go-perun-test-eth-keystore-")
	assert.NoError(t, err, "creating tempdir should not fail")
	const scryptN = 2
	const scryptP = 1
	ks := keystore.NewKeyStore(tmpDir, scryptN, scryptP)
	w := wallet.Wallet{Ks: ks}
	// Create alice and bobs account
	aliceAccETH, err := ks.NewAccount("secret")
	assert.NoError(t, err, "Creating alice account should not fail")
	assert.NoError(t, ks.Unlock(aliceAccETH, "secret"), "unlocking should not fail")
	bobAccETH, err := ks.NewAccount("secret")
	assert.NoError(t, err, "Creating alice account should not fail")
	assert.NoError(t, ks.Unlock(bobAccETH, "secret"), "unlocking should not fail")
	// Create SimulatedBackend
	backend := test.NewSimulatedBackend()
	// Fund both accounts
	backend.FundAddress(context.Background(), aliceAccETH.Address)
	backend.FundAddress(context.Background(), bobAccETH.Address)
	// Create contract backends
	cbAlice := channel.NewContractBackend(backend, ks, &aliceAccETH)
	cbBob := channel.NewContractBackend(backend, ks, &bobAccETH)
	// Deploy the contracts
	adjAddr, err := channel.DeployAdjudicator(cbAlice)
	assert.NoError(t, err, "Adjudicator should deploy successful")
	assetAddr, err := channel.DeployETHAssetholder(cbAlice, adjAddr)
	assert.NoError(t, err, "ETHAssetholder should deploy successful")
	// Create the funders
	funderAlice := channel.NewSimulatedFunder(cbAlice, assetAddr)
	funderBob := channel.NewSimulatedFunder(cbBob, assetAddr)
	aliceAcc := wallet.NewAccountFromEth(&w, &aliceAccETH)
	bobAcc := wallet.NewAccountFromEth(&w, &bobAccETH)
	// Create the settlers
	settlerAlice := channel.NewSimulatedSettler(cbAlice, ks, &aliceAccETH, adjAddr)
	settlerBob := channel.NewSimulatedSettler(cbBob, ks, &bobAccETH, adjAddr)

	setupAlice := clienttest.RoleSetup{
		Name:     "Alice",
		Identity: aliceAcc,
		Dialer:   hub.NewDialer(),
		Listener: hub.NewListener(aliceAcc.Address()),
		Funder:   &funderAlice,
		Settler:  &settlerAlice, // TODO
		Timeout:  defaultTimeout,
	}

	setupBob := clienttest.RoleSetup{
		Name:     "Bob",
		Identity: bobAcc,
		Dialer:   hub.NewDialer(),
		Listener: hub.NewListener(bobAcc.Address()),
		Funder:   &funderBob,
		Settler:  &settlerBob, // TODO
		Timeout:  defaultTimeout,
	}

	execConfig := clienttest.ExecConfig{
		PeerAddrs:       []peer.Address{aliceAcc.Address(), bobAcc.Address()},
		InitBals:        []*big.Int{big.NewInt(100), big.NewInt(100)},
		Asset:           &wallet.Address{Address: assetAddr},
		AppDef:          &wallet.Address{Address: assetAddr},
		NumUpdatesBob:   2,
		NumUpdatesAlice: 2,
		TxAmountBob:     big.NewInt(5),
		TxAmountAlice:   big.NewInt(3),
	}

	alice := clienttest.NewAlice(setupAlice, t)
	bob := clienttest.NewBob(setupBob, t)
	// enable close barrier synchronization
	var closeBarrier sync.WaitGroup
	alice.SetCloseBarrier(&closeBarrier)
	bob.SetCloseBarrier(&closeBarrier)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		log.Info("Starting Alice.Execute")
		alice.Execute(execConfig)
	}()

	go func() {
		defer wg.Done()
		log.Info("Starting Bob.Execute")
		bob.Execute(execConfig)
	}()

	wg.Wait()
	log.Info("Happy test done")
}
