// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/twilio/internal/store"

	"github.com/spf13/cobra"
)

// pp:client-call — this command performs real API calls to /Usage/Records/{period}.json
// per subaccount. The cross-subaccount aggregation is the join Twilio does not
// offer in any single API endpoint.
func newSubaccountSpendCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var period string
	var concurrency int
	var asCSV bool

	cmd := &cobra.Command{
		Use:   "subaccount-spend",
		Short: "One CSV with every subaccount's spend pivoted across SMS, MMS, voice, recording categories",
		Long: `Walks the synced accounts table to find every subaccount under the master
credential, then calls /Usage/Records/{period}.json per subaccount via the
real client. Pivots each row's category into a column so the output is one
row per subaccount, one column per category.

The cross-subaccount aggregation is the join Twilio does not offer in any
single API endpoint. Steampipe can do it with a Postgres FDW; this command
does it with one binary call.

Run 'twilio-pp-cli sync --resources accounts' first to discover the subaccount
list. Then set TWILIO_ACCOUNT_SID + TWILIO_AUTH_TOKEN (or the master Account
SID's API key) so the run has permission to query each subaccount.`,
		Example: `  twilio-pp-cli subaccount-spend --period last-month --csv > march.csv
  twilio-pp-cli subaccount-spend --period today --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			period = normalizeUsagePeriod(period)
			if dbPath == "" {
				dbPath = defaultDBPath("twilio-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			// Pull every subaccount Sid (and master Sid) from the local store.
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT
					json_extract(data, '$.sid') AS sid,
					COALESCE(json_extract(data, '$.friendly_name'), '') AS friendly_name
				FROM accounts_json
				WHERE json_extract(data, '$.status') = 'active'
				ORDER BY sid`)
			if err != nil {
				// Fall back to the master account if accounts_json hasn't been synced.
				masterSid := getAccountSidFromConfig(flags)
				if masterSid == "" {
					return fmt.Errorf("no synced accounts and no master Account SID configured")
				}
				return runSpendForOne(cmd, flags, []spendTarget{{Sid: masterSid}}, period, asCSV)
			}
			defer rows.Close()
			var targets []spendTarget
			for rows.Next() {
				var t spendTarget
				if err := rows.Scan(&t.Sid, &t.FriendlyName); err != nil {
					return err
				}
				targets = append(targets, t)
			}
			if len(targets) == 0 {
				masterSid := getAccountSidFromConfig(flags)
				if masterSid != "" {
					targets = []spendTarget{{Sid: masterSid}}
				}
			}
			if concurrency < 1 {
				concurrency = 4
			}
			return runSpendForOne(cmd, flags, targets, period, asCSV)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&period, "period", "last-month", "Usage period: today | this-month | last-month | yearly | all-time")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Parallel subaccount queries")
	cmd.Flags().BoolVar(&asCSV, "csv", false, "Emit as CSV with subaccounts as rows and categories as columns")
	return cmd
}

type spendTarget struct {
	Sid          string
	FriendlyName string
}

type spendRow struct {
	AccountSid   string             `json:"account_sid"`
	FriendlyName string             `json:"friendly_name,omitempty"`
	Categories   map[string]float64 `json:"categories"`
	Total        float64            `json:"total"`
	PriceUnit    string             `json:"price_unit,omitempty"`
}

func normalizeUsagePeriod(p string) string {
	// Twilio's URL segments: /Usage/Records/Today.json, ThisMonth.json, LastMonth.json,
	// AllTime.json, Daily.json, Monthly.json, Yearly.json, etc.
	switch p {
	case "today":
		return "Today"
	case "this-month":
		return "ThisMonth"
	case "last-month":
		return "LastMonth"
	case "all-time":
		return "AllTime"
	case "daily":
		return "Daily"
	case "monthly":
		return "Monthly"
	case "yearly":
		return "Yearly"
	}
	return p
}

func runSpendForOne(cmd *cobra.Command, flags *rootFlags, targets []spendTarget, period string, asCSV bool) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}

	results := make([]spendRow, len(targets))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	categories := map[string]struct{}{}
	var catMu sync.Mutex

	for i, t := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, t spendTarget) {
			defer wg.Done()
			defer func() { <-sem }()

			path := fmt.Sprintf("/2010-04-01/Accounts/%s/Usage/Records/%s.json", t.Sid, period)
			data, err := c.Get(path, nil)
			if err != nil {
				results[i] = spendRow{AccountSid: t.Sid, FriendlyName: t.FriendlyName,
					Categories: map[string]float64{"_error": -1}}
				return
			}
			var env struct {
				UsageRecords []map[string]json.RawMessage `json:"usage_records"`
			}
			if err := json.Unmarshal(data, &env); err != nil {
				return
			}
			cat := map[string]float64{}
			var priceUnit string
			var total float64
			for _, rec := range env.UsageRecords {
				var category string
				_ = json.Unmarshal(rec["category"], &category)
				var priceStr string
				_ = json.Unmarshal(rec["price"], &priceStr)
				price, _ := strconv.ParseFloat(priceStr, 64)
				if price < 0 {
					price = -price
				}
				cat[category] += price
				total += price
				if priceUnit == "" {
					_ = json.Unmarshal(rec["price_unit"], &priceUnit)
				}
				catMu.Lock()
				categories[category] = struct{}{}
				catMu.Unlock()
			}
			results[i] = spendRow{
				AccountSid:   t.Sid,
				FriendlyName: t.FriendlyName,
				Categories:   cat,
				Total:        total,
				PriceUnit:    priceUnit,
			}
		}(i, t)
	}
	wg.Wait()

	if asCSV {
		return writeSpendCSV(cmd.OutOrStdout(), results, categories)
	}
	envelope := map[string]any{
		"period":           period,
		"subaccount_count": len(results),
		"rows":             results,
	}
	return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
}

func writeSpendCSV(w interface{ Write([]byte) (int, error) }, rows []spendRow, cats map[string]struct{}) error {
	var sortedCats []string
	for c := range cats {
		sortedCats = append(sortedCats, c)
	}
	sort.Strings(sortedCats)

	// Convert w to io.Writer
	type writer interface {
		Write([]byte) (int, error)
	}
	cw := csv.NewWriter(&csvWriterAdapter{w: w})
	header := []string{"account_sid", "friendly_name", "total"}
	header = append(header, sortedCats...)
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range rows {
		rec := []string{
			r.AccountSid,
			r.FriendlyName,
			strconv.FormatFloat(r.Total, 'f', 4, 64),
		}
		for _, c := range sortedCats {
			rec = append(rec, strconv.FormatFloat(r.Categories[c], 'f', 4, 64))
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

type csvWriterAdapter struct {
	w interface{ Write([]byte) (int, error) }
}

func (a *csvWriterAdapter) Write(p []byte) (int, error) { return a.w.Write(p) }
