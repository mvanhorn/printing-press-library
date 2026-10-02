// Snapshot differences compare original exact source identities, never guessed availability.
// pp:data-source local
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/travelokacompare"
	"github.com/spf13/cobra"
	"strings"
)

func newNovelQuotesDiffCmd(f *rootFlags) *cobra.Command {
	var beforeFile, afterFile, beforeID, afterID, dbPath, kind, maxAgeValue string
	var limit, scan int
	cmd := &cobra.Command{Use: "diff", Short: "Compare exact matched offers across two same-context source snapshots",
		Long: "Use this command to compare earlier and later retrieval snapshots with identical search context. Do NOT use this command to rank current flight offers; use 'traveloka-pp-cli flights shortlist' instead, or to compare current same-room cancellation options; use 'traveloka-pp-cli hotels flexibility' instead.",
		Example: strings.Trim(`
  traveloka-pp-cli quotes diff --before /private/tmp/traveloka-flight-before.json --after /private/tmp/traveloka-flight-after.json --agent
  traveloka-pp-cli quotes diff --kind rooms --db /private/tmp/traveloka-public.sqlite --limit 20 --agent
`, "\n"),
		Annotations: novelAnnotations("local", "--data-source=local"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if travelokaBareHelp(cmd, args) {
				return cmd.Help()
			}
			if dryRunOK(f) {
				return writeDryRun(cmd.OutOrStdout(), f, "quotes diff")
			}
			if e := novelNoArgs(args); e != nil {
				return travelokaFail(cmd, f, e)
			}
			if e := travelokaMode(f, "local"); e != nil {
				return travelokaFail(cmd, f, e)
			}
			maxAge, e := novelParseMaxAge(maxAgeValue)
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			if e := novelLocalOptions(limit, scan, maxAge); e != nil {
				return travelokaFail(cmd, f, e)
			}
			if (beforeFile == "") != (afterFile == "") {
				return travelokaFail(cmd, f, novelInvalid("--before and --after must be supplied together"))
			}
			if (beforeID == "") != (afterID == "") {
				return travelokaFail(cmd, f, novelInvalid("--before-id and --after-id must be supplied together"))
			}
			if beforeFile != "" && beforeID != "" {
				return travelokaFail(cmd, f, novelInvalid("snapshot file pairs and snapshot ID pairs are mutually exclusive"))
			}
			if kind != "" && kind != "flights" && kind != "rooms" && kind != "hotels" {
				return travelokaFail(cmd, f, novelInvalid("--kind must be flights, rooms or hotels"))
			}
			ctx, cancel := boundCtx(cmd.Context(), f)
			defer cancel()
			var before, after *traveloka.Snapshot
			switch {
			case beforeFile != "":
				before, e = traveloka.ReadSnapshotFile(beforeFile)
				if e == nil {
					after, e = traveloka.ReadSnapshotFile(afterFile)
				}
			case beforeID != "":
				before, e = novelLoad(ctx, cmd, dbPath, "", beforeID, kind, maxAge)
				if e == nil {
					after, e = novelLoad(ctx, cmd, dbPath, "", afterID, kind, maxAge)
				}
			default:
				before, after, e = novelLatestPair(ctx, cmd, dbPath, kind, maxAge)
			}
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			if before == nil || after == nil {
				return novelEmpty(cmd, f, []string{"changes", "not_returned", "new_returned", "unmatched_before", "unmatched_after"}, "Run the same flight/hotel search twice to save two same-context source snapshots")
			}
			if kind != "" && (before.Kind != kind || after.Kind != kind) {
				return travelokaFail(cmd, f, novelInvalid("--kind differs from the supplied snapshots"))
			}
			if beforeFile != "" {
				hintIfStale(cmd, before, maxAge)
				hintIfStale(cmd, after, maxAge)
			}
			result, e := travelokacompare.Diff(before, after, limit, scan)
			if e != nil {
				return travelokaFail(cmd, f, e)
			}
			return f.printJSON(cmd, map[string]any{"status": "saved_snapshot_diff", "query": before.Query, "before_snapshot_id": before.ID, "after_snapshot_id": after.ID, "before_retrieved_at": before.RetrievedAt, "after_retrieved_at": after.RetrievedAt, "freshness": "saved_snapshot", "indicative": true, "changes": result.Changes, "not_returned": result.NotReturned, "new_returned": result.NewReturned, "unmatched_before": result.UnmatchedBefore, "unmatched_after": result.UnmatchedAfter, "scanned_before": result.ScannedBefore, "scanned_after": result.ScannedAfter, "matched_offers": result.Matched, "unchanged_offers": result.Unchanged, "change_count": result.ChangeCount, "truncated": result.Truncated, "note": result.Note})
		}}
	cmd.Flags().StringVar(&beforeFile, "before", "", "Optional public JSON snapshot file from the earlier retrieval; pair with --after")
	cmd.Flags().StringVar(&afterFile, "after", "", "Optional public JSON snapshot file from the later retrieval; pair with --before")
	cmd.Flags().StringVar(&beforeID, "before-id", "", "Optional exact earlier SQLite snapshot ID; pair with --after-id")
	cmd.Flags().StringVar(&afterID, "after-id", "", "Optional exact later SQLite snapshot ID; pair with --before-id")
	cmd.Flags().StringVar(&kind, "kind", "", "Optional snapshot kind flights, rooms or hotels; default latest two of the latest kind")
	cmd.Flags().StringVar(&dbPath, "db", "", "Read-only public SQLite history path when no snapshot files are supplied")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum entries returned per named change/unmatched section (1 to 100)")
	cmd.Flags().IntVar(&scan, "max-scan-records", 500, "Maximum offers examined from each snapshot (1 to 1000)")
	cmd.Flags().StringVar(&maxAgeValue, "max-age", "24h", "Warn on old stored snapshots; accepts durations 24h, 7d or 1w; zero disables hints")
	return cmd
}
