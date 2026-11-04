package client

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/config"
	"testing"
	"time"
)

func TestGovernmentCatalogCredentialIsolation(t *testing.T) {
	cfg := &config.Config{BaseURL: "https://api.gsa.gov/technology/datagov/v4", AuthHeaderVal: "fixture-saved-token"}
	c := New(cfg, time.Second, 0)
	ca, err := c.WithTier("california").authForRequest(context.Background())
	if err != nil || ca.Value != "" {
		t.Fatalf("California auth must be empty: %#v %v", ca, err)
	}
	federal, err := c.authForRequest(context.Background())
	if err != nil || federal.Value != "fixture-saved-token" {
		t.Fatalf("federal must resolve saved config credential: %#v %v", federal, err)
	}
}
