// Hand-written novel feature: links duplicates.
// Find every link in the workspace pointing to the same destination URL.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type dupGroup struct {
	URL   string         `json:"url"`
	Count int            `json:"count"`
	Links []dupLinkEntry `json:"links"`
}

type dupLinkEntry struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
	Key    string `json:"key"`
	Clicks int    `json:"clicks"`
}

func newLinksDuplicatesCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var minCount int
	var ignoreCase bool

	cmd := &cobra.Command{
		Use:   "duplicates",
		Short: "Find every link in the workspace pointing to the same destination URL",
		Long: `Group local links by their normalized destination URL and surface
groups with two or more entries — the consolidation candidates.

Useful after bulk-create overruns or when migrating UTMs and you want to
audit overlap before deleting.`,
		Example: strings.Trim(`
  # Default: groups of 2+ duplicates
  dub-pp-cli links duplicates --json

  # Tighter: only groups of 5+ (campaign-wide overlap)
  dub-pp-cli links duplicates --min-count 5 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT id, COALESCE(domain,''), COALESCE("key",''), COALESCE(data,'{}')
				FROM links
			`)
			if err != nil {
				return fmt.Errorf("querying links: %w", err)
			}
			defer rows.Close()

			groups := map[string][]dupLinkEntry{}
			seen := 0
			for rows.Next() {
				var id, domain, key string
				var blob sql.NullString
				if err := rows.Scan(&id, &domain, &key, &blob); err != nil {
					return err
				}
				seen++
				blobBytes := []byte(blob.String)
				if !blob.Valid {
					blobBytes = []byte("{}")
				}
				url := extractFromData(blobBytes, "url")
				if url == "" {
					continue
				}
				bucket := url
				if ignoreCase {
					bucket = strings.ToLower(url)
				}
				groups[bucket] = append(groups[bucket], dupLinkEntry{
					ID:     id,
					Domain: domain,
					Key:    key,
					Clicks: int(extractNumber(blobBytes, "clicks")),
				})
			}
			if seen == 0 {
				return hintEmptyStore("links")
			}

			var results []dupGroup
			for url, entries := range groups {
				if len(entries) < minCount {
					continue
				}
				results = append(results, dupGroup{URL: url, Count: len(entries), Links: entries})
			}
			// Sort largest groups first, then by url.
			for i := 0; i < len(results); i++ {
				for j := i + 1; j < len(results); j++ {
					if results[j].Count > results[i].Count ||
						(results[j].Count == results[i].Count && results[j].URL < results[i].URL) {
						results[i], results[j] = results[j], results[i]
					}
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No duplicate destinations found.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-6s %s\n", "COUNT", "URL")
			for _, g := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-6d %s\n", g.Count, truncate(g.URL, 90))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&minCount, "min-count", 2, "Minimum group size to report")
	cmd.Flags().BoolVar(&ignoreCase, "ignore-case", false, "Compare URLs case-insensitively")
	return cmd
}
