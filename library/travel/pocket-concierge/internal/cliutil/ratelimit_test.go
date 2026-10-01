package cliutil

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestLimiterCancellationDoesNotBlockNextRead(t *testing.T) {
	l := NewAdaptiveLimiter(100)
	if e := l.Wait(context.Background()); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := l.Wait(ctx); e == nil {
		t.Fatal("expected canceled pacing")
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	if e := l.Wait(ctx2); e != nil {
		t.Fatal(e)
	}
}
func TestRetryAfterUnits(t *testing.T) {
	for _, v := range []string{"1", "120"} {
		r := &http.Response{Header: http.Header{}}
		r.Header.Set("Retry-After", v)
		if RetryAfter(r) < time.Second {
			t.Fatal("lost seconds")
		}
	}
}
func TestLimiterNilIsSafe(t *testing.T) {
	var l *AdaptiveLimiter
	if e := l.Wait(context.Background()); e != nil {
		t.Fatal(e)
	}
	l.OnSuccess()
	l.OnRateLimit()
}
