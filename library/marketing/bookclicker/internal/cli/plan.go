// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cross-list launch planner.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/bookclicker/internal/store"

	"github.com/spf13/cobra"
)

// planCandidate is one newsletter scored against a launch window.
type planCandidate struct {
	ListID          int64    `json:"list_id"`
	Name            string   `json:"name"`
	PenName         string   `json:"pen_name"`
	Platform        string   `json:"platform"`
	Members         int64    `json:"active_member_count"`
	OpenRate        float64  `json:"open_rate"`
	ClickRate       float64  `json:"click_rate"`
	InvType         string   `json:"inv_type"`
	Price           *int64   `json:"price"`
	SwapOnly        bool     `json:"swap_only"`
	EstimatedOpens  float64  `json:"estimated_opens"`
	OpensPerDollar  *float64 `json:"opens_per_dollar"`
	AvailableDays   []string `json:"available_days"`
	MatchingDates   []string `json:"matching_dates"`
	MatchingDateCnt int      `json:"matching_date_count"`
}

type planResult struct {
	Book       string          `json:"book,omitempty"`
	From       string          `json:"from"`
	To         string          `json:"to"`
	Rank       string          `json:"rank"`
	InvType    string          `json:"inv_type"`
	Scanned    int             `json:"scanned_lists"`
	Candidates []planCandidate `json:"candidates"`
	Note       string          `json:"note,omitempty"`
}

var planWeekdayCols = []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}

func newNovelPlanCmd(flags *rootFlags) *cobra.Command {
	var (
		flagBook      string
		flagFrom      string
		flagTo        string
		flagMaxPrice  int
		flagInvType   string
		flagRank      string
		flagLimit     int
		flagMinMember int
		flagMinOpen   float64
		flagSwapOnly  bool
		flagPaidOnly  bool
		dbPath        string
	)

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Find every newsletter that can run your book in a date window, ranked by fit.",
		Long: "Rank every synced newsletter that can promote a book inside a date window.\n\n" +
			"Reads the local mirror, so run 'sync' first. Use this instead of paging the\n" +
			"marketplace one calendar at a time. To inspect a single newsletter, use 'lists get'.",
		Example:     "  bookclicker-pp-cli plan --from 2026-09-01 --to 2026-09-30 --max-price 25 --rank value --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "plan")
			}
			if flagFrom == "" || flagTo == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from and --to are required (YYYY-MM-DD)"))
			}
			from, err := time.Parse("2006-01-02", flagFrom)
			if err != nil {
				return usageErr(fmt.Errorf("--from must be YYYY-MM-DD: %w", err))
			}
			to, err := time.Parse("2006-01-02", flagTo)
			if err != nil {
				return usageErr(fmt.Errorf("--to must be YYYY-MM-DD: %w", err))
			}
			if to.Before(from) {
				return usageErr(fmt.Errorf("--to (%s) is before --from (%s)", flagTo, flagFrom))
			}
			switch flagRank {
			case "value", "reach", "price":
			default:
				return usageErr(fmt.Errorf("--rank must be one of: value, reach, price"))
			}
			switch flagInvType {
			case "solo", "feature", "mention":
			default:
				return usageErr(fmt.Errorf("--inv-type must be one of: solo, feature, mention"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("bookclicker-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: bookclicker-pp-cli sync --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), planResult{
						From: flagFrom, To: flagTo, Rank: flagRank, InvType: flagInvType,
						Candidates: make([]planCandidate, 0),
						Note:       "no local mirror; run sync first",
					}, flags)
				}
				return nil
			}

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			priceCol := flagInvType + "_price"
			swapCol := flagInvType + "_is_swap_only"
			query := fmt.Sprintf(`
				SELECT id, name, adopted_pen_name, platform, active_member_count,
				       open_rate, click_rate, %s, %s, data
				FROM lists`, priceCol, swapCol)

			rows, err := db.DB().QueryContext(ctx, query)
			if err != nil {
				return fmt.Errorf("querying lists: %w", err)
			}

			type rawRow struct {
				id       sql.NullString
				name     sql.NullString
				penName  sql.NullString
				platform sql.NullString
				members  sql.NullInt64
				openRate sql.NullString
				clickRte sql.NullString
				price    sql.NullInt64
				swapOnly sql.NullInt64
				data     sql.NullString
			}
			raws := make([]rawRow, 0)
			for rows.Next() {
				var r rawRow
				if err := rows.Scan(&r.id, &r.name, &r.penName, &r.platform, &r.members,
					&r.openRate, &r.clickRte, &r.price, &r.swapOnly, &r.data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning list row: %w", err)
				}
				raws = append(raws, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating lists: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing rows: %w", err)
			}

			windowDays := planWindowWeekdays(from, to)
			out := make([]planCandidate, 0, len(raws))
			for _, r := range raws {
				swap := r.swapOnly.Int64 == 1
				if flagSwapOnly && !swap {
					continue
				}
				if flagPaidOnly && swap {
					continue
				}
				if !swap && flagMaxPrice > 0 && r.price.Valid && r.price.Int64 > int64(flagMaxPrice) {
					continue
				}
				if flagMinMember > 0 && r.members.Int64 < int64(flagMinMember) {
					continue
				}
				openRate := planParseRate(r.openRate.String)
				if flagMinOpen > 0 && openRate < flagMinOpen {
					continue
				}

				days := planOfferedWeekdays(r.data.String, flagInvType)
				if len(days) == 0 {
					continue
				}
				dates := planMatchingDates(windowDays, days)
				if len(dates) == 0 {
					continue
				}

				listID, _ := strconv.ParseInt(strings.TrimSpace(r.id.String), 10, 64)
				estOpens := float64(r.members.Int64) * openRate
				cand := planCandidate{
					ListID:          listID,
					Name:            r.name.String,
					PenName:         r.penName.String,
					Platform:        r.platform.String,
					Members:         r.members.Int64,
					OpenRate:        openRate,
					ClickRate:       planParseRate(r.clickRte.String),
					InvType:         flagInvType,
					SwapOnly:        swap,
					EstimatedOpens:  estOpens,
					AvailableDays:   planDayNames(days),
					MatchingDates:   dates,
					MatchingDateCnt: len(dates),
				}
				if r.price.Valid {
					p := r.price.Int64
					cand.Price = &p
					if p > 0 {
						v := estOpens / float64(p)
						cand.OpensPerDollar = &v
					}
				}
				out = append(out, cand)
			}

			sort.SliceStable(out, func(i, j int) bool {
				switch flagRank {
				case "reach":
					return out[i].EstimatedOpens > out[j].EstimatedOpens
				case "price":
					return planPriceOrZero(out[i].Price) < planPriceOrZero(out[j].Price)
				default: // value
					return planValueScore(out[i]) > planValueScore(out[j])
				}
			})
			if flagLimit > 0 && len(out) > flagLimit {
				out = out[:flagLimit]
			}

			res := planResult{
				Book: flagBook, From: flagFrom, To: flagTo, Rank: flagRank,
				InvType: flagInvType, Scanned: len(raws), Candidates: out,
			}
			if len(out) == 0 {
				res.Note = fmt.Sprintf("scanned %d newsletters; none offer %s in this window under the given filters", len(raws), flagInvType)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), res.Note)
				return nil
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%-38s %-10s %8s %7s %7s %6s %5s\n", "NEWSLETTER", "PLATFORM", "MEMBERS", "OPEN%", "OPENS", "PRICE", "DATES")
			for _, c := range out {
				price := "swap"
				if !c.SwapOnly && c.Price != nil {
					price = "$" + strconv.FormatInt(*c.Price, 10)
				}
				fmt.Fprintf(w, "%-38s %-10s %8d %6.1f%% %7.0f %6s %5d\n",
					planTrunc(c.Name, 38), planTrunc(c.Platform, 10), c.Members,
					c.OpenRate*100, c.EstimatedOpens, price, c.MatchingDateCnt)
			}
			fmt.Fprintf(w, "\nShowing %d of %d scanned newsletters. To narrow: --max-price, --min-members, --min-open-rate, --limit.\n", len(out), len(raws))
			return nil
		},
	}

	cmd.Flags().StringVar(&flagBook, "book", "", "Book id this launch is for (recorded in output)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Launch window start (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Launch window end (YYYY-MM-DD)")
	cmd.Flags().IntVar(&flagMaxPrice, "max-price", 0, "Skip paid newsletters above this price (0 = no cap)")
	cmd.Flags().StringVar(&flagInvType, "inv-type", "solo", "Promotion type: solo, feature, or mention")
	cmd.Flags().StringVar(&flagRank, "rank", "value", "Ranking: value (opens per dollar), reach (estimated opens), or price")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum newsletters to return")
	cmd.Flags().IntVar(&flagMinMember, "min-members", 0, "Skip newsletters below this subscriber count")
	cmd.Flags().Float64Var(&flagMinOpen, "min-open-rate", 0, "Skip newsletters below this open rate (0.25 = 25%)")
	cmd.Flags().BoolVar(&flagSwapOnly, "swap-only", false, "Only newsletters that swap rather than sell")
	cmd.Flags().BoolVar(&flagPaidOnly, "paid-only", false, "Only newsletters that sell paid promotions")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// planParseRate converts Bookclicker's string rates ("0.2402") to a float.
