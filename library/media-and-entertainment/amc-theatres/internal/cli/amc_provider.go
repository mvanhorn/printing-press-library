// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/amc-theatres/internal/client"
)

const (
	amcProductionURL = "https://api.amctheatres.com"
	amcSandboxURL    = "https://api.sandbox-amctheatres.com"
)

func init() {
	registerClientHook(configureAMCClient)
}

func configureAMCClient(c *client.Client) error {
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("AMC_THEATRES_ENV")))
	switch environment {
	case "", "production":
	case "sandbox":
		if c.BaseURL == amcProductionURL || c.BaseURL == amcSandboxURL {
			c.BaseURL = amcSandboxURL
		}
	default:
		return fmt.Errorf("AMC_THEATRES_ENV must be production or sandbox")
	}
	if token := strings.TrimSpace(os.Getenv("AMC_THEATRES_AUTH_TOKEN")); token != "" {
		if c.Config.Headers == nil {
			c.Config.Headers = map[string]string{}
		}
		c.Config.Headers["X-AMC-Auth-Token"] = token
	}
	return nil
}
