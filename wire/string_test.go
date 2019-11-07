// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package wire

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeString(t *testing.T) {
	assert := assert.New(t)
	rng := rand.New(rand.NewSource(0xdeadbeef))
	uint8buf, uint16buf := make([]byte, math.MaxUint8), make([]byte, math.MaxUint16)
	rng.Read(uint8buf)
	rng.Read(uint16buf[:math.MaxUint8])                                // randomize first 256 bytes
	rng.Read(uint16buf[math.MaxUint16-math.MaxUint8 : math.MaxUint16]) // randomize last 256 bytes

	t.Run("valid strings", func(t *testing.T) {
		ss := []string{"", "a", "perun", string(uint8buf), string(uint16buf)}

		for _, s := range ss {
			waitGroup := new(sync.WaitGroup)
			r, w := io.Pipe()

			waitGroup.Add(2)

			go func() {
				defer waitGroup.Done()
				defer w.Close()
				assert.NoError(encodeString(w, s))
			}()

			go func() {
				defer waitGroup.Done()
				defer r.Close()

				var d string
				assert.NoError(decodeString(r, &d))
				assert.Equal(s, d)
			}()

			waitGroup.Wait()
		}
	})

	t.Run("too long string", func(t *testing.T) {
		tooLong := string(append(uint16buf, 42))
		var buf bytes.Buffer
		assert.Error(encodeString(&buf, tooLong))
		assert.Zero(buf.Len(), "nothing should have been written to the stream")
	})

	t.Run("short stream", func(t *testing.T) {
		var buf bytes.Buffer
		binary.Write(&buf, byteOrder, uint16(16))
		buf.Write(make([]byte, 8)) // 8 bytes missing

		var d string
		assert.Error(decodeString(&buf, &d))
		assert.Zero(buf.Len(), "buffer should be exhausted")
	})
}
