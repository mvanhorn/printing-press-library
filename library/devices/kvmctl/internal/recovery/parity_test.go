package recovery

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRecoverDryRunDoesNothing(t *testing.T) {
	f := &fakeNudge{snapshot: func(context.Context) ([]byte, error) { t.Fatal("should not be called"); return nil, nil }}
	if err := Recover(context.Background(), f, Options{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if f.nudges != 0 || f.opened != 0 {
		t.Fatal("dry run should not nudge/open")
	}
}

func TestRecoverPropagatesNonUnavailableError(t *testing.T) {
	want := errors.New("auth failed")
	f := &fakeNudge{snapshot: func(context.Context) ([]byte, error) { return nil, want }}
	if err := Recover(context.Background(), f, Options{}); !errors.Is(err, want) {
		t.Fatalf("got %v want %v", err, want)
	}
	if f.nudges != 0 {
		t.Fatal("should not nudge after non-retriable error")
	}
}

func TestRecoverTimeoutCancels(t *testing.T) {
	f := &fakeNudge{snapshot: func(context.Context) ([]byte, error) { return nil, ErrUnavailable }}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := Recover(ctx, f, Options{Attempts: 10, Delay: 10 * time.Millisecond, Timeout: 500 * time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		// parent timeout wins
		t.Fatalf("err %v", err)
	}
	if time.Since(start) > 200*time.Millisecond {
		t.Fatal("should cancel quickly")
	}
}
