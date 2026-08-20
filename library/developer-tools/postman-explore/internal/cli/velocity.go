package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// newVelocityCmd returns the `velocity` command — rank synced collections by
// fork-rate acceleration (weekFork × 4 / monthFork). Surfaces breakouts that
// haven't yet topped the popular sort.
func newVelocityCmd(flags *rootFlags) *cobra.Command {
	var entityType string
	var top int
	var categoryRef string
	var minMonth int64
	cmd := &cobra.Command{
		Use:         "velocity",
		Short:       "Rank entities by fork acceleration (weekFork × 4 / monthFork)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Acceleration metric: an entity with weekForkCount × 4 > monthForkCount is
forking faster this week than its monthly average. Useful for catching
collections breaking out before they top the all-time popular list.

Run 'sync' first; this command reads the local store.`,
		Example: strings.Trim(`
  # Top 10 accelerating collections
  postman-explore-pp-cli velocity --top 10

  # Filter to high-traffic entities (≥ 50 monthly forks) to drop noise
  postman-explore-pp-cli velocity --type collection --top 10 --min-monthly 50

  # Within payments category
  postman-explore-pp-cli velocity --category 7 --top 5 --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if !validEntityType(entityType) {
				return usageErr(fmt.Errorf("invalid --type %q (use one of collection, workspace, api, flow)", entityType))
			}
			db, err := openLocalStore(flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			extraWhere := ""
			if categoryRef != "" {
				resolver, err := newCategoryResolverFromFlags(flags)
				if err != nil {
					return err
				}
				categoryID, err := resolveCategoryID(resolver, categoryRef)
				if err != nil {
					return usageErr(err)
				}
				if categoryID > 0 {
					extraWhere = `data LIKE '%"id":` + fmt.Sprintf("%d", categoryID) + `,%'`
				}
			}
			items, err := queryNetworkEntities(db, entityType, extraWhere)
			if err != nil {
				return err
			}

			type entry struct {
				Name           string  `json:"name"`
				ID             string  `json:"id"`
				WeekForkCount  int64   `json:"weekForkCount"`
				MonthForkCount int64   `json:"monthForkCount"`
				Acceleration   float64 `json:"acceleration"`
				RedirectURL    string  `json:"redirectURL,omitempty"`
			}
			out := make([]entry, 0)
			for _, raw := range items {
				week := extractMetricValue(raw, "weekForkCount")
				month := extractMetricValue(raw, "monthForkCount")
				if month < minMonth {
					continue
				}
				if month == 0 {
					continue
				}
				acc := float64(week*4) / float64(month)
				if acc <= 0 {
					continue
				}
				var doc struct {
					ID          any    `json:"id"`
					Name        string `json:"name"`
					RedirectURL string `json:"redirectURL"`
				}
				_ = json.Unmarshal(raw, &doc)
				out = append(out, entry{
					Name:           doc.Name,
					ID:             stringify(doc.ID),
					WeekForkCount:  week,
					MonthForkCount: month,
					Acceleration:   acc,
					RedirectURL:    doc.RedirectURL,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Acceleration > out[j].Acceleration })
			if top > 0 && len(out) > top {
				out = out[:top]
			}
			if len(out) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd, []any{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), emptyMessage("no candidates after filtering — lower --min-monthly or run 'sync' first"))
				return nil
			}
			if flags.asJSON {
				return printJSONFiltered(cmd, out, flags)
			}
			tbl := make([][]string, 0, len(out))
			for i, e := range out {
				tbl = append(tbl, []string{
					fmt.Sprintf("%d", i+1),
					e.Name,
					fmt.Sprintf("%d", e.WeekForkCount),
					fmt.Sprintf("%d", e.MonthForkCount),
					fmt.Sprintf("%.2f×", e.Acceleration),
					e.RedirectURL,
				})
			}
			return flags.printTable(cmd, []string{"#", "Name", "WeekFork", "MonthFork", "Accel", "URL"}, tbl)
		},
	}
	cmd.Flags().StringVar(&entityType, "type", "collection", "Entity type (collection, workspace, api, flow)")
	cmd.Flags().IntVar(&top, "top", 10, "How many top accelerating entities to return")
	// PATCH: use the short slug form accepted by the category resolver in help text.
	cmd.Flags().StringVar(&categoryRef, "category", "", "Category to narrow results — accepts a slug (payments) or a numeric id (7)")
	cmd.Flags().Int64Var(&minMonth, "min-monthly", 0, "Drop entities with monthForkCount below this floor (denoising)")
	return cmd
}
