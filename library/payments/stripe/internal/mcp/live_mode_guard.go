// Copyright 2026 Chris Rodriguez and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/config"
)

// MCP endpoint dispatch bypasses Cobra's live-mode guard, so enforce the same
// explicit write intent against the credential the client will actually send.
func checkMCPLiveModeGuard(c *client.Client, method string, args map[string]any) error {
	confirmed := false
	if value, present := args["confirm_live"]; present {
		var valid bool
		confirmed, valid = value.(bool)
		if !valid {
			return fmt.Errorf("confirm_live must be a boolean")
		}
	}
	if confirmed || os.Getenv("STRIPE_CONFIRM_LIVE") == "1" {
		return nil
	}
	switch strings.ToUpper(method) {
	case "GET", "HEAD", "OPTIONS":
		return nil
	}
	if c.Config == nil || !config.IsLiveModeCredential(c.Config.AuthHeader()) {
		return nil
	}
	return fmt.Errorf("refusing live-mode write without top-level confirm_live=true or STRIPE_CONFIRM_LIVE=1")
}
