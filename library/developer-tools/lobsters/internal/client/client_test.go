// Copyright 2026 hiten-shah. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/lobsters/internal/config"
)

func TestCacheKeySeparatesQueryParamPairs(t *testing.T) {
	c := &Client{
		BaseURL: "https://lobste.rs",
		Config:  &config.Config{BaseURL: "https://lobste.rs"},
	}

	first := c.cacheKey("/search", map[string]string{"a": "bc", "d": "e"})
	second := c.cacheKey("/search", map[string]string{"a": "b", "cd": "e"})

	if first == second {
		t.Fatalf("cache keys collided: %s", first)
	}
}
