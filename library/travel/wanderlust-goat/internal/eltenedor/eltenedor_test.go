package eltenedor

import (
	"context"
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
)

func TestStub(t *testing.T) {
	c := NewClient()
	if c.Slug() != "eltenedor" {
		t.Errorf("Slug = %q, want eltenedor", c.Slug())
	}
	if c.Locale() != "es" {
		t.Errorf("Locale = %q, want es", c.Locale())
	}
	if !c.IsStub() {
		t.Error("IsStub should be true")
	}
	var _ sourcetypes.Client = c
	_, err := c.LookupByName(context.Background(), "x", "y", 5)
	if !errors.Is(err, sourcetypes.ErrNotImplemented) {
		t.Errorf("expected ErrNotImplemented, got %v", err)
	}
	if sourcetypes.StubReason(c) == "" {
		t.Error("StubReason should be non-empty")
	}
}
