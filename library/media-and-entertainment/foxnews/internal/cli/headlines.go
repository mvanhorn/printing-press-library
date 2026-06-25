package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/foxnews/internal/foxnews"
	"github.com/spf13/cobra"
)

func newHeadlinesCmd(flags *rootFlags) *cobra.Command {
	var section string
	var limit int
	cmd := &cobra.Command{
		Use:   "headlines",
		Short: "Fetch headlines from a Fox News Google Publisher RSS section",
		Long: `Fetch headlines from Fox News Google Publisher RSS feeds hosted at moxie.foxnews.com.

Defaults to the "latest" section (all topics). Use --section to pick world, politics,
science, health, sports, travel, tech, opinion, or video.

JSON output uses {"meta":{...},"results":[...]} with meta.source=live.`,
		Example: `  foxnews-pp-cli headlines --limit 10
  foxnews-pp-cli headlines --section politics --limit 10 --json --select title,link,published
  foxnews-pp-cli headlines --section sports --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be non-negative"))
			}
			sec, err := foxnews.ResolveSection(section)
			if err != nil {
				return usageErr(err)
			}
			feed, err := foxnews.Fetch(cmd.Context(), sec, "", flags.timeout)
			if err != nil {
				return wrapAPIErr(err)
			}
			items := foxnews.Limit(feed.Items, limit)
			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				meta := map[string]any{
					"source":   "live",
					"feed_url": feed.FeedURL,
					"section":  sec.ID,
					"count":    len(items),
				}
				return printMachineOutput(out, flags, meta, items)
			}
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "SECTION\t%s (%s)\n", sec.Label, feed.FeedURL)
			fmt.Fprintln(w, "PUBLISHED\tTITLE\tURL")
			for _, item := range items {
				when := ""
				if !item.Published.IsZero() {
					when = item.Published.Format("2006-01-02 15:04")
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", when, item.Title, item.Link)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&section, "section", "latest", "Feed section: latest, world, politics, science, health, sports, travel, tech, opinion, video")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum headlines to return (0 = all in feed)")
	return cmd
}
