package sequence

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// endpointDevice is a device that reports a real KVM address, like the KVMD
// client does in production.
type endpointDevice struct {
	recordingDevice
	endpoint string
}

func (d *endpointDevice) Endpoint() string { return d.endpoint }

// TestLockIdentityPrefersDeviceEndpoint is the regression test for the residual
// gap found while reviewing the first fix: canonicalizing the --target label is
// not enough, because the label is free-form and the real device address comes
// from the client configuration. Two operators can pass unrelated labels while
// pointing at one KVM.
func TestLockIdentityPrefersDeviceEndpoint(t *testing.T) {
	dev := &endpointDevice{endpoint: "https://kvm-real.local:443"}
	got := lockIdentity(dev, "some-friendly-label")
	if got != "https://kvm-real.local:443" {
		t.Fatalf("lockIdentity = %q, want the device endpoint; a free-form label must not decide the lock", got)
	}
}

func TestLockIdentityFallsBackToTarget(t *testing.T) {
	dev := &recordingDevice{} // reports no endpoint
	if got := lockIdentity(dev, "kvm-fallback.local"); got != "kvm-fallback.local" {
		t.Fatalf("lockIdentity = %q, want the target label as fallback", got)
	}

	blank := &endpointDevice{endpoint: "   "}
	if got := lockIdentity(blank, "kvm-blank.local"); got != "kvm-blank.local" {
		t.Fatalf("lockIdentity = %q, want the target label when the endpoint is blank", got)
	}
}

// TestExecuteAuthorizedSerializesDifferentLabelsOnOneDevice is the end-to-end
// proof: two workflows using completely different --target labels, but the same
// underlying KVM, must not interleave physical actions.
func TestExecuteAuthorizedSerializesDifferentLabelsOnOneDevice(t *testing.T) {
	base := t.TempDir()

	lockDir := filepath.Join(base, "locks")
	if err := os.Mkdir(lockDir, 0o777); err != nil {
		t.Fatalf("mkdir locks: %v", err)
	}
	if err := os.Chmod(lockDir, 0o777|os.ModeSticky); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Setenv(sharedLockDirEnv, lockDir)

	// Deliberately different labels for one physical device.
	labels := []string{"lab-kvm-a", "https://kvm-shared.local", "rack3-port7"}

	var mu sync.Mutex
	inside := 0
	maxInside := 0

	var wg sync.WaitGroup
	errs := make(chan error, len(labels))

	for i, label := range labels {
		storeDir := filepath.Join(base, "auth", label)
		if err := os.MkdirAll(storeDir, 0o700); err != nil {
			t.Fatalf("mkdir store: %v", err)
		}
		store := NewStore(filepath.Join(storeDir, "auth.json"))
		auth := NewAuthorizer(store, time.Now)

		plan := Plan{
			Target:      label,
			Actions:     []Action{{Type: "key", Value: "KeyA"}},
			MaxDuration: 5 * time.Second,
		}
		token, err := auth.Authorize(plan, label, true, 30*time.Second)
		if err != nil {
			t.Fatalf("Authorize %d: %v", i, err)
		}

		// All three share one endpoint: the same physical KVM.
		dev := &endpointDevice{endpoint: "https://kvm-shared.local:443"}
		dev.onAction = func() {
			mu.Lock()
			inside++
			if inside > maxInside {
				maxInside = inside
			}
			mu.Unlock()
			time.Sleep(30 * time.Millisecond)
			mu.Lock()
			inside--
			mu.Unlock()
		}

		wg.Add(1)
		go func(label, token string, plan Plan, dev Device, auth *Authorizer) {
			defer wg.Done()
			errs <- ExecuteAuthorized(context.Background(), auth, NewExecutor(), dev, token, label, plan, nil)
		}(label, token, plan, dev, auth)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("ExecuteAuthorized: %v", err)
		}
	}

	if maxInside != 1 {
		t.Fatalf("observed %d workflows driving one KVM concurrently; different labels for one device must serialize", maxInside)
	}
}
