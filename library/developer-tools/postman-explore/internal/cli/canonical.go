package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// newCanonicalCmd returns the `canonical <vendor>` command — the headline
// novel feature. It calls the live search-all endpoint, ranks hits by
// (publisher verification × fork count × recency), and returns the single
// most-likely-canonical Postman Collection plus runner-ups.
func newCanonicalCmd(flags *rootFlags) *cobra.Command {
	var entityType string
	var limit int
	cmd := &cobra.Command{
		Use:         "canonical <vendor>",
		Short:       "Find the canonical community Postman Collection for a vendor",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Search the public API network for community Postman entities matching <vendor>
and return the single most-likely-canonical pick plus runner-ups, ranked by
publisher verification, fork count, and recency.

This is the primary discovery move when you know a vendor name (Stripe, Twilio,
Notion) and want one good collection rather than a deduplicated search list.`,
		Example: strings.Trim(`
  # Find the canonical Stripe collection
  postman-explore-pp-cli canonical stripe

  # JSON output for piping
  postman-explore-pp-cli canonical twilio --json --limit 5

  # Narrow to APIs (default is collection)
  postman-explore-pp-cli canonical github --type api`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			vendor := strings.TrimSpace(args[0])
			if vendor == "" {
				return usageErr(fmt.Errorf("vendor name is required"))
			}
			if !validEntityType(entityType) {
				return usageErr(fmt.Errorf("invalid --type %q (use one of collection, workspace, api, flow)", entityType))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Map our friendly type to the dotted queryIndex
			indexByType := map[string]string{
				"collection": "runtime.collection",
				"workspace":  "collaboration.workspace",
				"api":        "runtime.collection", // API entries surface under runtime.collection in search
				"flow":       "flow.flow",
			}
			body := map[string]any{
				"queryText":         vendor,
				"queryIndices":      []string{indexByType[entityType]},
				"size":              25,
				"from":              0,
				"domain":            "public",
				"mergeEntities":     true,
				"nonNestedRequests": true,
			}
			data, _, err := c.Post("/search-all", body)
			if err != nil {
				return classifyAPIError(err)
			}

			candidates := extractSearchHits(data, entityType)
			if len(candidates) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd, []any{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), emptyMessage("no matching entity in the public network"))
				return nil
			}
			rankCanonical(candidates, vendor)
			if limit > 0 && len(candidates) > limit {
				candidates = candidates[:limit]
			}

			if flags.asJSON {
				return printJSONFiltered(cmd, candidates, flags)
			}
			rows := make([][]string, 0, len(candidates))
			for i, c := range candidates {
				star := " "
				if i == 0 {
					star = "★"
				}
				verified := ""
				if c.PublisherVerified {
					verified = "✓"
				}
				rows = append(rows, []string{
					star,
					c.Name,
					c.PublisherHandle,
					verified,
					fmt.Sprintf("%d", c.ForkCount),
					c.RedirectURL,
				})
			}
			return flags.printTable(cmd, []string{"", "Name", "Publisher", "Verified", "Forks", "URL"}, rows)
		},
	}
	cmd.Flags().StringVar(&entityType, "type", "collection", "Entity type to search (collection, workspace, api, flow)")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum results to return (top-1 is the canonical pick)")
	return cmd
}

// canonicalCandidate holds the fields used for ranking.
type canonicalCandidate struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Summary           string `json:"summary,omitempty"`
	PublisherID       string `json:"publisherId,omitempty"`
	PublisherHandle   string `json:"publisherHandle,omitempty"`
	PublisherVerified bool   `json:"publisherVerified"`
	EntityType        string `json:"entityType"`
	ForkCount         int64  `json:"forkCount"`
	WatcherCount      int64  `json:"watcherCount"`
	ViewCount         int64  `json:"viewCount"`
	CreatedAt         string `json:"createdAt,omitempty"`
	RedirectURL       string `json:"redirectURL,omitempty"`
	Score             int64  `json:"rankScore"`
}

// extractSearchHits walks the `/search-all` response and pulls out hits of the
// requested entityType. The proxy returns two shapes depending on whether
// queryIndices specified one or many indexes:
//
//   - Single index: data is a flat array [{score, document}, …]
//   - Multi-index or no index: data is keyed by entityType {collection: [...], …}
//
// We probe both forms so canonical works regardless of how the request was
// shaped upstream.
func extractSearchHits(raw json.RawMessage, entityType string) []canonicalCandidate {
	type rawHit struct {
		Score    float64         `json:"score"`
		Document json.RawMessage `json:"document"`
	}
	// First try array shape
	var arrayResp struct {
		Data []rawHit `json:"data"`
	}
	hits := []rawHit{}
	if err := json.Unmarshal(raw, &arrayResp); err == nil && arrayResp.Data != nil {
		hits = arrayResp.Data
	} else {
		var objectResp struct {
			Data map[string][]rawHit `json:"data"`
		}
		if err := json.Unmarshal(raw, &objectResp); err != nil {
			return nil
		}
		hits = objectResp.Data[entityType]
	}
	out := make([]canonicalCandidate, 0, len(hits))
	for _, hit := range hits {
		var doc struct {
			ID                  any    `json:"id"`
			Name                string `json:"name"`
			Summary             string `json:"summary"`
			PublisherID         any    `json:"publisherId"`
			PublisherHandle     string `json:"publisherHandle"`
			IsPublisherVerified bool   `json:"isPublisherVerified"`
			EntityType          string `json:"entityType"`
			ForkCount           int64  `json:"forkCount"`
			WatcherCount        int64  `json:"watcherCount"`
			ViewCount           int64  `json:"views"`
			CreatedAt           string `json:"createdAt"`
		}
		if err := json.Unmarshal(hit.Document, &doc); err != nil {
			continue
		}
		c := canonicalCandidate{
			ID:                stringify(doc.ID),
			Name:              doc.Name,
			Summary:           doc.Summary,
			PublisherID:       stringify(doc.PublisherID),
			PublisherHandle:   doc.PublisherHandle,
			PublisherVerified: doc.IsPublisherVerified,
			EntityType:        doc.EntityType,
			ForkCount:         doc.ForkCount,
			WatcherCount:      doc.WatcherCount,
			ViewCount:         doc.ViewCount,
			CreatedAt:         doc.CreatedAt,
		}
		if c.PublisherHandle != "" && c.ID != "" {
			c.RedirectURL = fmt.Sprintf("https://www.postman.com/%s/collection/%s", c.PublisherHandle, c.ID)
		}
		out = append(out, c)
	}
	return out
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%d", int64(t))
	}
	return ""
}

// rankCanonical scores each candidate using:
//
//	rank = (verified ? 1_000_000 : 0) + forkCount * 10 + watcherCount
//
// then sorts descending. With name-substring boost on top so an exact-name
// publisher (e.g. "Stripe Developers") wins over a fork named "Awesome Stripe
// Snippets" with more forks.
func rankCanonical(candidates []canonicalCandidate, query string) {
	q := strings.ToLower(query)
	for i := range candidates {
		c := &candidates[i]
		score := c.ForkCount*10 + c.WatcherCount
		if c.PublisherVerified {
			score += 1_000_000
		}
		if c.PublisherHandle != "" && strings.Contains(strings.ToLower(c.PublisherHandle), q) {
			score += 500_000
		}
		if c.Name != "" && strings.HasPrefix(strings.ToLower(c.Name), q) {
			score += 100_000
		}
		c.Score = score
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
}
