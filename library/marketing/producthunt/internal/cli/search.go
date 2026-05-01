package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgraphql"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		limit           int
		dbPath          string
		enrich          bool
		enrichThreshold int
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Full-text search the local post store",
		Long: `Run an FTS5 match against every post ever synced. The index covers
slug, title, tagline, and author. Works offline — empty store returns [].

FTS5 supports quoted phrases, boolean operators (AND, OR, NOT), and column
filters via the column:value shorthand. See SQLite FTS5 docs for the full
query grammar.

Pass --enrich to opportunistically hit the GraphQL API when local results
are thin. Requires Product Hunt GraphQL auth (run 'auth setup' first).
Silently skipped when no token is configured; the local result set is always
returned even if enrichment fails.`,
		Example: `  # Simple keyword
  producthunt-pp-cli search agent

  # Phrase + column filter
  producthunt-pp-cli search '"ai agent" author:hoover'

	  # Thin local results? Top up from GraphQL (requires auth setup)
	  producthunt-pp-cli search posthog --enrich

  # Agent-friendly narrow payload
  producthunt-pp-cli search "cli tool" --agent --select 'slug,title,tagline'`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return usageErr(fmt.Errorf("search query is required"))
			}
			autoWarm(flags, dbPath)
			db, err := openStore(dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			cfg, _ := config.Load(flags.configPath)
			wantEnrich := enrich || (cfg != nil && cfg.AutoEnrich)
			var enrichMeta *SearchEnrichMeta

			posts, err := db.SearchPostsFTS(query, limit)
			if err != nil {
				return apiErr(err)
			}

			if wantEnrich && len(posts) < enrichThreshold {
				// Opportunistic: best-effort upsert; fail-soft if it errors.
				enrichMeta, _ = attemptEnrich(cmd.Context(), flags, db, cfg, query)
				// Re-query so any upserted posts flow through the same FTS
				// path as locally-cached ones.
				posts, err = db.SearchPostsFTS(query, limit)
				if err != nil {
					return apiErr(err)
				}
			}
			out := postsToJSON(posts)
			if flags.asJSON || flags.agent {
				meta := map[string]any{}
				if enrichMeta != nil {
					meta["enrichment"] = enrichMeta
				} else if len(posts) < enrichThreshold && (cfg == nil || !cfg.HasGraphQLToken()) {
					meta["auth_hint"] = graphQLAuthHint("Local Atom history is thin for this search. GraphQL auth can enrich recent Product Hunt posts beyond snapshots you have accumulated.")
				}
				if len(meta) > 0 {
					out = withMeta(out, meta)
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Max results to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite store")
	cmd.Flags().BoolVar(&enrich, "enrich", false, "Opportunistically fetch GraphQL results when local store has <threshold results (requires Product Hunt GraphQL auth)")
	cmd.Flags().IntVar(&enrichThreshold, "enrich-threshold", 3, "Trigger --enrich only when local results drop below this count")
	return cmd
}

// attemptEnrich issues one narrow GraphQL posts query for the last 30 days
// and upserts the matching results into the store. Fail-soft: returns nil
// when no GraphQL token, budget is too low, or GraphQL errors — the caller always
// gets a useful result set from the local store either way.
//
// The enrichment is deliberately narrow:
//   - single page, ≤ BackfillPageSize posts
//   - 30-day window
//   - client-side topic filter (PH's posts field doesn't support fulltext)
type SearchEnrichMeta struct {
	Attempted       bool                 `json:"attempted"`
	SkippedReason   string               `json:"skipped_reason,omitempty"`
	Upserted        int                  `json:"upserted,omitempty"`
	Error           string               `json:"error,omitempty"`
	AuthHint        *authImprovementHint `json:"auth_hint,omitempty"`
	BudgetKnown     bool                 `json:"budget_known,omitempty"`
	BudgetRemaining float64              `json:"budget_remaining_pct,omitempty"`
}

func attemptEnrich(ctx context.Context, flags *rootFlags, db *store.Store, cfg *config.Config, topic string) (*SearchEnrichMeta, error) {
	_ = flags
	meta := &SearchEnrichMeta{}
	if cfg == nil || !cfg.HasGraphQLToken() {
		meta.SkippedReason = "missing_graphql_token"
		hint := graphQLAuthHint("GraphQL auth unlocks search enrichment when local Atom snapshots are thin.")
		meta.AuthHint = &hint
		return meta, nil
	}

	client := phgraphql.NewClient(cfg.AccessToken, userAgent())
	// If we've already seen a budget state and we're below the hard stop,
	// skip rather than risk a 429 mid-enrichment.
	if b := client.Budget(); b.Known() && b.PercentRemaining() < float64(BackfillBudgetHardStopPct)/100.0 {
		meta.SkippedReason = "budget_below_hard_stop"
		meta.BudgetKnown = true
		meta.BudgetRemaining = b.PercentRemaining()
		return meta, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	meta.Attempted = true

	postedAfter := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)
	postedBefore := time.Now().UTC().Format(time.RFC3339)

	resp, err := client.Execute(ctx, phgraphql.EnrichPostsQuery, map[string]any{
		"first":        BackfillPageSize,
		"postedAfter":  postedAfter,
		"postedBefore": postedBefore,
	})
	if err != nil {
		meta.Error = err.Error()
		return meta, nil
	}
	if b := client.Budget(); b.Known() {
		meta.BudgetKnown = true
		meta.BudgetRemaining = b.PercentRemaining()
	}
	if resp.HasErrors() {
		meta.Error = resp.ErrorMessage()
		return meta, nil
	}

	var envelope struct {
		Posts phgraphql.PostsPage `json:"posts"`
	}
	if err := json.Unmarshal(resp.Data, &envelope); err != nil {
		meta.Error = err.Error()
		return meta, nil
	}

	tokens := topicTokens(topic)
	tx, err := db.DB().Begin()
	if err != nil {
		meta.Error = err.Error()
		return meta, nil
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()
	for _, edge := range envelope.Posts.Edges {
		if !postMatchesTokens(edge.Node, tokens) {
			continue
		}
		if err := store.UpsertPost(tx, postNodeToStore(edge.Node)); err != nil {
			meta.Error = err.Error()
			return meta, nil
		}
		meta.Upserted++
	}
	if err := tx.Commit(); err != nil {
		meta.Error = err.Error()
		return meta, nil
	}
	rollback = false
	return meta, nil
}

// topicTokens splits a topic into lower-case content tokens used for the
// client-side enrichment filter. Drops 1-character tokens to avoid
// over-matching on common letters.
func topicTokens(topic string) []string {
	raw := strings.Fields(strings.ToLower(topic))
	out := make([]string, 0, len(raw))
	for _, w := range raw {
		w = strings.Trim(w, ".,;:!?'\"()[]{}<>")
		if len(w) < 2 {
			continue
		}
		out = append(out, w)
	}
	return out
}

// postMatchesTokens reports whether a GraphQL PostNode plausibly matches the
// topic by substring-checking slug, title, tagline, and author.
func postMatchesTokens(n phgraphql.PostNode, tokens []string) bool {
	if len(tokens) == 0 {
		return true
	}
	hay := strings.ToLower(n.Slug + " " + n.Name + " " + n.Tagline + " " + n.User.Name + " " + n.User.Username)
	for _, t := range tokens {
		if strings.Contains(hay, t) {
			return true
		}
	}
	return false
}
