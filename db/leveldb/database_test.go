// Copyright (c) 2020 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package leveldb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabase_PutBytes_NilArgs(t *testing.T) {
	err := new(Database).PutBytes("key", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "value")
}
