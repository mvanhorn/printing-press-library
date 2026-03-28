// Package cli provides the stale command for detecting zero-click links.
// Uses the Dub API to find links that haven't received any clicks within a configurable threshold.
// Calls GET /links to list all links, then filters by the `clicks` field.
// Supports --days flag (default 30), --domain filter, --tag filter.
// Outputs table by default, --json for machine consumption.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagDomain string
	var flagTag string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Find links with zero clicks in the last N days",
		Long: `Scan all links and report those with zero clicks that were created more than
N days ago. Useful for identifying abandoned or underperforming links.`,
		Example: `  # Find links with 0 clicks older than 30 days
  dub-pp-cli stale

  # Find stale links older than 90 days
  dub-pp-cli stale --days 90

  # Filter by domain
  dub-pp-cli stale --domain ac.me

  # Output as JSON
  dub-pp-cli stale --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "GET /links?pageSize=100 (paginate all, filter clicks=0, created before %d days ago)\n", flagDays)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			cutoff := time.Now().AddDate(0, 0, -flagDays)

			// Build query params for the links endpoint
			params := map[string]string{
				"pageSize": "100",
			}
			if flagDomain != "" {
				params["domain"] = flagDomain
			}
			if flagTag != "" {
				params["tagNames"] = flagTag
			}

			// Paginate through all links using startingAfter cursor
			var staleLinks []map[string]any
			cursor := ""

			for {
				if cursor != "" {
					params["startingAfter"] = cursor
				}

				data, err := c.Get("/links", params)
				if err != nil {
					return classifyAPIError(err)
				}

				var items []map[string]any
				if err := json.Unmarshal(data, &items); err != nil {
					break
				}
				if len(items) == 0 {
					break
				}

				for _, link := range items {
					// Extract clicks count
					clicks := 0
					if v, ok := link["clicks"]; ok {
						switch c := v.(type) {
						case float64:
							clicks = int(c)
						case int:
							clicks = c
						}
					}

					// Extract createdAt and check if older than cutoff
					createdAtStr, _ := link["createdAt"].(string)
					if createdAtStr == "" {
						continue
					}
					createdAt, err := time.Parse(time.RFC3339, createdAtStr)
					if err != nil {
						continue
					}

					if clicks == 0 && createdAt.Before(cutoff) {
						staleLinks = append(staleLinks, link)
						if flagLimit > 0 && len(staleLinks) >= flagLimit {
							break
						}
					}
				}

				if flagLimit > 0 && len(staleLinks) >= flagLimit {
					break
				}

				// Get cursor for next page from the last item's id
				lastItem := items[len(items)-1]
				lastID, _ := lastItem["id"].(string)
				if lastID == "" || len(items) < 100 {
					break
				}
				cursor = lastID
			}

			// Output results
			if flags.asJSON {
				result, _ := json.Marshal(staleLinks)
				return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(result), flags)
			}

			if len(staleLinks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No stale links found.")
				return nil
			}

			// Table output
			headers := []string{"SHORT_LINK", "URL", "CREATED", "CLICKS"}
			var rows [][]string
			for _, link := range staleLinks {
				shortLink, _ := link["shortLink"].(string)
				url, _ := link["url"].(string)
				createdAt, _ := link["createdAt"].(string)
				clicks := 0
				if v, ok := link["clicks"]; ok {
					if c, ok := v.(float64); ok {
						clicks = int(c)
					}
				}
				rows = append(rows, []string{
					truncate(shortLink, 40),
					truncate(url, 50),
					createdAt[:10],
					fmt.Sprintf("%d", clicks),
				})
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Found %d stale links (0 clicks, older than %d days)\n", len(staleLinks), flagDays)
			return flags.printTable(cmd, headers, rows)
		},
	}

	cmd.Flags().IntVar(&flagDays, "days", 30, "Minimum age in days for a link to be considered stale")
	cmd.Flags().StringVar(&flagDomain, "domain", "", "Filter links by domain (e.g. ac.me)")
	cmd.Flags().StringVar(&flagTag, "tag", "", "Filter links by tag name")
	cmd.Flags().IntVar(&flagLimit, "limit", 500, "Maximum stale links to return")

	return cmd
}
