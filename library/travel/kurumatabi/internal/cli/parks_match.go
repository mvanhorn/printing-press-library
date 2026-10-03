package cli

// pp:data-source local
import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/parks"
	"github.com/spf13/cobra"
	"sort"
)

func newNovelParksMatchCmd(f *rootFlags) *cobra.Command {
	var raw, member, db string
	var limit, max int
	cmd := &cobra.Command{Use: "match", Short: "Partition cached parks by proven required-service evidence", Long: "Use this command for screening cached parks by required services and membership evidence. Do NOT use this command for geographic ordering; use 'parks near' instead. Results are proven_match, needs_confirmation or ruled_out. Unknown facility, fee or requested membership evidence never becomes a proven match; search-card records require detail confirmation.", Example: "  kurumatabi-pp-cli parks match --require electricity=free,water,pets --membership nonmember --limit 10 --agent", Annotations: parkAnnotation("local", "--require=electricity,water,pets;--membership=nonmember"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "parks match")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("use --require facility,facility=free rather than positional arguments"))
		}
		if raw == "" && !hasChangedLocalFlags(cmd) && !f.agent && !f.asJSON {
			return cmd.Help()
		}
		reqs, e := parks.ParseRequirements(raw)
		if e != nil {
			return usageErr(e)
		}
		if _, e = parks.Match(parks.Park{}, reqs, member); e != nil {
			return usageErr(e)
		}
		if f.dataSource == "live" {
			return usageErr(fmt.Errorf("parks match has no live equivalent; cache parks detail first"))
		}
		if max < 1 || max > 10000 || limit < 1 || limit > 100 {
			return usageErr(fmt.Errorf("--max-scan-records must be 1..10000 and --limit 1..100"))
		}
		db = parkResolveDB(db)
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		ps, e := parks.Load(ctx, db)
		if e != nil {
			return e
		}
		parkCacheHint(cmd, f, db, ps)
		pool := len(ps)
		if len(ps) > max {
			ps = ps[:max]
		}
		rows := []parks.MatchResult{}
		parts := map[string][]string{"proven_match": {}, "needs_confirmation": {}, "ruled_out": {}}
		for _, p := range ps {
			r, e := parks.Match(p, reqs, member)
			if e != nil {
				return usageErr(e)
			}
			rows = append(rows, r)
		}
		rank := map[string]int{"proven_match": 0, "needs_confirmation": 1, "ruled_out": 2}
		sort.SliceStable(rows, func(i, j int) bool {
			if rank[rows[i].Decision] != rank[rows[j].Decision] {
				return rank[rows[i].Decision] < rank[rows[j].Decision]
			}
			return rows[i].ID < rows[j].ID
		})
		truncated := len(rows) > limit
		if truncated {
			rows = rows[:limit]
		}
		for _, r := range rows {
			parts[r.Decision] = append(parts[r.Decision], r.ID)
		}
		note := "Cached observed evidence only; current operation and date-specific availability require host confirmation."
		if len(rows) == 0 {
			note += " No cached candidates; cache parks detail or widen --max-scan-records."
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"meta": map[string]any{"source": "local", "cached_records": pool, "scanned_records": len(ps), "scan_cap_hit": pool > max, "output_truncated": truncated, "partitions": parts, "note": note}, "results": rows}, f)
	}}
	cmd.Flags().StringVar(&raw, "require", "", "Comma-separated facilities, optionally =free, =paid or =included")
	cmd.Flags().StringVar(&member, "membership", "unknown", "Requested membership: nonmember, member, standard, premium or unknown")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum candidate decisions returned, 1..100")
	cmd.Flags().IntVar(&max, "max-scan-records", 1000, "Maximum cached records scanned independently from output size")
	cmd.Flags().StringVar(&db, "db", "", "SQLite path for observations; default follows runtime CLI data paths")
	return cmd
}
