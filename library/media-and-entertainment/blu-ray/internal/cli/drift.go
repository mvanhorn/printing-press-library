package cli

// PATCH: Hand-built sitemap snapshot drift report command.
// pp:data-source local -- drift diffs locally stored sitemap snapshots from
// prior syncs; it reads only the local store and makes no live API calls.

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/store"
	"github.com/spf13/cobra"
)

type driftReport struct {
	FirstSyncComplete bool       `json:"first_sync_complete,omitempty"`
	ReleasesIndexed   int        `json:"releases_indexed,omitempty"`
	Added             []string   `json:"added,omitempty"`
	Removed           []string   `json:"removed,omitempty"`
	Changed           []string   `json:"changed,omitempty"`
	Snapshots         []snapshot `json:"snapshots,omitempty"`
}

type snapshot struct {
	TakenAt     string `json:"taken_at"`
	SitemapName string `json:"sitemap_name"`
	URLCount    int    `json:"url_count"`
	URLSetHash  string `json:"url_set_hash"`
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var since, kind string
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Compare current sitemap snapshots with previous sync snapshots.",
		// PATCH: Add agent-copyable examples for dogfood command detection.
		Example: strings.Trim(`
  blu-ray-pp-cli drift --since 2026-05-01 --kind bluray --json
  blu-ray-pp-cli drift --json --select added,removed,changed
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := store.OpenWithContext(cmd.Context(), defaultDBPath("blu-ray-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.MigrateBluRayCatalog(); err != nil {
				return err
			}
			kindLike := ""
			if kind != "" {
				// PATCH: Escape LIKE wildcards so a kind filter is literal text.
				kindLike = "%" + escapeLikePattern(kind) + "%"
			}
			storeSnaps, err := s.ListSitemapSnapshots(cmd.Context(), kindLike, since)
			if err != nil {
				return err
			}
			var snaps []snapshot
			for _, row := range storeSnaps {
				snaps = append(snaps, snapshot(row))
			}
			report := driftReport{Snapshots: snaps}
			if len(snaps) <= 1 {
				report.FirstSyncComplete = len(snaps) == 1
				if stats, err := s.CatalogStats(cmd.Context()); err == nil {
					report.ReleasesIndexed = stats.TotalRows
				}
			} else {
				// PATCH: Bucket snapshots by TakenAt so the comparison uses the two
				// most recent distinct sync timestamps. The prior implementation
				// (`prev := snaps[:len(snaps)-1]`) excluded only the final row, so
				// when one sync touched N sitemaps at the same TakenAt, N-1 ended up
				// being compared against themselves and reported as unchanged. Fixes
				// Greptile P1 on PR #634.
				latestTime := snaps[len(snaps)-1].TakenAt
				prevTime := ""
				for i := len(snaps) - 1; i >= 0; i-- {
					if snaps[i].TakenAt != latestTime {
						prevTime = snaps[i].TakenAt
						break
					}
				}
				prev := map[string]snapshot{}
				latest := map[string]snapshot{}
				for _, s := range snaps {
					switch s.TakenAt {
					case latestTime:
						latest[s.SitemapName] = s
					case prevTime:
						prev[s.SitemapName] = s
					}
				}
				if prevTime == "" {
					// Every snapshot shares one TakenAt — treat as the same first-sync
					// case rather than fabricating drift against an empty baseline.
					report.FirstSyncComplete = true
					if stats, err := s.CatalogStats(cmd.Context()); err == nil {
						report.ReleasesIndexed = stats.TotalRows
					}
					goto renderReport
				}
				for name, cur := range latest {
					old, ok := prev[name]
					switch {
					case !ok:
						report.Added = append(report.Added, name)
					case old.URLSetHash != cur.URLSetHash:
						report.Changed = append(report.Changed, name)
					}
				}
				for name := range prev {
					if _, ok := latest[name]; !ok {
						report.Removed = append(report.Removed, name)
					}
				}
			}
		renderReport:
			if flags.asJSON || flags.selectFields != "" || flags.csv || flags.quiet || flags.plain || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return flags.printJSON(cmd, report)
			}
			if report.FirstSyncComplete {
				fmt.Fprintf(cmd.OutOrStdout(), "First sync complete: %d releases indexed\n", report.ReleasesIndexed)
				return nil
			}
			// PATCH: Render human drift output when stdout is an interactive human sink.
			printDriftSection(cmd, "Added", report.Added)
			printDriftSection(cmd, "Removed", report.Removed)
			printDriftSection(cmd, "Changed", report.Changed)
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only consider snapshots taken on or after YYYY-MM-DD.")
	cmd.Flags().StringVar(&kind, "kind", "", "Sitemap kind filter: bluray, 4k, dvd, digital, or news.")
	return cmd
}

func escapeLikePattern(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func printDriftSection(cmd *cobra.Command, label string, items []string) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s (%d):\n", label, len(items))
	for _, item := range items {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", item)
	}
}
