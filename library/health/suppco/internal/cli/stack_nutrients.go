// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.
// Generated command body replaced by a retained provider projection.

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func newStackNutrientsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "nutrients",
		Short:       "Show product-bound ingredient rows and ancestry-derived relationships without summing them.",
		Example:     "  suppco-pp-cli stack nutrients",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"pp:endpoint": "stack.nutrients", "pp:method": "GET", "pp:path": "/api/users/me_compact/", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return nil
			}
			service, err := newSuppCoProvider(flags)
			if err != nil {
				return err
			}
			projection, err := service.Nutrients(cmd.Context())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(projection)
		},
	}
}
