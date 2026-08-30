package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

func TestTockCancelErrorCategory(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "outcome unknown forbids network retry", err: fmt.Errorf("wrapped: %w", tock.ErrCancelOutcomeUnknown), want: "outcome_unknown"},
		{name: "definite UI failure", err: tock.ErrCancelUIFlow, want: "cancel_ui_failed"},
		{name: "past window", err: tock.ErrPastCancellationWindow, want: "past_cancellation_window"},
		{name: "canary", err: tock.ErrCanaryUnrecognizedBody, want: "discriminator_drift"},
		{name: "ordinary transport", err: errors.New("transport failed"), want: "network_error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tockCancelErrorCategory(tc.err); got != tc.want {
				t.Fatalf("tockCancelErrorCategory(%v)=%q, want %q", tc.err, got, tc.want)
			}
		})
	}
}
