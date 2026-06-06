package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/foxnews/internal/foxnews"
	"github.com/spf13/cobra"
)

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check connectivity to the Fox News RSS feeds",
		Example: `  foxnews-pp-cli doctor
  foxnews-pp-cli doctor --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := map[string]any{
				"cli":     "foxnews-pp-cli",
				"auth":    "none",
				"default": "latest",
			}
			out := cmd.OutOrStdout()
			if dryRunOK(flags) {
				report["status"] = "dry-run"
				if !wantsHumanTable(out, flags) {
					return printMachineOutput(out, flags, map[string]any{"source": "live"}, report)
				}
				fmt.Fprintln(out, "dry-run: skipped live feed check")
				return nil
			}
			sec, _ := foxnews.ResolveSection("latest")
			feed, err := foxnews.Fetch(cmd.Context(), sec, "", flags.timeout)
			if err != nil {
				report["status"] = "error"
				report["error"] = err.Error()
				if !wantsHumanTable(out, flags) {
					return printMachineOutput(out, flags, map[string]any{"source": "live", "section": "latest"}, report)
				}
				return apiErr(fmt.Errorf("latest feed check failed: %w", err))
			}
			report["status"] = "ok"
			report["feed_url"] = feed.FeedURL
			report["item_count"] = len(feed.Items)
			if !wantsHumanTable(out, flags) {
				meta := map[string]any{
					"source":   "live",
					"feed_url": feed.FeedURL,
					"section":  "latest",
				}
				return printMachineOutput(out, flags, meta, report)
			}
			fmt.Fprintf(out, "ok: fetched %d items from %s\n", len(feed.Items), feed.FeedURL)
			return nil
		},
	}
	return cmd
}
