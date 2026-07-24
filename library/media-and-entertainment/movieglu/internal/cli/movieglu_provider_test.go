// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movieglu/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movieglu/internal/config"
)

func TestConfigureMovieGluClient(t *testing.T) {
	t.Setenv("MOVIEGLU_CLIENT", "evaluation-user")
	t.Setenv("MOVIEGLU_AUTHORIZATION", "Basic abc123")
	t.Setenv("MOVIEGLU_TERRITORY", "us")
	t.Setenv("MOVIEGLU_GEOLOCATION", "40.7128;-74.0060")

	c := client.New(&config.Config{}, 0, 0)
	if err := configureMovieGluClient(c); err != nil {
		t.Fatalf("configureMovieGluClient() error = %v", err)
	}
	want := map[string]string{
		"client":        "evaluation-user",
		"authorization": "Basic abc123",
		"territory":     "US",
		"api-version":   "v200",
		"geolocation":   "40.7128;-74.0060",
	}
	for key, value := range want {
		if got := c.Config.Headers[key]; got != value {
			t.Errorf("header %q = %q, want %q", key, got, value)
		}
	}
	if got := c.Config.Headers["device-datetime"]; !strings.Contains(got, "T") {
		t.Errorf("device-datetime = %q, want ISO local datetime", got)
	}
}

func TestConfigureMovieGluClientRejectsMissingProviderCredentials(t *testing.T) {
	t.Setenv("MOVIEGLU_CLIENT", "")
	t.Setenv("MOVIEGLU_AUTHORIZATION", "")
	t.Setenv("MOVIEGLU_TERRITORY", "")

	c := client.New(&config.Config{}, 0, 0)
	err := configureMovieGluClient(c)
	if err == nil || !strings.Contains(err.Error(), "is required") {
		t.Fatalf("configureMovieGluClient() error = %v, want missing credential error", err)
	}
}
