package cli

import (
	"fmt"
	"strings"

	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/spf13/cobra"
)

var (
	searchQuery string
	searchType  string
)

// searchCmd finds scenes and sources matching a query string.
// Supports searching across all scenes or within a specific scene.
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Search for scenes and sources by name",
	Long: `Search across all OBS scenes and sources for items matching the query.

Searches are case-insensitive substring matches against scene names and source names.
Use --type to limit results to "scene" or "source".`,
	Example: `  # Search for anything matching "interview"
  obs-pp-cli search --query "interview"

  # Search only sources
  obs-pp-cli search --query "camera" --type source

  # JSON output for agent piping
  obs-pp-cli search --query "guest" --format=json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		query := searchQuery
		kind := searchType

		c, err := client.New()
		if err != nil {
			return err
		}
		defer c.Disconnect()

		sceneList, err := c.Scenes.GetSceneList()
		if err != nil {
			return fmt.Errorf("failed to list scenes: %w", err)
		}

		type result struct {
			Type  string `json:"type"`
			Scene string `json:"scene,omitempty"`
			Name  string `json:"name"`
		}
		var results []result
		q := strings.ToLower(query)

		// Scene search
		if kind == "" || kind == "scene" {
			for _, s := range sceneList.Scenes {
				if q == "" || strings.Contains(strings.ToLower(s.SceneName), q) {
					results = append(results, result{Type: "scene", Name: s.SceneName})
				}
			}
		}

		// Source search (scan all scenes)
		if kind == "" || kind == "source" {
			for _, s := range sceneList.Scenes {
				items, err := c.SceneItems.GetSceneItemList(
					sceneitems.NewGetSceneItemListParams().WithSceneName(s.SceneName),
				)
				if err != nil {
					continue
				}
				for _, item := range items.SceneItems {
					if q == "" || strings.Contains(strings.ToLower(item.SourceName), q) {
						results = append(results, result{
							Type:  "source",
							Scene: s.SceneName,
							Name:  item.SourceName,
						})
					}
				}
			}
		}

		if currentFormat() == "json" {
			return printJSON(map[string]any{
				"query":   query,
				"count":   len(results),
				"results": results,
			})
		}

		if len(results) == 0 {
			fmt.Printf("No results for %q\n", query)
			return nil
		}

		w := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintf(w, "TYPE\tNAME\tSCENE\n")
		for _, r := range results {
			fmt.Fprintf(w, "%s\t%s\t%s\n", r.Type, r.Name, r.Scene)
		}
		return w.Flush()
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchQuery, "query", "", "Search query (case-insensitive substring match)")
	searchCmd.Flags().StringVar(&searchType, "type", "", `Limit results to "scene" or "source"`)
	_ = searchCmd.MarkFlagRequired("query")
	_ = searchCmd.MarkFlagRequired("query")
}
