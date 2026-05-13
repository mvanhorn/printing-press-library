// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// `edgar-pp-cli insider-summary <TICKER>` — Form 4 aggregation with
// senior-officer flagging and code-S (discretionary sale) vs code-F (RSU tax
// withholding) discrimination. THE marquee LODESTAR signal.

package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
	"github.com/spf13/cobra"
)

// Form4SkipEntry records one Form 4 filing that could not be ingested
// (missing primary doc, fetch failure, or XML parse failure). Surfaced
// in JSON output so downstream agents (LODESTAR) can see in-band that
// insider data is incomplete and trigger manual follow-up.
type Form4SkipEntry struct {
	Accession string `json:"accession"`
	Reason    string `json:"reason"`
}

// Form4SkipReport aggregates skip entries for one ingest pass.
type Form4SkipReport struct {
	Count   int              `json:"form4_skipped_count"`
	Entries []Form4SkipEntry `json:"form4_skipped_accessions"`
	Total   int              `json:"form4_total_seen"`
}

// warnForm4Skips writes a single WARN line to stderr summarizing the skip.
// Silent skip on Form 4 underreports insider activity, which feeds LODESTAR
// Gate 2 directly; LODESTAR-mandated loud-skip behavior (in-band JSON
// fields + stderr WARN).
func warnForm4Skips(rep Form4SkipReport, ticker, cik string) {
	if rep.Count == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "WARN: Form 4 ingest for %s (CIK %s) skipped %d of %d filings; insider data is incomplete. Run with --json to see skipped accessions.\n",
		ticker, cik, rep.Count, rep.Total)
}

type reporterSummary struct {
	Name                     string  `json:"name"`
	Title                    string  `json:"title,omitempty"`
	IsSenior                 bool    `json:"is_senior"`
	IsDirector               bool    `json:"is_director"`
	CodeSShares              float64 `json:"code_s_shares"`
	CodeSValue               float64 `json:"code_s_value_usd"`
	CodePShares              float64 `json:"code_p_shares"`
	CodePValue               float64 `json:"code_p_value_usd"`
	CodeAShares              float64 `json:"code_a_shares"`
	CodeFShares              float64 `json:"code_f_shares"`
	OtherShares              float64 `json:"other_shares"`
	NetDiscretionaryShares   float64 `json:"net_discretionary_shares"`
	NetDiscretionaryValueUSD float64 `json:"net_discretionary_value_usd"`
	TransactionCount         int     `json:"transaction_count"`
}

type insiderSummaryReport struct {
	Ticker    string            `json:"ticker"`
	CIK       string            `json:"cik"`
	Window    string            `json:"window"`
	Reporters []reporterSummary `json:"reporters"`
	Totals    reporterSummary   `json:"totals"`
	// Form4SkipReport — see Form4SkipReport godoc. Count>0 means insider
	// data is incomplete; LODESTAR should treat this as a signal to do
	// manual follow-up on the listed accessions.
	Form4Skipped Form4SkipReport `json:"form4_skipped"`
}

