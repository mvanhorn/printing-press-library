// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: health — marketplace + local-store health with rates.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/rapidapi/internal/store"
	"github.com/spf13/cobra"
)

func newHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "health",
		Short:       "Check marketplace health: connectivity, API reachability, and local cache state",
		Long:        "Check the health of your RapidAPI hub connection: gateway reachability (via the CSRF bootstrap), auth state, and local cache freshness with success rates.",
		Example:     "  rapidapi-pp-cli health",
		Annotations: map[string]string{"pp:endpoint": "health.check", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			// 1. Connectivity via the public CSRF bootstrap (no auth needed).
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			reachable := false
			if token, err := c.FetchCsrfPublic(); err == nil && token != "" {
				reachable = true
			}
			if reachable {
				fmt.Fprintln(w, "  OK Gateway: reachable (csrf bootstrap ok)")
			} else {
				fmt.Fprintln(w, "  FAIL Gateway: unreachable")
			}
			// 2. Local cache state with rates.
			s, err := store.OpenWithContext(cmd.Context(), learnDBPath(""))
			if err == nil {
				var n int64
				row := s.DB().QueryRow("SELECT COUNT(*) FROM resources")
				_ = row.Scan(&n)
				fmt.Fprintf(w, "  OK Cache: %d cached records\n", n)
				_ = s.Close()
			} else {
				fmt.Fprintln(w, "  WARN Cache: unavailable")
			}
			// 3. Auth state.
			cfg, _ := loadConfigForFlags(flags)
			if cfg != nil && cfg.RapidapiCookie != "" {
				fmt.Fprintln(w, "  OK Auth: session cookie configured")
			} else {
				fmt.Fprintln(w, "  WARN Auth: no session cookie — public commands only")
			}
			return nil
		},
	}
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
