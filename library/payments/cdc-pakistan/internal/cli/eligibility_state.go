// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/store"
)

// eligEvent is one classified transition.
type eligEvent struct {
	State      string `json:"state"`
	Title      string `json:"title"`
	NoticeURL  string `json:"notice_url"`
	Category   string `json:"category"`
	AsOfHint   string `json:"as_of_hint"`
	UploadYM   string `json:"upload_year_month,omitempty"`
	Confidence string `json:"confidence"`
}

type eligView struct {
	Subject       string         `json:"subject,omitempty"`
	Events        []eligEvent    `json:"events"`
	TerminalState string         `json:"terminal_state,omitempty"`
	StateCounts   map[string]int `json:"state_counts"`
	Note          string         `json:"note,omitempty"`
	Caveat        string         `json:"caveat"`
}

func newNovelEligibilityStateCmd(flags *rootFlags) *cobra.Command {
	var (
		isin    string
		symbol  string
		query   string
		state   string
		limit   int
		dbPath  string
		minConf string
	)

	cmd := &cobra.Command{
		Use:   "state",
		Short: "Get a security's full CDS-eligibility history folded from twenty years of notices into a six-state lifecycle.",
		Long: strings.Trim(`
Fold CDC's notices and circulars into a CDS-eligibility lifecycle.

The lifecycle has EIGHT observed states, not three:

  declared -> intention-to-suspend -> suspended -> extension-of-suspension
    -> removal-of-suspension | revoked | admission-terminated
  (plus removal-of-intention, which cancels a pending suspension)

Reading a suspension chain without the intention states mis-dates the start of a
restriction, which is why they are modelled explicitly.

HONEST LIMITS. This is the least reliable extractor in the CLI:

  * State comes from classifying document TITLE text across two decades of
    naming drift. Every event carries a confidence, and 24% of the corpus does
    not classify at all -- those are reported as unclassified, never silently
    dropped or defaulted.
  * CDC does not publish a machine-readable security identifier on these
    notices, so subject matching is TEXT matching against the title. An ISIN or
    symbol that never appears in a title cannot be found.
  * CDS eligibility is a CUSTODY status. It is NOT a PSX trading suspension and
    must not be used as a tradability gate for a backtest.

Requires a document index; run 'coverage map --probe' first.
`, "\n"),
		Example:     "  cdc-pakistan-pp-cli eligibility state --isin PK0069501016 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--query=eligibility;--limit=5", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "eligibility state")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			subject := firstNonEmpty(isin, symbol, query)
			if subject == "" && state == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("one of --isin, --symbol, --query or --state is required"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cdc-pakistan-pp-cli")
			}
			db, ready, oerr := openCDCMirror(ctx, dbPath, "cdc_documents")
			if oerr != nil {
				return oerr
			}
			if !ready {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: cdc-pakistan-pp-cli coverage map --probe --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), &eligView{
						Events: []eligEvent{}, StateCounts: map[string]int{},
						Caveat: eligCaveat,
					}, flags)
				}
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return configErr(fmt.Errorf("opening store: %w", err))
			}
			defer db.Close()

			q := `SELECT COALESCE(event_state,''), title, url, category,
			             COALESCE(meta_day_month,''), COALESCE(upload_year,'')||'-'||COALESCE(upload_month,''),
			             COALESCE(event_confidence,'')
			        FROM cdc_documents
			       WHERE event_kind = 'eligibility'`
			var argv []any
			if subject != "" {
				q += ` AND title LIKE ?`
				argv = append(argv, "%"+subject+"%")
			}
			if state != "" {
				q += ` AND event_state = ?`
				argv = append(argv, state)
			}
			if minConf == "high" {
				q += ` AND event_confidence = 'high'`
			}
			q += ` ORDER BY COALESCE(upload_year,''), COALESCE(upload_month,''), url`
			if limit > 0 {
				q += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(ctx, q, argv...)
			if err != nil {
				return apiErr(fmt.Errorf("querying eligibility events: %w", err))
			}
			view := &eligView{Subject: subject, Events: []eligEvent{}, StateCounts: map[string]int{}, Caveat: eligCaveat}
			for rows.Next() {
				var e eligEvent
				if err := rows.Scan(&e.State, &e.Title, &e.NoticeURL, &e.Category,
					&e.AsOfHint, &e.UploadYM, &e.Confidence); err != nil {
					_ = rows.Close()
					return apiErr(fmt.Errorf("scanning event: %w", err))
				}
				e.UploadYM = strings.Trim(e.UploadYM, "-")
				view.Events = append(view.Events, e)
				view.StateCounts[e.State]++
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return apiErr(err)
			}
			if err := rows.Close(); err != nil {
				return apiErr(err)
			}

			if len(view.Events) > 0 {
				view.TerminalState = view.Events[len(view.Events)-1].State
			} else {
				view.Note = fmt.Sprintf("no eligibility events matched %q. This means no NOTICE TITLE contained that text -- it does not mean the security was never subject to an eligibility action.", subject)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Events) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%-24s %-14s %s\n", "STATE", "AS-OF HINT", "NOTICE")
			for _, e := range view.Events {
				fmt.Fprintf(w, "%-24s %-14s %s\n", e.State, e.AsOfHint, truncStr(e.Title, 72))
			}
			fmt.Fprintf(w, "\nterminal state: %s\n", view.TerminalState)
			fmt.Fprintf(w, "caveat: %s\n", view.Caveat)
			return nil
		},
	}
	cmd.Flags().StringVar(&isin, "isin", "", "match notices whose title contains this ISIN")
	cmd.Flags().StringVar(&symbol, "symbol", "", "match notices whose title contains this symbol")
	cmd.Flags().StringVar(&query, "query", "", "free-text match against notice titles")
	cmd.Flags().StringVar(&state, "state", "", "filter to one lifecycle state (declared, suspended, revoked, ...)")
	cmd.Flags().StringVar(&minConf, "min-confidence", "", "set to 'high' to exclude low-confidence classifications")
	cmd.Flags().IntVar(&limit, "limit", 200, "maximum events to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

const eligCaveat = "CDS eligibility is a CUSTODY status, not a PSX trading suspension; do not use it as a tradability gate. State is classified from notice title text across 20 years of naming drift -- check the confidence field."

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
