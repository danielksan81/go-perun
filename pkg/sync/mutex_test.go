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

// TestTryLockCtx_Nil tests that TryLock() can lock an empty mutex, and that
// locked mutexes cannot be locked again.
func TestTryLockCtx_Nil(t *testing.T) {
	t.Parallel()

	var m Mutex
	// Try instant lock without context.
	assert.True(t, m.TryLock(), "TryLock on new mutex must succeed")
	assert.False(t, m.TryLock(), "TryLock() on locked mutex must fail")
}

// TestTryLockCtx_DoneContext tests that a cancelled context can never be used to
// acquire the mutex.
func TestTryLockCtx_DoneContext(t *testing.T) {
	t.Parallel()

	var m Mutex
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Try often because of random `select` case choices.
	for i := 0; i < 256; i++ {
		assert.False(t, m.TryLockCtx(ctx), "TryLockCtx on closed context must fail")
	}
}

// TestTryLockCtx_WithTimeout tests that the context's timeout is properly adhered to.
func TestTryLockCtx_WithTimeout(t *testing.T) {
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
		done <- m.TryLockCtx(ctx)
	}()

	select {
	case <-time.NewTimer(1000 * time.Millisecond).C:
		t.Error("TryLockCtx should have returned")
	case success := <-done:
		assert.True(t, success, "TryLockCtx should have succeeded")
	}
}

// TestTryLockCtx_WithTimeout_Fail tests that TryLockCtx fails if it times out.
func TestTryLockCtx_WithTimeout_Fail(t *testing.T) {
	t.Parallel()

	var m Mutex
	m.Lock()
	// Try delayed lock with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 1000*time.Millisecond)
	defer cancel()
	done := make(chan bool, 1)
	go func() {
		done <- m.TryLockCtx(ctx)
	}()

	// Check that it does not return early.
	select {
	case <-time.NewTimer(500 * time.Millisecond).C:
	case success := <-done:
		t.Errorf("TryLockCtx should not have returned, but returned %t", success)
	}

	// Check that it fails.
	select {
	case <-time.NewTimer(1000 * time.Millisecond).C:
	case success := <-done:
		assert.False(t, success, "TryLockCtx should have timed out")
	}
}

// TestUnlock tests that unlocking a locked mutex will make it lockable again.
func TestUnlock(t *testing.T) {
	t.Parallel()

	var m Mutex
	m.Lock()
	m.Unlock()
	assert.True(t, m.TryLock(), "Unlock must make the next TryLock succeed")
}
