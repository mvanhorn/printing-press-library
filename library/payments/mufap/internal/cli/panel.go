// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/mufap"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
	"github.com/spf13/cobra"
)

// panelView is one panel query's result.
//
// UniverseWidth counts distinct funds in the UNFILTERED date range, and is
// reported alongside the filtered count on purpose: a caller who asks for one
// category and gets four rows otherwise cannot tell "four funds in that
// category" from "four funds in the entire mirror", and would read a
// half-backfilled store as a collapsed industry.
//
// UniverseWidthReliable is false on the two tabs MUFAP does not date-filter,
// where the row count describes current reference data instead of the
// requested date. Unreadable counts stored rows whose payload no longer
// decodes, so a partially corrupt mirror shows as a loss rather than as a
// genuinely narrower industry.
type panelView struct {
	Tab                   string           `json:"tab"`
	Resource              string           `json:"resource"`
	From                  string           `json:"from,omitempty"`
	To                    string           `json:"to,omitempty"`
	Dates                 int              `json:"dates"`
	UniverseWidth         int              `json:"universe_width"`
	UniverseWidthReliable bool             `json:"universe_width_reliable"`
	Unreadable            int              `json:"unreadable_rows"`
	Matched               int              `json:"matched"`
	Returned              int              `json:"returned"`
	Limit                 int              `json:"limit"`
	Truncated             bool             `json:"truncated"`
	Rows                  []map[string]any `json:"rows"`
}

func newNovelPanelCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagTab string
	var flagCategory string
	var flagSector string
	var flagFund string
	var flagLimit int
	var flagDB string
	var flagRawValues bool

	cmd := &cobra.Command{
		Use:   "panel",
		Short: "Query the stored daily NAV and return panel by date, fund, category or sector.",
		Long: "Query the local mirror's daily panel across a date range.\n\n" +
			"Reads only what `backfill daily` already stored -- it never calls MUFAP -- so a\n" +
			"range that returns nothing means the mirror is thin there, not that the industry\n" +
			"was quiet. Every result carries universe_width, the number of distinct funds in\n" +
			"the unfiltered range, so a narrow filter is never mistaken for a narrow universe.\n\n" +
			"--tab pricing and --tab ter are current reference data rather than a dated panel,\n" +
			"so their row counts are not a universe width for the requested dates; both are\n" +
			"flagged universe_width_reliable=false and carry a note in the human summary.\n\n" +
			"Rows always carry date, fund, category and sector, plus every other column of the\n" +
			"stored tab verbatim (NAV, YTD, 30 Days, ... for --tab returns).\n\n" +
			"MUFAP writes every negative in ACCOUNTING NOTATION and never with a minus sign --\n" +
			"\"(4.97)\" means -4.97, and 96 of 388 YTD cells were parenthesised on 2026-09-04 --\n" +
			"so numeric cells are DECODED by default: they come back as real JSON numbers, and\n" +
			"with a leading minus in the table. A cell that is not a number (\"N/A\", \"-\", fund\n" +
			"names, ratings, \"Sep 04, 2026\") passes through as the text MUFAP published, and\n" +
			"the decision is made per value rather than per column. --raw-values turns the\n" +
			"decoding off and returns every cell exactly as MUFAP published it.",
		Example: "  mufap-pp-cli panel --from 2026-09-01 --to 2026-09-04 --limit 5\n" +
			"  mufap-pp-cli panel --from 2026-09-01 --to 2026-09-04 --category \"Money Market\" --agent\n" +
			"  mufap-pp-cli panel --from 2026-09-04 --to 2026-09-04 --fund \"ABL Cash\" --json --select date,fund,NAV\n" +
			"  mufap-pp-cli panel --tab nav --from 2026-09-01 --to 2026-09-04 --sector \"Open-End\"",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "--from=2026-09-01;--to=2026-09-04;--limit=5",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "panel")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			tab := strings.ToLower(strings.TrimSpace(flagTab))
			if tab == "" {
				tab = "returns"
			}
			knownTab := false
			for _, t := range MUFAPDailyTabNames {
				if t == tab {
					knownTab = true
					break
				}
			}
			if !knownTab {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown --tab %q: want one of %s", flagTab, strings.Join(MUFAPDailyTabNames, ", ")))
			}
			if flagLimit <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be greater than 0"))
			}

			// MUFAP's display form ("Sep 04, 2026") is accepted as well as ISO
			// because that is what the row's own date column shows ("Validity
			// Date" on every tab but payout, whose column is "Payout Date"),
			// and it is what a caller copies out of a previous result. The
			// store keys on ISO, and a lexical range compare against the
			// display form matches nothing at all rather than erroring, so it
			// must be converted here.
			panelISODate := func(name, v string) (string, error) {
				v = strings.TrimSpace(v)
				if v == "" {
					return "", nil
				}
				if iso, ok := mufap.NormalizeValidityDate(v); ok {
					return iso, nil
				}
				if _, err := time.Parse("2006-01-02", v); err != nil {
					return "", fmt.Errorf("--%s %q is not a date: want YYYY-MM-DD", name, v)
				}
				return v, nil
			}
			from, err := panelISODate("from", flagFrom)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			to, err := panelISODate("to", flagTo)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if from != "" && to != "" && from > to {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from %s is after --to %s", from, to))
			}

			dbPath := strings.TrimSpace(flagDB)
			if dbPath == "" {
				dbPath = defaultDBPath("mufap-pp-cli")
			}

			resource := mufapResourceForTab(tab)
			// Measured 2026-09-04: tab=pricing and tab=ter return 551 rows for
			// an explicit date while tab=returns returns 388, because those two
			// are current reference data and are not date-filtered. Their row
			// count must never be read as a universe width for the requested
			// range, so the caveat travels with every payload, populated even
			// on the empty paths where the width is 0.
			widthReliable := MUFAPTabIsDateFiltered(tab)
			// Shaped like a populated result so a machine caller parses one
			// schema whether or not the mirror exists yet.
			panelEmpty := func() panelView {
				return panelView{
					Tab:                   tab,
					Resource:              resource,
					From:                  from,
					To:                    to,
					UniverseWidthReliable: widthReliable,
					Limit:                 flagLimit,
					Rows:                  make([]map[string]any, 0),
				}
			}

			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: mufap-pp-cli backfill daily --from <date> --to <date>\n", dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), panelEmpty(), flags)
				}
				return nil
			}

			st, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()

			// The mirror file is shared with the platform's own tables, so it
			// can exist with no MUFAP schema at all (a `teach` run creates it).
			// A read-only handle cannot create the schema, and LoadMUFAPObs
			// would surface that as a raw "no such table: mufap_obs" instead of
			// the one command that fixes it.
			var mirrorTable sql.NullString
			if scanErr := st.DB().QueryRowContext(ctx,
				`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'mufap_obs'`,
			).Scan(&mirrorTable); scanErr != nil {
				if !errors.Is(scanErr, sql.ErrNoRows) {
					return fmt.Errorf("panel: reading mirror schema: %w", scanErr)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "no MUFAP observations in %s\nrun: mufap-pp-cli backfill daily --from <date> --to <date>\n", dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), panelEmpty(), flags)
				}
				return nil
			}

			obs, err := store.LoadMUFAPObs(ctx, st, resource, from, to)
			if err != nil {
				return err
			}

			panelFold := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
			wantCategory := panelFold(flagCategory)
			wantSector := panelFold(flagSector)
			wantFund := panelFold(flagFund)

			view := panelEmpty()
			universe := make(map[string]bool)
			dates := make(map[string]bool)
			// Counted so the human summary can disclose the decoding rather
			// than silently changing the sign a reader sees.
			panelNegatives := 0
			for _, o := range obs {
				payload := make(map[string]string)
				if err := json.Unmarshal([]byte(o.Payload), &payload); err != nil {
					// One unreadable payload must not fail an otherwise good
					// range, but dropping it silently would shrink the very
					// aggregate this command exists to report and read as a
					// narrower industry, so it is counted instead.
					view.Unreadable++
					continue
				}
				// MUFAP labels this column "Fund Name" on tab=returns and
				// plain "Fund" on nav, pricing, payout and ter, so the header
				// is probed rather than named; reading only "Fund Name" left
				// four of the five tabs falling back to the store's row key.
				fund := MUFAPRowName(payload)
				if fund == "" {
					fund = o.Key
				}
				category := payload["Category"]
				sector := payload["Sector"]

				// Universe width is counted before the filters so it describes
				// the mirror, not the query, and it counts the store's row key
				// -- unique per (resource, date) -- rather than the display
				// name. Measured 2026-09-04, tab=returns: 388 rows carry only
				// 339 distinct names, because VPS pension funds legitimately
				// reuse one name across three sub-fund series ("ABL Pension
				// Fund" is VPS-Money Market, VPS-Debt and VPS-Equity). Keying
				// on the name undercounts by 12.6% and disagrees with the
				// `universe` command, which counts these same rows by key.
				dates[o.Date] = true
				universe[o.Key] = true

				if wantCategory != "" && !strings.Contains(panelFold(category), wantCategory) {
					continue
				}
				if wantSector != "" && !strings.Contains(panelFold(sector), wantSector) {
					continue
				}
				if wantFund != "" && !strings.Contains(panelFold(fund), wantFund) {
					continue
				}
				view.Matched++
				if len(view.Rows) >= flagLimit {
					continue
				}
				row := map[string]any{
					"date":     o.Date,
					"fund":     fund,
					"category": category,
					"sector":   sector,
				}
				for k, v := range payload {
					switch k {
					case "Fund Name", "Fund", "Category", "Sector", "date", "fund", "category", "sector":
						// Already promoted to the canonical keys above. Both
						// spellings of the name column are skipped so the
						// non-returns tabs do not carry a redundant verbatim
						// "Fund" alongside the promoted "fund".
						continue
					}
					// MUFAP publishes "(4.97)" for -4.97 and never uses a
					// minus sign, so a cell passed through as text reads as
					// a POSITIVE number to anything that strips the
					// punctuation; see dumpPanelDecodeCell.
					if !flagRawValues && strings.HasPrefix(strings.TrimSpace(v), "(") {
						if _, isNum := mufap.ParseNumber(v); isNum {
							panelNegatives++
						}
					}
					row[k] = dumpPanelDecodeCell(v, flagRawValues)
				}
				view.Rows = append(view.Rows, row)
			}
			view.Dates = len(dates)
			view.UniverseWidth = len(universe)
			view.Returned = len(view.Rows)
			view.Truncated = view.Matched > view.Returned

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			out := cmd.OutOrStdout()
			rangeLabel := "the whole mirror"
			switch {
			case from != "" && to != "":
				rangeLabel = from + ".." + to
			case from != "":
				rangeLabel = "on or after " + from
			case to != "":
				rangeLabel = "on or before " + to
			}
			panelWidthNote := func() {
				if !view.UniverseWidthReliable {
					fmt.Fprintf(out, "note: --tab %s is current reference data, not a dated panel, so its width is not the universe for %s\n", tab, rangeLabel)
				}
				if view.Unreadable > 0 {
					fmt.Fprintf(out, "note: %d stored row(s) did not decode and are absent from every count above\n", view.Unreadable)
					fmt.Fprintf(out, "re-fetch them: mufap-pp-cli backfill daily --tab %s --from <date> --to <date> --force\n", tab)
				}
				if panelNegatives > 0 {
					fmt.Fprintf(out, "note: %d cell(s) MUFAP publishes in accounting notation -- \"(4.97)\" -- are shown as -4.97; --raw-values keeps MUFAP's text\n", panelNegatives)
				}
			}

			if view.Returned == 0 {
				if view.UniverseWidth == 0 {
					if view.Unreadable > 0 {
						// A wholly undecodable range is a mirror-format
						// mismatch, not an empty industry, and must not be
						// reported as a gap in MUFAP's publishing.
						fmt.Fprintf(out, "no readable %s rows for %s: all %d stored row(s) failed to decode\n", tab, rangeLabel, view.Unreadable)
						fmt.Fprintf(out, "re-fetch them: mufap-pp-cli backfill daily --tab %s --from <date> --to <date> --force\n", tab)
						return nil
					}
					// MUFAP publishes nothing on weekends and holidays, so an
					// empty range is as likely to be the calendar as a gap.
					fmt.Fprintf(out, "no %s rows stored for %s\n", tab, rangeLabel)
					fmt.Fprintf(out, "check what was fetched: mufap-pp-cli coverage --resource %s\n", resource)
					fmt.Fprintf(out, "then fill the gap:      mufap-pp-cli backfill daily --tab %s --from <date> --to <date>\n", tab)
					return nil
				}
				fmt.Fprintf(out, "no rows matched, but %d funds reported over %d dates in %s\n", view.UniverseWidth, view.Dates, rangeLabel)
				fmt.Fprintf(out, "the filters, not the mirror, emptied this result; widen --category/--sector/--fund\n")
				panelWidthNote()
				return nil
			}

			// Column order for --tab returns; other tabs fall back to their own
			// stored columns, sorted, because their headers differ.
			first := view.Rows[0]
			firstHeaders := make([]string, 0, len(first))
			for k := range first {
				firstHeaders = append(firstHeaders, k)
			}
			sort.Strings(firstHeaders)

			extras := make([]string, 0, 8)
			picked := make(map[string]bool)
			addExtra := func(k string) {
				if k == "" || picked[k] {
					return
				}
				picked[k] = true
				extras = append(extras, k)
			}
			// The row's own observation-date column leads. tab=payout has no
			// "Validity Date" column at all -- its date is "Payout Date" -- so
			// the header is probed instead of named, or payout rows would fall
			// through to the generic fallback and lose their date entirely.
			addExtra(MUFAPValidityColumn(firstHeaders))
			valueCols := 0
			for _, want := range []string{"NAV", "YTD", "MTD", "1 Day", "30 Days", "90 Days", "365 Days"} {
				if _, ok := first[want]; ok {
					addExtra(want)
					valueCols++
				}
			}
			if valueCols == 0 {
				for _, k := range firstHeaders {
					switch k {
					case "date", "fund", "category", "sector":
						continue
					}
					if len(extras) >= 6 {
						break
					}
					addExtra(k)
				}
			}

			// A decoded numeric cell is a json.Number, not a string, so the
			// table renders by type: reading only .(string) would print "-"
			// -- this CLI's mark for "did not report" -- over every number.
			panelCellText := func(v any) string {
				switch t := v.(type) {
				case nil:
					return "-"
				case string:
					if t == "" {
						return "-"
					}
					return t
				case json.Number:
					return t.String()
				default:
					return fmt.Sprintf("%v", t)
				}
			}

			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, strings.Join(append([]string{"DATE", "FUND", "CATEGORY", "SECTOR"}, extras...), "\t"))
			for _, r := range view.Rows {
				cells := make([]string, 0, 4+len(extras))
				for _, k := range []string{"date", "fund", "category", "sector"} {
					cells = append(cells, panelCellText(r[k]))
				}
				for _, k := range extras {
					cells = append(cells, panelCellText(r[k]))
				}
				fmt.Fprintln(tw, strings.Join(cells, "\t"))
			}
			if err := tw.Flush(); err != nil {
				return err
			}

			fmt.Fprintf(out, "\n%d of %d matching rows; %d distinct funds reported across %d dates in %s\n",
				view.Returned, view.Matched, view.UniverseWidth, view.Dates, rangeLabel)
			panelWidthNote()
			if view.Truncated {
				fmt.Fprintf(out, "raise --limit (now %d) to see the rest\n", flagLimit)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Earliest observation date, inclusive: the row's Validity Date, or its Payout Date on --tab payout. ISO YYYY-MM-DD (\"Sep 04, 2026\" also accepted). Open-ended if unset.")
	cmd.Flags().StringVar(&flagTo, "to", "", "Latest observation date, inclusive: the row's Validity Date, or its Payout Date on --tab payout. ISO YYYY-MM-DD (\"Sep 04, 2026\" also accepted). Open-ended if unset.")
	cmd.Flags().StringVar(&flagTab, "tab", "returns", "Stored daily tab: "+strings.Join(MUFAPDailyTabNames, ", ")+". pricing and ter are reference data, not a dated panel.")
	cmd.Flags().StringVar(&flagCategory, "category", "", "Keep rows whose Category contains this text (case-insensitive), e.g. \"Money Market\".")
	cmd.Flags().StringVar(&flagSector, "sector", "", "Keep rows whose Sector contains this text (case-insensitive), e.g. \"Open-End\".")
	cmd.Flags().StringVar(&flagFund, "fund", "", "Keep rows whose fund name contains this text (case-insensitive), e.g. \"ABL Cash\". The column is \"Fund Name\" on --tab returns and \"Fund\" on the other tabs.")
	cmd.Flags().IntVar(&flagLimit, "limit", 200, "Maximum rows to return. Caps output only; universe_width still counts the whole range.")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local mirror. Defaults to the app data directory.")
	cmd.Flags().BoolVar(&flagRawValues, "raw-values", false, "Return cells as MUFAP's own text, accounting negatives included (\"(4.97)\" for -4.97). Off by default: numeric cells are decoded to real numbers.")
	return cmd
}
