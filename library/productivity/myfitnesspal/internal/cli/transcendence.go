// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// HAND-WRITTEN — novel transcendence commands defined for this CLI:
//   pull-diary       — sync a date range of diary entries into local SQLite
//   export csv       — per-food CSV across a date range (#21)
//   diary find       — FTS5 search across diary_entry (#26)
//   context          — agent-shaped composite of recent diary state (#28)
//   analytics top-foods   — Pareto query: which N foods drove X% of nutrient (#22)
//   analytics streak      — longest run inside ±tolerance of calorie goal (#31)
//
// Other transcendence features in the absorb manifest are deferred to
// /printing-press-polish (see README ## Known Gaps).

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/myfitnesspal/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/myfitnesspal/internal/store"
)

// ---- helpers shared by transcendence commands -----------------------------

func openLocalStore(_ *config.Config) (*store.Store, string, error) {
	dbPath := defaultDBPath("myfitnesspal-pp-cli")
	s, err := store.Open(dbPath)
	if err != nil {
		return nil, dbPath, fmt.Errorf("opening local store at %s: %w", dbPath, err)
	}
	return s, dbPath, nil
}

func parseDateOrToday(s string) (time.Time, error) {
	if s == "" {
		t := time.Now()
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return time.Parse("2006-01-02", s)
}

func loadConfigOrErr(flags *rootFlags) (*config.Config, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config (run `myfitnesspal-pp-cli auth login --chrome` first): %w", err)
	}
	return cfg, nil
}

// ---- pull-diary ----------------------------------------------------------

func newPullDiaryCmd(flags *rootFlags) *cobra.Command {
	var fromStr, toStr, username string

	cmd := &cobra.Command{
		Use:   "pull-diary",
		Short: "Sync a date range of food diary days into the local SQLite store.",
		Long: `Walks each date in [from, to], fetches /food/diary on
www.myfitnesspal.com using your imported browser session, parses each page
through the diary HTML parser, and upserts per-food rows into diary_entry.
Conservative 1 req/sec pacing per the python-myfitnesspal pattern.`,
		Example: "  myfitnesspal-pp-cli pull-diary --from 2024-01-01 --to 2024-01-31",
		Annotations: map[string]string{
			"mcp:read-only": "false", // populates the local store
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync diary from", fromStr, "to", toStr)
				return nil
			}
			from, err := parseDateOrToday(fromStr)
			if err != nil {
				return fmt.Errorf("parsing --from: %w", err)
			}
			to, err := parseDateOrToday(toStr)
			if err != nil {
				return fmt.Errorf("parsing --to: %w", err)
			}
			if to.Before(from) {
				return fmt.Errorf("--to (%s) is before --from (%s)", toStr, fromStr)
			}

			cfg, err := loadConfigOrErr(flags)
			if err != nil {
				return err
			}
			s, dbPath, err := openLocalStore(cfg)
			if err != nil {
				return err
			}
			defer s.Close()

			fmt.Fprintf(cmd.OutOrStdout(), "Syncing diary %s → %s into %s\n",
				from.Format("2006-01-02"), to.Format("2006-01-02"), dbPath)

			total, err := SyncDiaryRange(cmd.Context(), cfg, s, cmd.OutOrStdout(), from, to, username)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nSynced %d entries.\n", total)
			return nil
		},
	}
	cmd.Flags().StringVar(&fromStr, "from", "", "Start date YYYY-MM-DD (required).")
	cmd.Flags().StringVar(&toStr, "to", "", "End date YYYY-MM-DD (defaults to today).")
	cmd.Flags().StringVar(&username, "username", "", "Optional username override.")
	return cmd
}

// ---- export csv -----------------------------------------------------------

func newExportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export local diary data in formats MFP itself doesn't ship.",
	}
	cmd.AddCommand(newExportCsvCmd(flags))
	return cmd
}

