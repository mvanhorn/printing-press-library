// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Regression: drift must reject analysis spans under 2 years (the midpoint
// split needs a non-empty year on each side) and must reject missing bounds.
func TestValidateDriftSpan(t *testing.T) {
	tests := []struct {
		name    string
		from    int
		to      int
		wantErr bool
	}{
		{"both missing", 0, 0, true},
		{"to missing", 2020, 0, true},
		{"from missing", 0, 2024, true},
		{"span 0 rejected", 2020, 2020, true},
		{"span 1 rejected", 2020, 2021, true},
		{"span 2 accepted", 2020, 2022, false},
		{"wide span accepted", 2015, 2025, false},
		{"decade span accepted", 2020, 2030, false},
		{"inverted span rejected", 2025, 2015, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDriftSpan(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateDriftSpan(%d, %d) error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
			}
			if err == nil {
				return
			}
			// The rejection must be a usage error (exit code 2), consistent
			// with the CLI's other flag-validation failures.
			var ce *cliError
			if !errors.As(err, &ce) || ce.code != 2 {
				t.Errorf("span rejection should be a usage error (code 2), got %v", err)
			}
			if !strings.Contains(err.Error(), "at least 2 years") {
				t.Errorf("error message should explain the 2-year minimum, got %q", err.Error())
			}
			// The message must echo the rejected bounds so the user can see
			// which values were too close together.
			if want := fmt.Sprintf("--from %d --to %d", tt.from, tt.to); !strings.Contains(err.Error(), want) {
				t.Errorf("error message should contain %q, got %q", want, err.Error())
			}
		})
	}
}
