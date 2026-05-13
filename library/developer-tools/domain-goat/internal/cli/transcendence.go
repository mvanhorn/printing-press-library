// Transcendence commands: compare, shortlist, budget, drops, why-killed,
// pricing-arbitrage, drop-bid-window, tld-affinity.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/scoring"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/store"
)

// CompareRow is the per-domain comparison view.
type CompareRow struct {
	FQDN          string     `json:"fqdn"`
	TLD           string     `json:"tld"`
	Length        int        `json:"length"`
	Score         int        `json:"score"`
	Available     *bool      `json:"available,omitempty"`
	Status        string     `json:"status,omitempty"`
	Price         *PriceInfo `json:"price,omitempty"`
	Prestige      int        `json:"tld_prestige"`
	ExpiresAt     string     `json:"expires_at,omitempty"`
	DropAt        string     `json:"drop_at,omitempty"`
	CombinedScore int        `json:"combined_score"`
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare [domain...]",
		Short: "Side-by-side comparison: score, length, TLD prestige, price, RDAP status, drop flag.",
		Long: `Joins everything we know locally about each domain into one structured row:
brandability score, TLD prestige, Porkbun price, RDAP status, drop date.
Pulls from the local store; run "check" first to populate availability,
"pricing sync" first to populate prices.`,
		Example: `  domain-goat-pp-cli compare kindred.io kindred.ai kindred.studio
  domain-goat-pp-cli compare kindred.io lumen.ai novella.io --json --select fqdn,score,price`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			fqdns, err := normalizeAll(args)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "fqdns": fqdns})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			rows := make([]CompareRow, 0, len(fqdns))
			for _, f := range fqdns {
				rows = append(rows, buildCompareRow(cmd.Context(), s, f))
			}
			// rank by combined_score desc
			sort.Slice(rows, func(i, j int) bool { return rows[i].CombinedScore > rows[j].CombinedScore })
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, rows)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DOMAIN\tSCORE\tCOMBINED\tAVAIL\tSTATUS\tPRICE\tPRESTIGE\tEXPIRES")
			for _, r := range rows {
				avail := "?"
				if r.Available != nil {
					if *r.Available {
						avail = "yes"
					} else {
						avail = "no"
					}
				}
				price := ""
				if r.Price != nil {
					price = fmt.Sprintf("$%.2f", r.Price.Registration)
				}
				fmt.Fprintf(tw, "%s\t%d\t%d\t%s\t%s\t%s\t%d\t%s\n",
					r.FQDN, r.Score, r.CombinedScore, avail, truncate(r.Status, 20), price, r.Prestige, r.ExpiresAt)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func buildCompareRow(ctx context.Context, s *store.Store, fqdn string) CompareRow {
	r := CompareRow{FQDN: fqdn}
	parts := strings.SplitN(fqdn, ".", 2)
	if len(parts) == 2 {
		r.TLD = parts[1]
		r.Length = len(parts[0])
	}
	sc := scoring.Compute(fqdn)
	r.Score = sc.Total
	r.Prestige = sc.TLDPrestige

	d, _ := s.GetDomain(ctx, fqdn)
	if d != nil {
		r.Status = d.Status
		if d.Status == "available" || d.Status == "404" {
			t := true
			r.Available = &t
		} else if d.Status != "" {
			f := false
			r.Available = &f
		}
		r.ExpiresAt = d.ExpiresAt
		r.DropAt = d.DropAt
	}
	p, _ := s.PricingForFQDN(ctx, fqdn)
	if p != nil {
		r.Price = &PriceInfo{
			Registrar: p.Registrar, Registration: p.Registration,
			Renewal: p.Renewal, Transfer: p.Transfer,
		}
	}
	// combined = score - price_penalty + availability_bonus
	combined := r.Score
	if r.Price != nil {
		// penalty: $1 per $20 of registration, capped at -30
		penalty := int(r.Price.Registration / 20)
		if penalty > 30 {
			penalty = 30
		}
		combined -= penalty
	}
	if r.Available != nil && *r.Available {
		combined += 25
	}
	if combined < 0 {
		combined = 0
	}
	r.CombinedScore = combined
	return r
}

func newShortlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shortlist",
		Short: "Promote and manage finalist shortlists from candidate lists.",
	}
	cmd.AddCommand(newShortlistPromoteCmd(flags))
	return cmd
}

