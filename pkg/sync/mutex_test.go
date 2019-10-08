// Copyright (c) 2019 The Perun Authors. All rights reserved.
// This file is part of go-perun. Use of this source code is governed by a
// MIT-style license that can be found in the LICENSE file.

package sync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestLock tests that an empty mutex can be locked.
func TestLock(t *testing.T) {
	t.Parallel()

	var m Mutex

	done := make(chan struct{}, 1)
	go func() {
		m.Lock()
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.NewTimer(500 * time.Millisecond).C:
		t.Error("lock on new mutex did not instantly succeed")
	}
}

// TestTryLock_Nil tests that TryLock(nil) can lock an empty mutex, and that
// locked mutexes cannot be locked again.
func TestTryLock_Nil(t *testing.T) {
	t.Parallel()

	var m Mutex
	// Try instant lock without context.
	assert.True(t, m.TryLock(nil), "TryLock on new mutex must succeed")
	assert.False(t, m.TryLock(nil), "TryLock(nil) on locked mutex must fail")
}

// TestTryLock_WithTimeout tests that the context's timeout is properly adhered to.
func TestTryLock_WithTimeout(t *testing.T) {
	t.Parallel()

	var m Mutex
	m.Lock()
	// Try delayed lock with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()

	go func() {
		<-time.NewTimer(500 * time.Millisecond).C
		m.Unlock()
	}()
	done := make(chan bool, 1)
	go func() {
		done <- m.TryLock(ctx)
	}()

	select {
	case <-time.NewTimer(1000 * time.Millisecond).C:
		t.Error("TryLock should have returned")
	case success := <-done:
		assert.True(t, success, "TryLock should have succeeded")
	}
}

// TestTryLock_WithTimeout_Fail tests that TryLock fails if it times out.
func TestTryLock_WithTimeout_Fail(t *testing.T) {
	t.Parallel()

	var m Mutex
	m.Lock()
	// Try delayed lock with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()
	done := make(chan bool, 1)
	go func() {
		done <- m.TryLock(ctx)
	}()

	// Check that it does not return early.
	select {
	case <-time.NewTimer(500 * time.Millisecond).C:
	case success := <-done:
		t.Errorf("TryLock should not have returned, but returned %t", success)
	}

	// Check that it fails.
	select {
	case <-time.NewTimer(1000 * time.Millisecond).C:
	case success := <-done:
		assert.False(t, success, "TryLock should have timed out")
	}
}

// TestUnlock tests that unlocking a locked mutex will make it lockable again.
func TestUnlock(t *testing.T) {
	t.Parallel()

	var m Mutex
	m.Lock()
	m.Unlock()
	assert.True(t, m.TryLock(nil), "Unlock must make the next TryLock succeed")
}
