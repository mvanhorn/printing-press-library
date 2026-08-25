// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Phase 3 foundation: structured business profile via the
// internal/vagaro sibling client. generate --force preserves implemented bodies.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newBusinessGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <slug>",
		Short:   "Get a business profile (name, category, location, businessID)",
		Example: "  vagaro-pp-cli business get centralbarber",
		// pp:data-source live
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "slug=centralbarber"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := strings.Trim(strings.TrimSpace(args[0]), "/")
			if slug == "" {
				return usageErr(fmt.Errorf("slug is required\nUsage: %s <slug>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newVagaroClient(flags)
			prof, err := c.FetchProfile(ctx, slug)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			return emitVagaro(cmd, flags, prof)
		},
	}
	return cmd
}
