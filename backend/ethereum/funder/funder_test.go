// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

// Package funder provides the ethereum funder.
package funder

import (
	"context"
	"io"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"perun.network/go-perun/backend/ethereum/wallet"
	"perun.network/go-perun/channel"
	"perun.network/go-perun/channel/test"
	perunio "perun.network/go-perun/pkg/io"
	perunwallet "perun.network/go-perun/wallet"
)

const nodeURL = "https://testnet.perun.network"

const (
	keyDir   = "../wallet/testdata"
	password = "secret"

	keystoreAddr = "0xf4c288068b32474dedc3620233c"
	keyStorePath = "UTC--2019-06-07T12-12-48.775026092Z--3c5a96ff258b1f4c288068b32474dedc3620233c"
)

func TestFunder_ConnectToClient(t *testing.T) {
	f := &Funder{}
	invalidURL := "8.8.8.8"
	assert.Error(t, f.ConnectToClient(invalidURL), "connecting to invalid url should fail")
	assert.NoError(t, f.ConnectToClient(nodeURL), "connecting to a valid node should succeed")
}

func Test_calcPartIDs(t *testing.T) {
	tests := []struct {
		name         string
		participants []perunwallet.Address
		channelID    [32]byte
		want         [][32]byte
	}{
		{"Test nil array, empty channelID", nil, [32]byte{}, nil},
		{"Test nil array, non-empty channelID", nil, [32]byte{1}, nil},
		{"Test empty array, non-empty channelID", []perunwallet.Address{}, [32]byte{1}, nil},
		{"Test non-empty array, empty channelID", []perunwallet.Address{&wallet.Address{}},
			[32]byte{}, [][32]byte{[32]byte{173, 50, 40, 182, 118, 247, 211, 205, 66, 132, 165, 68, 63, 23, 241, 150, 43, 54, 228, 145, 179, 10, 64, 178, 64, 88, 73, 229, 151, 186, 95, 181}}},
		{"Test non-empty array, non-empty channelID", []perunwallet.Address{&wallet.Address{}},
			[32]byte{1}, [][32]byte{[32]byte{130, 172, 39, 157, 178, 106, 32, 109, 155, 165, 169, 76, 7, 255, 148, 10, 234, 75, 59, 253, 232, 130, 14, 201, 95, 78, 250, 10, 207, 208, 213, 188}}},
		{"Test non-empty array, non-empty channelID", []perunwallet.Address{&wallet.Address{Address: common.BytesToAddress([]byte{})}},
			[32]byte{1}, [][32]byte{[32]byte{130, 172, 39, 157, 178, 106, 32, 109, 155, 165, 169, 76, 7, 255, 148, 10, 234, 75, 59, 253, 232, 130, 14, 201, 95, 78, 250, 10, 207, 208, 213, 188}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calcPartIDs(tt.participants, tt.channelID)
			if err != nil {
				t.Errorf("calculating PartIDs should not produce errors.")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("calcPartIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

type testInvalidAsset [33]byte

func (t *testInvalidAsset) Encode(w io.Writer) error {
	return errors.New("Unimplemented")
}

func (t *testInvalidAsset) Decode(r io.Reader) error {
	return errors.New("Unimplemented")
}

func Test_assetToAddress(t *testing.T) {

	var invAsset testInvalidAsset
	tests := []struct {
		name    string
		asset   perunio.Serializable
		want    *wallet.Address
		wantErr bool
	}{
		{"Test invalid address", &invAsset, &wallet.Address{}, true},
		{"Test valid address", &wallet.Address{}, &wallet.Address{}, false},
		{"Test valid address",
			&wallet.Address{Address: common.Address{1, 2, 3, 4, 5}},
			&wallet.Address{Address: common.Address{1, 2, 3, 4, 5}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := assetToAddress(tt.asset)
			if (err != nil) != tt.wantErr {
				t.Errorf("assetToAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("assetToAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFunder_NewTransactor(t *testing.T) {
	f := &Funder{}
	_, err := f.newTransactor(big.NewInt(0), 1000)
	assert.Error(t, err, "Funder has to have a context set")
	f.context = context.Background()
	_, err = f.newTransactor(big.NewInt(0), 1000)
	assert.Error(t, err, "Creating a transactor without a connection should fail")
	// Connect to client
	assert.NoError(t, f.ConnectToClient(nodeURL), "connecting to a valid node should succeed")
	_, err = f.newTransactor(big.NewInt(0), 1000)
	assert.Error(t, err, "Creating Transactor without valid keystore should fail")
	// Set KeyStore
	f = newFunder()
	transactor, err := f.newTransactor(big.NewInt(0), 1000)
	assert.NoError(t, err, "Creating Transactor should succeed")
	assert.Equal(t, f.account.Address, transactor.From, "Transactor address not properly set")
	assert.Equal(t, uint64(1000), transactor.GasLimit, "Gas limit not set properly")
	assert.Equal(t, big.NewInt(0), transactor.Value, "Transaction value not set properly")
	transactor, err = f.newTransactor(big.NewInt(12345), 12345)
	assert.NoError(t, err, "Creating Transactor should succeed")
	assert.Equal(t, f.account.Address, transactor.From, "Transactor address not properly set")
	assert.Equal(t, uint64(12345), transactor.GasLimit, "Gas limit not set properly")
	assert.Equal(t, big.NewInt(12345), transactor.Value, "Transaction value not set properly")
}

func TestFunder_NewWatchOpts(t *testing.T) {
	f := &Funder{}
	watchOpts := f.newWatchOpts(0)
	assert.Equal(t, nil, watchOpts.Context, "Creating watchopts with no context should succeed")
	assert.Equal(t, uint64(0), *watchOpts.Start, "Creating watchopts with no context should succeed")
	f.context = context.Background()
	watchOpts = f.newWatchOpts(123)
	assert.Equal(t, context.Background(), watchOpts.Context, "Creating watchopts with context should succeed")
	assert.Equal(t, uint64(123), *watchOpts.Start, "Creating watchopts with no context should succeed")
}

func TestFunder_Fund(t *testing.T) {
	f := newFunder()
	assert.Error(t, f.Fund(context.Background(), channel.FundingReq{}), "Funding with invalid funding req should fail")
	req := channel.FundingReq{
		Params:     &channel.Params{},
		Allocation: &channel.Allocation{},
		Idx:        0,
	}
	assert.NoError(t, f.Fund(context.Background(), req), "Funding with no assets should succeed")
	parts := []perunwallet.Address{
		&wallet.Address{Address: f.account.Address},
		&wallet.Address{Address: f.account.Address},
	}
	params, err := channel.NewParams(10, parts, new(test.NoApp), big.NewInt(0))
	assert.NoError(t, err, "New Params should not throw error")
	req = channel.FundingReq{
		Params:     params,
		Allocation: &channel.Allocation{},
		Idx:        0,
	}
}

func newFunder() *Funder {
	f := &Funder{}
	f.context = context.Background()
	// Set KeyStore
	wall := new(wallet.Wallet)
	wall.Connect(keyDir, password)
	acc := wall.Accounts()[0].(*wallet.Account)
	acc.Unlock(password)
	ks := wall.KeyStore()
	f.ks = ks
	f.account = acc.Account
	f.ConnectToClient(nodeURL)
	return f
}
