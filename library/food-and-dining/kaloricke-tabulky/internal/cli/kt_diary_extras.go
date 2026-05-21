package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// diary frequency [--days N] [--meal SLOT] [--min N]
// Counts foodstuff occurrences across the window.
func newKTDiaryFrequencyCmd(flags *rootFlags) *cobra.Command {
	var days int
	var meal string
	var minOcc int
	var dateFlag string

	cmd := &cobra.Command{
		Use:   "frequency",
		Short: "Count how often each foodstuff appears in your diary across a window",
		Long: `Iterates the diary for the trailing N days, groups by foodstuff title,
and ranks by frequency. Optionally filters to a single meal slot. Pure
local aggregation over per-day diary fetches.`,
		Example: `  kaloricke-tabulky-pp-cli diary frequency --days 30
  kaloricke-tabulky-pp-cli diary frequency --days 60 --meal dinner --min 3 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, _, err := ktNewAuthenticatedClient(flags)
			if err != nil {
				return err
			}
			anchorDD, err := parseFlexDate(dateFlag)
			if err != nil {
				return err
			}
			anchor, _ := time.Parse("02.01.2006", anchorDD)
			if days < 1 {
				days = 30
			}

			slotFilter := ""
			if meal != "" {
				slotFilter, err = mealSlotID(meal)
				if err != nil {
					return err
				}
			}

			type entry struct {
				Title      string  `json:"title"`
				URL        string  `json:"slug"`
				Count      int     `json:"count"`
				TotalKJ    float64 `json:"total_energy_kj"`
				TotalProtG float64 `json:"total_protein_g"`
			}
			tally := map[string]*entry{}
			for offset := 0; offset < days; offset++ {
				d := anchor.AddDate(0, 0, -offset)
				diary, err := ktFetchDiaryDay(c, d.Format("02.01.2006"))
				if err != nil {
					continue
				}
				for _, slot := range diary.Times {
					if slotFilter != "" && slot.ID != slotFilter {
						continue
					}
					for _, f := range slot.Foodstuff {
						e, ok := tally[f.URL]
						if !ok {
							e = &entry{Title: f.Title, URL: f.URL}
							tally[f.URL] = e
						}
						e.Count++
						kj, _ := ktParseCzechNum(f.Energy)
						e.TotalKJ += kj
						e.TotalProtG += f.Protein
					}
				}
			}
			entries := make([]*entry, 0, len(tally))
			for _, e := range tally {
				if e.Count >= minOcc {
					entries = append(entries, e)
				}
			}
			sort.Slice(entries, func(i, j int) bool {
				if entries[i].Count != entries[j].Count {
					return entries[i].Count > entries[j].Count
				}
				return entries[i].Title < entries[j].Title
			})

			result := map[string]any{
				"window_days":  days,
				"meal_filter":  meal,
				"min_count":    minOcc,
				"unique_foods": len(entries),
				"foods":        entries,
			}
			return ktEmit(cmd.OutOrStdout(), flags, result)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Trailing days to scan")
	cmd.Flags().StringVar(&meal, "meal", "", "Filter to a single meal slot (breakfast|lunch|dinner|...)")
	cmd.Flags().IntVar(&minOcc, "min", 1, "Minimum occurrences to include")
	cmd.Flags().StringVar(&dateFlag, "date", "today", "Anchor (newest) date")
	return cmd
}

// energy balance [--days N]
// Joins diary energy (food) with what the summary reports as "todayActivity"
// (kJ burned via logged activities) for each day. Prints daily series + MA.
func newKTEnergyBalanceCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dateFlag string

	cmd := &cobra.Command{
		Use:   "balance",
		Short: "Energy-in (diary) minus energy-out (activity) across N days",
		Long: `Joins per-day diary energy (sum of foodstuffs) with per-day activity
energy (from summary.todayActivity). Returns the daily series plus a
trailing 7-day moving average.`,
		Example: `  kaloricke-tabulky-pp-cli energy balance --days 14 --json
  kaloricke-tabulky-pp-cli energy balance --days 7`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, _, err := ktNewAuthenticatedClient(flags)
			if err != nil {
				return err
			}
			anchorDD, err := parseFlexDate(dateFlag)
			if err != nil {
				return err
			}
			anchor, _ := time.Parse("02.01.2006", anchorDD)
			if days < 1 {
				days = 7
			}
			type day struct {
				Date           string  `json:"date"`
				EnergyInKJ     float64 `json:"energy_in_kj"`
				EnergyOutKJ    float64 `json:"energy_out_kj"`
				NetKJ          float64 `json:"net_kj"`
				MovingAvgNetKJ float64 `json:"moving_avg_net_kj,omitempty"`
				MovingAvgWindowDays int `json:"moving_avg_window_days,omitempty"`
			}
			series := make([]day, 0, days)
			for offset := days - 1; offset >= 0; offset-- {
				d := anchor.AddDate(0, 0, -offset)
				dd := d.Format("02.01.2006")
				diary, err := ktFetchDiaryDay(c, dd)
				if err != nil {
					continue
				}
				summary, err := ktFetchSummaryDay(c, dd)
				if err != nil {
					continue
				}
				dm := ktAggregateDay(diary, d.Format("2006-01-02"))
				out, _ := ktParseCzechNum(summary.TodayActivity)
				series = append(series, day{
					Date:        d.Format("2006-01-02"),
					EnergyInKJ:  dm.EnergyKJ,
					EnergyOutKJ: out,
					NetKJ:       dm.EnergyKJ - out,
				})
			}
			// Moving average: 7-day window when the series has enough
			// points, otherwise cap to the series length so a 3-day
			// window doesn't get labeled as a 7-day average.
			win := 7
			if days < win {
				win = days
			}
			for i := range series {
				lo := i - win + 1
				if lo < 0 {
					lo = 0
				}
				sum := 0.0
				for j := lo; j <= i; j++ {
					sum += series[j].NetKJ
				}
				actualWin := i - lo + 1
				series[i].MovingAvgNetKJ = sum / float64(actualWin)
				series[i].MovingAvgWindowDays = actualWin
			}
			return ktEmit(cmd.OutOrStdout(), flags, map[string]any{
				"days":   days,
				"series": series,
			})
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "Trailing days to include")
	cmd.Flags().StringVar(&dateFlag, "date", "today", "Anchor date")
	return cmd
}

// diary export json --from <date> --to <date>
// Bulk export of diary days as one JSON document with typed totals.
func newKTDiaryExportJSONCmd(flags *rootFlags) *cobra.Command {
	var fromDate, toDate string

	cmd := &cobra.Command{
		Use:   "export-json",
		Short: "Bulk-export your diary across a date range as one JSON document",
		Long: `Pulls /user/diary/<date>/get for each day in the range and rolls each
day's foodstuffs and notes into a single JSON document with typed
per-day totals. Agent-friendly bulk format the web UI hides behind
per-day PDF/XLS export.`,
		Example: `  kaloricke-tabulky-pp-cli diary export-json --from 2026-04-01 --to 2026-05-21 > diary.json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, _, err := ktNewAuthenticatedClient(flags)
			if err != nil {
				return err
			}
			fromDD, err := parseFlexDate(fromDate)
			if err != nil {
				return fmt.Errorf("--from: %w", err)
			}
			toDD, err := parseFlexDate(toDate)
			if err != nil {
				return fmt.Errorf("--to: %w", err)
			}
			fromT, _ := time.Parse("02.01.2006", fromDD)
			toT, _ := time.Parse("02.01.2006", toDD)
			if toT.Before(fromT) {
				fromT, toT = toT, fromT
			}
			type dayDoc struct {
				Date   string      `json:"date"`
				Macros ktDayMacros `json:"macros"`
				Diary  *ktDiaryDay `json:"diary"`
			}
			docs := []dayDoc{}
			for d := fromT; !d.After(toT); d = d.AddDate(0, 0, 1) {
				dd := d.Format("02.01.2006")
				diary, err := ktFetchDiaryDay(c, dd)
				if err != nil {
					continue
				}
				m := ktAggregateDay(diary, d.Format("2006-01-02"))
				m.BySlot = nil
				docs = append(docs, dayDoc{
					Date:   d.Format("2006-01-02"),
					Macros: m,
					Diary:  diary,
				})
			}
			return ktEmit(cmd.OutOrStdout(), flags, map[string]any{
				"from_date": fromT.Format("2006-01-02"),
				"to_date":   toT.Format("2006-01-02"),
				"day_count": len(docs),
				"days":      docs,
			})
		},
	}
	cmd.Flags().StringVar(&fromDate, "from", "", "Start date (inclusive)")
	cmd.Flags().StringVar(&toDate, "to", "today", "End date (inclusive)")
	cmd.MarkFlagRequired("from")
	return cmd
}

