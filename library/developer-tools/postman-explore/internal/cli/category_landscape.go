package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// newCategoryLandscapeCmd returns the `category landscape <slug>` subcommand.
// Combines per-type entity counts, top publishers by aggregate forks, and top
// entities by viewCount into a single per-category snapshot.
//
// Wired in by the existing category command tree (see registerNovelCommands).
func newCategoryLandscapeCmd(flags *rootFlags) *cobra.Command {
	var topEntities int
	var topPublishers int
	// PATCH: category landscape examples prefer short slugs accepted by the resolver and show slug discovery.
	cmd := &cobra.Command{
		Use:         "landscape <slug>",
		Short:       "Per-category snapshot: counts, top publishers, top entities",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Hits the live category endpoint for the slug, then walks the local store
to compute (a) per-type entity counts, (b) top publishers by aggregate
forkCount, and (c) top entities by viewCount within the category. Output is a
single structured JSON payload (or human-readable summary).

Run 'sync' first; the publisher and entity rankings come from the local store.`,
		Example: strings.Trim(`
  # Snapshot for the payments category
  postman-explore-pp-cli category landscape payments

  # JSON output
  postman-explore-pp-cli category landscape developer-productivity --json

  # Discover valid category slugs and IDs
  postman-explore-pp-cli category list-categories --json --select id,name,slug`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := strings.TrimSpace(args[0])
			if slug == "" {
				return usageErr(fmt.Errorf("category slug is required"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			catData, err := c.Get("/v2/api/category/"+slug, nil)
			if err != nil {
				return classifyAPIError(err)
			}
			var catResp struct {
				Data struct {
					ID      int    `json:"id"`
					Name    string `json:"name"`
					Slug    string `json:"slug"`
					Summary string `json:"summary"`
				} `json:"data"`
			}
			if err := json.Unmarshal(catData, &catResp); err != nil {
				return fmt.Errorf("decoding category: %w", err)
			}
			if catResp.Data.ID == 0 {
				return notFoundErr(fmt.Errorf("category %q not found on the network", slug))
			}

			db, err := openLocalStore()
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			// Fetch every locally-cached entity that mentions this category id.
			// Sync routes typed entities through the generic upsert path, so we
			// query the resources table filtered to networkentity sub-types.
			items, err := queryAllNetworkEntities(db, `data LIKE '%"id":`+fmt.Sprintf("%d", catResp.Data.ID)+`,%'`)
			if err != nil {
				return err
			}

			counts := map[string]int{}
			byPublisher := map[string]int64{}
			pubEntities := map[string]int{}
			type viewItem struct {
				ID          string
				Name        string
				EntityType  string
				ViewCount   int64
				RedirectURL string
			}
			var byView []viewItem
			for _, raw := range items {
				var doc struct {
					ID          any    `json:"id"`
					Name        string `json:"name"`
					EntityType  string `json:"entityType"`
					RedirectURL string `json:"redirectURL"`
				}
				_ = json.Unmarshal(raw, &doc)
				counts[doc.EntityType]++
				pid := entityPublisherID(raw)
				if pid != "" {
					byPublisher[pid] += extractMetricValue(raw, "forkCount")
					pubEntities[pid]++
				}
				byView = append(byView, viewItem{
					ID:          stringify(doc.ID),
					Name:        doc.Name,
					EntityType:  doc.EntityType,
					ViewCount:   extractMetricValue(raw, "viewCount"),
					RedirectURL: doc.RedirectURL,
				})
			}

			type pubRank struct {
				PublisherID  string `json:"publisherId"`
				PublicHandle string `json:"publicHandle,omitempty"`
				Entities     int    `json:"entities"`
				TotalForks   int64  `json:"totalForkCount"`
			}
			pubs := make([]pubRank, 0, len(byPublisher))
			handles := lookupPublisherHandles(db.DB())
			for id, forks := range byPublisher {
				pubs = append(pubs, pubRank{
					PublisherID:  id,
					PublicHandle: handles[id],
					Entities:     pubEntities[id],
					TotalForks:   forks,
				})
			}
			sort.Slice(pubs, func(i, j int) bool { return pubs[i].TotalForks > pubs[j].TotalForks })
			if topPublishers > 0 && len(pubs) > topPublishers {
				pubs = pubs[:topPublishers]
			}

			sort.Slice(byView, func(i, j int) bool { return byView[i].ViewCount > byView[j].ViewCount })
			if topEntities > 0 && len(byView) > topEntities {
				byView = byView[:topEntities]
			}

			type viewRank struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				EntityType  string `json:"entityType"`
				ViewCount   int64  `json:"viewCount"`
				RedirectURL string `json:"redirectURL,omitempty"`
			}
			topViews := make([]viewRank, 0, len(byView))
			for _, v := range byView {
				topViews = append(topViews, viewRank(v))
			}

			report := map[string]any{
				"category": map[string]any{
					"id":      catResp.Data.ID,
					"name":    catResp.Data.Name,
					"slug":    catResp.Data.Slug,
					"summary": catResp.Data.Summary,
				},
				"localCounts":   counts,
				"localTotal":    len(items),
				"topPublishers": pubs,
				"topEntities":   topViews,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd, report, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Category %s (id=%d) — %s\n", catResp.Data.Name, catResp.Data.ID, catResp.Data.Summary)
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintf(cmd.OutOrStdout(), "Local entities: %d\n", len(items))
			for _, t := range []string{"collection", "workspace", "api", "flow"} {
				if n := counts[t]; n > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d\n", t, n)
				}
			}
			if len(pubs) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "")
				fmt.Fprintln(cmd.OutOrStdout(), "Top publishers:")
				for i, p := range pubs {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s (id=%s) — %d entities, %d total forks\n", i+1, defaultStr(p.PublicHandle, "(no handle)"), p.PublisherID, p.Entities, p.TotalForks)
				}
			}
			if len(topViews) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "")
				fmt.Fprintln(cmd.OutOrStdout(), "Top entities by views:")
				for i, v := range topViews {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s [%s] — %d views\n", i+1, v.Name, v.EntityType, v.ViewCount)
				}
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "")
				fmt.Fprintln(cmd.OutOrStdout(), "(no synced entities for this category — run 'sync' first)")
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&topPublishers, "top-publishers", 5, "Number of publishers to include in the report")
	cmd.Flags().IntVar(&topEntities, "top-entities", 5, "Number of top-viewed entities to include in the report")
	return cmd
}

func defaultStr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
