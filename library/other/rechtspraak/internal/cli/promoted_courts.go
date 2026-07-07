// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCourtsPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "courts",
		Short:       "List every Dutch rechtsprekende instantie (Instanties vocab, ~260 entries)",
		Long:        "List every Dutch court with PSI URI, full name, afkorting code, type, and Begin/EndDate. Cached locally; first call fetches and caches for 14 days.",
		Example:     "  rechtspraak-pp-cli courts\n  rechtspraak-pp-cli courts --json --select name,code,type",
		Annotations: map[string]string{"pp:endpoint": "courts.list", "pp:method": "GET", "pp:path": "/Waardelijst/Instanties", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			idx, err := getCourtIndex(cmd.Context())
			if err != nil {
				return err
			}
			courts := idx.All()
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), courts)
			}
			for _, c := range courts {
				name := c.Name
				if len(name) > 60 {
					name = name[:57] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-7s  %-22s  %s\n",
					orDash(c.Afkorting), orDash(c.Type), strings.TrimSpace(name))
			}
			return nil
		},
	}
	return cmd
}
