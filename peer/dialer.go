// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package peer

import (
	"context"
)

// Dialer is an interface that allows creating a connection to a peer via its
// Perun address.
type Dialer interface {
	Dial(Address, *context.Context) (*Peer, error)
}
