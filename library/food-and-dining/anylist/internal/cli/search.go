package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"

	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var kind string

	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "Full-text search across items and recipes in the local cache",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Search items and recipes in the local SQLite cache using full-text search.
Returns matching items (with list and checked status) and recipes (with
rating). Requires sync — run 'anylist-pp-cli sync' first.`,
		Example: `  anylist-pp-cli search "chicken"
  anylist-pp-cli search "pasta" --kind recipe --json
  anylist-pp-cli search "milk" --kind item`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			st, err := store.Open(cfg)
			if err != nil {
				return fmt.Errorf("no local data found — run 'anylist-pp-cli sync' first")
			}
			defer st.Close()

			type result struct {
				Kind     string `json:"kind"`
				Name     string `json:"name"`
				List     string `json:"list,omitempty"`
				Quantity string `json:"quantity,omitempty"`
				Checked  bool   `json:"checked,omitempty"`
				Rating   int    `json:"rating,omitempty"`
				Note     string `json:"note,omitempty"`
			}

			var results []result

			if kind == "" || kind == "item" {
				items, _ := st.SearchItems(query)
				for _, it := range items {
					results = append(results, result{
						Kind:     "item",
						Name:     it.Name,
						List:     it.ListName,
						Quantity: it.Quantity,
						Checked:  it.Checked,
					})
				}
			}

			if kind == "" || kind == "recipe" {
				recipes, _ := st.SearchRecipesByName(query)
				for _, r := range recipes {
					results = append(results, result{
						Kind:   "recipe",
						Name:   r.Name,
						Rating: r.Rating,
						Note:   r.Note,
					})
				}
			}

			if results == nil {
				results = []result{}
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No results")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "KIND\tNAME\tDETAIL")
			for _, r := range results {
				detail := r.List
				if r.Kind == "recipe" && r.Rating > 0 {
					detail = fmt.Sprintf("rating:%d", r.Rating)
				}
				if r.Checked {
					detail += " (checked)"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Kind, r.Name, detail)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&kind, "kind", "", "Filter by kind: item, recipe (default: all)")
	return cmd
}
