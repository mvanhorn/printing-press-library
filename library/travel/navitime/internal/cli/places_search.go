// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPlacesSearchCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var limit int
	var refresh bool
	cmd := &cobra.Command{
		Use:   "search [QUERY]",
		Short: "Return bilingual source candidates without choosing an ambiguous place.",
		Long:  "Search Japanese or English QUERY, optionally filtered by --type and bounded by --limit. Returns source references and ambiguity; choose the intended candidate before routing.",
		Example: strings.Trim(`
  navitime-pp-cli places search 大久保 --type station --limit 5
  navitime-pp-cli places search "Tokyo Skytree" --type spot --select places.ref,places.name
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "query=大久保;--type=station;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return navitimeDryRun(cmd, flags, "places search")
			}
			if navitimeHelpOnly(cmd, args) {
				return cmd.Help()
			}
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return usageErr(fmt.Errorf("places search requires one QUERY; quote names with spaces, such as 'Tokyo Skytree'"))
			}
			if kind != "station" && kind != "spot" && kind != "all" {
				return usageErr(fmt.Errorf("--type must be station, spot or all"))
			}
			if err := boundedLimit(limit, 10); err != nil {
				return err
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("places search needs the public website or response cache; use --data-source auto or live"))
			}
			return runNavitime(cmd, flags, refresh, func(ctx context.Context, c navitimeService) (any, error) {
				// pp:client-call
				result, err := c.Places(ctx, strings.TrimSpace(args[0]), kind, limit)
				return result, err
			})
		},
	}
	cmd.Flags().StringVar(&kind, "type", "all", "Candidate kind to retain: station, spot or all")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum source candidates to return, between 1 and 10")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Fetch fresh candidates instead of reading the public response cache")
	return cmd
}
