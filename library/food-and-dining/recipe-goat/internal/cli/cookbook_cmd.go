package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/recipe-goat/internal/store"

	"github.com/spf13/cobra"
)

func newCookbookCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cookbook",
		Short:   "Manage your saved cookbook (list, search, tag, match)",
		Example: "  recipe-goat-pp-cli cookbook list --tag weeknight",
	}
	cmd.AddCommand(newCookbookListCmd(flags))
	cmd.AddCommand(newCookbookSearchCmd(flags))
	cmd.AddCommand(newCookbookRemoveCmd(flags))
	cmd.AddCommand(newCookbookTagCmd(flags))
	cmd.AddCommand(newCookbookUntagCmd(flags))
	cmd.AddCommand(newCookbookMatchCmd(flags))
	cmd.AddCommand(newCookbookListCmd(flags))
	cmd.AddCommand(newCookbookSearchCmd(flags))
	cmd.AddCommand(newCookbookRemoveCmd(flags))
	cmd.AddCommand(newCookbookTagCmd(flags))
	cmd.AddCommand(newCookbookUntagCmd(flags))
	cmd.AddCommand(newCookbookMatchCmd(flags))
	return cmd
}

func newCookbookListCmd(flags *rootFlags) *cobra.Command {
	var (
		tag, site, author string
		limit             int
	)
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List recipes saved to the local cookbook, optionally filtered by tag, site, or author",
		Example:     "  recipe-goat-pp-cli cookbook list --tag weeknight --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer st.Close()
			recs, err := st.ListRecipes(tag, site, author, limit, 0)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, recs)
			}
			headers := []string{"ID", "TITLE", "SITE", "AUTHOR", "TIME"}
			rows := make([][]string, 0, len(recs))
			for _, r := range recs {
				rows = append(rows, []string{
					strconv.FormatInt(r.ID, 10),
					truncate(r.Title, 60),
					r.Site,
					truncate(r.Author, 24),
					formatDuration(r.TotalTimeS),
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no saved recipes — try `save <url>` or `goat <query> --save-all`)")
				return nil
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	cmd.Flags().StringVar(&site, "site", "", "Filter by site hostname")
	cmd.Flags().StringVar(&author, "author", "", "Filter by author (substring)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max results")
	return cmd
}

func newCookbookSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		with, without string
		limit         int
	)
	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Full-text search the local cookbook by title and ingredients, with optional include/exclude filters",
		Example:     "  recipe-goat-pp-cli cookbook search \"chicken\" --with rice --without shellfish",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			st, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer st.Close()
			recs, err := st.SearchRecipesFTS(query, limit)
			if err != nil {
				return fmt.Errorf("fts search: %w", err)
			}
			// Post-filter with/without.
			if with != "" || without != "" {
				withList := splitCSV(with)
				withoutList := splitCSV(without)
				filtered := []*store.StoredRecipe{}
				for _, r := range recs {
					keep := true
					joined := strings.ToLower(strings.Join(r.Ingredients, " "))
					for _, w := range withList {
						if !strings.Contains(joined, strings.ToLower(w)) {
							keep = false
							break
						}
					}
					for _, w := range withoutList {
						if strings.Contains(joined, strings.ToLower(w)) {
							keep = false
							break
						}
					}
					if keep {
						filtered = append(filtered, r)
					}
				}
				recs = filtered
			}
			if flags.asJSON {
				return flags.printJSON(cmd, recs)
			}
			headers := []string{"ID", "TITLE", "SITE", "AUTHOR", "TIME"}
			rows := make([][]string, 0, len(recs))
			for _, r := range recs {
				rows = append(rows, []string{
					strconv.FormatInt(r.ID, 10),
					truncate(r.Title, 60),
					r.Site,
					truncate(r.Author, 24),
					formatDuration(r.TotalTimeS),
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no matches)")
				return nil
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().StringVar(&with, "with", "", "Require these ingredients (CSV)")
	cmd.Flags().StringVar(&without, "without", "", "Exclude these ingredients (CSV)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	return cmd
}

func newCookbookRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <id>",
		Short:   "Remove a recipe from the cookbook",
		Example: "  recipe-goat-pp-cli cookbook remove 42 --dry-run",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("recipe id must be an integer"))
			}
			st, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer st.Close()
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would remove recipe %d\n", id)
				return nil
			}
			if err := st.RemoveRecipe(id); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed: %d\n", id)
			return nil
		},
	}
}

func newCookbookTagCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "tag <id> <tag>[,<tag>]",
		Short:   "Attach one or more tags to a recipe",
		Example: "  recipe-goat-pp-cli cookbook tag 42 weeknight,comfort",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("recipe id must be an integer"))
			}
			tags := splitCSV(args[1])
			st, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer st.Close()
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would tag %d with: %s\n", id, strings.Join(tags, ", "))
				return nil
			}
			if err := st.TagRecipe(id, tags); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "tagged %d: %s\n", id, strings.Join(tags, ", "))
			return nil
		},
	}
}

func newCookbookUntagCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "untag <id> <tag>",
		Short:   "Remove one tag from a recipe",
		Example: "  recipe-goat-pp-cli cookbook untag 42 weeknight",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("recipe id must be an integer"))
			}
			st, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer st.Close()
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would untag %d: %s\n", id, args[1])
				return nil
			}
			if err := st.UntagRecipe(id, args[1]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "untagged %d: %s\n", id, args[1])
			return nil
		},
	}
}

func newCookbookMatchCmd(flags *rootFlags) *cobra.Command {
	var (
		have       string
		missingMax int
		limit      int
	)
	cmd := &cobra.Command{
		Use:         "match",
		Short:       "Find recipes you can make right now from pantry ingredients",
		Example:     "  recipe-goat-pp-cli cookbook match --have \"chicken,rice,broccoli\" --missing-max 2",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			haveList := splitCSV(have)
			if len(haveList) == 0 {
				return usageErr(fmt.Errorf("--have is required (CSV of ingredients you have)"))
			}
			st, err := openRecipeStore()
			if err != nil {
				return err
			}
			defer st.Close()
			matches, err := st.MatchByIngredients(haveList, missingMax)
			if err != nil {
				return err
			}
			if limit > 0 && len(matches) > limit {
				matches = matches[:limit]
			}
			if flags.asJSON {
				return flags.printJSON(cmd, matches)
			}
			if len(matches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no matches — try --missing-max to allow more gaps)")
				return nil
			}
			headers := []string{"ID", "TITLE", "MISSING", "NEEDS"}
			rows := make([][]string, 0, len(matches))
			for _, m := range matches {
				miss := ""
				if len(m.Missing) > 0 {
					miss = truncate(strings.Join(m.Missing, "; "), 80)
				} else {
					miss = "nothing"
				}
				rows = append(rows, []string{
					strconv.FormatInt(m.Recipe.ID, 10),
					truncate(m.Recipe.Title, 60),
					strconv.Itoa(len(m.Missing)),
					miss,
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	cmd.Flags().StringVar(&have, "have", "", "Comma-separated pantry ingredients you have")
	cmd.Flags().IntVar(&missingMax, "missing-max", 0, "Max ingredients you're willing to not have")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results")
	return cmd
}

func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
