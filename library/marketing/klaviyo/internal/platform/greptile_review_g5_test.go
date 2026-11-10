package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestG5ConcurrentFirstFingerprint(t *testing.T) {
	for round := 0; round < 12; round++ {
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		const callers = 64
		fingerprints := make([]string, callers)
		failures := make([]error, callers)
		start := make(chan struct{})
		var group sync.WaitGroup
		for i := range callers {
			group.Add(1)
			go func() {
				defer group.Done()
				<-start
				fingerprints[i], failures[i] = credentialFingerprint(ResolvedCredentials{"api_key": []byte("synthetic-key")})
			}()
		}
		close(start)
		group.Wait()
		for i, err := range failures {
			if err != nil {
				t.Fatalf("round %d, invocation %d: %v", round, i, err)
			}
			if len(fingerprints[i]) != 64 || fingerprints[i] != fingerprints[0] {
				t.Fatalf("round %d, invocation %d did not use the winning key", round, i)
			}
		}
		root, err := stateRoot()
		if err != nil {
			t.Fatal(err)
		}
		key, err := readFingerprintKey(filepath.Join(root, "cache-key"))
		if err != nil || len(key) != 32 {
			t.Fatalf("published key length = %d, error = %v", len(key), err)
		}
		files, err := os.ReadDir(root)
		if err != nil || len(files) != 1 || files[0].Name() != "cache-key" {
			t.Fatalf("temporary key files remain: %v, error = %v", files, err)
		}
		got, err := credentialFingerprint(ResolvedCredentials{"api_key": []byte("synthetic-key")})
		if err != nil || got != fingerprints[0] {
			t.Fatalf("a later invocation replaced the winning key: %v", err)
		}
	}
}

func TestG5InvalidFingerprintKeyStillFailsClosed(t *testing.T) {
	for _, test := range []struct {
		name string
		mode os.FileMode
		data []byte
		want string
	}{
		{"length", 0o600, nil, "invalid cache fingerprint key length"},
		{"permissions", 0o644, make([]byte, 32), "cache fingerprint key permissions must be 0600, got 0644"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			root, err := stateRoot()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(root, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "cache-key"), test.data, test.mode); err != nil {
				t.Fatal(err)
			}
			if _, err := loadOrCreateFingerprintKey(); err == nil || !strings.HasPrefix(err.Error(), test.want) {
				t.Fatalf("error = %v, want %s", err, test.want)
			}
		})
	}
}

func g5Limiter(t *testing.T) *EndpointLimiter {
	t.Helper()
	return &EndpointLimiter{
		budget: EndpointBudget{Steady: 1, Interval: time.Second, Burst: 1},
		path:   filepath.Join(t.TempDir(), "ledger.json"),
		now:    func() time.Time { return time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC) },
	}
}

func TestG5CanceledWaitReleasesLatestReservation(t *testing.T) {
	limiter := g5Limiter(t)
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	var waits []time.Duration
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		limiter.sleep = func(_ context.Context, wait time.Duration) error {
			waits = append(waits, wait)
			cancel()
			return ctx.Err()
		}
		if err := limiter.Wait(ctx); !errors.Is(err, context.Canceled) || err.Error() != "context canceled" {
			t.Fatalf("canceled wait = %v", err)
		}
		cancel()
	}
	limiter.sleep = func(_ context.Context, wait time.Duration) error {
		waits = append(waits, wait)
		return nil
	}
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(waits, []time.Duration{time.Second, time.Second, time.Second, time.Second, time.Second, time.Second}) {
		t.Fatalf("canceled requests consumed future budget: %v", waits)
	}
}

func TestG5AlreadyCanceledWaitDoesNotConsumeBurst(t *testing.T) {
	limiter := g5Limiter(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := limiter.Wait(ctx); err == nil || err.Error() != "context canceled" {
		t.Fatalf("already canceled wait = %v", err)
	}
	limiter.sleep = func(_ context.Context, wait time.Duration) error {
		t.Errorf("already canceled request consumed burst: wait = %s", wait)
		return nil
	}
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestG5CanceledWaitPreservesLaterReservation(t *testing.T) {
	first := g5Limiter(t)
	second := &EndpointLimiter{budget: first.budget, path: first.path, now: first.now}
	if err := first.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	second.sleep = func(_ context.Context, wait time.Duration) error {
		if wait != 2*time.Second {
			t.Errorf("dependent reservation wait = %s", wait)
		}
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first.sleep = func(_ context.Context, _ time.Duration) error {
		if err := second.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
		cancel()
		return ctx.Err()
	}
	if err := first.Wait(ctx); err == nil || err.Error() != "context canceled" {
		t.Fatalf("canceled wait = %v", err)
	}
	second.sleep = func(_ context.Context, wait time.Duration) error {
		if wait != 3*time.Second {
			t.Errorf("later reservation was overwritten: wait = %s", wait)
		}
		return nil
	}
	if err := second.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
}
