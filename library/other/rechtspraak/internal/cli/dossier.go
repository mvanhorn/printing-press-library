// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newNovelDossierCmd(flags *rootFlags) *cobra.Command {
	var flagCourt string
	var flagFrom string
	var flagTo string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "dossier <zaaknummer>",
		Short: "List every decision sharing a case number across instances",
		Long: `Track a Dutch case file (zaaknummer) across instances. The upstream API
exposes no zaaknummer filter; this walks the search index over a date range
and filters locally by matching zaaknummer on each decision's metadata.

Default date window is the last 10 years ending today — wide enough to
capture a typical case chain (rechtbank → gerechtshof → cassatie can
easily span 5+ years per instance). Without --court that's a substantial
number of API calls, paced via the shared 10 req/s limiter; the page
hard cap (20 pages of 1000 entries) is the upper bound on the scan.

Provide --court to restrict to a single court (orders of magnitude faster)
or --from / --to to narrow the date range. --timeout bounds the entire
scan; on cancellation dossier returns the partial result with a clean
warning instead of silently truncating.`,
		Example: `  rechtspraak-pp-cli dossier 22/00155
  rechtspraak-pp-cli dossier 22/00155 --court HR
  rechtspraak-pp-cli dossier 22/00155 --from 2022-01-01 --to 2024-12-31 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			zaak := strings.TrimSpace(args[0])
			if !looksLikeZaaknummer(zaak) {
				return fmt.Errorf("invalid zaaknummer %q: expected a case-file identifier such as 22/00155, AMS-21-006186, or 99-1491 (must contain at least one digit and one separator or be alphanumeric and >= 3 chars)", zaak)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			http := mustHTTP()
			courtIdx, err := getCourtIndex(ctx)
			if err != nil {
				return err
			}
			from, to := defaultDossierWindow(flagFrom, flagTo)
			baseParams := rechtspraak.SearchParams{
				Dates: []string{from, to},
				Max:   1000,
				Sort:  "ASC",
			}
			if flagCourt != "" {
				if uri := courtIdx.URI(flagCourt); uri != "" {
					baseParams.Creators = []string{uri}
				} else {
					return fmt.Errorf("unknown court %q (try `rechtspraak-pp-cli code %s`)", flagCourt, flagCourt)
				}
			}
			// Page until total is exhausted, an early --limit kicks in, or
			// the page hard-cap fires. The hard cap exists so an unbounded
			// 10-year window without --court doesn't silently fetch hundreds
			// of pages — agents are expected to scope before invoking.
			const maxPages = 20
			entries, totalSeen, err := paginateDossier(ctx, http, baseParams, flagLimit, maxPages)
			if err != nil {
				return err
			}
			// Warn on stderr when the window was wide enough that pagination
			// hit its ceiling — the result set is likely truncated. The
			// guidance points the user at the cheap fix (--court, narrower
			// dates).
			if totalSeen >= maxPages*baseParams.Max && flagCourt == "" {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"dossier: scanned %d candidate ECLIs (page ceiling) without finishing the window — narrow with --court or --from/--to for a complete result\n",
					totalSeen)
			}
			matches, scanErr := collectDossierMatches(ctx, http, entries, zaak, flagLimit)
			if scanErr != nil {
				// Context cancellation (--timeout, Ctrl-C, agent deadline)
				// returns partial results plus the context error. Warn the
				// user that the result set is incomplete BEFORE returning,
				// so an agent piping --json output gets both the partial
				// set on stdout (next branch) AND the typed error
				// surfaced as a non-zero exit.
				fmt.Fprintf(cmd.ErrOrStderr(), "dossier: scan interrupted (%v); partial result has %d match(es). Re-run with a wider --timeout to complete.\n", scanErr, len(matches))
				return scanErr
			}
			sort.SliceStable(matches, func(i, j int) bool {
				return matches[i].DecisionDate < matches[j].DecisionDate
			})
			// Emit a stderr nudge when the upstream API returned no
			// matches in the search window. The default --from/--to is
			// a 10-year window ending today; an empty result usually
			// means either the zaaknummer is genuinely absent in that
			// range or the --court filter is too narrow. The nudge
			// keeps JSON-mode callers (agents) from interpreting
			// {count: 0} as a CLI bug.
			if len(matches) == 0 {
				if flagCourt != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "no decisions found for zaaknummer %q in court %q between %s and %s; try widening with --from/--to or dropping --court\n", zaak, flagCourt, from, to)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "no decisions found for zaaknummer %q between %s and %s; try widening with --from/--to\n", zaak, from, to)
				}
			}
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), map[string]any{
					"zaaknummer": zaak,
					"count":      len(matches),
					"decisions":  matches,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Dossier %s — %d decisions\n", zaak, len(matches))
			for _, m := range matches {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s  %s  (%s)\n", m.DecisionDate, m.ECLI, m.Court, m.Type)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagCourt, "court", "", "Restrict to a single court (afkorting, name, or PSI URI)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Decision-date floor (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Decision-date ceiling (YYYY-MM-DD)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Stop scanning after N candidate ECLIs (0 = no limit)")
	return cmd
}

type dossierEntry struct {
	ECLI         string `json:"ecli"`
	Court        string `json:"court"`
	DecisionDate string `json:"decision_date"`
	Type         string `json:"type"`
	Procedure    string `json:"procedure,omitempty"`
}

func defaultDossierWindow(from, to string) (string, string) {
	if from != "" && to != "" {
		return from, to
	}
	// Default to a generous 10-year window ending today so the typical case
	// chain (rechtbank → hof → cassatie) is captured.
	now := nowYMD()
	if from == "" {
		from = priorYearYMD(10)
	}
	if to == "" {
		to = now
	}
	return from, to
}

// paginateDossier walks /uitspraken/zoeken via repeated From offsets until
// the upstream total is exhausted, the candidate --limit ceiling is hit, or
// the page hard cap fires. Returns the accumulated entries plus the total
// candidates scanned (for ceiling reporting).
func paginateDossier(ctx context.Context, http *rechtspraak.HTTP, base rechtspraak.SearchParams, limit, maxPages int) ([]rechtspraak.SearchEntry, int, error) {
	all := make([]rechtspraak.SearchEntry, 0, base.Max)
	var grandTotal int
	for page := 0; page < maxPages; page++ {
		p := base
		p.From = page * base.Max
		entries, total, err := http.Search(ctx, p)
		if err != nil {
			return nil, grandTotal, err
		}
		all = append(all, entries...)
		grandTotal = len(all)
		if limit > 0 && grandTotal >= limit {
			return all, grandTotal, nil
		}
		if len(entries) < base.Max {
			// Final page.
			return all, grandTotal, nil
		}
		if total > 0 && grandTotal >= total {
			return all, grandTotal, nil
		}
	}
	return all, grandTotal, nil
}

// collectDossierMatches fetches each entry's content and keeps the ones
// whose zaaknummer matches. The earlier title-substring prefilter was
// dropped because the Atom title formats zaaknummer differently across
// instances ("22/00155" vs "22 / 00155" vs "22-00155") and was producing
// false negatives. The cost (one content fetch per candidate) is bounded
// by the page hard cap upstream and by ctx (--timeout / Ctrl-C / agent
// deadline). On context cancellation the function returns whatever it
// found so far together with the context error — callers MUST distinguish
// nil from non-nil to know whether the result is complete.
func collectDossierMatches(ctx context.Context, http *rechtspraak.HTTP, entries []rechtspraak.SearchEntry, zaak string, limit int) ([]dossierEntry, error) {
	wantNormalized := normalizeZaaknummer(zaak)
	matches := make([]dossierEntry, 0, 8)
	for i, e := range entries {
		// Honour context cancellation before each potentially expensive
		// HTTP call. Without this the loop swallows every error from
		// http.Get with `continue`, including context.Canceled and
		// context.DeadlineExceeded, and the caller would emit a partial
		// match set as a successful result with no warning.
		if err := ctx.Err(); err != nil {
			return matches, err
		}
		if limit > 0 && i >= limit {
			break
		}
		d, err := http.Get(ctx, e.ECLI, false)
		if err != nil {
			// Distinguish context errors from transient per-ECLI fetch
			// failures. The per-entry skip is the right call for a 5xx
			// or parse error on a single decision; for a cancelled
			// context we MUST return so the partial-result warning fires.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return matches, ctxErr
			}
			continue
		}
		for _, z := range d.Zaaknummer {
			if normalizeZaaknummer(z) == wantNormalized {
				matches = append(matches, dossierEntry{
					ECLI:         d.ECLI,
					Court:        d.Court,
					DecisionDate: d.DecisionDate,
					Type:         d.Type,
					Procedure:    d.Procedure,
				})
				break
			}
		}
	}
	return matches, nil
}

// looksLikeZaaknummer rejects obvious garbage early so dogfood error-path
// probes (which pass tokens like "__printing_press_invalid__") and human
// typos surface a clean error instead of an empty result set. A real
// zaaknummer:
//   - contains at least one digit
//   - either contains a separator (/ - .) OR is alphanumeric and >= 3 chars
//   - contains no leading/trailing underscores (used as sentinels by dogfood)
func looksLikeZaaknummer(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 3 {
		return false
	}
	if strings.HasPrefix(s, "_") || strings.HasSuffix(s, "_") {
		return false
	}
	hasDigit := false
	hasSeparator := false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case r == '/' || r == '-' || r == '.':
			hasSeparator = true
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == ' ':
			// fine
		default:
			return false
		}
	}
	if !hasDigit {
		return false
	}
	if !hasSeparator && len(s) < 4 {
		return false
	}
	return true
}

// normalizeZaaknummer collapses zaaknummer formatting variants so
// "22/00155" / "22 / 00155" / "22-00155" all compare equal. Whitespace
// removed, common separators normalized to slash, lowercased.
func normalizeZaaknummer(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "/")
	s = strings.ReplaceAll(s, ".", "/")
	return s
}