func newExportCsvCmd(flags *rootFlags) *cobra.Command {
	var fromStr, toStr, mealFilter, outPath string

	cmd := &cobra.Command{
		Use:   "csv",
		Short: "Export your food diary to CSV with one row per logged food.",
		Long: `Exports per-food rows from the local store. Premium MFP only ships
per-meal CSVs; this command writes one row per food entry with the snapshotted
nutrient panel. Run pull-diary first to populate the store.`,
		Example: "  myfitnesspal-pp-cli export csv --from 2024-01-01 --to 2024-01-31 --out diary.csv",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			from, err := parseDateOrToday(fromStr)
			if err != nil {
				return fmt.Errorf("parsing --from: %w", err)
			}
			to, err := parseDateOrToday(toStr)
			if err != nil {
				return fmt.Errorf("parsing --to: %w", err)
			}

			cfg, err := loadConfigOrErr(flags)
			if err != nil {
				return err
			}
			s, _, err := openLocalStore(cfg)
			if err != nil {
				return err
			}
			defer s.Close()

			rows, err := s.QueryDiaryEntries(cmd.Context(),
				from.Format("2006-01-02"), to.Format("2006-01-02"))
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if outPath != "" {
				f, err := openCSVTarget(outPath)
				if err != nil {
					return err
				}
				defer f.Close()
				out = f
			}
			w := csv.NewWriter(out)
			defer w.Flush()

			header := []string{"date", "meal", "position", "food_name", "calories", "carbohydrates", "fat", "protein", "sodium", "sugar", "fiber", "cholesterol"}
			if err := w.Write(header); err != nil {
				return err
			}
			count := 0
			for _, r := range rows {
				if mealFilter != "" && !strings.EqualFold(r.Meal, mealFilter) {
					continue
				}
				if err := w.Write([]string{
					r.Date, r.Meal, strconv.Itoa(r.Position), r.FoodName,
					formatFloat(r.Calories), formatFloat(r.Carbohydrates),
					formatFloat(r.Fat), formatFloat(r.Protein),
					formatFloat(r.Sodium), formatFloat(r.Sugar),
					formatFloat(r.Fiber), formatFloat(r.Cholesterol),
				}); err != nil {
					return err
				}
				count++
			}
			if outPath != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %d rows to %s\n", count, outPath)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fromStr, "from", "", "Start date YYYY-MM-DD (required).")
	cmd.Flags().StringVar(&toStr, "to", "", "End date YYYY-MM-DD (defaults to today).")
	cmd.Flags().StringVar(&mealFilter, "meal", "", "Filter to one meal name (breakfast/lunch/dinner/snacks).")
	cmd.Flags().StringVar(&outPath, "out", "", "Output file (defaults to stdout).")
	return cmd
}

func formatFloat(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// ---- diary find ----------------------------------------------------------

func newDiaryFindCmd(flags *rootFlags) *cobra.Command {
	var query, fromStr, toStr string
	var limit int

	cmd := &cobra.Command{
		Use:   "find",
		Short: "FTS search across local diary entries (every time you logged a food).",
		Long: `Queries the local diary_entries_fts index. Returns date, meal,
servings, and calories for every diary entry whose food name or meal name
matches. Run pull-diary first to populate the FTS index.`,
		Example: "  myfitnesspal-pp-cli find --food \"Chipotle Bowl\" --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if query == "" {
				if flags.dryRun {
					return cmd.Help()
				}
				return fmt.Errorf("required flag \"food\" not set")
			}
			if flags.dryRun {
				return nil
			}
			cfg, err := loadConfigOrErr(flags)
			if err != nil {
				return err
			}
			s, _, err := openLocalStore(cfg)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.EnsureDiaryEntries(cmd.Context()); err != nil {
				return err
			}

			rows, err := s.FindDiaryEntries(cmd.Context(), query, fromStr, toStr, limit)
			if err != nil {
				return err
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d matches:\n", len(rows))
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-10s  %s  (%.0f kcal)\n",
					r.Date, r.Meal, r.FoodName, r.Calories)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "food", "", "Search query (required).")
	cmd.Flags().StringVar(&fromStr, "from", "", "Earliest date (YYYY-MM-DD; default unbounded).")
	cmd.Flags().StringVar(&toStr, "to", "", "Latest date (YYYY-MM-DD; default unbounded).")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max matches to return.")
	return cmd
}

// ---- context -------------------------------------------------------------

type AgentContextDump struct {
	GeneratedAt   string                `json:"generated_at"`
	Days          int                   `json:"days"`
	From          string                `json:"from"`
	To            string                `json:"to"`
	DailyTotals   []DailyTotalsRow      `json:"daily_totals"`
	TopFoods      []TopFoodsRow         `json:"top_foods"`
	RecentEntries []store.DiaryEntryRow `json:"recent_entries,omitempty"`
	GoalSnapshot  json.RawMessage       `json:"latest_goal_snapshot,omitempty"`
	Note          string                `json:"note,omitempty"`
}

type DailyTotalsRow struct {
	Date          string  `json:"date"`
	Calories      float64 `json:"calories"`
	Protein       float64 `json:"protein"`
	Carbohydrates float64 `json:"carbohydrates"`
	Fat           float64 `json:"fat"`
	Complete      bool    `json:"complete"`
}

func newContextCmd(flags *rootFlags) *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:   "context",
		Short: "Single-call agent context: last N days of diary totals, goals, top foods, recent entries.",
		Long: `Composes diary totals, top foods, latest goal snapshot, and the most
recent entries from the local store into a single JSON payload sized for an
agent context window. AdamWalt MCP requires N tool calls for the same picture.`,
		Example: "  myfitnesspal-pp-cli context --days 14 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigOrErr(flags)
			if err != nil {
				return err
			}
			s, _, err := openLocalStore(cfg)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.EnsureDiaryEntries(cmd.Context()); err != nil {
				return err
			}

			to := time.Now().UTC()
			from := to.AddDate(0, 0, -days+1)
			dump := &AgentContextDump{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				Days:        days,
				From:        from.Format("2006-01-02"),
				To:          to.Format("2006-01-02"),
			}

			rows, err := s.QueryDiaryEntries(cmd.Context(),
				dump.From, dump.To)
			if err != nil {
				return err
			}

			dump.DailyTotals = rollUpDailyTotals(rows)
			dump.TopFoods = computeTopFoods(rows, "calories", 5, 0.8)
			if len(rows) > 20 {
				dump.RecentEntries = rows[len(rows)-20:]
			} else {
				dump.RecentEntries = rows
			}

			if metas, err := s.QueryDiaryDayMeta(cmd.Context(), dump.From, dump.To); err == nil && len(metas) > 0 {
				dump.GoalSnapshot = metas[len(metas)-1].GoalsJSON
			}

			if len(rows) == 0 {
				dump.Note = "No local diary data — run `pull-diary --from " + dump.From + " --to " + dump.To + "` first."
			}

			return printJSONFiltered(cmd.OutOrStdout(), dump, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "Lookback window (default 14 days).")
	return cmd
}

