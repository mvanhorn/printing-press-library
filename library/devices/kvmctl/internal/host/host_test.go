package host

import (
	"context"
	"testing"
	"time"
)

type fakeRunner struct{ calls [][]string }

func (f *fakeRunner) Run(_ context.Context, a []string, _ time.Duration) (Result, error) {
	f.calls = append(f.calls, a)
	switch a[0] {
	case "hostname":
		return Result{Stdout: "pve1\n"}, nil
	case "cat":
		return Result{Stdout: "NAME=Debian\nVERSION_ID=12\nPRETTY_NAME=\"Debian GNU/Linux\"\n"}, nil
	}
	return Result{}, nil
}
func TestProbeUsesAllowlistedArgv(t *testing.T) {
	f := &fakeRunner{}
	got, err := Probe(context.Background(), f, Profile{})
	if err != nil {
		t.Fatal(err)
	}
	if got["hostname"] != "pve1" {
		t.Fatal(got)
	}
	if len(f.calls) != 2 || f.calls[1][0] != "cat" {
		t.Fatalf("calls=%v", f.calls)
	}
}
func TestRebootRequiresYesAndBindsTarget(t *testing.T) {
	f := &fakeRunner{}
	if _, err := Reboot(context.Background(), f, Profile{}, "pve1", false); err == nil {
		t.Fatal("expected confirmation")
	}
	if _, err := Reboot(context.Background(), f, Profile{}, "pve1", true); err != nil {
		t.Fatal(err)
	}
	last := f.calls[len(f.calls)-1]
	if len(last) != 3 || last[2] != "--yes" {
		t.Fatalf("argv=%v", last)
	}
}
