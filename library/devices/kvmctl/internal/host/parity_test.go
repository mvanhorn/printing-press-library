package host

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// RED: parity with Python oracle — these must pass after expansion.

type scriptedRunner struct {
	outputs map[string]Result
	calls   [][]string
}

func (s *scriptedRunner) Run(_ context.Context, argv []string, _ time.Duration) (Result, error) {
	s.calls = append(s.calls, argv)
	key := strings.Join(argv, "\x00")
	if v, ok := s.outputs[key]; ok {
		return v, nil
	}
	// fallback by first element
	if len(argv) > 0 {
		switch argv[0] {
		case "hostname":
			return Result{Stdout: "pve1\n"}, nil
		case "cat":
			return Result{Stdout: "NAME=Debian\nVERSION_ID=12\nPRETTY_NAME=\"Debian GNU/Linux\"\n"}, nil
		}
	}
	return Result{}, nil
}

func join(argv []string) string { return strings.Join(argv, "\x00") }

func TestRunProbeIdentity(t *testing.T) {
	r := &scriptedRunner{outputs: map[string]Result{
		join([]string{"hostname"}):               {Code: 0, Stdout: "edge-01\n"},
		join([]string{"cat", "/etc/os-release"}): {Code: 0, Stdout: "NAME=Ubuntu\nVERSION_ID=\"24.04\"\nPRETTY_NAME=\"Ubuntu 24.04 LTS\"\n"},
	}}
	got, err := RunProbe(context.Background(), "host.identity.inspect", r, 65536, Profile{})
	if err != nil {
		t.Fatal(err)
	}
	if got["hostname"] != "edge-01" {
		t.Fatalf("hostname %v", got)
	}
}

func TestRunProbeGraphics(t *testing.T) {
	r := &scriptedRunner{outputs: map[string]Result{
		join([]string{"lspci", "-nnk"}): {Code: 0, Stdout: "00:02.0 VGA compatible controller [0300]: Intel Corporation UHD [8086:9a49]\n\tSubsystem: Example [1234:5678]\n\tKernel driver in use: i915\n\tKernel modules: i915\n"},
		join([]string{"find", "/dev/dri", "-maxdepth", "1", "-type", "c", "-printf", "%f\\n"}): {Code: 0, Stdout: "card0\nrenderD128\n"},
	}}
	got, err := RunProbe(context.Background(), "host.graphics.inspect", r, 65536, Profile{})
	if err != nil {
		t.Fatal(err)
	}
	if got["probe"] != "host.graphics.inspect" {
		t.Fatalf("probe %v", got)
	}
}

func TestRunProbeRenderAccess(t *testing.T) {
	r := &scriptedRunner{outputs: map[string]Result{
		join([]string{"systemctl", "is-active", "--quiet", "kvm-render"}): {Code: 0, Stdout: ""},
		join([]string{"test", "-r", "/dev/dri/renderD128"}):               {Code: 0, Stdout: ""},
		join([]string{"test", "-w", "/dev/dri/renderD128"}):               {Code: 1, Stdout: ""},
	}}
	got, err := RunProbe(context.Background(), "service.render_access.inspect", r, 65536, Profile{})
	if err != nil {
		t.Fatal(err)
	}
	if got["active"] != true || got["readable"] != true || got["writable"] != false {
		t.Fatalf("render %v", got)
	}
}

func TestRunProbeUnknownFails(t *testing.T) {
	r := &scriptedRunner{}
	if _, err := RunProbe(context.Background(), "host.reboot", r, 65536, Profile{}); err == nil {
		t.Fatal("expected unknown probe error")
	}
}

func TestRunProbeBoundAndSecret(t *testing.T) {
	r := &scriptedRunner{outputs: map[string]Result{
		join([]string{"hostname"}): {Code: 0, Stdout: "edge-01\npassword=secret\n"},
	}}
	if _, err := RunProbe(context.Background(), "host.identity.inspect", r, 32, Profile{}); err == nil {
		t.Fatal("expected bound/secret error")
	}
}

func TestRunProbeTimeout(t *testing.T) {
	slow := &slowRunner{}
	if _, err := RunProbe(context.Background(), "host.identity.inspect", slow, 65536, Profile{Timeout: 10 * time.Millisecond}); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout, got %v", err)
	}
}

type slowRunner struct{}

func (s *slowRunner) Run(ctx context.Context, argv []string, d time.Duration) (Result, error) {
	time.Sleep(100 * time.Millisecond)
	return Result{Stdout: "pve1\n"}, nil
}

func TestRebootConfirmationDeterministic(t *testing.T) {
	a := RebootConfirmation("pve1", "host.reboot")
	b := RebootConfirmation("pve1", "host.reboot")
	if a != b || len(a) != 64 {
		t.Fatalf("confirmation %q", a)
	}
	if RebootConfirmation("pve2", "host.reboot") == a {
		t.Fatal("different target should differ")
	}
}

func TestHostAdapterRebootRequiresConfirmationAndCheckpoint(t *testing.T) {
	j := &fakeJournal{}
	// runner that simulates disappeared on first post-reboot identity call
	calls := 0
	var r Runner = &rebootSimRunner{calls: &calls}
	adapter := NewAdapter(r, 65536, j, Profile{Timeout: 50 * time.Millisecond})
	target := "pve1"
	token := RebootConfirmation(target, "host.reboot")
	if _, err := adapter.Reboot(context.Background(), "pve1", "bad", true); err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("expected confirmation error got %v", err)
	}
	res, err := adapter.Reboot(context.Background(), target, token, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected ok %v", res)
	}
	if len(j.records) == 0 {
		t.Fatal("expected checkpoint records")
	}
	found := false
	for _, rec := range j.records {
		if rec["transition"] == "reboot_requested" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing reboot_requested checkpoint %v", j.records)
	}
}

type rebootSimRunner struct {
	calls *int
}

func (r *rebootSimRunner) Run(_ context.Context, argv []string, _ time.Duration) (Result, error) {
	*r.calls++
	switch argv[0] {
	case "hostname":
		// preflight (1), preflight in second Reboot (2), then post-reboot loop: first call fails (disappeared), second succeeds
		if *r.calls == 4 {
			return Result{}, fmt.Errorf("probe error simulated down")
		}
		return Result{Code: 0, Stdout: "pve1\n"}, nil
	case "cat":
		return Result{Code: 0, Stdout: "NAME=Debian\nVERSION_ID=12\nPRETTY_NAME=\"Debian\"\n"}, nil
	case "systemctl":
		return Result{Code: 0, Stdout: ""}, nil
	}
	return Result{}, nil
}

type fakeJournal struct {
	records []map[string]any
}

func (j *fakeJournal) Append(m map[string]any) error {
	j.records = append(j.records, m)
	return nil
}
func (j *fakeJournal) Checkpoint(operation, target, transition string, details map[string]any) error {
	rec := map[string]any{"operation": operation, "target": target, "transition": transition}
	for k, v := range details {
		rec[k] = v
	}
	j.records = append(j.records, rec)
	return nil
}
