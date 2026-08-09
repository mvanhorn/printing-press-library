// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/movieglu/internal/client"
)

func init() {
	registerClientHook(configureMovieGluClient)
}

func configureMovieGluClient(c *client.Client) error {
	if c.IsDryRun() || os.Getenv("PRINTING_PRESS_VERIFY") == "1" {
		return nil
	}
	c.EnableMovieGluHeaderValidation()
	provider := map[string]string{
		"MOVIEGLU_CLIENT":        strings.TrimSpace(os.Getenv("MOVIEGLU_CLIENT")),
		"MOVIEGLU_AUTHORIZATION": strings.TrimSpace(os.Getenv("MOVIEGLU_AUTHORIZATION")),
		"MOVIEGLU_TERRITORY":     strings.ToUpper(strings.TrimSpace(os.Getenv("MOVIEGLU_TERRITORY"))),
	}
	if c.Config.Headers == nil {
		c.Config.Headers = map[string]string{}
	}
	for env, header := range map[string]string{
		"MOVIEGLU_CLIENT":        "client",
		"MOVIEGLU_AUTHORIZATION": "authorization",
		"MOVIEGLU_TERRITORY":     "territory",
	} {
		if value := provider[env]; value != "" {
			c.Config.Headers[header] = value
		}
	}
	c.Config.Headers["api-version"] = "v200"
	c.Config.Headers["device-datetime"] = time.Now().Format("2006-01-02T15:04:05.000")
	if geolocation := strings.TrimSpace(os.Getenv("MOVIEGLU_GEOLOCATION")); geolocation != "" {
		c.Config.Headers["geolocation"] = geolocation
	}
	return nil
}
