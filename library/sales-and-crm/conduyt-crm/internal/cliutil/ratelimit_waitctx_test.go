// PATCH(retry-policy): tests for WaitContext; see .printing-press-patches.json.

package cliutil

import (
	"context"
	"testing"
	"time"
)

func TestWaitContextReleasesReservationOnCancel(t *testing.T) {
	l := NewAdaptiveLimiter(1)   // one request per second
	l.Wait(context.Background()) // consume the free slot
	before := l.lastRequest
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := l.WaitContext(ctx); err == nil {
		t.Fatal("want the context error")
	}
	l.mu.Lock()
	after := l.lastRequest
	l.mu.Unlock()
	if !after.Equal(before) {
		t.Fatalf("cancelled wait left a reservation: before=%v after=%v", before, after)
	}
}

func TestWaitContextAlreadyCancelledFreeSlotKeepsNoReservation(t *testing.T) {
	l := NewAdaptiveLimiter(1000) // slot is free immediately
	time.Sleep(5 * time.Millisecond)
	l.mu.Lock()
	before := l.lastRequest
	l.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.WaitContext(ctx); err == nil {
		t.Fatal("want the context error")
	}
	l.mu.Lock()
	after := l.lastRequest
	l.mu.Unlock()
	if !after.Equal(before) {
		t.Fatalf("cancelled free-slot wait kept a reservation: before=%v after=%v", before, after)
	}
}

func TestWaitContextCancelledBeforeReservingLeavesNoSlot(t *testing.T) {
	l := NewAdaptiveLimiter(1)
	l.Wait(context.Background())
	l.mu.Lock()
	before := l.lastRequest
	l.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.WaitContext(ctx); err == nil {
		t.Fatal("want the context error")
	}
	l.mu.Lock()
	after := l.lastRequest
	l.mu.Unlock()
	if !after.Equal(before) {
		t.Fatalf("cancelled call reserved a slot: before=%v after=%v", before, after)
	}
}
