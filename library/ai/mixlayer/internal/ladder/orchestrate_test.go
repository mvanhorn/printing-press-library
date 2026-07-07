package ladder

import "testing"

func TestRungsDefaultUsesKnownModels(t *testing.T) {
	got := Rungs("")
	if len(got) == 0 {
		t.Fatal("default rungs should not be empty")
	}
	if got[0] != "qwen/qwen3.5-4b-free" {
		t.Fatalf("first default rung = %q", got[0])
	}
}

func TestRungsParsesCommaSeparatedSpec(t *testing.T) {
	got := Rungs(" qwen/a , , moonshotai/kimi-k2.7-code ")
	want := []string{"qwen/a", "moonshotai/kimi-k2.7-code"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rung[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFirstConfidentSkipsErrorsAndEmptyAnswers(t *testing.T) {
	got := FirstConfident([]Result{
		{Model: "bad", Error: "boom"},
		{Model: "empty"},
		{Model: "good", Answer: "usable"},
	})
	if got != "good" {
		t.Fatalf("FirstConfident = %q, want good", got)
	}
}
