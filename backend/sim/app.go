// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package sim

import (
	"perun.network/go-perun/channel"
	"perun.network/go-perun/wallet"
)

type App struct {
	definition Address
}

var _ channel.App = new(App)

func NewApp(definition Address) *App {
	return &App{definition}
}

func (a App) Def() wallet.Address {
	return &a.definition
}

func (a App) ValidTransition(parameters *channel.Params, from, to *channel.State) (bool, error) {
	return true, nil
}
