// Package cli provides the health command for checking link destination URLs.
// HTTP HEAD each link's destination URL to find broken links (4xx, 5xx, timeout).
// Supports --domain filter, --tag filter, --timeout for HEAD requests.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

func newHealthCmd(flags *rootFlags) *cobra.Command {
	var flagDomain string
	var flagTag string
	var flagHeadTimeout time.Duration
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check destination URL health for all links",
		Long: `Fetch all links and HTTP HEAD each destination URL to detect broken links.
Reports status as OK, BROKEN (4xx/5xx), TIMEOUT, or REDIRECT (3xx).`,
		Example: `  # Check all links
  dub-pp-cli health

  # Check links for a specific domain
  dub-pp-cli health --domain ac.me

  # Use a longer timeout for slow destinations
  dub-pp-cli health --head-timeout 10s

  # Output as JSON
  dub-pp-cli health --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Build query params
			params := map[string]string{
				"pageSize": "100",
			}
			if flagDomain != "" {
				params["domain"] = flagDomain
			}
			if flagTag != "" {
				params["tagNames"] = flagTag
			}

			// Collect all links with pagination
			var allLinks []map[string]any
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

				allLinks = append(allLinks, items...)

				if flagLimit > 0 && len(allLinks) >= flagLimit {
					allLinks = allLinks[:flagLimit]
					break
				}

				lastItem := items[len(items)-1]
				lastID, _ := lastItem["id"].(string)
				if lastID == "" || len(items) < 100 {
					break
				}
				cursor = lastID
			}

			if len(allLinks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No links found.")
				return nil
			}

			// HEAD each destination URL
			httpClient := &http.Client{
				Timeout: flagHeadTimeout,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}

			type healthResult struct {
				ShortLink   string `json:"shortLink"`
				Destination string `json:"destination"`
				Status      string `json:"status"`
				StatusCode  int    `json:"statusCode,omitempty"`
			}

			var results []healthResult
			var okCount, brokenCount, timeoutCount, redirectCount int

			for i, link := range allLinks {
				shortLink, _ := link["shortLink"].(string)
				destURL, _ := link["url"].(string)
				if destURL == "" {
					continue
				}

				fmt.Fprintf(cmd.ErrOrStderr(), "\rChecking %d/%d...", i+1, len(allLinks))

				var status string
				var statusCode int

				resp, err := httpClient.Head(destURL)
				if err != nil {
					status = "TIMEOUT"
					timeoutCount++
				} else {
					statusCode = resp.StatusCode
					resp.Body.Close()
					switch {
					case statusCode >= 200 && statusCode < 300:
						status = "OK"
						okCount++
					case statusCode >= 300 && statusCode < 400:
						status = "REDIRECT"
						redirectCount++
					default:
						status = "BROKEN"
						brokenCount++
					}
				}

				results = append(results, healthResult{
					ShortLink:   shortLink,
					Destination: destURL,
					Status:      status,
					StatusCode:  statusCode,
				})
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "\r") // clear progress line

			// Output results
			if flags.asJSON {
				output := map[string]any{
					"results": results,
					"summary": map[string]any{
						"total":    len(results),
						"ok":       okCount,
						"broken":   brokenCount,
						"timeout":  timeoutCount,
						"redirect": redirectCount,
					},
				}
				data, _ := json.Marshal(output)
				return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
			}

			// Table output
			headers := []string{"LINK", "DESTINATION", "STATUS"}
			var rows [][]string
			for _, r := range results {
				rows = append(rows, []string{
					truncate(r.ShortLink, 40),
					truncate(r.Destination, 50),
					r.Status,
				})
			}

			if err := flags.printTable(cmd, headers, rows); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n%d of %d links healthy, %d broken, %d timeout\n",
				okCount, len(results), brokenCount, timeoutCount)
			return nil
		},
	}

	cmd.Flags().StringVar(&flagDomain, "domain", "", "Filter links by domain (e.g. ac.me)")
	cmd.Flags().StringVar(&flagTag, "tag", "", "Filter links by tag name")
	cmd.Flags().DurationVar(&flagHeadTimeout, "head-timeout", 5*time.Second, "Timeout for HEAD requests to destination URLs")
	cmd.Flags().IntVar(&flagLimit, "limit", 500, "Maximum links to check")

	return cmd
}
