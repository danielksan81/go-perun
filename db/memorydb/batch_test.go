// Copyright (c) 2019 Chair of Applied Cryptography, Technische Universität
// Darmstadt, Germany. All rights reserved. This file is part of go-perun. Use
// of this source code is governed by a MIT-style license that can be found in
// the LICENSE file.

package memorydb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"perun.network/go-perun/db/test"
)

func TestBatch(t *testing.T) {
	t.Run("Generic Batch test", func(t *testing.T) {
		test.GenericBatchTest(t, NewDatabase())
	})
	return
}

func TestBatch_PutBytes_NilArgs(t *testing.T) {
	err := new(Batch).PutBytes("key", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "value")
}