func newShortlistPromoteCmd(flags *rootFlags) *cobra.Command {
	var srcList, destList, by string
	var top int
	cmd := &cobra.Command{
		Use:   "promote",
		Short: "Promote the top-N candidates from a list into a finalist sub-list.",
		Long: `Ranks candidates by combined score (brandability + availability bonus - price
penalty), or by raw score / price, and moves the top-N into a finalist list.
Default ranker is "combined".`,
		Example: `  domain-goat-pp-cli shortlist promote --list ai-startup --top 10
  domain-goat-pp-cli shortlist promote --list ai-startup --top 10 --by combined --dest finalists --json`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if srcList == "" {
				return usageErr(fmt.Errorf("--list required"))
			}
			if top <= 0 {
				top = 10
			}
			if destList == "" {
				destList = srcList + "-finalists"
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "list": srcList, "top": top, "dest": destList})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			candidates, err := s.ListCandidates(cmd.Context(), srcList, false)
			if err != nil {
				return apiErr(err)
			}
			if len(candidates) == 0 {
				return notFoundErr(fmt.Errorf("no live candidates in list %s", srcList))
			}
			rows := make([]CompareRow, 0, len(candidates))
			for _, c := range candidates {
				rows = append(rows, buildCompareRow(cmd.Context(), s, c.FQDN))
			}
			sort.Slice(rows, func(i, j int) bool {
				switch by {
				case "score":
					return rows[i].Score > rows[j].Score
				case "price":
					pi, pj := 999999.0, 999999.0
					if rows[i].Price != nil {
						pi = rows[i].Price.Registration
					}
					if rows[j].Price != nil {
						pj = rows[j].Price.Registration
					}
					return pi < pj
				default:
					return rows[i].CombinedScore > rows[j].CombinedScore
				}
			})
			if len(rows) > top {
				rows = rows[:top]
			}
			promoted := 0
			// PATCH(shortlist-promote-error-surfacing): collect per-candidate AddCandidate failures under `errors` in the output map; previously errors were swallowed so operators saw `promoted < top` with no diagnostic on DB-full / ctx-cancel / writeMu-contention.
			var promoteErrs []map[string]string
			if err := s.CreateList(cmd.Context(), destList, "Auto-promoted from "+srcList); err != nil {
				return apiErr(err)
			}
			for _, r := range rows {
				if err := s.AddCandidate(cmd.Context(), store.CandidateRow{
					ListName: destList, FQDN: r.FQDN,
					Tags:  "promoted",
					Notes: fmt.Sprintf("score=%d combined=%d (by %s)", r.Score, r.CombinedScore, by),
				}); err == nil {
					promoted++
				} else {
					// Surface per-candidate failures (full DB, ctx cancel,
					// writeMu contention past deadline) so operators don't see
					// a silent "promoted < top" with no diagnostic.
					promoteErrs = append(promoteErrs, map[string]string{
						"fqdn":  r.FQDN,
						"error": err.Error(),
					})
				}
			}
			out := map[string]any{
				"src":      srcList,
				"dest":     destList,
				"by":       by,
				"promoted": promoted,
				"top":      rows,
			}
			if len(promoteErrs) > 0 {
				out["errors"] = promoteErrs
			}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&srcList, "list", "", "Source list name (required)")
	cmd.Flags().StringVar(&destList, "dest", "", "Destination list name (default: <list>-finalists)")
	cmd.Flags().StringVar(&by, "by", "combined", "Rank by: combined|score|price")
	cmd.Flags().IntVar(&top, "top", 10, "Promote top-N")
	return cmd
}

