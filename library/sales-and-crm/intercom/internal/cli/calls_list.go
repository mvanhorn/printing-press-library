// PATCH(intercom-calls-v2-14): see calls.go.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCallsListCmd(flags *rootFlags) *cobra.Command {
	var perPage int
	var page int

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List phone calls (paginated)",
		Example:     "  intercom-pp-cli calls list --per-page 50",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			if perPage > 0 {
				params["per_page"] = fmt.Sprintf("%d", perPage)
			}
			if page > 0 {
				params["page"] = fmt.Sprintf("%d", page)
			}
			data, err := c.GetWithHeaders(cmd.Context(), "/calls", params, callsHeaderOverrides())
			if err != nil {
				// Friendlier hint when the workspace doesn't have the Phone feature.
				if strings.Contains(err.Error(), "404") {
					return notFoundErr(fmt.Errorf("calls endpoint returned 404 — Phone may not be enabled on this workspace"))
				}
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().IntVar(&perPage, "per-page", 25, "Page size")
	cmd.Flags().IntVar(&page, "page", 1, "1-indexed page number")
	return cmd
}
