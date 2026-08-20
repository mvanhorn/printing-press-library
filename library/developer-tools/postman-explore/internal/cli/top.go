package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// newTopCmd returns the `top` command — rank synced entities by any metric
// dimension (weekForkCount, monthViewCount, etc.) with optional category and
// entity-type narrowing. Reads exclusively from the local store.
func newTopCmd(flags *rootFlags) *cobra.Command {
	var metric string
	var entityType string
	var categoryRef string
	var limit int
	cmd := &cobra.Command{
		Use:         "top",
		Short:       "Rank entities by any metric (weekForkCount, monthViewCount, etc.)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Rank locally-synced entities by a chosen metric dimension. Unlike the proxy's
single sort=popular option, this command exposes all 10 metric dimensions
returned in entity payloads for trend ranking with category narrowing.

Run 'sync' first; this command reads from the local store.`,
		Example: strings.Trim(`
  # Top 10 collections by weekly fork count
  postman-explore-pp-cli top --metric weekForkCount --type collection --limit 10

  # Top 5 monthly-view collections in the payments category (id 7)
  postman-explore-pp-cli top --metric monthViewCount --type collection --category 7 --limit 5

  # JSON for piping
  postman-explore-pp-cli top --metric forkCount --type api --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if !validMetric(metric) {
				return usageErr(fmt.Errorf("invalid --metric %q (use one of %s)", metric, metricNamesList()))
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
			if len(items) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd, []any{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), emptyMessage("no synced entities of this type — run 'sync' first"))
				return nil
			}

			type ranked struct {
				Name        string `json:"name"`
				Publisher   string `json:"publisherHandle,omitempty"`
				EntityType  string `json:"entityType"`
				MetricName  string `json:"metric"`
				MetricValue int64  `json:"value"`
				ID          string `json:"id"`
				RedirectURL string `json:"redirectURL,omitempty"`
			}
			out := make([]ranked, 0, len(items))
			for _, raw := range items {
				value := extractMetricValue(raw, metric)
				if value == 0 {
					continue
				}
				var doc struct {
					ID          any    `json:"id"`
					Name        string `json:"name"`
					EntityType  string `json:"entityType"`
					RedirectURL string `json:"redirectURL"`
					Meta        struct {
						PublisherID string `json:"publisherId"`
					} `json:"meta"`
				}
				_ = json.Unmarshal(raw, &doc)
				out = append(out, ranked{
					Name:        doc.Name,
					Publisher:   doc.Meta.PublisherID,
					EntityType:  doc.EntityType,
					MetricName:  metric,
					MetricValue: value,
					ID:          stringify(doc.ID),
					RedirectURL: doc.RedirectURL,
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].MetricValue > out[j].MetricValue })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			if flags.asJSON {
				return printJSONFiltered(cmd, out, flags)
			}
			rows2 := make([][]string, 0, len(out))
			for i, r := range out {
				rows2 = append(rows2, []string{
					fmt.Sprintf("%d", i+1),
					r.Name,
					fmt.Sprintf("%d", r.MetricValue),
					r.RedirectURL,
				})
			}
			return flags.printTable(cmd, []string{"#", "Name", metric, "URL"}, rows2)
		},
	}
	cmd.Flags().StringVar(&metric, "metric", "weekForkCount", "Metric to rank by")
	cmd.Flags().StringVar(&entityType, "type", "collection", "Entity type (collection, workspace, api, flow)")
	cmd.Flags().StringVar(&categoryRef, "category", "", "Category to narrow results — accepts a slug (developer-productivity) or a numeric id (4)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum results to return")
	return cmd
}
