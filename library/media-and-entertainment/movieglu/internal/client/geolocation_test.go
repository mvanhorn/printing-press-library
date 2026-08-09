// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"errors"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movieglu/internal/config"
)

func TestValidateMovieGluRequestHeadersScopesGeolocationToLiveEndpointPaths(t *testing.T) {
	configured := func(geolocation string) *config.Config {
		return &config.Config{
			MoviegluCredentials: "api-key",
			Headers: map[string]string{
				"client": "evaluation-user", "authorization": "Basic abc123", "territory": "US", "geolocation": geolocation,
			},
		}
	}
	for _, path := range []string{"/cinemasNearby/", "/filmShowTimes/", "/closestShowing/"} {
		err := validateMovieGluRequestHeaders(path, configured(""), true)
		if err == nil || !strings.Contains(err.Error(), "MOVIEGLU_GEOLOCATION is required") {
			t.Fatalf("%s missing-geolocation error = %v", path, err)
		}
		if err := validateMovieGluRequestHeaders(path, configured("40.7128;-74.0060"), true); err != nil {
			t.Fatalf("%s with geolocation rejected: %v", path, err)
		}
	}
	for _, path := range []string{"/filmsNowShowing/", "/cinemaShowTimes/", "/purchaseConfirmation/"} {
		if err := validateMovieGluRequestHeaders(path, configured(""), true); err != nil {
			t.Fatalf("location-independent %s rejected: %v", path, err)
		}
	}
}

func TestValidateMovieGluRequestHeadersClassifiesMissingLiveConfiguration(t *testing.T) {
	err := validateMovieGluRequestHeaders("/filmsNowShowing/", &config.Config{}, true)
	if !IsLocalConfigurationError(err) {
		t.Fatalf("missing credentials error = %v, want LocalConfigurationError", err)
	}
}

func TestValidateCachedRequestAuthPreservesPlaceholderIdentityAndLocalClassification(t *testing.T) {
	c := New(&config.Config{MoviegluCredentials: "<your-token>"}, 0, 0)
	err := c.validateCachedRequestAuth(t.Context())
	if !IsLocalConfigurationError(err) {
		t.Fatalf("placeholder error = %v, want LocalConfigurationError", err)
	}
	if !errors.Is(err, ErrPlaceholderCredential) {
		t.Fatalf("placeholder error = %v, want errors.Is(ErrPlaceholderCredential)", err)
	}
}
