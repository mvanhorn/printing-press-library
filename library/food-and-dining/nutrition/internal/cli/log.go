// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/nutrition/internal/store"

	"github.com/spf13/cobra"
)

func openLogStore(cmd *cobra.Command) (*store.Store, error) {
	dbPath := defaultDBPath("nutrition-pp-cli")
	return store.OpenWithContext(cmd.Context(), dbPath)
}

func todayDate() string { return time.Now().Format("2006-01-02") }

type logTotals struct {
	CaloriesKcal float64 `json:"calories_kcal"`
	ProteinG     float64 `json:"protein_g"`
	FatG         float64 `json:"fat_g"`
	CarbsG       float64 `json:"carbs_g"`
	FiberG       float64 `json:"fiber_g"`
}

func sumEntries(entries []store.LogEntry) logTotals {
	var t logTotals
	for _, e := range entries {
		t.CaloriesKcal += e.Calories
		t.ProteinG += e.Protein
		t.FatG += e.Fat
		t.CarbsG += e.Carbs
		t.FiberG += e.Fiber
	}
	t.CaloriesKcal = round2(t.CaloriesKcal)
	t.ProteinG = round2(t.ProteinG)
	t.FatG = round2(t.FatG)
	t.CarbsG = round2(t.CarbsG)
	t.FiberG = round2(t.FiberG)
	return t
}

func newNovelLogCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Daily food diary with targets (local SQLite)",
		Long: "A persistent daily food diary with macro targets, backed by local SQLite.\n\n" +
			"Use this command for the persistent daily diary and target tracking. Do NOT use it " +
			"for a one-off total across several foods; use 'meal' instead.",
		Example:     "  nutrition-pp-cli log progress --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newLogAddCmd(flags))
	cmd.AddCommand(newLogTodayCmd(flags))
	cmd.AddCommand(newLogSummaryCmd(flags))
	cmd.AddCommand(newLogProgressCmd(flags))
	cmd.AddCommand(newLogTargetsCmd(flags))
	cmd.AddCommand(newLogRemoveCmd(flags))
	return cmd
}

func newLogAddCmd(flags *rootFlags) *cobra.Command {
	var flagGrams float64
	var flagDate string
	cmd := &cobra.Command{
		Use:         "add <fdcId>",
		Short:       "Log a food to the diary (fetches macros from USDA once)",
		Example:     "  nutrition-pp-cli log add 173414 --grams 150",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would fetch USDA macros and add a diary entry")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an FDC id is required, e.g. log add 173414 --grams 150"))
			}
			if flagGrams <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--grams must be > 0"))
			}
			fdcID := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			food, _, err := fetchUSDAFood(cmd.Context(), c, fdcID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			date := flagDate
			if date == "" {
				date = todayDate()
			}
			entry := store.LogEntry{
				Date:     date,
				FdcID:    fdcID,
				Name:     food.Description,
				Grams:    flagGrams,
				Calories: round2(scaleAmount(food.Calories(), flagGrams)),
				Protein:  round2(scaleAmount(food.Protein(), flagGrams)),
				Fat:      round2(scaleAmount(food.Fat(), flagGrams)),
				Carbs:    round2(scaleAmount(food.Carbs(), flagGrams)),
				Fiber:    round2(scaleAmount(food.Fiber(), flagGrams)),
			}
			s, err := openLogStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			id, err := s.AddLogEntry(cmd.Context(), entry)
			if err != nil {
				return err
			}
			entry.ID = id
			return emitNutritionJSON(cmd.OutOrStdout(), map[string]any{"added": entry}, flags)
		},
	}
	cmd.Flags().Float64Var(&flagGrams, "grams", 0, "Grams consumed")
	cmd.Flags().StringVar(&flagDate, "date", "", "Date (YYYY-MM-DD); defaults to today")
	return cmd
}

func newLogTodayCmd(flags *rootFlags) *cobra.Command {
	var flagDate string
	cmd := &cobra.Command{
		Use:         "today",
		Short:       "Show diary entries and totals for a day",
		Example:     "  nutrition-pp-cli log today --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would list today's diary entries")
			}
			date := flagDate
			if date == "" {
				date = todayDate()
			}
			s, err := openLogStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			entries, err := s.LogEntriesForDate(cmd.Context(), date)
			if err != nil {
				return err
			}
			if entries == nil {
				entries = []store.LogEntry{}
			}
			return emitNutritionJSON(cmd.OutOrStdout(), map[string]any{
				"date":    date,
				"entries": entries,
				"totals":  sumEntries(entries),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Date (YYYY-MM-DD); defaults to today")
	return cmd
}

func newLogSummaryCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	cmd := &cobra.Command{
		Use:         "summary",
		Short:       "Summarize diary totals and daily averages over N days",
		Example:     "  nutrition-pp-cli log summary --days 7 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would summarize the diary")
			}
			if flagDays <= 0 {
				flagDays = 7
			}
			start := time.Now().AddDate(0, 0, -(flagDays - 1)).Format("2006-01-02")
			s, err := openLogStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			entries, err := s.LogEntriesSince(cmd.Context(), start)
			if err != nil {
				return err
			}
			totals := sumEntries(entries)
			avg := logTotals{
				CaloriesKcal: round2(totals.CaloriesKcal / float64(flagDays)),
				ProteinG:     round2(totals.ProteinG / float64(flagDays)),
				FatG:         round2(totals.FatG / float64(flagDays)),
				CarbsG:       round2(totals.CarbsG / float64(flagDays)),
				FiberG:       round2(totals.FiberG / float64(flagDays)),
			}
			return emitNutritionJSON(cmd.OutOrStdout(), map[string]any{
				"days":          flagDays,
				"since":         start,
				"entry_count":   len(entries),
				"totals":        totals,
				"daily_average": avg,
			}, flags)
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 7, "Number of days to summarize")
	return cmd
}

