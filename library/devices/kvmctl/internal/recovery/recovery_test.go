package recovery

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fake struct {
	n      int
	nudges int
	opened int
}

func (f *fake) Snapshot(ctx context.Context) ([]byte, error) {
	f.n++
	if f.n == 1 {
		return nil, ErrUnavailable
	}
	return []byte("frame"), nil
}
func (f *fake) Nudge(ctx context.Context) error { f.nudges++; return nil }
func (f *fake) Open(ctx context.Context) error  { f.opened++; return nil }

func TestRecoverToleratesUnavailableAndOpensStream(t *testing.T) {
	f := &fake{}
	if err := Recover(context.Background(), f, Options{Attempts: 3}); err != nil {
		t.Fatal(err)
	}
	if f.nudges != 1 || f.opened != 1 {
		t.Fatalf("nudge=%d open=%d", f.nudges, f.opened)
	}
}
func TestRecoverHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &fake{}
	if err := Recover(ctx, f, Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestRecover_ExactSequenceWithInjectedSleep(t *testing.T) {
	// Snapshot fails twice after nudge, succeeds third.
	type seq struct {
		calls  int
		nudges int
		opened int
		sleeps []time.Duration
	}
	s := &struct {
		calls  int
		nudges int
		opened int
	}{}
	// use custom fake
	call := 0
	f := &fakeNudge{
		snapshot: func(ctx context.Context) ([]byte, error) {
			call++
			if call == 1 {
				return nil, ErrUnavailable // initial probe
			}
			if call <= 3 {
				return nil, ErrUnavailable // first two retries after nudge
			}
			return []byte("ok"), nil
		},
	}
	var sleeps []time.Duration
	sleep := func(ctx context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
	err := Recover(context.Background(), f, Options{Attempts: 5, Delay: time.Second, Sleep: sleep})
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if f.nudges != 1 || f.opened != 1 {
		t.Fatalf("nudge %d open %d", f.nudges, f.opened)
	}
	// After nudge, retries: attempt0 no sleep, attempt1 sleep(delay), attempt2 sleep(delay) -> 2 sleeps before success
	if len(sleeps) != 2 {
		t.Fatalf("sleeps %v want 2", sleeps)
	}
	for _, d := range sleeps {
		if d != time.Second {
			t.Fatalf("sleep %v", d)
		}
	}
	_ = s
}

type fakeNudge struct {
	snapshot func(context.Context) ([]byte, error)
	nudges   int
	opened   int
}

func (f *fakeNudge) Snapshot(ctx context.Context) ([]byte, error) { return f.snapshot(ctx) }
func (f *fakeNudge) Nudge(ctx context.Context) error              { f.nudges++; return nil }
func (f *fakeNudge) Open(ctx context.Context) error               { f.opened++; return nil }

func TestRecover_ReturnsLastErrorWhenExhausted(t *testing.T) {
	f := &fakeNudge{snapshot: func(ctx context.Context) ([]byte, error) { return nil, ErrUnavailable }}
	f2 := &fake{}
	_ = f2
	err := Recover(context.Background(), f, Options{Attempts: 2, Delay: 0})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err %v", err)
	}
	if f.opened != 0 {
		t.Fatal("should not open on failure")
	}
}

func TestConstants(t *testing.T) {
	if DefaultNudgeFPS != 40 || DefaultNudgeQuality != 80 || StreamWSPath != "/api/ws?stream=1" {
		t.Fatalf("constants mismatch")
	}
}
