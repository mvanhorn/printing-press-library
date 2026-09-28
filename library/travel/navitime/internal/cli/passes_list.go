// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/navitime"
	"github.com/spf13/cobra"
)

func newNovelPassesListCmd(flags *rootFlags) *cobra.Command {
	var limit, offset int
	var refresh bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Page through the source-advertised single-pass catalogue.",
		Long:  "Page advertised pass IDs with --limit/--offset; --refresh uses one reference-route request. Returns source labels and verification status; select one ID for routes search --pass.",
		Example: strings.Trim(`
  navitime-pp-cli passes list --limit 10
  navitime-pp-cli passes list --offset 10 --limit 10
  navitime-pp-cli passes list --refresh --select passes.id,passes.name,passes.live_tested
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--limit=10"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return navitimeDryRun(cmd, flags, "passes list")
			}
			if len(args) != 0 {
				return usageErr(fmt.Errorf("passes list accepts --limit and --offset, with no positional arguments"))
			}
			if err := boundedLimit(limit, 100); err != nil {
				return err
			}
			if offset < 0 {
				return usageErr(fmt.Errorf("--offset must be zero or greater"))
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("passes list needs the public website or response cache; use --data-source auto or live"))
			}
			return runNavitime(cmd, flags, refresh, func(ctx context.Context, c navitimeService) (any, error) {
				// pp:client-call
				result, err := c.Passes(ctx)
				if err != nil {
					return nil, err
				}
				total := len(result.Passes)
				start := offset
				if start > total {
					start = total
				}
				end := start + limit
				if end > total {
					end = total
				}
				page := append(make([]navitime.Pass, 0, end-start), result.Passes[start:end]...)
				notes := append(make([]string, 0, len(result.Notes)+1), result.Notes...)
				if len(page) == 0 {
					notes = append(notes, "No advertised passes at this local offset; use a smaller --offset.")
				}
				return struct {
					Meta       navitime.Metadata `json:"meta"`
					Passes     []navitime.Pass   `json:"passes"`
					Total      int               `json:"total"`
					Offset     int               `json:"offset"`
					Limit      int               `json:"limit"`
					Pagination string            `json:"pagination"`
					Notes      []string          `json:"notes"`
				}{result.Meta, page, total, offset, limit, "local catalogue slicing; no source pagination", notes}, nil
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum advertised passes to return, between 1 and 100")
	cmd.Flags().IntVar(&offset, "offset", 0, "Number of source catalogue entries to skip locally")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh the catalogue with one dated Tokyo to Kyoto reference route")
	return cmd
}