type progressRow struct {
	Nutrient    string  `json:"nutrient"`
	Consumed    float64 `json:"consumed"`
	Target      float64 `json:"target"`
	Remaining   float64 `json:"remaining"`
	PctOfTarget float64 `json:"pct_of_target"`
}

func newLogProgressCmd(flags *rootFlags) *cobra.Command {
	var flagDate string
	cmd := &cobra.Command{
		Use:         "progress",
		Short:       "Show today's intake versus configured targets",
		Example:     "  nutrition-pp-cli log progress --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would compare today's intake to targets")
			}
			date := flagDate
			if date == "" {
				date = todayDate()
			}
			s, err := openLogStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			entries, err := s.LogEntriesForDate(cmd.Context(), date)
			if err != nil {
				return err
			}
			totals := sumEntries(entries)
			targets, err := s.Targets(cmd.Context())
			if err != nil {
				return err
			}
			consumed := map[string]float64{
				"calories": totals.CaloriesKcal,
				"protein":  totals.ProteinG,
				"fat":      totals.FatG,
				"carbs":    totals.CarbsG,
				"fiber":    totals.FiberG,
			}
			rows := make([]progressRow, 0, len(consumed))
			for _, name := range []string{"calories", "protein", "fat", "carbs", "fiber"} {
				tgt, ok := targets[name]
				if !ok {
					continue
				}
				c := consumed[name]
				row := progressRow{
					Nutrient:  name,
					Consumed:  round2(c),
					Target:    round2(tgt),
					Remaining: round2(tgt - c),
				}
				if tgt > 0 {
					row.PctOfTarget = round2(c / tgt * 100.0)
				}
				rows = append(rows, row)
			}
			note := ""
			if len(rows) == 0 {
				note = "no targets set; use 'log targets set --calories N --protein N ...'"
			}
			return emitNutritionJSON(cmd.OutOrStdout(), map[string]any{
				"date":     date,
				"progress": rows,
				"totals":   totals,
				"note":     note,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Date (YYYY-MM-DD); defaults to today")
	return cmd
}

func newLogTargetsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "targets",
		Short:       "View or set daily nutrient targets",
		Example:     "  nutrition-pp-cli log targets set --calories 2200 --protein 180",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would list targets")
			}
			s, err := openLogStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			targets, err := s.Targets(cmd.Context())
			if err != nil {
				return err
			}
			if targets == nil {
				targets = map[string]float64{}
			}
			return emitNutritionJSON(cmd.OutOrStdout(), map[string]any{"targets": targets}, flags)
		},
	}
	cmd.AddCommand(newLogTargetsSetCmd(flags))
	return cmd
}

func newLogTargetsSetCmd(flags *rootFlags) *cobra.Command {
	var cal, protein, fat, carbs, fiber float64
	cmd := &cobra.Command{
		Use:         "set",
		Short:       "Set daily nutrient targets",
		Example:     "  nutrition-pp-cli log targets set --calories 2200 --protein 180",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would set targets")
			}
			s, err := openLogStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			set := map[string]float64{}
			apply := func(name string, v float64, changed bool) error {
				if !changed {
					return nil
				}
				if err := s.SetTarget(cmd.Context(), name, v); err != nil {
					return err
				}
				set[name] = v
				return nil
			}
			if err := apply("calories", cal, cmd.Flags().Changed("calories")); err != nil {
				return err
			}
			if err := apply("protein", protein, cmd.Flags().Changed("protein")); err != nil {
				return err
			}
			if err := apply("fat", fat, cmd.Flags().Changed("fat")); err != nil {
				return err
			}
			if err := apply("carbs", carbs, cmd.Flags().Changed("carbs")); err != nil {
				return err
			}
			if err := apply("fiber", fiber, cmd.Flags().Changed("fiber")); err != nil {
				return err
			}
			if len(set) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("set at least one target, e.g. --calories 2200"))
			}
			return emitNutritionJSON(cmd.OutOrStdout(), map[string]any{"set": set}, flags)
		},
	}
	cmd.Flags().Float64Var(&cal, "calories", 0, "Daily calorie target (kcal)")
	cmd.Flags().Float64Var(&protein, "protein", 0, "Daily protein target (g)")
	cmd.Flags().Float64Var(&fat, "fat", 0, "Daily fat target (g)")
	cmd.Flags().Float64Var(&carbs, "carbs", 0, "Daily carbs target (g)")
	cmd.Flags().Float64Var(&fiber, "fiber", 0, "Daily fiber target (g)")
	return cmd
}

func newLogRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "remove <entryId>",
		Short:       "Remove a diary entry by id",
		Example:     "  nutrition-pp-cli log remove 3",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would remove a diary entry")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an entry id is required"))
			}
			var id int64
			if _, err := fmt.Sscan(args[0], &id); err != nil {
				return usageErr(fmt.Errorf("invalid entry id %q", args[0]))
			}
			s, err := openLogStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			n, err := s.DeleteLogEntry(cmd.Context(), id)
			if err != nil {
				return err
			}
			return emitNutritionJSON(cmd.OutOrStdout(), map[string]any{"removed": n, "id": id}, flags)
		},
	}
	return cmd
}
