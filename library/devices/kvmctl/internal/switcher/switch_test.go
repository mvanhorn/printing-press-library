package switcher

import (
	"context"
	"testing"
	"time"
)

type fakeHID struct {
	events []string
}

func (f *fakeHID) KeyDown(ctx context.Context, k string) error {
	f.events = append(f.events, k+":down")
	return nil
}
func (f *fakeHID) KeyUp(ctx context.Context, k string) error {
	f.events = append(f.events, k+":up")
	return nil
}

type fakeOTG struct {
	calls []map[string]string
}

func (f *fakeOTG) Request(ctx context.Context, method, path string, params map[string]string) error {
	f.calls = append(f.calls, params)
	return nil
}

func TestPlanTH413IsDiscreteAndOrdered(t *testing.T) {
	got, err := Plan(TH413, 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []Event{{"ControlRight", "down"}, {"ControlRight", "up"}, {"ControlRight", "down"}, {"ControlRight", "up"}, {"Digit3", "down"}, {"Digit3", "up"}, {"Enter", "down"}, {"Enter", "up"}}
	if len(got) != len(want) {
		t.Fatalf("got %d events", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestPlanRejectsOutOfRangePort(t *testing.T) {
	if _, err := Plan(TH413, 5); err == nil {
		t.Fatal("expected port validation error")
	}
}

func TestRearm_ExactSleepSequence(t *testing.T) {
	otg := &fakeOTG{}
	var sleeps []time.Duration
	sleep := func(ctx context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
	if err := Rearm(context.Background(), otg, sleep); err != nil {
		t.Fatal(err)
	}
	if len(otg.calls) != 2 || otg.calls[0]["start_cdrom"] != "true" || otg.calls[1]["start_cdrom"] != "false" {
		t.Fatalf("otg calls %v", otg.calls)
	}
	if len(sleeps) != 2 || sleeps[0] != OTGOnWait || sleeps[1] != OTGOffWait {
		t.Fatalf("sleeps %v want [%v %v]", sleeps, OTGOnWait, OTGOffWait)
	}
	if OTGOnWait != 8*time.Second || OTGOffWait != 12*time.Second {
		t.Fatalf("constants wrong")
	}
}

func TestExecute_HeldKeySleepSequence(t *testing.T) {
	hid := &fakeHID{}
	var sleeps []time.Duration
	sleep := func(ctx context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
	_, err := Execute(context.Background(), hid, TH413Held, 2, sleep)
	if err != nil {
		t.Fatal(err)
	}
	// 4 taps: each hold+gap = 8 sleeps, plus settle =9
	if len(sleeps) != 9 {
		t.Fatalf("sleeps %v", sleeps)
	}
	for i := 0; i < 8; i += 2 {
		if sleeps[i] != HoldMS || sleeps[i+1] != GapMS {
			t.Fatalf("tap %d sleeps %v", i/2, sleeps[i:i+2])
		}
	}
	if sleeps[8] != TH413Held.SettleDelay {
		t.Fatalf("settle %v", sleeps[8])
	}
	want := []string{"ControlRight:down", "ControlRight:up", "ControlRight:down", "ControlRight:up", "Digit2:down", "Digit2:up", "Enter:down", "Enter:up"}
	for i, w := range want {
		if hid.events[i] != w {
			t.Fatalf("event %d %q", i, hid.events[i])
		}
	}
}

func TestExecute_LegacyTapGapBefore(t *testing.T) {
	hid := &fakeHID{}
	var sleeps []time.Duration
	sleep := func(ctx context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
	p := Profile{Name: "legacy", MinPort: 1, MaxPort: 4, Sequence: [][2]string{{"ControlRight", "tap"}, {"Enter", "tap"}}, InterKeyDelay: 200 * time.Millisecond, SettleDelay: time.Second}
	_, err := Execute(context.Background(), hid, p, 1, sleep)
	if err != nil {
		t.Fatal(err)
	}
	// events: down up down up => gaps before events 1,2,3 =3 gaps + settle =4
	if len(sleeps) != 4 {
		t.Fatalf("sleeps %v", sleeps)
	}
	for i := 0; i < 3; i++ {
		if sleeps[i] != 200*time.Millisecond {
			t.Fatalf("gap %d %v", i, sleeps[i])
		}
	}
	if sleeps[3] != time.Second {
		t.Fatalf("settle %v", sleeps[3])
	}
}

func TestExecute_HonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	hid := &fakeHID{}
	if _, err := Execute(ctx, hid, TH413Held, 1, nil); err == nil {
		t.Fatal("expected canceled")
	}
}