func newBudgetCmd(flags *rootFlags) *cobra.Command {
	var list string
	var years, top int
	var maxAnnualCost float64
	var availableOnly bool
	// PATCH(cli-flag-rename-max-annual-cost): renamed budget's --max-renewal → --max-annual-cost. The flag actually caps amortised annual spend (total/years), not raw renewal price. gen suggest's --max-renewal is unchanged because it correctly caps raw renewal.
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Filter candidates by 5-year true cost (registration + N renewals).",
		Long: `Surfaces the year-2-jump that registrar UIs hide until checkout.
Computes total = registration_price + (years - 1) × renewal_price and
filters to candidates whose amortised annual spend (total / years) is at
or below --max-annual-cost.`,
		Example: `  domain-goat-pp-cli budget --list ai-startup --max-annual-cost 50 --years 5
  domain-goat-pp-cli budget --list brand-sprint --max-annual-cost 100 --years 5 --top 20 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if list == "" {
				return usageErr(fmt.Errorf("--list required"))
			}
			if years <= 0 {
				years = 5
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "list": list, "years": years})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			candidates, err := s.ListCandidates(cmd.Context(), list, false)
			if err != nil {
				return apiErr(err)
			}
			type Row struct {
				FQDN         string  `json:"fqdn"`
				Registration float64 `json:"registration_price"`
				Renewal      float64 `json:"renewal_price"`
				TotalCost    float64 `json:"total_cost"`
				Score        int     `json:"score"`
				Years        int     `json:"years"`
			}
			rows := []Row{}
			for _, c := range candidates {
				if availableOnly {
					d, _ := s.GetDomain(cmd.Context(), c.FQDN)
					if d == nil || (d.Status != "available" && d.Status != "404") {
						continue
					}
				}
				p, _ := s.PricingForFQDN(cmd.Context(), c.FQDN)
				if p == nil {
					continue
				}
				total := p.Registration + float64(years-1)*p.Renewal
				if maxAnnualCost > 0 && total > maxAnnualCost*float64(years) {
					continue
				}
				rows = append(rows, Row{
					FQDN: c.FQDN, Registration: p.Registration, Renewal: p.Renewal,
					TotalCost: total, Score: scoring.Compute(c.FQDN).Total, Years: years,
				})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].TotalCost < rows[j].TotalCost })
			if top > 0 && len(rows) > top {
				rows = rows[:top]
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, rows)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "DOMAIN\tYEAR1\tRENEW\t%dYR_TOTAL\tSCORE\n", years)
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t$%.2f\t$%.2f\t$%.2f\t%d\n", r.FQDN, r.Registration, r.Renewal, r.TotalCost, r.Score)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&list, "list", "", "Source list name (required)")
	cmd.Flags().IntVar(&years, "years", 5, "Years of total cost to compute")
	cmd.Flags().Float64Var(&maxAnnualCost, "max-annual-cost", 0, "Cap on amortised annual spend = (registration + (years-1)×renewal) / years (0 = no cap)")
	cmd.Flags().IntVar(&top, "top", 0, "Show top-N cheapest (0 = all)")
	cmd.Flags().BoolVar(&availableOnly, "available-only", false, "Filter to candidates whose last availability check returned 'available' (run `check` first)")
	_ = cmd.MarkFlagRequired("list")
	return cmd
}

func newDropsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drops",
		Short: "Surface domains in their expiry / pendingDelete / redemption window.",
	}
	cmd.AddCommand(newDropsTimelineCmd(flags))
	return cmd
}

func newDropsTimelineCmd(flags *rootFlags) *cobra.Command {
	var days, minScore int
	var tldsCSV string
	cmd := &cobra.Command{
		Use:   "timeline",
		Short: "Time-axis view of upcoming drops (pendingDelete / redemptionPeriod) by score.",
		Long: `Reads persisted RDAP records (events_json), finds domains whose expiry
or pending-delete date is within the next N days, filters by brandability
score and TLD, returns an ordered timeline.`,
		Example: `  domain-goat-pp-cli drops timeline --days 30 --min-score 7 --tld io,ai
  domain-goat-pp-cli drops timeline --days 14 --json --select fqdn,drop_at,score`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if days <= 0 {
				days = 30
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "days": days})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			tlds := joinTLDs(tldsCSV)
			tldSet := map[string]bool{}
			for _, t := range tlds {
				tldSet[t] = true
			}
			now := time.Now().UTC()
			until := now.AddDate(0, 0, days)

			// PATCH(drops-timeline-null-scan-and-dedup): scan score/tld as sql.NullInt64/NullString so rdap-only domains (no row in `domains`) aren't silently dropped; ROW_NUMBER() window replaces the MAX(fetched_at) join because 1-second SQLite resolution let two same-second writes both match and the domain appeared twice.
			// Latest RDAP row per fqdn. Window function (ROW_NUMBER) tiebreaks
			// on rowid so two writes within the same second (datetime('now')
			// has 1-second resolution) collapse to one row instead of emitting
			// the same fqdn twice in the timeline output.
			rows, err := s.Query(`
				WITH latest AS (
					SELECT fqdn, status, events_json, fetched_at,
					       ROW_NUMBER() OVER (
					           PARTITION BY fqdn
					           ORDER BY fetched_at DESC, rowid DESC
					       ) AS rn
					FROM rdap_records
				)
				SELECT l.fqdn, l.status, l.events_json, l.fetched_at, d.score, d.tld
				FROM latest l
				LEFT JOIN domains d ON d.fqdn = l.fqdn
				WHERE l.rn = 1
				ORDER BY l.fqdn`)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type DropRow struct {
				FQDN       string `json:"fqdn"`
				TLD        string `json:"tld"`
				Score      int    `json:"score"`
				Status     string `json:"status"`
				DropAt     string `json:"drop_at"`
				DropReason string `json:"drop_reason"`
				ExpiresAt  string `json:"expires_at,omitempty"`
			}
			results := []DropRow{}
			for rows.Next() {
				var fqdn, status, eventsJSON, fetchedAt string
				// score/tld come from a LEFT JOIN to domains — NULL when the
				// fqdn was looked up via `rdap` (writes rdap_records only) and
				// never went through `check` (which UpsertDomains). Scanning
				// NULL into plain int/string errors and silently drops the row.
				var score sql.NullInt64
				var tld sql.NullString
				if err := rows.Scan(&fqdn, &status, &eventsJSON, &fetchedAt, &score, &tld); err != nil {
					continue
				}
				scoreVal := int(score.Int64)
				tldVal := tld.String
				if len(tldSet) > 0 && !tldSet[tldVal] {
					continue
				}
				if scoreVal < minScore {
					continue
				}
				if eventsJSON == "" || eventsJSON == "[]" {
					continue
				}
				var events []struct {
					Action string `json:"action"`
					Date   string `json:"date"`
				}
				if err := json.Unmarshal([]byte(eventsJSON), &events); err != nil {
					continue
				}
				var dropAt, reason, expires string
				for _, e := range events {
					switch strings.ToLower(e.Action) {
					case "expiration":
						expires = e.Date
					case "pendingdelete", "deletion":
						dropAt = e.Date
						reason = e.Action
					case "redemption period", "redemption":
						if dropAt == "" {
							dropAt = e.Date
							reason = e.Action
						}
					}
				}
				if dropAt == "" {
					// fall back to expires + 75 day grace (ICANN: 30d redemption + 5d pendingDelete)
					if expires == "" {
						continue
					}
					et, err := time.Parse(time.RFC3339, expires)
					if err != nil {
						continue
					}
					et = et.AddDate(0, 0, 75)
					dropAt = et.Format(time.RFC3339)
					reason = "estimated-from-expiry"
				}
				t, err := time.Parse(time.RFC3339, dropAt)
				if err != nil {
					continue
				}
				if t.Before(now) || t.After(until) {
					continue
				}
				results = append(results, DropRow{
					FQDN: fqdn, TLD: tldVal, Score: scoreVal, Status: status,
					DropAt: dropAt, DropReason: reason, ExpiresAt: expires,
				})
			}
			sort.Slice(results, func(i, j int) bool { return results[i].DropAt < results[j].DropAt })
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, results)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DROP_AT\tDOMAIN\tSCORE\tREASON")
			for _, r := range results {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", r.DropAt, r.FQDN, r.Score, r.DropReason)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Look-ahead window in days")
	cmd.Flags().IntVar(&minScore, "min-score", 0, "Minimum brandability score (0..100)")
	cmd.Flags().StringVar(&tldsCSV, "tld", "", "Comma-separated TLDs to filter to")
	return cmd
}

func newWhyKilledCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "why-killed <domain>",
		Short: "Show why a domain isn't on the live shortlist: status, score, notes, tags.",
		Long: `Pulls every candidate row for this domain across all your lists,
plus the last pricing and RDAP snapshot. Useful weeks after the kill decision
to recover institutional memory.`,
		Example: `  domain-goat-pp-cli why-killed kindred.studio
  domain-goat-pp-cli why-killed kindred.studio --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Short-circuit before normalizeAll so verify's synthetic
			// positional ("mock-value") doesn't fail TLD validation.
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "fqdn": args[0]})
			}
			fqdns, err := normalizeAll(args)
			if err != nil {
				return usageErr(err)
			}
			fqdn := fqdns[0]
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.Query(`SELECT list_name, notes, tags, killed, kill_reason, added_at, updated_at FROM candidates WHERE fqdn = ? ORDER BY updated_at DESC`, fqdn)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type Entry struct {
				List       string `json:"list"`
				Notes      string `json:"notes"`
				Tags       string `json:"tags"`
				Killed     bool   `json:"killed"`
				KillReason string `json:"kill_reason"`
				AddedAt    string `json:"added_at"`
				UpdatedAt  string `json:"updated_at"`
			}
			entries := []Entry{}
			for rows.Next() {
				var e Entry
				var killed int
				if err := rows.Scan(&e.List, &e.Notes, &e.Tags, &killed, &e.KillReason, &e.AddedAt, &e.UpdatedAt); err != nil {
					continue
				}
				e.Killed = killed == 1
				entries = append(entries, e)
			}
			d, _ := s.GetDomain(cmd.Context(), fqdn)
			p, _ := s.PricingForFQDN(cmd.Context(), fqdn)
			raw, status, eventsJSON, fetchedAt, _ := s.GetLatestRDAP(cmd.Context(), fqdn)
			out := map[string]any{
				"fqdn":    fqdn,
				"entries": entries,
				"score":   scoring.Compute(fqdn),
				"domain":  d,
				"price":   p,
				"rdap": map[string]any{
					"status":      status,
					"events_json": eventsJSON,
					"fetched_at":  fetchedAt,
				},
			}
			if len(entries) == 0 && d == nil {
				out["found"] = false
				return emitJSON(cmd, flags, out)
			}
			_ = raw
			return emitJSON(cmd, flags, out)
		},
	}
	return cmd
}

