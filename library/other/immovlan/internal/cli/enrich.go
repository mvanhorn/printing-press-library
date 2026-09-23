// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

type enrichReport struct {
	Missing   []string `json:"missing"`
	Selected  int      `json:"selected"`
	Fetched   int      `json:"fetched"`
	Updated   int      `json:"updated"`
	NotFound  int      `json:"not_found"`
	Errors    int      `json:"errors"`
	Remaining int      `json:"remaining"`
	Note      string   `json:"note,omitempty"`
	Failures  []string `json:"failures,omitempty"`
}

func newNovelEnrichCmd(flags *rootFlags) *cobra.Command {
	var flagMissing, flagPostcode, flagDeal, flagRetryAfter, dbPath string
	var flagTop int
	var flagRate float64
	cmd := &cobra.Command{
		Use:   "enrich",
		Short: "Fill missing detail fields (PEB letter, street, geo, rented, cadastral income, agency software) for many stored listings in one resumable pass",
		Long: `Search cards never carry the street, geo, rented flag, cadastral income, year or
the agency's CRM software; some carry no PEB letter. enrich reads the listing page
of every stored listing that lacks the requested fields, writes the fields back,
and prints only counts. It is resumable: never-read pages go first, then the
oldest reads, and a page read within --retry-after that still lacks the field
is not fetched again (the field is simply absent on that listing).
Use this command to fill missing detail fields for many stored listings without
printing them. To read one listing, use 'show'.`,
		Example: strings.Trim(`
  immovlan-pp-cli enrich --missing epc,rented --top 150 --agent
  immovlan-pp-cli enrich --missing detail --postcode 1030 --rate 1 --agent`, "\n"),
		// read-only in the MCP sense, like find: it reads public pages and only
		// fills the CLI's own local cache; nothing on immovlan.be changes.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--missing=epc;--top=2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "enrich stored listings from their detail pages")
			}
			if err := rejectDataSource(flags, "live"); err != nil {
				return usageErr(err)
			}
			missing := []string{}
			for _, m := range immovlan.SplitCSV(flagMissing) {
				m = strings.ToLower(strings.TrimSpace(m))
				if _, ok := store.MissingColumn(m); !ok {
					return usageErr(fmt.Errorf("unknown --missing field %q (use epc, street, geo, rented, cadastral, software, year, condition, photos or detail)", m))
				}
				missing = appendUniq(missing, m)
			}
			if len(missing) == 0 {
				missing = []string{"detail"}
			}
			retryAfter, err := parseWindow(flagRetryAfter)
			if err != nil {
				return usageErr(fmt.Errorf("--retry-after: %w", err))
			}
			if flagTop <= 0 {
				flagTop = 100
			}
			if cliutil.IsAnyHarness() && flagTop > 2 {
				flagTop = 2
			}
			ctx, cancel := harvestCtx(cmd, flags)
			defer cancel()
			db, err := openVlanStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			f := store.ListingFilter{MissingAny: missing, OldestDetailFirst: true, DetailBefore: time.Now().Add(-retryAfter).UTC().Format(time.RFC3339)}
			if f.Deal, err = parseDealFlag(flagDeal); err != nil {
				return err
			}
			if f.PostalCodes, err = parsePostcodes(flagPostcode); err != nil {
				return err
			}
			rows, err := db.QueryVlanListings(ctx, f)
			if err != nil {
				return err
			}
			rep := enrichReport{Missing: missing, Selected: len(rows)}
			todo := rows
			if len(todo) > flagTop {
				todo = todo[:flagTop]
			}
			rep.Remaining = len(rows) - len(todo)
			if len(todo) == 0 {
				rep.Note = "nothing to enrich: every stored listing already has these fields (run find first to store listings)"
				// Tell "every listing has the field" apart from "the pages that
				// lack it were read within --retry-after and do not state it",
				// so the user is not sent in a loop by peb-trap's hint.
				held := f
				held.DetailBefore = ""
				if rows, err := db.QueryVlanListings(ctx, held); err == nil && len(rows) > 0 {
					rep.Note = fmt.Sprintf("nothing to fetch: %d listing(s) still lack %s but their pages were read within --retry-after %s and do not state them; shorten --retry-after to re-read, or use --include-unknown on peb-trap", len(rows), strings.Join(missing, ","), flagRetryAfter)
				}
				return printEnrich(cmd, flags, rep)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			delay := time.Duration(0)
			if flagRate > 0 {
				delay = time.Duration(float64(time.Second) / flagRate)
			}
			for i, r := range todo {
				if i > 0 && delay > 0 {
					select {
					case <-ctx.Done():
						rep.Note = "stopped: " + ctx.Err().Error()
						return printEnrich(cmd, flags, rep)
					case <-time.After(delay):
					}
				}
				d, err := fetchDetail(ctx, c, r.ID)
				rep.Fetched++
				if err != nil {
					if isNotFound(err) {
						rep.NotFound++
						if _, err := db.MarkGoneNow(ctx, []string{r.ID}); err != nil {
							return err
						}
						continue
					}
					rep.Errors++
					if len(rep.Failures) < 5 {
						rep.Failures = append(rep.Failures, r.ID+": "+termSafe(err.Error()))
					}
					if rep.Errors >= 5 && rep.Updated == 0 {
						rep.Note = "stopped after repeated errors; check immovlan-pp-cli doctor"
						break
					}
					continue
				}
				if err := db.SaveVlanDetail(ctx, d, time.Now()); err != nil {
					return err
				}
				rep.Updated++
			}
			return printEnrich(cmd, flags, rep)
		},
	}
	cmd.Flags().StringVar(&flagMissing, "missing", "detail", "Fields to fill, comma-separated: epc, street, geo, rented, cadastral, software, year, condition, photos, detail (any listing never enriched)")
	cmd.Flags().IntVar(&flagTop, "top", 100, "Maximum listing pages to fetch this run (never-read listings first, then the oldest reads)")
	cmd.Flags().StringVar(&flagRetryAfter, "retry-after", "7d", "Do not re-read a page fetched more recently than this that still lacks the field (e.g. 7d, 24h)")
	cmd.Flags().Float64Var(&flagRate, "rate", 2, "Requests per second (0 = client default)")
	cmd.Flags().StringVar(&flagPostcode, "postcode", "", "Only listings in these postal codes, comma-separated")
	cmd.Flags().StringVar(&flagDeal, "deal", "", "Only this deal (sale, rent, public-sale, colocation)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	_ = cmd.Flags().MarkHidden("db")
	return cmd
}

func printEnrich(cmd *cobra.Command, flags *rootFlags, rep enrichReport) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printView(cmd.OutOrStdout(), rep, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "enrich: %d selected · %d fetched · %d updated · %d gone · %d errors · %d remaining\n", rep.Selected, rep.Fetched, rep.Updated, rep.NotFound, rep.Errors, rep.Remaining)
	if rep.Note != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), rep.Note)
	}
	for _, f := range rep.Failures {
		fmt.Fprintln(cmd.ErrOrStderr(), "  "+f)
	}
	return nil
}
