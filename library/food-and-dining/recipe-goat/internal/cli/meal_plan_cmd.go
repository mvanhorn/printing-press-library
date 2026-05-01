package cli

import (
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newMealPlanCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "meal-plan",
		Short:   "Plan meals by date and slot, and generate shopping lists",
		Example: "  recipe-goat-pp-cli meal-plan show --week",
	}
	cmd.AddCommand(newMealPlanSetCmd(flags))
	cmd.AddCommand(newMealPlanShowCmd(flags))
	cmd.AddCommand(newMealPlanRemoveCmd(flags))
	cmd.AddCommand(newMealPlanShoppingCmd(flags))
	cmd.AddCommand(newMealPlanSetCmd(flags))
	cmd.AddCommand(newMealPlanShowCmd(flags))
	cmd.AddCommand(newMealPlanRemoveCmd(flags))
	cmd.AddCommand(newMealPlanShoppingCmd(flags))
	return cmd
}

var validMeals = map[string]bool{"breakfast": true, "lunch": true, "dinner": true, "snack": true}

func newMealPlanSetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "set <date> <meal> <recipe-id>",
		Short:   "Plan a recipe for a specific date and meal slot",
		Example: "  recipe-goat-pp-cli meal-plan set 2026-04-15 dinner 42",
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			date := args[0]
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return usageErr(fmt.Errorf("date must be YYYY-MM-DD: %s", date))
			}
			meal := strings.ToLower(args[1])
			if !validMeals[meal] {
				return usageErr(fmt.Errorf("meal must be breakfast|lunch|dinner|snack"))
			}
			id, err := strconv.ParseInt(args[2], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("recipe-id must be an integer"))
			}
			st, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer st.Close()
			r, err := st.GetRecipeByID(id)
			if err != nil {
				return err
			}
			if r == nil {
				return notFoundErr(fmt.Errorf("recipe %d not found — save it first", id))
			}
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would set %s %s → %d %s\n", date, meal, id, r.Title)
				return nil
			}
			if err := st.SetMealPlan(date, meal, id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "planned: %s %s → %d %s\n", date, meal, id, r.Title)
			return nil
		},
	}
}

func newMealPlanShowCmd(flags *rootFlags) *cobra.Command {
	var from, to string
	var week bool
	cmd := &cobra.Command{
		Use:         "show",
		Short:       "Show planned meals over a date range",
		Example:     "  recipe-goat-pp-cli meal-plan show --week",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			f, t := resolveDateRange(from, to, week)
			st, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer st.Close()
			entries, err := st.GetMealPlan(f, t)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, entries)
			}
			if len(entries) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "(no meals planned between %s and %s)\n", f, t)
				return nil
			}
			headers := []string{"DATE", "MEAL", "ID", "RECIPE"}
			rows := make([][]string, 0, len(entries))
			for _, e := range entries {
				rows = append(rows, []string{e.Date, e.Meal, strconv.FormatInt(e.RecipeID, 10), truncate(e.Title, 60)})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&week, "week", false, "Use the current Mon–Sun window")
	return cmd
}

func newMealPlanRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <date> <meal>",
		Short:   "Clear a planned meal slot",
		Example: "  recipe-goat-pp-cli meal-plan remove 2026-04-15 dinner --dry-run",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			date := args[0]
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return usageErr(fmt.Errorf("date must be YYYY-MM-DD"))
			}
			meal := strings.ToLower(args[1])
			if !validMeals[meal] {
				return usageErr(fmt.Errorf("meal must be breakfast|lunch|dinner|snack"))
			}
			st, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer st.Close()
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would remove %s %s\n", date, meal)
				return nil
			}
			if err := st.RemoveMealPlan(date, meal); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed: %s %s\n", date, meal)
			return nil
		},
	}
}