func newPricingArbitrageCmd(flags *rootFlags) *cobra.Command {
	var by string
	var top int
	cmd := &cobra.Command{
		Use:   "pricing-arbitrage",
		Short: "Rank TLDs by year-1-trap risk (renewal-delta) or prestige-to-price ratio.",
		Long: `Aggregates the Porkbun pricing snapshot to surface structural facts:
TLDs where renewal_price >> registration_price (year-2 traps), or where
prestige is high relative to registration cost. Public pricing data is the
only no-auth source for this comparison.`,
		Example: `  domain-goat-pp-cli pricing-arbitrage --by renewal-delta --top 20
  domain-goat-pp-cli pricing-arbitrage --by prestige-value --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "by": by})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := s.ListPricing(cmd.Context(), "porkbun", 0)
			if err != nil {
				return apiErr(err)
			}
			type Arb struct {
				TLD           string  `json:"tld"`
				Registration  float64 `json:"registration"`
				Renewal       float64 `json:"renewal"`
				Delta         float64 `json:"delta"`
				Prestige      int     `json:"prestige"`
				PrestigeValue float64 `json:"prestige_value"`
			}
			out := []Arb{}
			for _, p := range rows {
				if p.Registration <= 0 {
					continue
				}
				delta := p.Renewal - p.Registration
				prestige := prestige(p.TLD)
				pv := 0.0
				if p.Registration > 0 {
					pv = float64(prestige) / p.Registration
				}
				out = append(out, Arb{
					TLD: p.TLD, Registration: p.Registration, Renewal: p.Renewal,
					Delta: delta, Prestige: prestige, PrestigeValue: pv,
				})
			}
			sort.Slice(out, func(i, j int) bool {
				switch by {
				case "prestige-value":
					return out[i].PrestigeValue > out[j].PrestigeValue
				default:
					return out[i].Delta > out[j].Delta
				}
			})
			if top > 0 && len(out) > top {
				out = out[:top]
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, out)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "TLD\tREGISTRATION\tRENEWAL\tDELTA\tPRESTIGE")
			for _, a := range out {
				fmt.Fprintf(tw, ".%s\t$%.2f\t$%.2f\t$%.2f\t%d\n", a.TLD, a.Registration, a.Renewal, a.Delta, a.Prestige)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&by, "by", "renewal-delta", "Sort key: renewal-delta|prestige-value")
	cmd.Flags().IntVar(&top, "top", 20, "Top-N results (0 = all)")
	return cmd
}

// PATCH(drop-bid-window-redemption-fallback): fall through pendingDelete → redemption → expiration when computing the bid window; redemption-only domains (active 30-day grace) now estimate redemption+35d instead of erroring.
func newDropBidWindowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drop-bid-window <domain>",
		Short: "Compute exact UTC re-release window for a domain in pendingDelete (RDAP + RFC grace).",
		Long: `Reads the latest RDAP record for the domain, finds the pendingDelete or
expiration event, adds the ICANN RFC pending-delete grace period (5 days),
and returns the precise UTC window the domain will become re-available.`,
		Example: `  domain-goat-pp-cli drop-bid-window expiring.io
  domain-goat-pp-cli drop-bid-window expiring.io --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Short-circuit before normalizeAll so verify's synthetic
			// positional ("mock-value") doesn't fail TLD validation.
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "fqdn": args[0]})
			}
			fqdns, err := normalizeAll(args)
			if err != nil {
				return usageErr(err)
			}
			fqdn := fqdns[0]
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			_, status, eventsJSON, fetchedAt, _ := s.GetLatestRDAP(cmd.Context(), fqdn)
			if eventsJSON == "" || eventsJSON == "[]" {
				return notFoundErr(fmt.Errorf("no RDAP records for %s — run `domain-goat-pp-cli check %s` first", fqdn, fqdn))
			}
			var events []struct {
				Action string `json:"action"`
				Date   string `json:"date"`
			}
			if err := json.Unmarshal([]byte(eventsJSON), &events); err != nil {
				return apiErr(err)
			}
			var pendingDelete, expiration, redemption string
			for _, e := range events {
				switch strings.ToLower(e.Action) {
				case "pendingdelete", "deletion":
					pendingDelete = e.Date
				case "expiration":
					expiration = e.Date
				case "redemption period", "redemption":
					redemption = e.Date
				}
			}
			out := map[string]any{
				"fqdn":             fqdn,
				"rdap_status":      status,
				"rdap_fetched_at":  fetchedAt,
				"expiration_event": expiration,
				"redemption_event": redemption,
				"pending_delete":   pendingDelete,
			}
			// Preference order: pendingDelete (most precise) → redemption (next
			// most precise; ~30d redemption + 5d pendingDelete grace remains)
			// → expiration (least precise; auto-renew grace is registrar
			// policy, treated as 0 here). Each ladder rung is "estimated"
			// vs the registry's actual drop time and should be refreshed.
			var bidStart, bidEnd time.Time
			var basis string
			if pendingDelete != "" {
				t, err := time.Parse(time.RFC3339, pendingDelete)
				if err == nil {
					bidStart = t.AddDate(0, 0, 5)
					bidEnd = bidStart.Add(48 * time.Hour)
					basis = "pendingDelete+5d"
				}
			} else if redemption != "" {
				t, err := time.Parse(time.RFC3339, redemption)
				if err == nil {
					// redemption period (~30d) → pendingDelete (5d) → drop
					bidStart = t.AddDate(0, 0, 35)
					bidEnd = bidStart.Add(48 * time.Hour)
					basis = "redemption+35d (estimated; verify with later RDAP refresh)"
				}
			} else if expiration != "" {
				t, err := time.Parse(time.RFC3339, expiration)
				if err == nil {
					// expiry → 30d redemption → 5d pendingDelete → drop
					bidStart = t.AddDate(0, 0, 35)
					bidEnd = bidStart.Add(48 * time.Hour)
					basis = "expiration+35d (estimated; verify with later RDAP refresh)"
				}
			}
			if !bidStart.IsZero() {
				out["bid_window_start"] = bidStart.Format(time.RFC3339)
				out["bid_window_end"] = bidEnd.Format(time.RFC3339)
				out["basis"] = basis
				out["hours_until_start"] = int(time.Until(bidStart).Hours())
			} else {
				out["error"] = "no expiration, redemption, or pendingDelete event found"
			}
			return emitJSON(cmd, flags, out)
		},
	}
	return cmd
}