func planParseRate(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

// planOfferedWeekdays returns the weekday indexes a list offers for invType,
// read from the embedded inventories array on the list's JSON payload.
func planOfferedWeekdays(data, invType string) map[int]bool {
	out := map[int]bool{}
	if data == "" {
		return out
	}
	var payload struct {
		Inventories []map[string]any `json:"inventories"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return out
	}
	for _, inv := range payload.Inventories {
		t, _ := inv["inv_type"].(string)
		if !strings.EqualFold(t, invType) {
			continue
		}
		for i, col := range planWeekdayCols {
			if planTruthy(inv[col]) {
				out[i] = true
			}
		}
	}
	return out
}

func planTruthy(v any) bool {
	switch t := v.(type) {
	case float64:
		return t == 1
	case int:
		return t == 1
	case bool:
		return t
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	}
	return false
}

// planWindowWeekdays expands a date range into (date, weekday) pairs, capped
// so an absurd range cannot blow up memory.
func planWindowWeekdays(from, to time.Time) map[string]int {
	out := map[string]int{}
	const maxDays = 366
	n := 0
	for d := from; !d.After(to) && n < maxDays; d = d.AddDate(0, 0, 1) {
		out[d.Format("2006-01-02")] = int(d.Weekday())
		n++
	}
	return out
}

func planMatchingDates(window map[string]int, offered map[int]bool) []string {
	dates := make([]string, 0)
	for date, wd := range window {
		if offered[wd] {
			dates = append(dates, date)
		}
	}
	sort.Strings(dates)
	return dates
}

func planDayNames(offered map[int]bool) []string {
	names := make([]string, 0, len(offered))
	for i, col := range planWeekdayCols {
		if offered[i] {
			names = append(names, col)
		}
	}
	return names
}

func planPriceOrZero(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// planValueScore ranks swaps by reach (they cost nothing) and paid spots by
// opens per dollar, so the two are comparable in one ordering.
func planValueScore(c planCandidate) float64 {
	if c.OpensPerDollar != nil {
		return *c.OpensPerDollar
	}
	if c.SwapOnly {
		return c.EstimatedOpens
	}
	return 0
}

func planTrunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
