// Hand-authored: novel feature `ledger`.
//
// Derived spend/activity views over outreach.people (rows-as-proxy for
// enrichments). No audit tables required.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/prospeo/internal/supa"
)

func newLedgerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ledger",
		Short: "Derived spend/activity views over outreach.people (rows-as-proxy for enrichments). No audit tables required.",
		Long:  "Derived spend/activity views over outreach.people (rows-as-proxy for enrichments). No audit tables required.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newLedgerBySourceCmd(flags))
	cmd.AddCommand(newLedgerByDayCmd(flags))
	cmd.AddCommand(newLedgerRecentCmd(flags))
	return cmd
}

// LedgerSourceRow is one row of the by-source rollup.
type LedgerSourceRow struct {
	Source   string `json:"source"`
	Rows     int    `json:"rows"`
	LastSeen string `json:"last_seen"`
}

// LedgerDayRow is one row of the by-day rollup.
type LedgerDayRow struct {
	Day  string `json:"day"`
	Rows int    `json:"rows"`
}

type peopleActivityRow struct {
	SourceLists []string `json:"source_lists"`
	UpdatedAt   string   `json:"updated_at"`
}

func fetchPeopleActivity(ctx context.Context, sc *supa.Client, days int) ([]peopleActivityRow, error) {
	start := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
	params := url.Values{}
	params.Set("select", "source_lists,updated_at")
	params.Set("updated_at", "gte."+start)
	params.Set("order", "updated_at.desc")
	params.Set("limit", "10000")
	raw, err := sc.Select(ctx, "people", params)
	if err != nil {
		return nil, fmt.Errorf("supabase outreach.people: %w", err)
	}
	var rows []peopleActivityRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("decode people: %w", err)
	}
	return rows, nil
}

func newLedgerBySourceCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "by-source",
		Short: "Rows-per-source-tag rolled up from outreach.people.source_lists.",
		Example: "  prospeo-pp-cli ledger by-source\n" +
			"  prospeo-pp-cli ledger by-source --days 7",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would query outreach.people for last %d days, group by source_lists\n", days)
				return nil
			}
			sc, err := requireSupa()
			if err != nil {
				return err
			}
			rows, err := fetchPeopleActivity(cmd.Context(), sc, days)
			if err != nil {
				return err
			}
			agg := map[string]*LedgerSourceRow{}
			for _, r := range rows {
				for _, tag := range r.SourceLists {
					if tag == "" {
						continue
					}
					b, ok := agg[tag]
					if !ok {
						b = &LedgerSourceRow{Source: tag}
						agg[tag] = b
					}
					b.Rows++
					if r.UpdatedAt > b.LastSeen {
						b.LastSeen = r.UpdatedAt
					}
				}
			}
			out := make([]LedgerSourceRow, 0, len(agg))
			for _, v := range agg {
				out = append(out, *v)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Rows > out[j].Rows })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Look back this many days.")
	return cmd
}

func newLedgerByDayCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:         "by-day",
		Short:       "Rows-per-day rolled up from outreach.people.updated_at.",
		Example:     "  prospeo-pp-cli ledger by-day --days 14",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would query outreach.people for last %d days, group by day\n", days)
				return nil
			}
			sc, err := requireSupa()
			if err != nil {
				return err
			}
			rows, err := fetchPeopleActivity(cmd.Context(), sc, days)
			if err != nil {
				return err
			}
			buckets := map[string]int{}
			for _, r := range rows {
				if len(r.UpdatedAt) < 10 {
					continue
				}
				buckets[r.UpdatedAt[:10]]++
			}
			out := make([]LedgerDayRow, 0, len(buckets))
			for d, n := range buckets {
				out = append(out, LedgerDayRow{Day: d, Rows: n})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Day > out[j].Day })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Look back this many days.")
	return cmd
}

func newLedgerRecentCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "recent",
		Short:       "Most recently updated rows from outreach.people (thin projection).",
		Example:     "  prospeo-pp-cli ledger recent --limit 25",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would query outreach.people (limit %d) ordered by updated_at desc\n", limit)
				return nil
			}
			sc, err := requireSupa()
			if err != nil {
				return err
			}
			params := url.Values{}
			params.Set("select", "id,full_name,linkedin_url,source_lists,updated_at")
			params.Set("order", "updated_at.desc")
			params.Set("limit", strconv.Itoa(limit))
			raw, err := sc.Select(cmd.Context(), "people", params)
			if err != nil {
				return fmt.Errorf("supabase outreach.people: %w", err)
			}
			var rows []map[string]any
			_ = json.Unmarshal(raw, &rows)
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Max rows to return.")
	return cmd
}

// requireSupa is the standard Supabase-required preflight used by hand-
// authored novel commands.
func requireSupa() (*supa.Client, error) {
	if !supa.IsConfigured() {
		return nil, configErr(fmt.Errorf("SUPABASE_URL and SUPABASE_SERVICE_KEY must be set"))
	}
	cfg, err := supa.LoadConfig()
	if err != nil {
		return nil, configErr(fmt.Errorf("supabase config: %w", err))
	}
	return supa.New(cfg), nil
}

// ensure context import used (Go vet)
var _ = context.Background