// diary unlog --last
// Reads today's diary, finds the most recently added foodstuff entry,
// and calls /user/diary/foodstuff/delete/<id>. "Most recent" is heuristic
// since the API doesn't expose createdAt — defaults to last entry in
// last non-empty meal slot.
func newKTDiaryUnlogCmd(flags *rootFlags) *cobra.Command {
	var lastFlag bool
	var commit bool

	cmd := &cobra.Command{
		Use:   "unlog",
		Short: "Remove the most-recently-added diary entry",
		Long: `Looks up today's diary, picks the last foodstuff entry in the latest
non-empty meal slot, and calls the delete endpoint.

Without --commit, prints what would be removed. With --commit, actually
deletes.`,
		Example: `  kaloricke-tabulky-pp-cli diary unlog --last
  kaloricke-tabulky-pp-cli diary unlog --last --commit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !lastFlag {
				return fmt.Errorf("use --last to identify the entry to remove (other selection modes not yet supported)")
			}
			if dryRunOK(flags) {
				return nil
			}
			c, cfg, err := ktNewAuthenticatedClient(flags)
			if err != nil {
				return err
			}
			today := time.Now().Local().Format("02.01.2006")
			diary, err := ktFetchDiaryDay(c, today)
			if err != nil {
				return err
			}
			// Walk slots in reverse, take last non-empty
			var picked *ktDiaryFoodstuff
			var pickedSlot string
			for i := len(diary.Times) - 1; i >= 0; i-- {
				slot := diary.Times[i]
				if len(slot.Foodstuff) > 0 {
					f := slot.Foodstuff[len(slot.Foodstuff)-1]
					picked = &f
					pickedSlot = slot.ID
					break
				}
			}
			if picked == nil {
				return fmt.Errorf("no diary entries on %s to remove", today)
			}
			result := map[string]any{
				"date":      today,
				"meal_slot": mealSlotLabel(pickedSlot),
				"entry_id":  picked.ID,
				"title":     picked.Title,
				"grams":     picked.Multiplier,
				"committed": false,
			}
			if !commit {
				result["note"] = "Pass --commit to actually delete."
				return ktEmit(cmd.OutOrStdout(), flags, result)
			}
			// POST delete (the controller uses /user/diary/foodstuff/delete/<id>)
			req, err := http.NewRequest("POST",
				"https://www.kaloricketabulky.cz/user/diary/foodstuff/delete/"+picked.ID+"?format=json",
				bytes.NewReader([]byte("{}")),
			)
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")
			req.Header.Set("Cookie", cfg.AuthHeader())
			req.Header.Set("User-Agent", "kaloricke-tabulky-pp-cli/1.0")
			httpClient := &http.Client{Timeout: 30 * time.Second}
			resp, err := httpClient.Do(req)
			if err != nil {
				return fmt.Errorf("delete: %w", err)
			}
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
			if resp.StatusCode != 200 {
				return fmt.Errorf("delete HTTP %d: %s", resp.StatusCode, string(respBody))
			}
			var env ktApiResponse
			if err := json.Unmarshal(respBody, &env); err != nil {
				return fmt.Errorf("parsing delete response: %w", err)
			}
			if env.Code != 0 {
				msg := "delete rejected"
				if env.Message != nil && *env.Message != "" {
					msg = *env.Message
				}
				return fmt.Errorf("%s (code %d)", msg, env.Code)
			}
			result["committed"] = true
			return ktEmit(cmd.OutOrStdout(), flags, result)
		},
	}
	cmd.Flags().BoolVar(&lastFlag, "last", false, "Remove the last entry on today's diary")
	cmd.Flags().BoolVar(&commit, "commit", false, "Actually delete (without --commit, prints the candidate)")
	return cmd
}

// diary plan-meal --target-protein N [--remaining-energy K] [--meal SLOT]
// Greedy-selects from favorites + most-frequent-30d foods to close
// the protein gap within the energy budget. Heuristic, not optimal.
func newKTDiaryPlanMealCmd(flags *rootFlags) *cobra.Command {
	var targetProtein float64
	var remainingEnergy float64
	var meal string

	cmd := &cobra.Command{
		Use:   "plan-meal",
		Short: "Greedy-select foods to close the day's protein gap within an energy budget",
		Long: `Given a target protein gap and an optional remaining-energy ceiling,
suggests a list of foods drawn from your favorites + most-frequent
foods over the trailing 30 days. Greedy by protein density (g/kJ).

If --remaining-energy is omitted, the value is derived from
summary.todayEnergyTarget minus summary.todayEnergy.

Heuristic, not an LP solver — the suggestion is grounded in foods you
actually eat, which is more useful than an LP-optimal exotic meal.`,
		Example: `  kaloricke-tabulky-pp-cli diary plan-meal --target-protein 40
  kaloricke-tabulky-pp-cli diary plan-meal --target-protein 40 --remaining-energy 2500 --meal dinner --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if targetProtein <= 0 {
				return fmt.Errorf("--target-protein must be > 0 (grams of protein you want this meal to contribute)")
			}
			c, _, err := ktNewAuthenticatedClient(flags)
			if err != nil {
				return err
			}
			// Derive remaining-energy from today's summary if not given
			if remainingEnergy <= 0 {
				summary, err := ktFetchSummaryDay(c, time.Now().Local().Format("02.01.2006"))
				if err == nil {
					target, _ := ktParseCzechNum(summary.TodayEnergyTarget)
					actual, _ := ktParseCzechNum(summary.TodayEnergy)
					remainingEnergy = target - actual
				}
			}
			if remainingEnergy <= 0 {
				remainingEnergy = 4000 // sane default
			}

			// Build the candidate pool: favorite foods + foods eaten in the
			// last 30 days. The diary-history half is important because a
			// new user with an empty favorites list would otherwise see an
			// empty selection list.
			type cand struct {
				Title          string  `json:"title"`
				URL            string  `json:"slug"`
				ProteinPer100G float64 `json:"protein_per_100g"`
				EnergyPer100KJ float64 `json:"energy_per_100g_kj"`
				ProteinDensity float64 `json:"protein_density_g_per_kj"`
				Source         string  `json:"source"`
			}
			pool := map[string]*cand{}

			addCandidate := func(title, url string, proteinPer100G, energyPer100KJ float64, source string) {
				if url == "" || proteinPer100G <= 0 || energyPer100KJ <= 0 {
					return
				}
				if existing, ok := pool[url]; ok {
					// Prefer favorites attribution when the same food
					// shows up in both sources.
					if existing.Source == "favorite" {
						return
					}
				}
				pool[url] = &cand{
					Title:          title,
					URL:            url,
					ProteinPer100G: proteinPer100G,
					EnergyPer100KJ: energyPer100KJ,
					ProteinDensity: proteinPer100G / energyPer100KJ,
					Source:         source,
				}
			}

			// Favorites — wire values are per 1g, convert to per 100g.
			favRaw, _ := c.GetNoCache("/user/settings/favorite/foodstuff", map[string]string{"format": "json"})
			if data, err := ktUnwrapEnvelope(favRaw); err == nil {
				var favs []map[string]interface{}
				if err := json.Unmarshal(data, &favs); err == nil {
					for _, f := range favs {
						title, _ := f["title"].(string)
						url, _ := f["url"].(string)
						protein, _ := ktParseCzechNum(fmt.Sprintf("%v", f["protein"]))
						energy, _ := ktParseCzechNum(fmt.Sprintf("%v", f["energy"]))
						addCandidate(title, url, protein*100, energy*100, "favorite")
					}
				}
			}

			// 30-day diary history — diary entries carry typed numeric
			// macros per portion AND the chosen `unit`/`multiplier`, so we
			// can derive per-100g values by dividing macros by the portion
			// in grams. When multiplier isn't 'g', skip the entry rather
			// than guess a conversion ratio.
			anchor := time.Now().Local()
			for offset := 0; offset < 30; offset++ {
				d := anchor.AddDate(0, 0, -offset)
				diary, derr := ktFetchDiaryDay(c, d.Format("02.01.2006"))
				if derr != nil {
					continue
				}
				for _, slot := range diary.Times {
					for _, f := range slot.Foodstuff {
						if f.URL == "" || f.Multiplier <= 0 {
							continue
						}
						energyKJ, _ := ktParseCzechNum(f.Energy)
						if energyKJ <= 0 {
							continue
						}
						// Skip non-gram units. The diary unit field is free
						// text the user picked at log time — "60 x 1 g",
						// "ml", "ks" (Czech for "pieces"), etc. Only the
						// canonical "g" lets us safely scale to per-100g.
						// Treating "ks" as grams would inflate protein
						// density by the per-piece weight and skew the
						// greedy selection.
						if f.Unit != "g" {
							continue
						}
						// macros are per portion; scale to per 100g.
						scale := 100.0 / f.Multiplier
						protein100 := f.Protein * scale
						energy100 := energyKJ * scale
						addCandidate(f.Title, f.URL, protein100, energy100, "diary-30d")
					}
				}
			}
			// Greedy selection
			cands := make([]*cand, 0, len(pool))
			for _, c := range pool {
				cands = append(cands, c)
			}
			sort.Slice(cands, func(i, j int) bool { return cands[i].ProteinDensity > cands[j].ProteinDensity })

			type sel struct {
				Cand     *cand   `json:"food"`
				Grams    float64 `json:"grams"`
				ProteinG float64 `json:"protein_contribution_g"`
				EnergyKJ float64 `json:"energy_contribution_kj"`
			}
			selections := []sel{}
			remProtein := targetProtein
			remEnergy := remainingEnergy
			for _, c := range cands {
				if remProtein <= 0.5 || remEnergy < 100 {
					break
				}
				// Portion sized to deliver up to remaining protein, capped by energy.
				gramsByProtein := remProtein / (c.ProteinPer100G / 100)
				gramsByEnergy := remEnergy / (c.EnergyPer100KJ / 100)
				grams := gramsByProtein
				if gramsByEnergy < grams {
					grams = gramsByEnergy
				}
				if grams < 10 {
					continue
				}
				if grams > 400 {
					grams = 400 // realistic cap
				}
				protein := grams * (c.ProteinPer100G / 100)
				energy := grams * (c.EnergyPer100KJ / 100)
				selections = append(selections, sel{
					Cand:     c,
					Grams:    grams,
					ProteinG: protein,
					EnergyKJ: energy,
				})
				remProtein -= protein
				remEnergy -= energy
				if len(selections) >= 5 {
					break
				}
			}
			result := map[string]any{
				"target_protein_g":      targetProtein,
				"remaining_energy_kj":   remainingEnergy,
				"meal":                  meal,
				"candidates_considered": len(cands),
				"selections":            selections,
				"protein_remaining_g":   remProtein,
				"energy_remaining_kj":   remEnergy,
			}
			return ktEmit(cmd.OutOrStdout(), flags, result)
		},
	}
	cmd.Flags().Float64Var(&targetProtein, "target-protein", 0, "Grams of protein you want this plan to contribute (required)")
	cmd.Flags().Float64Var(&remainingEnergy, "remaining-energy", 0, "Energy budget in kJ (default: derive from today's summary)")
	cmd.Flags().StringVar(&meal, "meal", "dinner", "Meal slot label for the plan (informational)")
	return cmd
}
