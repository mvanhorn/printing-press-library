package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/recipe-goat/internal/store"

	"github.com/spf13/cobra"
)

func newCookCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cook",
		Short:   "Track what you cooked — log sessions and view history",
		Example: "  recipe-goat-pp-cli cook log 42 --rating 5",
	}
	cmd.AddCommand(newCookLogCmd(flags))
	cmd.AddCommand(newCookHistoryCmd(flags))
	cmd.AddCommand(newCookLogCmd(flags))
	cmd.AddCommand(newCookHistoryCmd(flags))
	return cmd
}

func newCookLogCmd(flags *rootFlags) *cobra.Command {
	var (
		rating int
		notes  string
		date   string
	)
	cmd := &cobra.Command{
		Use:     "log <recipe-id>",
		Short:   "Log a cooking session",
		Example: "  recipe-goat-pp-cli cook log 42 --rating 5 --notes \"too salty, used less next time\"",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("recipe-id must be an integer"))
			}
			cookedAt := time.Now().UTC()
			if date != "" {
				t, err := time.Parse("2006-01-02", date)
				if err != nil {
					return usageErr(fmt.Errorf("--date must be YYYY-MM-DD"))
				}
				cookedAt = t.UTC()
			}
			st, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer st.Close()
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would log recipe %d on %s (rating=%d)\n", id, cookedAt.Format("2006-01-02"), rating)
				return nil
			}
			if err := st.LogCook(id, rating, notes, cookedAt); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "logged: recipe %d on %s\n", id, cookedAt.Format("2006-01-02"))
			return nil
		},
	}
	cmd.Flags().IntVar(&rating, "rating", 0, "Your rating 1–5")
	cmd.Flags().StringVar(&notes, "notes", "", "Free-form notes")
	cmd.Flags().StringVar(&date, "date", "", "Date cooked (YYYY-MM-DD, default today)")
	return cmd
}

func newCookHistoryCmd(flags *rootFlags) *cobra.Command {
	var (
		recipeID int64
		limit    int
		since    string
	)
	cmd := &cobra.Command{
		Use:         "history",
		Short:       "List past cooking sessions with ratings and notes, most recent first",
		Example:     "  recipe-goat-pp-cli cook history --since 30d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer st.Close()
			var sinceT time.Time
			if since != "" {
				d, err := parseDurationShorthand(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since: %w", err))
				}
				sinceT = time.Now().Add(-d)
			}
			var entries []store.CookLogEntry
			if recipeID > 0 {
				raw, err := st.CookLogFor(recipeID)
				if err != nil {
					return err
				}
				entries = raw
			} else {
				raw, err := st.CookLogAll(limit, sinceT)
				if err != nil {
					return err
				}
				entries = raw
			}
			if flags.asJSON {
				return flags.printJSON(cmd, entries)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no cook log entries)")
				return nil
			}
			headers := []string{"ID", "RECIPE", "COOKED", "RATING", "NOTES"}
			rows := make([][]string, 0, len(entries))
			for _, e := range entries {
				rows = append(rows, []string{
					strconv.FormatInt(e.ID, 10),
					strconv.FormatInt(e.RecipeID, 10),
					e.CookedAt.Format("2006-01-02"),
					strconv.Itoa(e.Rating),
					truncate(e.Notes, 48),
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().Int64Var(&recipeID, "recipe-id", 0, "Filter by recipe id")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max entries")
	cmd.Flags().StringVar(&since, "since", "", "Show entries newer than this (e.g., 30d)")
	return cmd
}
