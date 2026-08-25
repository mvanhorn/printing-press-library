package june

import (
	"context"
	"testing"
	"time"
)

func TestTempConversions(t *testing.T) {
	cases := []struct {
		f      float64
		milliC int
	}{{350, 176667}, {375, 190556}, {450, 232222}}
	for _, c := range cases {
		if got := FahrenheitToMilliC(c.f); got != c.milliC {
			t.Errorf("FahrenheitToMilliC(%v)=%d want %d", c.f, got, c.milliC)
		}
	}
	if got := MilliCToFahrenheit(176667); got != 350 {
		t.Errorf("MilliCToFahrenheit(176667)=%d want 350", got)
	}
	if got := MilliCToCelsius(52000); got != 52 {
		t.Errorf("MilliCToCelsius(52000)=%d want 52", got)
	}
}

func openTestStore(t *testing.T) *CookStore {
	t.Helper()
	t.Setenv("JUNEOVEN_DATA_DIR", t.TempDir())
	cs, err := OpenCookStore(context.Background())
	if err != nil {
		t.Fatalf("open cook store: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func TestSessionRoundTrip(t *testing.T) {
	cs := openTestStore(t)
	ctx := context.Background()
	start := time.Unix(1700000000, 0)
	id, err := cs.StartSession(ctx, "sourdough", "bake", 450, start)
	if err != nil {
		t.Fatalf("start session: %v", err)
	}
	// Samples: climb from 100 to 460 (crosses target 450).
	for i, f := range []int{100, 200, 300, 400, 460} {
		if err := cs.AppendSample(ctx, id, start.Add(time.Duration(i)*time.Minute), f, i*20); err != nil {
			t.Fatalf("append sample: %v", err)
		}
	}
	if err := cs.EndSession(ctx, id, "completed", start.Add(30*time.Minute)); err != nil {
		t.Fatalf("end session: %v", err)
	}

	sessions, err := cs.ListSessions(ctx, 10, 0)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("list sessions: got %d err %v", len(sessions), err)
	}
	if s := sessions[0]; s.Outcome != "completed" || s.TargetF != 450 || s.DurationMin != 30 {
		t.Errorf("session fields wrong: %+v", s)
	}

	samples, err := cs.SessionSamples(ctx, id)
	if err != nil || len(samples) != 5 {
		t.Fatalf("samples: got %d err %v", len(samples), err)
	}

	stats, err := cs.PreheatStats(ctx, "")
	if err != nil || len(stats) != 1 {
		t.Fatalf("preheat stats: got %d err %v", len(stats), err)
	}
	// Reaches 450 at minute 4 (240s) after the first sample.
	if stats[0].MedianSeconds != 240 {
		t.Errorf("median seconds to target = %v want 240", stats[0].MedianSeconds)
	}
}

func TestPresetRoundTrip(t *testing.T) {
	cs := openTestStore(t)
	ctx := context.Background()
	if err := cs.SavePreset(ctx, Preset{Name: "roast", Mode: "roast", TargetF: 375, TimerMin: 90}); err != nil {
		t.Fatalf("save preset: %v", err)
	}
	// Upsert with a changed target.
	if err := cs.SavePreset(ctx, Preset{Name: "roast", Mode: "roast", TargetF: 400, TimerMin: 60}); err != nil {
		t.Fatalf("upsert preset: %v", err)
	}
	p, ok, err := cs.GetPreset(ctx, "roast")
	if err != nil || !ok {
		t.Fatalf("get preset: ok=%v err=%v", ok, err)
	}
	if p.TargetF != 400 || p.TimerMin != 60 {
		t.Errorf("preset not upserted: %+v", p)
	}
	if _, ok, _ := cs.GetPreset(ctx, "missing"); ok {
		t.Errorf("missing preset should not be found")
	}
	all, err := cs.ListPresets(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("list presets: got %d err %v", len(all), err)
	}
}