func newInsiderSummaryCmd(flags *rootFlags) *cobra.Command {
	var seniorOnly bool
	var since string
	cmd := &cobra.Command{
		Use:   "insider-summary <ticker-or-cik>",
		Short: "Form 4 aggregation with senior-officer flagging and S/F discrimination",
		Long: `Aggregate Form 4 transactions for an issuer. Discriminates code-S
(discretionary sale, signal) from code-F (RSU tax withholding, mechanical)
and other Table I/II codes (P open-market purchase, A grant, etc.). Senior
officers (CEO/CFO/COO/CTO/Chairman/President) flagged via title regex.

LODESTAR cites the net_discretionary_shares (P − S) figure directly; the
S/F collapse is the bug we exist to prevent.`,
		Example:     "  edgar-pp-cli insider-summary AAPL --senior-only --since 12mo",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := store.OpenWithContext(cmd.Context(), edgarDBPath())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := db.EnsureEdgarSchema(cmd.Context()); err != nil {
				return err
			}
			ec, err := resolveCIKOrTicker(cmd.Context(), c, db, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if since == "" {
				since = "12mo"
			}
			sinceISO, perr := parseSinceDate(since)
			if perr != nil {
				return usageErr(perr)
			}

			// Ensure Form 4 filings are synced into edgar_insider_transactions.
			skipRep, err := ingestForm4ForCIK(cmd.Context(), c, db, ec.CIK, sinceISO)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			warnForm4Skips(skipRep, ec.Ticker, ec.CIK)

			rows, err := db.ListEdgarInsiderTransactions(cmd.Context(), ec.CIK, sinceISO, seniorOnly)
			if err != nil {
				return err
			}

			// Aggregate by (reporter_name)
			byReporter := map[string]*reporterSummary{}
			var totals reporterSummary
			for _, r := range rows {
				key := r.ReporterName
				if byReporter[key] == nil {
					byReporter[key] = &reporterSummary{
						Name: r.ReporterName, Title: r.ReporterTitle,
						IsSenior: r.IsSeniorOfficer, IsDirector: r.IsDirector,
					}
				}
				rs := byReporter[key]
				rs.TransactionCount++
				totals.TransactionCount++
				switch r.TransactionCode {
				case "S":
					rs.CodeSShares += r.Shares
					rs.CodeSValue += r.ValueUSD
					totals.CodeSShares += r.Shares
					totals.CodeSValue += r.ValueUSD
				case "P":
					rs.CodePShares += r.Shares
					rs.CodePValue += r.ValueUSD
					totals.CodePShares += r.Shares
					totals.CodePValue += r.ValueUSD
				case "A":
					rs.CodeAShares += r.Shares
					totals.CodeAShares += r.Shares
				case "F":
					rs.CodeFShares += r.Shares
					totals.CodeFShares += r.Shares
				default:
					rs.OtherShares += r.Shares
					totals.OtherShares += r.Shares
				}
			}
			for _, rs := range byReporter {
				rs.NetDiscretionaryShares = rs.CodePShares - rs.CodeSShares
				rs.NetDiscretionaryValueUSD = rs.CodePValue - rs.CodeSValue
			}
			totals.NetDiscretionaryShares = totals.CodePShares - totals.CodeSShares
			totals.NetDiscretionaryValueUSD = totals.CodePValue - totals.CodeSValue

			var reporters []reporterSummary
			for _, rs := range byReporter {
				reporters = append(reporters, *rs)
			}
			report := insiderSummaryReport{
				Ticker: ec.Ticker, CIK: ec.CIK, Window: since, Reporters: reporters, Totals: totals,
				Form4Skipped: skipRep,
			}
			return emitJSON(cmd, flags, report)
		},
	}
	cmd.Flags().BoolVar(&seniorOnly, "senior-only", false, "Only include senior-officer reporters")
	cmd.Flags().StringVar(&since, "since", "12mo", "Earliest transaction date (ISO or 90d/12mo/1y)")
	return cmd
}

// ingestForm4ForCIK fetches Form 4 filings for a CIK since sinceISO, downloads
// the primary XML, parses each, and upserts transaction rows. Returns a
// Form4SkipReport enumerating any filings that could not be ingested with
// a per-accession reason (LODESTAR-mandated loud-skip; see Form4SkipEntry).
func ingestForm4ForCIK(ctx context.Context, c *client.Client, db *store.Store, cik, sinceISO string) (Form4SkipReport, error) {
	var skip Form4SkipReport
	if _, err := fetchSubmissions(ctx, c, db, cik); err != nil {
		return skip, err
	}
	filings, err := db.ListEdgarFilings(ctx, cik, []string{"4", "4/A"}, sinceISO, 200)
	if err != nil {
		return skip, err
	}
	skip.Total = len(filings)
	for _, f := range filings {
		body, urlUsed, reason := fetchForm4XML(ctx, c, cik, f.Accession, f.PrimaryDocURL)
		if body == nil {
			skip.Entries = append(skip.Entries, Form4SkipEntry{
				Accession: f.Accession,
				Reason:    reason,
			})
			skip.Count++
			continue
		}
		_ = urlUsed
		txs, perr := parseForm4(f.Accession, cik, body)
		if perr != nil {
			skip.Entries = append(skip.Entries, Form4SkipEntry{
				Accession: f.Accession,
				Reason:    "XML parse failed even after index.json fallback (" + urlUsed + "): " + perr.Error(),
			})
			skip.Count++
			continue
		}
		for _, tx := range txs {
			_ = db.UpsertEdgarInsiderTransaction(ctx, tx)
		}
	}
	return skip, nil
}

// _ to keep strconv stable.
var _ = strconv.Atoi
