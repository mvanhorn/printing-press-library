// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movieglu/internal/config"
)

func TestValidateMovieGluRequestHeadersScopesGeolocationToLiveEndpointPaths(t *testing.T) {
	for _, path := range []string{"/cinemasNearby/", "/filmShowTimes/", "/closestShowing/"} {
		err := validateMovieGluRequestHeaders(path, &config.Config{})
		if err == nil || !strings.Contains(err.Error(), "MOVIEGLU_GEOLOCATION is required") {
			t.Fatalf("%s missing-geolocation error = %v", path, err)
		}
		if err := validateMovieGluRequestHeaders(path, &config.Config{Headers: map[string]string{"geolocation": "40.7128;-74.0060"}}); err != nil {
			t.Fatalf("%s with geolocation rejected: %v", path, err)
		}
	}
	for _, path := range []string{"/filmsNowShowing/", "/cinemaShowTimes/", "/purchaseConfirmation/"} {
		if err := validateMovieGluRequestHeaders(path, &config.Config{}); err != nil {
			t.Fatalf("location-independent %s rejected: %v", path, err)
		}
	}
}
