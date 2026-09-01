package sequence

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

type workflowDevice struct{ calls []string }

func (d *workflowDevice) Key(_ context.Context, key string) error {
	d.calls = append(d.calls, "key:"+key)
	return nil
}
func (d *workflowDevice) Text(_ context.Context, text string) error {
	d.calls = append(d.calls, "text:"+text)
	return nil
}
func (d *workflowDevice) ReleaseAll(_ context.Context) error {
	d.calls = append(d.calls, "release")
	return nil
}

type workflowJournal struct{ records []map[string]any }

func (j *workflowJournal) Append(v map[string]any) error {
	j.records = append(j.records, v)
	return nil
}

func TestExecuteAuthorizedBindsAuthorizationAndJournals(t *testing.T) {
	now := time.Unix(100, 0)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	a := NewAuthorizer(NewStore(dir+"/auth.json"), func() time.Time { return now })
	p := Plan{Target: "host-a", Actions: []Action{{Type: "key", Value: "Enter"}}, MaxDuration: time.Second}
	tok, err := a.Authorize(p, "host-a", true, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	d := &workflowDevice{}
	j := &workflowJournal{}
	e := NewExecutor()
	e.now = func() time.Time { return now }
	if err := ExecuteAuthorized(context.Background(), a, e, d, tok, "host-a", p, j); err != nil {
		t.Fatal(err)
	}
	if len(j.records) != 3 || len(d.calls) != 2 {
		t.Fatalf("records=%d calls=%v", len(j.records), d.calls)
	}
	if _, err := a.Take(context.Background(), tok, "host-a", p); err == nil {
		t.Fatal("token was reusable")
	}
}

func TestExecuteAuthorizedRejectsCancelledContextBeforeDevice(t *testing.T) {
	now := time.Unix(100, 0)
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	a := NewAuthorizer(NewStore(dir+"/auth.json"), func() time.Time { return now })
	p := Plan{Target: "host-a", Actions: []Action{{Type: "key", Value: "Enter"}}, MaxDuration: time.Second}
	tok, _ := a.Authorize(p, "host-a", true, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := &workflowDevice{}
	err := ExecuteAuthorized(ctx, a, NewExecutor(), d, tok, "host-a", p, &workflowJournal{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if len(d.calls) != 0 {
		t.Fatalf("device called: %v", d.calls)
	}
}
