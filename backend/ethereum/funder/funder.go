// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

// Package funder provides the ethereum funder.
package funder // import "perun.network/go-perun/backend/ethereum/funder"

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/pkg/errors"

	"perun.network/go-perun/backend/ethereum/wallet"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/log"
	"perun.network/go-perun/pkg/io"
	perunwallet "perun.network/go-perun/wallet"
)

const gasLimit = 200000
const startBlockOffset = 100

// ETHAssetHolder is the on-chain address of the ETH asset holder.
// This is needed to distinguish between ETH and ERC-20 transactions.
var ETHAssetHolder = common.Address{}

// Funder implements the channel.Funder interface for Ethereum.
type Funder struct {
	wallet  wallet.Wallet
	client  *ethclient.Client
	ks      *keystore.KeyStore
	account *accounts.Account
	// A list of all contracts
	contracts []*AssetHolder
	context   context.Context
}

// compile time check that we implement the perun funder interface
var _ channel.Funder = (*Funder)(nil)

// ConnectToClient connects to an ethereum node under the specified url.
func (f *Funder) ConnectToClient(url string) (err error) {
	f.client, err = ethclient.Dial(url)
	return err
}

// Fund implements the funder interface.
// It can be used to fund state channels on the ethereum blockchain.
func (f *Funder) Fund(context context.Context, request channel.FundingReq) error {
	f.context = context

	if request.Params == nil || request.Allocation == nil {
		return errors.New("Invalid funding request")
	}
	var channelID = request.Params.ID()
	log.Debugf("Funding Channel with ChannelID %d", channelID)

	partIDs, err := calcPartIDs(request.Params.Parts, request.Params.ID())
	if err != nil {
		return err
	}

	// Fund each asset.
	for assetIndex, asset := range request.Allocation.Assets {
		// Decode and set the asset address.
		assetAddr, err := assetToAddress(asset)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Could not decode asset %d", assetIndex))
		}
		if err = f.setContractAt(assetAddr.Address, assetIndex); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Could not connect to asset holder %d", assetIndex))
		}
		// Create a new transaction.
		balance := request.Allocation.OfParts[request.Idx][assetIndex]
		var auth *bind.TransactOpts
		// If we want to fund the channel with ether, send eth in transaction.
		if assetAddr.Address == ETHAssetHolder {
			auth, err = f.newTransactor(balance, gasLimit)
		} else {
			auth, err = f.newTransactor(big.NewInt(0), gasLimit)
		}
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Could not create Transactor for asset %d", assetIndex))
		}
		// Call the asset holder contract.
		tx, err := f.contracts[assetIndex].Deposit(auth, partIDs[request.Idx], balance)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf(("Deposit failed for asset %d"), assetIndex))
		}
		log.Debugf("Sending transaction to the blockchain with txHash: ", tx.Hash().Hex())
	}
	// After the funding process we wait for confirmation.
	return f.waitForFundingConfirmations(request, partIDs)
}

// waitForFundingConfirmations waits for the confirmations events on the blockchain that
// both we and all peers sucessfully funded the channel.
func (f *Funder) waitForFundingConfirmations(request channel.FundingReq, partIDs [][32]byte) error {
	latestBlock, err := f.client.BlockByNumber(f.context, nil)
	if err != nil {
		return errors.Wrap(err, "Could not retrieve latest block")
	}
	var blockNum uint64
	if latestBlock.NumberU64() > startBlockOffset {
		blockNum = latestBlock.NumberU64() - startBlockOffset
	} else {
		blockNum = 0
	}

	deposited := make(chan *AssetHolderDeposited)
	// Wait for confirmation on each asset.
	for assetIndex := range request.Allocation.Assets {
		watchOpts := f.newWatchOpts(blockNum)
		_, err := f.contracts[assetIndex].WatchDeposited(watchOpts, deposited, partIDs)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("WatchDeposit on asset %d failed", assetIndex))
		}
	}

	allocation := request.Allocation.Clone()
	// Precompute assets -> address
	var assets []*wallet.Address
	for aIdx, a := range request.Allocation.Assets {
		asset, err := assetToAddress(a)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Could not convert asset %d", aIdx))
		}
		assets = append(assets, asset)
	}

	for i := 0; i < len(request.Params.Parts)*len(request.Allocation.Assets); i++ {
		select {
		case event := <-deposited:
			// Calculate the position in the participant array.
			idx := sort.Search(len(partIDs), func(i int) bool {
				return partIDs[i] == event.ParticipantID
			})
			// Retrieve the position in the asset array.
			assetIdx := sort.Search(len(request.Allocation.Assets), func(i int) bool {
				return assets[i].Address == event.Raw.Address
			})
			// TODO needs better event in assetholder
			//if allocation[idx][assetIdx] != event.Amount {}
			//	return errors.New("deposit in asset %d from pariticipant %d does not match agrreed upon asset")
			allocation.OfParts[idx][assetIdx] = big.NewInt(0)
		case _ = <-f.context.Done():
			return errors.New("Waiting for events cancelled by context")
		}
	}
	// Check if everyone funded correctly.
	for i := 0; i < len(allocation.OfParts); i++ {
		for k := 0; k < len(allocation.OfParts[i]); k++ {
			if allocation.OfParts[i][k].Cmp(big.NewInt(0)) != 0 {
				var err channel.PeerTimedOutFundingError
				err.TimedOutPeerIdx = uint16(i)
				return err
			}
		}
	}
	return nil
}

func (f *Funder) newWatchOpts(startBlock uint64) *bind.WatchOpts {
	return &bind.WatchOpts{
		Start:   &startBlock,
		Context: f.context,
	}
}

func (f *Funder) newTransactor(value *big.Int, gasLimit uint64) (*bind.TransactOpts, error) {
	if f.client == nil || f.context == nil || f.ks == nil {
		return nil, errors.New("funder is not configured properly")
	}
	nonce, err := f.client.PendingNonceAt(f.context, f.account.Address)
	if err != nil {
		return nil, err
	}

	gasPrice, err := f.client.SuggestGasPrice(f.context)
	if err != nil {
		return nil, err
	}

	auth, err := bind.NewKeyStoreTransactor(f.ks, *f.account)
	if err != nil {
		return nil, err
	}

	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = value       // in wei
	auth.GasLimit = gasLimit // in units
	auth.GasPrice = gasPrice

	return auth, nil
}

func (f *Funder) setContractAt(addr common.Address, index int) error {
	contract, err := NewAssetHolder(addr, f.client)
	if index > len(f.contracts) {
		f.contracts = append(f.contracts, make([]*AssetHolder, index-len(f.contracts))...)
	}
	f.contracts[index] = contract
	return err
}

func assetToAddress(asset io.Encoder) (*wallet.Address, error) {
	var buf []byte
	buffer := bytes.NewBuffer(buf)
	asset.Encode(buffer)
	var addr wallet.Address
	return &addr, addr.Decode(buffer)
}

func calcPartIDs(participants []perunwallet.Address, channelID [32]byte) ([][32]byte, error) {
	var partIDs [][32]byte
	abibytes32, _ := abi.NewType("bytes32", nil)
	abiaddress, _ := abi.NewType("address", nil)
	args := abi.Arguments{{Type: abibytes32}, {Type: abiaddress}}
	for _, pID := range participants {
		address := pID.(*wallet.Address)
		bytes, err := args.Pack(channelID, address.Address)
		if err != nil {
			return nil, errors.Wrap(err, "Could not pack values")
		}
		var hash = [32]byte{1, 2}
		copy(hash[:], crypto.Keccak256(bytes))
		partIDs = append(partIDs, hash)
	}
	return partIDs, nil
}