func newTLDAffinityCmd(flags *rootFlags) *cobra.Command {
	var topN int
	cmd := &cobra.Command{
		Use:   "tld-affinity <seed>",
		Short: "Rank TLDs by fit for a seed keyword: semantic suffix + price tier + historical availability.",
		Long: `Joins tlds × pricing × candidates from the local store to recommend which TLD
to look at first for a given seed. Combines:
  - suffix-semantics: does .studio extend "kindred"? does .ai extend "lumin"?
  - price tier: cheaper is better, all else equal
  - prestige: .com/.ai/.io > .xyz/.online
  - historical availability rate in your local domains table`,
		Example: `  domain-goat-pp-cli tld-affinity kindred --top 10
  domain-goat-pp-cli tld-affinity lumen --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			seed := strings.ToLower(args[0])
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "seed": seed})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			tlds, err := s.ListTLDs(cmd.Context())
			if err != nil {
				return apiErr(err)
			}
			type Aff struct {
				Seed                string  `json:"seed"`
				TLD                 string  `json:"tld"`
				FQDN                string  `json:"example_fqdn"`
				Prestige            int     `json:"prestige"`
				Registration        float64 `json:"registration"`
				Renewal             float64 `json:"renewal"`
				SuffixHit           bool    `json:"suffix_extension"`
				HistoricalAvailable int     `json:"historical_available_rate_pct"`
				AffinityScore       int     `json:"affinity_score"`
			}
			out := []Aff{}
			for _, t := range tlds {
				p, _ := s.GetPricing(cmd.Context(), t.TLD, "porkbun")
				a := Aff{Seed: seed, TLD: t.TLD, FQDN: seed + "." + t.TLD, Prestige: prestige(t.TLD)}
				if p != nil {
					a.Registration = p.Registration
					a.Renewal = p.Renewal
				}
				// Suffix semantics
				a.SuffixHit = strings.HasSuffix(seed, t.TLD)
				// Historical availability — sample at most 100 domains per tld.
				// Best-effort estimate: if the cursor errors mid-scan (context
				// cancellation, transient SQLite issue), zero the partial counts
				// instead of reporting a misleadingly low rate.
				totalAvail, totalCount := 0, 0
				// PATCH(transcendence-rows-err-and-defer): IIFE around the inner SELECT...LIMIT 100 with defer rows.Close() and rows.Err() — missing rows.Err() let a mid-scan cursor error produce a misleadingly low historical-availability rate; missing defer would have leaked a SQL connection on panic.
				// Wrap in IIFE so defer rows.Close() runs per iteration on any
				// exit path (including a hypothetical panic from rows.Scan),
				// returning the SQL connection to the pool deterministically.
				func() {
					rows, qerr := s.Query(`SELECT status FROM domains WHERE tld = ? LIMIT 100`, t.TLD)
					if qerr != nil {
						return
					}
					defer rows.Close()
					for rows.Next() {
						var st string
						if scanErr := rows.Scan(&st); scanErr == nil {
							totalCount++
							if st == "available" || st == "404" {
								totalAvail++
							}
						}
					}
					if rows.Err() != nil {
						totalAvail, totalCount = 0, 0
					}
				}()
				if totalCount > 0 {
					a.HistoricalAvailable = totalAvail * 100 / totalCount
				}
				// Affinity scoring: prestige × 1 + suffix-bonus × 50 - price × 1.5 + avail × 0.5
				score := a.Prestige
				if a.SuffixHit {
					score += 50
				}
				if a.Registration > 0 {
					score -= int(a.Registration * 1.5)
				}
				score += a.HistoricalAvailable / 2
				if score < 0 {
					score = 0
				}
				a.AffinityScore = score
				out = append(out, a)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].AffinityScore > out[j].AffinityScore })
			if topN > 0 && len(out) > topN {
				out = out[:topN]
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, out)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "SEED\tTLD\tFQDN\tAFFINITY\tPRESTIGE\tREG\tSUFFIX\tHISTORICAL_AVAIL_PCT\n")
			for _, a := range out {
				fmt.Fprintf(tw, "%s\t.%s\t%s\t%d\t%d\t$%.2f\t%v\t%d%%\n", a.Seed, a.TLD, a.FQDN, a.AffinityScore, a.Prestige, a.Registration, a.SuffixHit, a.HistoricalAvailable)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&topN, "top", 10, "Top-N TLDs to return (0 = all)")
	return cmd
}
