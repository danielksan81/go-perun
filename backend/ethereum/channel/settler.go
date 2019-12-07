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
	"perun.network/go-perun/channel"
	"perun.network/go-perun/log"
	perunwallet "perun.network/go-perun/wallet"
)

// Settler implements the channel.Settler interface for Ethereum.
type Settler struct {
	client  contractBackend
	ks      *keystore.KeyStore
	account *accounts.Account
	adjAddr common.Address
}

// compile time check that we implement the perun settler interface
var _ channel.Settler = (*Settler)(nil)

// NewETHSettler creates a new ethereum funder.
func NewETHSettler(client *ethclient.Client, keystore *keystore.KeyStore, account *accounts.Account, adjAddr common.Address) Settler {
	return Settler{
		client:  contractBackend{client},
		ks:      keystore,
		account: account,
		adjAddr: adjAddr,
	}
}

// Settle provides the settle functionality.
func (s *Settler) Settle(ctx context.Context, req channel.SettleReq, acc perunwallet.Account) error {
	if req.Params == nil || req.Tx.State == nil {
		panic("invalid settlement request")
	}
	if req.Tx.State.IsFinal == true {
		return s.cooperativeSettle(ctx, req)
	}
	return s.uncooperativeSettle(ctx, req)
}

func (s *Settler) cooperativeSettle(ctx context.Context, req channel.SettleReq) error {
	adjInstance, trans, err := s.connectToContract(ctx)
	if err != nil {
		return errors.Wrap(err, "cooperative settle")
	}
	// Listen for blockchain events.
	confirmation := make(chan error)
	go func() {
		confirmation <- s.waitForSettlingConfirmation(ctx, adjInstance, req.Params.ID())
	}()
	// Call concludeFinal on the adjudicator.
	ethParams := channelParamsToEthParams(req.Params)
	ethState := channelStateToEthState(req.Tx.State)
	tx, err := adjInstance.ConcludeFinal(trans, ethParams, ethState, req.Tx.Sigs)
	if err != nil {
		return errors.WithMessage(err, "failed to call concludeFinal")
	}
	log.Debugf("Sending transaction to the blockchain with txHash: ", tx.Hash().Hex())
	receipt, err := bind.WaitMined(ctx, s.client, tx)
	if err != nil {
		return errors.WithMessage(err, "Failed to execute transaction")
	}
	if receipt.Status == types.ReceiptStatusFailed {
		return errors.New("Failed to execute transaction")
	}
	log.Debugf("Transaction mined with txHash: ", receipt.TxHash.Hex())
	return <-confirmation
}

func (s *Settler) uncooperativeSettle(ctx context.Context, req channel.SettleReq) error {
	panic("Settling with non-final state currently not implemented")
}

func (s *Settler) waitForSettlingConfirmation(ctx context.Context, adjInstance *adjudicator.Adjudicator, channelID [32]byte) error {
	watchOpts, err := s.client.newWatchOpts(ctx)
	if err != nil {
		return errors.Wrap(err, "could not create new watchOpts")
	}
	concluded := make(chan *adjudicator.AdjudicatorFinalConcluded)
	sub, err := adjInstance.WatchFinalConcluded(watchOpts, concluded, [][32]byte{channelID})
	if err != nil {
		return errors.WithMessage(err, "WatchFinalConcluded failed")
	}
	select {
	case <-concluded:
		return nil
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "Waiting for final concluded event cancelled by context")
	case err = <-sub.Err():
		return errors.Wrap(err, "Error while waiting for events")
	}
}

func (s *Settler) connectToContract(ctx context.Context) (*adjudicator.Adjudicator, *bind.TransactOpts, error) {
	adjInstance, err := adjudicator.NewAdjudicator(s.adjAddr, s.client)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "failed to connect to adjudicator")
	}
	trans, err := s.client.newTransactor(ctx, s.ks, s.account, big.NewInt(0), gasLimit)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "fialed to create transactor")
	}
	return adjInstance, trans, nil
}
