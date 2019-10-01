// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package channel // import "perun.network/go-perun/channel"

import (
	"testing"

	wire "perun.network/go-perun/wire/msg"
)

func newChannelMsg() (m msg) {
	// Set a non-0 channel ID, so that we can detect serializing errors.
	for i := range m.channelID {
		m.channelID[i] = byte(i)
	}
	return
}

func TestDummyChannelMsg(t *testing.T) {
	wire.TestMsg(t, &DummyChannelMsg{newChannelMsg(), int64(-0x7172635445362718)})
}
