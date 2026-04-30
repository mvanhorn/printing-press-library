package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newBrowseCmd returns the `browse <type>` command — a friendly top-level
// wrapper around the proxy's networkentity browse with positional <type>
// instead of a flag, plus a --verified-only filter that the underlying API
// does not expose as a parameter.
func newBrowseCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var offset int
	var sort string
	var categoryRef string
	var minMonthlyForks int
	var verifiedOnly bool
	cmd := &cobra.Command{
		Use:         "browse <type>",
		Short:       "Browse public entities, with --verified-only filter",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Browse public entities of a given type (collection, workspace, api, or flow)
on the public API network. Hits the proxy live; --verified-only filters
results client-side using the publisher info attached to the response so the
output contains only entities published by verified teams.`,
		Example: strings.Trim(`
  # Top 20 popular collections
  postman-explore-pp-cli browse collection --limit 20

  # Verified-only collections in the payments category (id 7)
  postman-explore-pp-cli browse collection --category 7 --verified-only --limit 10

  # JSON output for piping
  postman-explore-pp-cli browse collection --verified-only --json --limit 5`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			entityType := args[0]
			if !validEntityType(entityType) {
				return usageErr(fmt.Errorf("invalid type %q (use one of collection, workspace, api, flow)", entityType))
			}
			if sort != "popular" {
				return usageErr(fmt.Errorf("--sort %q not supported by the API; only 'popular' works", sort))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{
				"entityType": entityType,
				"limit":      fmt.Sprintf("%d", limit),
				"offset":     fmt.Sprintf("%d", offset),
				"sort":       sort,
			}
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
					params["categoryId"] = fmt.Sprintf("%d", categoryID)
				}
			}
			if minMonthlyForks > 0 {
				params["minMonthlyForkCount"] = fmt.Sprintf("%d", minMonthlyForks)
			}

			data, err := c.Get("/v1/api/networkentity", params)
			if err != nil {
				return classifyAPIError(err)
			}

			// Parse response so we can apply --verified-only client-side
			var resp struct {
				Data []json.RawMessage `json:"data"`
				Meta json.RawMessage   `json:"meta"`
			}
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("decoding browse response: %w", err)
			}

			verifiedMap := publisherInfoMap(data)
			out := make([]json.RawMessage, 0, len(resp.Data))
			for _, item := range resp.Data {
				if verifiedOnly {
					pid := entityPublisherID(item)
					if pid == "" || !verifiedMap[pid] {
						continue
					}
				}
				out = append(out, item)
			}
			if flags.asJSON {
				envelope := map[string]any{"data": out}
				if len(out) == 0 {
					envelope["data"] = []any{}
				}
				return printJSONFiltered(cmd, envelope, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), emptyMessage("no matching entities (try without --verified-only or widen --category)"))
				return nil
			}
			tbl := make([][]string, 0, len(out))
			for _, raw := range out {
				var doc struct {
					ID          any    `json:"id"`
					Name        string `json:"name"`
					Summary     string `json:"summary"`
					EntityType  string `json:"entityType"`
					RedirectURL string `json:"redirectURL"`
				}
				_ = json.Unmarshal(raw, &doc)
				marker := " "
				if pid := entityPublisherID(raw); pid != "" && verifiedMap[pid] {
					marker = "✓"
				}
				tbl = append(tbl, []string{
					marker,
					stringify(doc.ID),
					doc.Name,
					truncate(doc.Summary, 60),
					doc.RedirectURL,
				})
			}
			return flags.printTable(cmd, []string{"V", "ID", "Name", "Summary", "URL"}, tbl)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Number of results to return per page")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	cmd.Flags().StringVar(&sort, "sort", "popular", "Sort order (only 'popular' is supported by the proxy)")
	cmd.Flags().StringVar(&categoryRef, "category", "", "Filter to a category — accepts a slug (developer-productivity) or a numeric id (4)")
	cmd.Flags().IntVar(&minMonthlyForks, "min-monthly-forks", 0, "Drop entities with fewer than N monthly forks")
	cmd.Flags().BoolVar(&verifiedOnly, "verified-only", false, "Filter to entities owned by verified publishers")
	return cmd
}