func newMealPlanShoppingCmd(flags *rootFlags) *cobra.Command {
	var from, to, export string
	var week, aisle bool
	cmd := &cobra.Command{
		Use:         "shopping-list",
		Short:       "Aggregate ingredients across planned meals into a shopping list, optionally grouped by grocery aisle",
		Example:     "  recipe-goat-pp-cli meal-plan shopping-list --week --aisle",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			f, t := resolveDateRange(from, to, week)
			st, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer st.Close()
			entries, err := st.GetMealPlan(f, t)
			if err != nil {
				return err
			}
			// Aggregate: ingredient → list of recipe titles.
			counts := map[string]int{}
			sources := map[string][]string{}
			for _, e := range entries {
				r, err := st.GetRecipeByID(e.RecipeID)
				if err != nil || r == nil {
					continue
				}
				for _, ing := range r.Ingredients {
					key := strings.TrimSpace(ing)
					counts[key]++
					sources[key] = append(sources[key], r.Title)
				}
			}
			// Sort keys for deterministic output.
			keys := make([]string, 0, len(counts))
			for k := range counts {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			// Aisle grouping: purely heuristic keyword match. We list this as
			// experimental in the footer.
			aisleFor := func(s string) string {
				l := strings.ToLower(s)
				switch {
				case strings.Contains(l, "milk") || strings.Contains(l, "cream") || strings.Contains(l, "cheese") || strings.Contains(l, "yogurt") || strings.Contains(l, "butter") || strings.Contains(l, "egg"):
					return "Dairy"
				case strings.Contains(l, "chicken") || strings.Contains(l, "beef") || strings.Contains(l, "pork") || strings.Contains(l, "bacon") || strings.Contains(l, "sausage") || strings.Contains(l, "turkey"):
					return "Meat"
				case strings.Contains(l, "salmon") || strings.Contains(l, "shrimp") || strings.Contains(l, "tuna") || strings.Contains(l, "fish"):
					return "Seafood"
				case strings.Contains(l, "lettuce") || strings.Contains(l, "spinach") || strings.Contains(l, "tomato") || strings.Contains(l, "carrot") || strings.Contains(l, "onion") || strings.Contains(l, "garlic") || strings.Contains(l, "pepper") || strings.Contains(l, "mushroom"):
					return "Produce"
				case strings.Contains(l, "flour") || strings.Contains(l, "sugar") || strings.Contains(l, "rice") || strings.Contains(l, "pasta") || strings.Contains(l, "oats"):
					return "Pantry/Grains"
				case strings.Contains(l, "oil") || strings.Contains(l, "vinegar") || strings.Contains(l, "sauce") || strings.Contains(l, "salt") || strings.Contains(l, "pepper") || strings.Contains(l, "spice"):
					return "Condiments/Spices"
				default:
					return "Other"
				}
			}

			if flags.asJSON {
				out := []map[string]any{}
				for _, k := range keys {
					out = append(out, map[string]any{
						"ingredient": k,
						"count":      counts[k],
						"sources":    sources[k],
						"aisle":      aisleFor(k),
					})
				}
				return flags.printJSON(cmd, out)
			}

			switch strings.ToLower(export) {
			case "md":
				fmt.Fprintf(cmd.OutOrStdout(), "# Shopping list (%s — %s)\n\n", f, t)
				if aisle {
					printShoppingByAisle(cmd, keys, counts, aisleFor, "- ")
				} else {
					for _, k := range keys {
						fmt.Fprintf(cmd.OutOrStdout(), "- %s (×%d)\n", k, counts[k])
					}
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\n_unit reconciliation pending — quantities not yet combined_")
				return nil
			case "txt":
				for _, k := range keys {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%d\n", k, counts[k])
				}
				return nil
			case "csv":
				w := csv.NewWriter(cmd.OutOrStdout())
				_ = w.Write([]string{"ingredient", "count", "aisle"})
				for _, k := range keys {
					_ = w.Write([]string{k, strconv.Itoa(counts[k]), aisleFor(k)})
				}
				w.Flush()
				return w.Error()
			}

			// Plain output.
			if aisle {
				printShoppingByAisle(cmd, keys, counts, aisleFor, "  ")
			} else {
				for _, k := range keys {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s (×%d)\n", k, counts[k])
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintln(cmd.OutOrStdout(), "note: unit reconciliation pending — quantities not yet combined across recipes.")
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&week, "week", false, "Use current Mon–Sun window")
	cmd.Flags().BoolVar(&aisle, "aisle", false, "Group output by aisle")
	cmd.Flags().StringVar(&export, "export", "", "Export format: md|txt|csv")
	return cmd
}

func printShoppingByAisle(cmd *cobra.Command, keys []string, counts map[string]int, aisleFor func(string) string, indent string) {
	byAisle := map[string][]string{}
	for _, k := range keys {
		a := aisleFor(k)
		byAisle[a] = append(byAisle[a], k)
	}
	aisles := make([]string, 0, len(byAisle))
	for a := range byAisle {
		aisles = append(aisles, a)
	}
	sort.Strings(aisles)
	for _, a := range aisles {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", a)
		for _, k := range byAisle[a] {
			fmt.Fprintf(cmd.OutOrStdout(), "%s%s (×%d)\n", indent, k, counts[k])
		}
	}
}

// resolveDateRange resolves the effective [from, to] window. --week wins if
// set; otherwise explicit from/to; otherwise defaults to "today → today + 7d".
func resolveDateRange(from, to string, week bool) (string, string) {
	if week {
		now := time.Now()
		wd := int(now.Weekday())
		if wd == 0 {
			wd = 7
		}
		mon := now.AddDate(0, 0, -(wd - 1))
		sun := mon.AddDate(0, 0, 6)
		return mon.Format("2006-01-02"), sun.Format("2006-01-02")
	}
	if from != "" && to != "" {
		return from, to
	}
	now := time.Now()
	return now.Format("2006-01-02"), now.AddDate(0, 0, 7).Format("2006-01-02")
}