func rollUpDailyTotals(rows []store.DiaryEntryRow) []DailyTotalsRow {
	byDate := map[string]*DailyTotalsRow{}
	for _, r := range rows {
		t := byDate[r.Date]
		if t == nil {
			t = &DailyTotalsRow{Date: r.Date}
			byDate[r.Date] = t
		}
		t.Calories += r.Calories
		t.Protein += r.Protein
		t.Carbohydrates += r.Carbohydrates
		t.Fat += r.Fat
	}
	out := make([]DailyTotalsRow, 0, len(byDate))
	for _, v := range byDate {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

// ---- analytics top-foods + streak ---------------------------------------

type TopFoodsRow struct {
	FoodName        string  `json:"food_name"`
	NutrientTotal   float64 `json:"nutrient_total"`
	NutrientShare   float64 `json:"nutrient_share"`
	OccurrenceCount int     `json:"occurrence_count"`
}

func newAnalyticsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Local-SQLite analytics over your synced diary.",
	}
	cmd.AddCommand(newAnalyticsTopFoodsCmd(flags))
	cmd.AddCommand(newAnalyticsStreakCmd(flags))
	return cmd
}

func newAnalyticsTopFoodsCmd(flags *rootFlags) *cobra.Command {
	var nutrient string
	var days, limit int
	var cumulativePercent float64

	cmd := &cobra.Command{
		Use:   "top-foods",
		Short: "Pareto query: which foods drove most of one nutrient over a window.",
		Long: `Computes which foods you logged that contributed the most of a chosen
nutrient (calories/protein/carbohydrates/fat/fiber/sugar/sodium) over the last
--days days. Output is ordered by total contribution and cumulative-share-cut
to show the Pareto top of your eating.`,
		Example: "  myfitnesspal-pp-cli analytics top-foods --nutrient protein --days 60 --cumulative-percent 0.8 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigOrErr(flags)
			if err != nil {
				return err
			}
			s, _, err := openLocalStore(cfg)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.EnsureDiaryEntries(cmd.Context()); err != nil {
				return err
			}

			to := time.Now().UTC()
			from := to.AddDate(0, 0, -days+1)
			rows, err := s.QueryDiaryEntries(cmd.Context(), from.Format("2006-01-02"), to.Format("2006-01-02"))
			if err != nil {
				return err
			}

			top := computeTopFoods(rows, nutrient, limit, cumulativePercent)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), top, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Top foods by %s, last %d days:\n", nutrient, days)
			for i, t := range top {
				fmt.Fprintf(cmd.OutOrStdout(), "  %2d. %-40s %8.1f  (%.1f%%, %d times)\n",
					i+1, t.FoodName, t.NutrientTotal, t.NutrientShare*100, t.OccurrenceCount)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&nutrient, "nutrient", "calories", "Nutrient to rank (calories/protein/carbohydrates/fat/sodium/sugar/fiber).")
	cmd.Flags().IntVar(&days, "days", 30, "Lookback window.")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max foods to return.")
	cmd.Flags().Float64Var(&cumulativePercent, "cumulative-percent", 0.0, "If >0, cap output at the Pareto cumulative share (e.g. 0.8 = top 80%).")
	return cmd
}

