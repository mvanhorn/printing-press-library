package sequence

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

type fakeKVMD struct {
	mu          sync.Mutex
	events      []string
	failRelease bool
}

func (f *fakeKVMD) KVMDKey(_ context.Context, key string, down bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, key+":"+map[bool]string{true: "down", false: "up"}[down])
	if !down && f.failRelease && key == "Held" {
		return errors.New("release failed")
	}
	return nil
}
func TestKVMDDeviceReleasesHeldKeyAfterReleaseFailure(t *testing.T) {
	f := &fakeKVMD{failRelease: true}
	d := NewKVMDDevice(f)
	if err := d.Key(context.Background(), "Held"); err == nil {
		t.Fatal("expected release failure")
	}
	if err := d.ReleaseAll(context.Background()); err == nil {
		t.Fatal("expected cleanup attempt")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.events) != 3 {
		t.Fatalf("events=%v", f.events)
	}
}

type advancingDevice struct {
	workflowDevice
	now *time.Time
}

func (d *advancingDevice) Key(ctx context.Context, key string) error {
	err := d.workflowDevice.Key(ctx, key)
	*d.now = d.now.Add(3 * time.Second)
	return err
}
func TestExecuteAuthorizedChecksExpiryBetweenActionsAndCleansUp(t *testing.T) {
	now := time.Unix(100, 0)
	dir := t.TempDir()
	_ = os.Chmod(dir, 0700)
	a := NewAuthorizer(NewStore(dir+"/auth.json"), func() time.Time { return now })
	p := Plan{Target: "host", Actions: []Action{{Type: "key", Value: "A"}, {Type: "key", Value: "B"}}, MaxDuration: 10 * time.Second}
	tok, err := a.Authorize(p, "host", true, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	d := &advancingDevice{now: &now}
	e := NewExecutor()
	e.now = func() time.Time { return now }
	j := &workflowJournal{}
	if err := ExecuteAuthorized(context.Background(), a, e, d, tok, "host", p, j); err == nil || err.Error() != "authorization expired" {
		t.Fatalf("err=%v", err)
	}
	if len(d.calls) != 2 {
		t.Fatalf("calls=%v", d.calls)
	}
}
