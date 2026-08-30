// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: system csrf — fetch the session-bound CSRF bootstrap token.

package cli

import (
	"github.com/spf13/cobra"
)

func newSystemCsrfCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "csrf",
		Short:       "Fetch the session-bound CSRF token used to authenticate gateway calls",
		Long:        "Fetch the RapidAPI hub CSRF bootstrap token (GET /gateway/csrf). The token is bound to the session cookie and is required as the x-csrf-token header on every GraphQL gateway call.",
		Example:     "  rapidapi-pp-cli system csrf",
		Annotations: map[string]string{"pp:endpoint": "system.csrf", "pp:method": "GET", "pp:path": "/gateway/csrf", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			token, err := c.FetchCsrfPublic()
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, mustJSON(map[string]any{"csrfToken": token}), map[string]bool{"csrfToken": true})
		},
	}
	return cmd
}