func computeTopFoods(rows []store.DiaryEntryRow, nutrient string, limit int, cumulativeCut float64) []TopFoodsRow {
	totals := map[string]*TopFoodsRow{}
	var grand float64
	for _, r := range rows {
		v := nutrientValue(r, nutrient)
		t := totals[r.FoodName]
		if t == nil {
			t = &TopFoodsRow{FoodName: r.FoodName}
			totals[r.FoodName] = t
		}
		t.NutrientTotal += v
		t.OccurrenceCount++
		grand += v
	}

	out := make([]TopFoodsRow, 0, len(totals))
	for _, v := range totals {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NutrientTotal > out[j].NutrientTotal })

	cum := 0.0
	final := []TopFoodsRow{}
	for i, t := range out {
		if grand > 0 {
			t.NutrientShare = t.NutrientTotal / grand
		}
		final = append(final, t)
		cum += t.NutrientShare
		if cumulativeCut > 0 && cum >= cumulativeCut {
			break
		}
		if limit > 0 && i+1 >= limit {
			break
		}
	}
	return final
}

func nutrientValue(r store.DiaryEntryRow, nutrient string) float64 {
	switch nutrient {
	case "calories":
		return r.Calories
	case "protein":
		return r.Protein
	case "carbohydrates", "carbs":
		return r.Carbohydrates
	case "fat":
		return r.Fat
	case "sodium":
		return r.Sodium
	case "sugar":
		return r.Sugar
	case "fiber":
		return r.Fiber
	case "cholesterol":
		return r.Cholesterol
	default:
		if r.Extras != nil {
			return r.Extras[nutrient]
		}
		return 0
	}
}

