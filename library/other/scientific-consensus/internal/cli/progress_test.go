// Hand-authored tests for the progress enable/disable gate. Not generated.
package cli

import "testing"

func TestProgressAllowedByFlags(t *testing.T) {
	tests := []struct {
		name  string
		flags *rootFlags
		want  bool
	}{
		{"nil", nil, false},
		{"plain interactive defaults", &rootFlags{}, true},
		{"json off", &rootFlags{asJSON: true}, false},
		{"agent off", &rootFlags{agent: true}, false},
		{"compact off", &rootFlags{compact: true}, false},
		{"csv off", &rootFlags{csv: true}, false},
		{"quiet off", &rootFlags{quiet: true}, false},
		{"plain off", &rootFlags{plain: true}, false},
		{"select off", &rootFlags{selectFields: "id,title"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := progressAllowedByFlags(tt.flags); got != tt.want {
				t.Errorf("progressAllowedByFlags(%+v) = %v, want %v", tt.flags, got, tt.want)
			}
		})
	}
}

// TestProgressEnabledNonTTY asserts the full gate is disabled when stderr is not
// a terminal (always true under `go test`, whose stderr is a pipe) — even when
// the flags alone would allow a progress line. This covers the non-TTY case;
// the flag gate itself is covered by TestProgressAllowedByFlags.
func TestProgressEnabledNonTTY(t *testing.T) {
	if progressEnabled(&rootFlags{}) {
		t.Error("progressEnabled = true under non-TTY stderr, want false")
	}
	// A nil/disabled progress must be safe to drive.
	p := newProgress(&rootFlags{}, "x", 3)
	if p.enabled {
		t.Error("newProgress.enabled = true under non-TTY stderr, want false")
	}
	p.update(1) // must not panic or write
	p.done()
	var nilp *progress
	nilp.update(1) // nil-safe
	nilp.done()
}
