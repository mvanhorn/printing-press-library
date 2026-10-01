// Copyright 2026 rowdy and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/devonthink/internal/config"
)

func TestDoLocalDEVONthinkBlockedMutationReturnsError(t *testing.T) {
	c := New(&config.Config{BaseURL: "local"}, 0, 0)

	data, status, err := c.doLocalDEVONthink(
		context.Background(),
		http.MethodPost,
		"/batch/apply",
		map[string]string{"plan": "reviewed-plan.json"},
		map[string]any{"yes": true},
		nil,
		false,
	)
	if err == nil {
		t.Fatalf("blocked batch apply returned nil error: status=%d body=%s", status, data)
	}
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", status, http.StatusForbidden)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("APIError status = %d, want %d", apiErr.StatusCode, http.StatusForbidden)
	}
}
