package cli

// pp:data-source local

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/nccpl/internal/store"
)

type nccplSearchView struct {
	Query     string           `json:"query"`
	Resources []string         `json:"resources,omitempty"`
	From      string           `json:"from,omitempty"`
	To        string           `json:"to,omitempty"`
	Hits      []store.NCCPLHit `json:"hits"`
	HitCount  int              `json:"hit_count"`
	Limit     int              `json:"limit"`
	Truncated bool             `json:"truncated"`
	DBPath    string           `json:"db_path"`
	Note      string           `json:"note,omitempty"`
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		resourcesCSV string
		fromDate     string
		toDate       string
		limit        int
		dbPath       string
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Find stored observations by symbol, date or any payload text, across every resource",
		Long: strings.Trim(`
Substring-search every observation in the local store and report, for each hit, the
resource it belongs to, the settlement date, the key it was filed under, and the
observed_at vintage recorded when the value was first seen.

This is the only read path that does not require knowing the resource first. coverage,
panel, universe, leverage and risk-changes all take --resource; answering "where does
this symbol appear at all?" previously meant exporting all 24 panels and grepping them.

Matching is literal and case-insensitive, over both the row key and the payload, so an
identifier or a date matches as typed. A key match is flagged as matched_key, which
separates "this row IS about HUBC" from "HUBC appears somewhere inside this row".

Search reads and never writes. It reports what is stored and nothing else: a session
that was never fetched stays absent from the results rather than being interpolated,
and a zero-hit result is a zero-hit result. Run 'coverage' to see which sessions are
actually held before reading anything into an empty search.
`, "\n"),
		Example: strings.Trim(`
  nccpl-pp-cli search HUBC
  nccpl-pp-cli search HUBC --resources mts,slb --limit 200 --json
  nccpl-pp-cli search OGDC --from 2026-08-01 --to 2026-09-04
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "HUBC",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search")
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required"))
			}
			if limit <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}
			resources, err := nccplSearchResources(resourcesCSV)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if err := nccplSearchDateBounds(fromDate, toDate); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("nccpl-pp-cli")
			}
			view := nccplSearchView{
				Query:     query,
				Resources: resources,
				From:      fromDate,
				To:        toDate,
				Hits:      make([]store.NCCPLHit, 0),
				Limit:     limit,
				DBPath:    dbPath,
			}
			syncHint := fmt.Sprintf("run: nccpl-pp-cli sync --resources fipi --latest-only --db %s\n", dbPath)

			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\n%s", dbPath, syncHint)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			ready, err := store.NCCPLSchemaReady(ctx, db)
			if err != nil {
				return err
			}
			if !ready {
				fmt.Fprintf(cmd.ErrOrStderr(), "no NCCPL data synced yet at %s\n%s", dbPath, syncHint)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}

			hits, err := store.SearchNCCPLObservations(ctx, db, query, store.NCCPLSearchOptions{
				Resources: resources,
				From:      fromDate,
				To:        toDate,
				Limit:     limit,
			})
			if err != nil {
				return err
			}
			view.Hits = hits
			view.HitCount = len(hits)
			view.Truncated = len(hits) == limit
			if view.HitCount == 0 {
				view.Note = "nothing stored matches this query; check 'coverage' for which sessions are held before reading anything into an empty result"
			} else if view.Truncated {
				view.Note = fmt.Sprintf("result truncated at --limit %d; raise it or narrow with --resources/--from/--to", limit)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			if view.HitCount == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no stored observation matches %q\n", query)
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Note)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-16s %-30s %-4s %-22s %s\n",
				"DATE", "RESOURCE", "KEY", "HIT", "OBSERVED_AT", "PAYLOAD")
			for _, hit := range view.Hits {
				where := "row"
				if hit.MatchedKey {
					where = "key"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-16s %-30s %-4s %-22s %s\n",
					hit.Date,
					hit.Resource,
					truncate(hit.Key, 30),
					where,
					hit.ObservedAt,
					truncate(nccplSearchPayloadPreview(hit.Payload), 60))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d hit(s)\n", view.HitCount)
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", view.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&resourcesCSV, "resources", "", "Comma-separated resources to search; empty means every stored resource")
	cmd.Flags().StringVar(&fromDate, "from", "", "Earliest settlement date to search (YYYY-MM-DD)")
	cmd.Flags().StringVar(&toDate, "to", "", "Latest settlement date to search (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum hits to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// nccplSearchResources validates a --resources CSV against the NCCPL registry so
// a typo narrows the search to nothing loudly instead of silently.
func nccplSearchResources(csv string) ([]string, error) {
	if strings.TrimSpace(csv) == "" {
		return nil, nil
	}
	selected, err := nccplSelectResources(csv)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(selected))
	for _, r := range selected {
		out = append(out, r.Name)
	}
	return out, nil
}

// nccplSearchDateBounds rejects a malformed or inverted date window before it
// silently returns zero hits.
func nccplSearchDateBounds(from, to string) error {
	for _, bound := range []struct {
		flag  string
		value string
	}{{"--from", from}, {"--to", to}} {
		if bound.value == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", bound.value); err != nil {
			return fmt.Errorf("%s %q is not a YYYY-MM-DD date", bound.flag, bound.value)
		}
	}
	if from != "" && to != "" && from > to {
		return fmt.Errorf("--from %s is after --to %s", from, to)
	}
	return nil
}

// nccplSearchPayloadPreview collapses a payload's whitespace so one hit stays on
// one line of the human table. The full payload is always in --json output.
func nccplSearchPayloadPreview(payload string) string {
	return strings.Join(strings.Fields(payload), " ")
}
