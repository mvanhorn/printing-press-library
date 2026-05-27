// Copyright 2026 rushyant-m. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/store"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newBSESyncCmd replaces the generated no-scrip sync. The BSE announcements
// feed returns nothing without a scrip code, so this command iterates the
// portfolio holdings and pulls each one's announcements over a date window.
func newBSESyncCmd(flags *rootFlags) *cobra.Command {
	var scrip string
	var since string
	var maxPages int

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync announcements for every holding (or one --scrip) into the local store.",
		Long: strings.Trim(`
Sync BSE corporate announcements into the local SQLite store, scoped to your
portfolio holdings. The BSE announcements feed returns nothing without a scrip
code, so sync walks each holding and pages through its filings over the window.

Run 'holdings list' to see (and edit) the portfolio that sync iterates.`, "\n"),
		Example: strings.Trim(`
  # Sync all holdings over the last year
  bse-filings-pp-cli sync

  # Sync one holding over the last 90 days
  bse-filings-pp-cli sync --scrip 500325 --since 90d`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			window, err := parseSinceDuration(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
			}
			from := ymd(window)
			to := ymd(time.Now())

			s, err := openBSEStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := seedHoldingsIfEmpty(s); err != nil {
				return err
			}

			// Build the list of scrips to sync.
			var scrips []store.Holding
			if scrip != "" {
				scrips = []store.Holding{{ScripCode: strings.TrimSpace(scrip)}}
			} else {
				scrips, err = s.ListHoldings()
				if err != nil {
					return err
				}
			}

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				for _, h := range scrips {
					fmt.Fprintf(cmd.ErrOrStderr(), "would sync announcements for scrip %s (%s..%s)\n", h.ScripCode, from, to)
				}
				return flags.printJSON(cmd, map[string]any{"status": "dry_run", "scrips": len(scrips), "from": from, "to": to})
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			type holdingResult struct {
				ScripCode string `json:"scrip_code"`
				Stored    int    `json:"stored"`
				Error     string `json:"error,omitempty"`
			}
			var results []holdingResult
			total := 0
			for _, h := range scrips {
				stored, serr := syncHoldingAnnouncements(c, s, h.ScripCode, from, to, maxPages)
				hr := holdingResult{ScripCode: h.ScripCode, Stored: stored}
				if serr != nil {
					hr.Error = serr.Error()
				} else {
					_ = s.TouchHoldingSync(h.ScripCode)
				}
				total += stored
				results = append(results, hr)
				if !flags.asJSON {
					if serr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "  %s: error: %v\n", h.ScripCode, serr)
					} else {
						fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %d filings stored\n", h.ScripCode, stored)
					}
				}
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Sync complete: %d filings across %d holding(s)\n", total, len(scrips))
			return flags.printJSON(cmd, map[string]any{
				"total_stored": total,
				"holdings":     len(scrips),
				"results":      results,
			})
		},
	}
	cmd.Flags().StringVar(&scrip, "scrip", "", "Sync only this scrip code (default: every holding).")
	cmd.Flags().StringVar(&since, "since", "365d", "Window to fetch, e.g. 90d, 24h, 1w (default 365d).")
	cmd.Flags().IntVar(&maxPages, "max-pages", 20, "Maximum pages of announcements to fetch per holding.")
	return cmd
}

// syncHoldingAnnouncements pages through one scrip's announcements and upserts
// each page's Table array. Returns the count of rows stored.
func syncHoldingAnnouncements(c interface {
	Get(string, map[string]string) (json.RawMessage, error)
}, s *store.Store, scrip, from, to string, maxPages int) (int, error) {
	if maxPages <= 0 {
		maxPages = 20
	}
	stored := 0
	for page := 1; page <= maxPages; page++ {
		params := map[string]string{
			"strScrip":    scrip,
			"strPrevDate": from,
			"strToDate":   to,
			"strType":     "C",
			"strSearch":   "P",
			"strCat":      "-1",
			"pageno":      fmt.Sprintf("%d", page),
		}
		data, err := c.Get("/AnnSubCategoryGetData/w", params)
		if err != nil {
			return stored, err
		}
		var env struct {
			Table []json.RawMessage `json:"Table"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			// No-scrip / empty responses come back as {} — nothing to do.
			break
		}
		if len(env.Table) == 0 {
			break
		}
		// Batch-upsert the page. UpsertBatch resolves the announcements
		// primary key via the NEWSID override; the single-record
		// UpsertAnnouncements path only knows the generic id/name keys and
		// would drop every BSE row (no lowercase "id" field present).
		n, _, err := s.UpsertBatch("announcements", env.Table)
		if err != nil {
			return stored, err
		}
		stored += n
		// BSE returns the same fixed page size; a short page means we're done.
		if len(env.Table) < 50 {
			break
		}
	}
	return stored, nil
}
