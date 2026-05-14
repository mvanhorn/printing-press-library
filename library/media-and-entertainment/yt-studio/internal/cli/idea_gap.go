package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstore"
)

func newIdeaGapCmd(flags *rootFlags) *cobra.Command {
	var (
		days     int
		dbPath   string
		channels string
	)
	cmd := &cobra.Command{
		Use:   "idea-gap",
		Short: "Topics watchlist covered in last N days that own channel hasn't",
		Long: strings.TrimSpace(`
Reads competitor video snapshots from the local store (populated by
` + "`yt-studio-pp-cli sync --watchlist`" + `) and returns titles that don't
share at least 2 significant tokens with any video on your own channel.

This is a heuristic — false positives are expected. Treat the output as an
ideation feed, not a strict "we haven't covered this" set.`),
		Example:     "  yt-studio-pp-cli idea-gap --days 14 --json --select gaps",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()

			var ownIDs []string
			if channels != "" {
				ownIDs = splitCSV(channels)
			}
			gaps, err := ytstore.FindIdeaGaps(ctx, db, days, ownIDs)
			if err != nil {
				return err
			}
			res := map[string]any{
				"days":  days,
				"gaps":  gaps,
				"count": len(gaps),
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}
			if len(gaps) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no idea gaps detected; run `yt-studio-pp-cli sync --watchlist` first)")
				return nil
			}
			for _, g := range gaps {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", g.CompetitorChannelID, g.Title)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "Window in days")
	cmd.Flags().StringVar(&channels, "channels", "", "Comma-separated own channel IDs (default: all 'own' channels)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}
