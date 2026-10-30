package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/client"
)

func TestListingBlockedHint(t *testing.T) {
	blocked := fmt.Errorf("fetching: %w", &client.APIError{Method: "GET", Path: "/x", StatusCode: 403})
	hinted := listingBlockedHint(blocked)
	if !strings.Contains(hinted.Error(), "auth import-file") {
		t.Fatalf("403 error has no hint: %v", hinted)
	}
	if !listingBlocked(hinted) {
		t.Fatal("hinted error no longer reads as a 403, so comps would drop the hint")
	}

	other := &client.APIError{Method: "GET", Path: "/x", StatusCode: 500}
	if got := listingBlockedHint(other); got != error(other) {
		t.Fatalf("non-403 changed: %v", got)
	}
	plain := errors.New("boom")
	if got := listingBlockedHint(plain); got != plain {
		t.Fatalf("plain error changed: %v", got)
	}
}
