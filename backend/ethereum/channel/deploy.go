// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package channel

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"
	"perun.network/go-perun/backend/ethereum/bindings/adjudicator"
	"perun.network/go-perun/backend/ethereum/bindings/assets"
	"perun.network/go-perun/log"
)

// The Deployer can be used to deploy the smart contracts on ethereum.
// It should only be used in tests as deploying contracts costs a lot of gas.
// The Deployer is not threadsafe, it should only be used once at the start of an end-to-end test.
type Deployer struct {
	contractBackend
}

// NewETHDeployer creates a new ethereum deployer.
func NewETHDeployer(client *ethclient.Client, keystore *keystore.KeyStore, account *accounts.Account) Deployer {
	return Deployer{contractBackend{client, keystore, account}}
}

// DeployETHAssetholder deploys a new ETHAssetHolder contract.
func (d *Deployer) DeployETHAssetholder(adjudicatorAddr common.Address) (common.Address, error) {
	auth, err := d.newTransactor(context.Background(), d.ks, d.account, big.NewInt(0), 6600000)
	if err != nil {
		return common.Address{}, errors.WithMessage(err, "could not create transactor")
	}
	addr, tx, _, err := assets.DeployAssetHolderETH(auth, d, adjudicatorAddr)
	if err != nil {
		return common.Address{}, errors.WithMessage(err, "could not create transaction")
	}
	if err = d.execSuccessful(tx); err != nil {
		return common.Address{}, nil
	}
	log.Warnf("Sucessfully deployed AssetHolderETH at %v.", addr.Hex())
	return addr, nil
}

// DeployAdjudicator deploys a new Adjudicator contract.
func (d *Deployer) DeployAdjudicator() (common.Address, error) {
	auth, err := d.newTransactor(context.Background(), d.ks, d.account, big.NewInt(0), 6600000)
	if err != nil {
		return common.Address{}, errors.WithMessage(err, "could not create transactor")
	}
	addr, tx, _, err := adjudicator.DeployAdjudicator(auth, d)
	if err != nil {
		return common.Address{}, errors.WithMessage(err, "could not create transaction")
	}
	if err = d.execSuccessful(tx); err != nil {
		return common.Address{}, nil
	}
	log.Warnf("Sucessfully deployed Adjudicator at %v.", addr.Hex())
	return addr, nil
}

func (d *Deployer) execSuccessful(tx *types.Transaction) error {
	receipt, err := bind.WaitMined(context.Background(), d, tx)
	if err != nil {
		return errors.WithMessage(err, "could not execute transaction")
	}
	if receipt.Status == types.ReceiptStatusFailed {
		return errors.New("transaction failed")
	}
	return nil
}
