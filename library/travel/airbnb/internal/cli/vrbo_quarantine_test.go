package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/airbnb/internal/source/vrbo"
)

// TestVRBOSearchReturnsDisabled confirms the source layer no longer fabricates
// fallback results. Pre-quarantine, vrbo.Search returned six hardcoded Tahoe
// listings stamped with the queried city when the upstream Akamai check fired.
func TestVRBOSearchReturnsDisabled(t *testing.T) {
	props, _, err := vrbo.Search(context.Background(), vrbo.SearchParams{Location: "San Francisco"})
	if !errors.Is(err, vrbo.ErrDisabled) {
		t.Fatalf("expected vrbo.ErrDisabled, got: %v", err)
	}
	if len(props) != 0 {
		t.Fatalf("expected no properties, got %d", len(props))
	}
}

func TestVRBOGetReturnsDisabled(t *testing.T) {
	prop, err := vrbo.Get(context.Background(), "9076001848", vrbo.GetParams{})
	if !errors.Is(err, vrbo.ErrDisabled) {
		t.Fatalf("expected vrbo.ErrDisabled, got: %v", err)
	}
	if prop != nil {
		t.Fatalf("expected nil property, got %v", prop)
	}
}

func TestVRBOIsDisabledHelper(t *testing.T) {
	if !vrbo.IsDisabled(vrbo.ErrDisabled) {
		t.Fatal("IsDisabled(ErrDisabled) should be true")
	}
	if vrbo.IsDisabled(errors.New("other")) {
		t.Fatal("IsDisabled should reject unrelated errors")
	}
}
