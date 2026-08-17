// Copyright 2026 Felix Banuchi and contributors. Licensed under Apache-2.0. See LICENSE.
// Generated command body replaced by a retained provider projection.

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func newStackProductsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "products",
		Short:       "Show the minimum configured stack product fields.",
		Example:     "  suppco-pp-cli stack products",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"pp:endpoint": "stack.products", "pp:method": "GET", "pp:path": "/api/users/me_compact/", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				return nil
			}
			service, err := newSuppCoProvider(flags)
			if err != nil {
				return err
			}
			projection, err := service.Products(cmd.Context())
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(projection)
		},
	}
}
