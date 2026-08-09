// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
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
	required := map[string]string{
		"MOVIEGLU_CLIENT":        strings.TrimSpace(os.Getenv("MOVIEGLU_CLIENT")),
		"MOVIEGLU_AUTHORIZATION": strings.TrimSpace(os.Getenv("MOVIEGLU_AUTHORIZATION")),
		"MOVIEGLU_TERRITORY":     strings.ToUpper(strings.TrimSpace(os.Getenv("MOVIEGLU_TERRITORY"))),
		"MOVIEGLU_GEOLOCATION":   strings.TrimSpace(os.Getenv("MOVIEGLU_GEOLOCATION")),
	}
	for name, value := range required {
		if value == "" {
			return fmt.Errorf("%s is required; request evaluation credentials at https://developer.movieglu.com/", name)
		}
	}
	if len(required["MOVIEGLU_TERRITORY"]) != 2 {
		return fmt.Errorf("MOVIEGLU_TERRITORY must be a two-letter country code")
	}
	if c.Config.Headers == nil {
		c.Config.Headers = map[string]string{}
	}
	c.Config.Headers["client"] = required["MOVIEGLU_CLIENT"]
	c.Config.Headers["authorization"] = required["MOVIEGLU_AUTHORIZATION"]
	c.Config.Headers["territory"] = required["MOVIEGLU_TERRITORY"]
	c.Config.Headers["api-version"] = "v200"
	c.Config.Headers["device-datetime"] = time.Now().Format("2006-01-02T15:04:05.000")
	c.Config.Headers["geolocation"] = required["MOVIEGLU_GEOLOCATION"]
	return nil
}