type StreakResult struct {
	Days         []StreakDay `json:"days"`
	LongestRun   int         `json:"longest_run_days"`
	CurrentRun   int         `json:"current_run_days"`
	Tolerance    float64     `json:"tolerance_pct"`
	GoalCalories float64     `json:"goal_calories,omitempty"`
}

type StreakDay struct {
	Date     string  `json:"date"`
	Calories float64 `json:"calories"`
	Within   bool    `json:"within_tolerance"`
}

func newAnalyticsStreakCmd(flags *rootFlags) *cobra.Command {
	var days int
	var tolerance float64
	var goalCalories float64

	cmd := &cobra.Command{
		Use:   "streak",
		Short: "Longest run of consecutive days inside ±tolerance of your calorie goal.",
		Long: `Walks the last --days days from the local diary and counts the
longest run of days whose total calories fall within --tolerance of the
calorie goal. Pass --goal-calories to override the per-day goal manually
(otherwise reads goal_snapshot from diary_day_meta).`,
		Example: "  myfitnesspal-pp-cli analytics streak --days 60 --tolerance 0.05 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigOrErr(flags)
			if err != nil {
				return err
			}
			s, _, err := openLocalStore(cfg)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.EnsureDiaryEntries(cmd.Context()); err != nil {
				return err
			}

			to := time.Now().UTC()
			from := to.AddDate(0, 0, -days+1)
			rows, err := s.QueryDiaryEntries(cmd.Context(), from.Format("2006-01-02"), to.Format("2006-01-02"))
			if err != nil {
				return err
			}

			daily := rollUpDailyTotals(rows)

			if goalCalories <= 0 {
				goalCalories = inferGoalCalories(cmd.Context(), s, dump_or(dump_to(from), dump_to(to)))
			}

			result := computeStreak(daily, goalCalories, tolerance)
			result.Tolerance = tolerance
			result.GoalCalories = goalCalories
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 60, "Lookback window.")
	cmd.Flags().Float64Var(&tolerance, "tolerance", 0.05, "Fractional tolerance around the goal (0.05 = ±5%).")
	cmd.Flags().Float64Var(&goalCalories, "goal-calories", 0, "Override goal calories per day (otherwise read from goal_snapshot).")
	return cmd
}

func computeStreak(daily []DailyTotalsRow, goal float64, tol float64) StreakResult {
	res := StreakResult{}
	if goal <= 0 || tol <= 0 {
		// Without a goal we can't compute streaks; return per-day data only.
		for _, d := range daily {
			res.Days = append(res.Days, StreakDay{Date: d.Date, Calories: d.Calories})
		}
		return res
	}
	low, high := goal*(1-tol), goal*(1+tol)
	current := 0
	longest := 0
	for _, d := range daily {
		within := d.Calories >= low && d.Calories <= high
		res.Days = append(res.Days, StreakDay{Date: d.Date, Calories: d.Calories, Within: within})
		if within {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 0
		}
	}
	res.LongestRun = longest
	res.CurrentRun = current
	return res
}

// inferGoalCalories pulls the most recent goal_snapshot from diary_day_meta
// and returns its calories field if present.
func inferGoalCalories(ctx context.Context, s *store.Store, _ string) float64 {
	metas, err := s.QueryDiaryDayMeta(ctx, "0000-00-00", "9999-12-31")
	if err != nil {
		return 0
	}
	for i := len(metas) - 1; i >= 0; i-- {
		if metas[i].GoalsJSON == nil {
			continue
		}
		var goal map[string]float64
		if err := json.Unmarshal(metas[i].GoalsJSON, &goal); err == nil {
			if v, ok := goal["calories"]; ok && v > 0 {
				return v
			}
		}
	}
	return 0
}

// dump_to/dump_or: tiny helpers to keep the streak signature stable across the
// two paths that compute it (with explicit goal vs inferred). They're private
// stubs because Go's lint complains about unused params; treat them as no-ops.
func dump_to(t time.Time) string { return t.Format("2006-01-02") }
func dump_or(a, _ string) string { return a }
