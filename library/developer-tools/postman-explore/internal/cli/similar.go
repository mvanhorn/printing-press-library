package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newSimilarCmd returns the `similar <id>` command — given a seed entity's
// numeric id, return collections that look similar based on FTS5 MATCH
// against the local store.
func newSimilarCmd(flags *rootFlags) *cobra.Command {
	var entityType string
	var limit int
	cmd := &cobra.Command{
		Use:         "similar <id>",
		Short:       "Find entities similar to a seed (FTS5 more-like-this)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Given a seed entity's numeric id (the 'id' from listNetworkEntities or
get), surface other entities of the same type whose name/summary overlap
with the seed's text. Powered by the local store's FTS5 index.

Run 'sync' first; this command reads the local store.`,
		Example: strings.Trim(`
  # Find collections similar to id 10289 (Salesforce Platform APIs)
  postman-explore-pp-cli similar 10289

  # JSON output, narrow to APIs
  postman-explore-pp-cli similar 10289 --type collection --limit 5 --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			seedID := strings.TrimSpace(args[0])
			if seedID == "" {
				return usageErr(fmt.Errorf("seed id is required"))
			}
			if !validEntityType(entityType) {
				return usageErr(fmt.Errorf("invalid --type %q (use one of collection, workspace, api, flow)", entityType))
			}

			db, err := openLocalStore(flags)
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			// Load the seed entity from the resources table (sync routes typed
			// entities through the generic upsert path).
			row := db.DB().QueryRow(`SELECT data FROM resources WHERE id = ? AND resource_type = ?`, seedID, entityType)
			var seedDataText string
			if err := row.Scan(&seedDataText); err != nil {
				return notFoundErr(fmt.Errorf("seed entity %q (type %s) not in local store — sync it first or pass --type that matches", seedID, entityType))
			}
			seedData := json.RawMessage(seedDataText)
			var seedDoc struct {
				Name    string `json:"name"`
				Summary string `json:"summary"`
			}
			_ = json.Unmarshal(seedData, &seedDoc)
			seedTokens := tokensForFTS(seedDoc.Name, seedDoc.Summary)
			if seedTokens == "" {
				return fmt.Errorf("seed entity %q has no usable name/summary text for similarity matching", seedID)
			}

			ftsQuery := `SELECT id FROM resources_fts WHERE resources_fts MATCH ? AND resource_type = ? LIMIT ?`
			rows, err := db.DB().Query(ftsQuery, seedTokens, entityType, limit*4)
			if err != nil {
				// FTS5 syntax sensitive to special chars — fall back to simple LIKE
				return fmt.Errorf("FTS query failed (try simpler seed text): %w", err)
			}
			defer rows.Close()
			matchedIDs := []string{}
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					continue
				}
				if id == seedID {
					continue
				}
				matchedIDs = append(matchedIDs, id)
				if len(matchedIDs) >= limit*2 {
					break
				}
			}
			if len(matchedIDs) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd, []any{}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), emptyMessage("no similar entities found in the local store"))
				return nil
			}

			// Hydrate matched ids
			type entry struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Summary     string `json:"summary,omitempty"`
				EntityType  string `json:"entityType"`
				ForkCount   int64  `json:"forkCount"`
				RedirectURL string `json:"redirectURL,omitempty"`
			}
			out := make([]entry, 0, len(matchedIDs))
			for _, id := range matchedIDs {
				var rawText string
				if err := db.DB().QueryRow(`SELECT data FROM resources WHERE id = ? AND resource_type = ?`, id, entityType).Scan(&rawText); err != nil {
					continue
				}
				raw := json.RawMessage(rawText)
				var doc struct {
					ID          any    `json:"id"`
					Name        string `json:"name"`
					Summary     string `json:"summary"`
					EntityType  string `json:"entityType"`
					RedirectURL string `json:"redirectURL"`
				}
				if err := json.Unmarshal(raw, &doc); err != nil {
					continue
				}
				out = append(out, entry{
					ID:          stringify(doc.ID),
					Name:        doc.Name,
					Summary:     truncate(doc.Summary, 80),
					EntityType:  doc.EntityType,
					ForkCount:   extractMetricValue(raw, "forkCount"),
					RedirectURL: doc.RedirectURL,
				})
				if len(out) >= limit {
					break
				}
			}

			if flags.asJSON {
				return printJSONFiltered(cmd, out, flags)
			}
			tbl := make([][]string, 0, len(out))
			for _, e := range out {
				tbl = append(tbl, []string{e.Name, fmt.Sprintf("%d", e.ForkCount), e.RedirectURL})
			}
			return flags.printTable(cmd, []string{"Name", "Forks", "URL"}, tbl)
		},
	}
	cmd.Flags().StringVar(&entityType, "type", "collection", "Entity type to compare within")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum similar entities to return")
	return cmd
}

// tokensForFTS extracts a small set of FTS-safe tokens from a seed entity's
// name and summary. Drops short or punctuation-heavy words and joins with OR
// so FTS5 ranks any-overlap rather than requiring all-overlap.
func tokensForFTS(name, summary string) string {
	combined := name + " " + summary
	words := strings.Fields(combined)
	tokens := make([]string, 0, len(words))
	seen := map[string]bool{}
	for _, w := range words {
		w = strings.Trim(w, `.,;:!?()[]{}'"-_/\`)
		if len(w) < 4 {
			continue
		}
		lower := strings.ToLower(w)
		if seen[lower] {
			continue
		}
		// Drop FTS-special characters; keep ASCII letters and digits only
		safe := filterFTSToken(w)
		if safe == "" {
			continue
		}
		tokens = append(tokens, safe)
		seen[lower] = true
		if len(tokens) >= 8 {
			break
		}
	}
	if len(tokens) == 0 {
		return ""
	}
	return strings.Join(tokens, " OR ")
}

func filterFTSToken(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		}
	}
	return string(out)
}
