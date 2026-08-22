// Copyright 2026 Abdelrahman Shaaban and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/rundown/internal/store"
)

// rdUseCaseRow is one candidate answer to "are there any workflows for X".
type rdUseCaseRow struct {
	Rank        int      `json:"rank"`
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	UpvoteCount int      `json:"upvoteCount"`
	Author      string   `json:"author"`
	Tools       []string `json:"tools"`
	Industries  []string `json:"industries"`
	Age         string   `json:"age"`
	// MatchedVia records which search found the post: "semantic" (the API's
	// own q= ranking), "local" (offline full-text), or "both".
	MatchedVia string `json:"matchedVia"`
	Snippet    string `json:"snippet"`
	URL        string `json:"url"`
}

type rdUseCaseResult struct {
	Query          string         `json:"query"`
	Matches        []rdUseCaseRow `json:"matches"`
	SemanticHits   int            `json:"semanticHits"`
	LocalHits      int            `json:"localHits"`
	MirrorSearched bool           `json:"mirrorSearched"`
	Note           string         `json:"note,omitempty"`
}

func newNovelUseCasesCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLimit  int
		flagDBPath string
		flagLocal  bool
	)

	cmd := &cobra.Command{
		Use:   "use-cases <topic>",
		Short: "Find community workflows for a topic, blending semantic and offline search",
		Long: strings.Trim(`
Answer "are there any workflows for X" against the community corpus.

Two searches run and are merged:

  semantic  the API's own q= ranking, which finds conceptually related
            workflows even when they never use your exact words
  local     full-text search over the synced mirror, which is offline and
            covers every post the API's ranker skips. A multi-word topic
            prefers posts carrying every word, then falls back to posts
            missing one of them, rarest word first

Results are de-duplicated, ranked by upvotes, and tagged with which search
found them. Use --local to stay fully offline.

Use this command to check whether a problem has already been solved before
designing a workflow. To read one of the results end to end, use
'rundown-pp-cli show <id>'.
`, "\n"),
		Example: strings.Trim(`
  rundown-pp-cli use-cases "cold email outreach"
  rundown-pp-cli use-cases "invoice reconciliation" --limit 5
  rundown-pp-cli use-cases "meeting notes" --agent --select title,upvoteCount,url
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "topic=email;--limit=5",
			// A topic that matches nothing is a valid empty search result, not
			// bad input — there is no such thing as a malformed free-text
			// query here, so there is no error path to probe.
			"pp:no-error-path-probe": "true",
			"pp:data-source":         "auto",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "use-cases")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a topic is required, e.g. rundown-pp-cli use-cases \"cold email\""))
			}
			if flagLimit <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}
			topic := strings.TrimSpace(strings.Join(args, " "))
			if topic == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("the topic must not be blank"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// merged is keyed by post id so the two searches can vote.
			merged := map[string]*rdUseCaseRow{}
			byID := map[string]rdPost{}
			semanticHits, localHits := 0, 0

			record := func(p rdPost, via string) {
				if p.ID == "" {
					return
				}
				byID[p.ID] = p
				if existing, ok := merged[p.ID]; ok {
					if existing.MatchedVia != via {
						existing.MatchedVia = "both"
					}
					return
				}
				merged[p.ID] = &rdUseCaseRow{MatchedVia: via}
			}

			// --- semantic pass (live API) ---
			var semanticErr error
			if !flagLocal {
				posts, err := rdSemanticSearch(ctx, flags, topic)
				if err != nil {
					semanticErr = err
				} else {
					semanticHits = len(posts)
					for _, p := range posts {
						record(p, "semantic")
					}
				}
			}

			// --- local pass (synced mirror) ---
			mirrorSearched := false
			dbPath := rdResolveDBPath(flagDBPath)
			if _, statErr := os.Stat(dbPath); statErr == nil {
				db, err := rdOpenMirrorStore(ctx, dbPath)
				if err == nil {
					defer db.Close()
					mirrorSearched = true
					if local, err := rdLocalSearch(db, topic, 50); err == nil {
						localHits = len(local)
						for _, p := range local {
							record(p, "local")
						}
					}
				}
			}

			if len(merged) == 0 && semanticErr != nil && !mirrorSearched {
				return semanticErr
			}

			ordered := make([]rdPost, 0, len(merged))
			for id := range merged {
				ordered = append(ordered, byID[id])
			}
			rdSortByUpvotes(ordered)
			if len(ordered) > flagLimit {
				ordered = ordered[:flagLimit]
			}

			rows := make([]rdUseCaseRow, 0, len(ordered))
			for i, p := range ordered {
				industries := make([]string, 0, len(p.Industries))
				for _, ind := range p.Industries {
					industries = append(industries, ind.Slug)
				}
				rows = append(rows, rdUseCaseRow{
					Rank:        i + 1,
					ID:          p.ID,
					Title:       p.Title,
					UpvoteCount: p.UpvoteCount,
					Author:      p.authorName(),
					Tools:       p.toolSlugs(),
					Industries:  industries,
					Age:         rdAgo(p.CreatedAt),
					MatchedVia:  merged[p.ID].MatchedVia,
					Snippet:     rdSnippet(p.Body, topic),
					URL:         rdPostURL(p.ID),
				})
			}

			result := rdUseCaseResult{
				Query:          topic,
				Matches:        rows,
				SemanticHits:   semanticHits,
				LocalHits:      localHits,
				MirrorSearched: mirrorSearched,
			}
			switch {
			case !mirrorSearched && !flagLocal:
				result.Note = "local mirror not found; semantic search only. Run 'rundown-pp-cli sync' for offline coverage."
			case semanticErr != nil:
				result.Note = "semantic search unavailable (" + semanticErr.Error() + "); local results only."
			case len(rows) == 0:
				result.Note = "no workflows matched; try a broader topic."
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			out := cmd.OutOrStdout()
			if len(rows) == 0 {
				fmt.Fprintf(out, "No community workflows matched %q.\n", topic)
				if result.Note != "" {
					fmt.Fprintf(out, "%s\n", result.Note)
				}
				return nil
			}
			fmt.Fprintf(out, "%d workflow(s) for %q\n\n", len(rows), topic)
			for _, r := range rows {
				fmt.Fprintf(out, "%d. %s\n", r.Rank, r.Title)
				fmt.Fprintf(out, "   %d upvotes · %s · %s · matched: %s\n",
					r.UpvoteCount, r.Author, r.Age, r.MatchedVia)
				if len(r.Tools) > 0 {
					fmt.Fprintf(out, "   tools: %s\n", strings.Join(r.Tools, ", "))
				}
				if r.Snippet != "" {
					fmt.Fprintf(out, "   \"%s\"\n", r.Snippet)
				}
				fmt.Fprintf(out, "   read: rundown-pp-cli show %s\n\n", r.ID)
			}
			if result.Note != "" {
				fmt.Fprintf(out, "%s\n", result.Note)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&flagLimit, "limit", 8, "Maximum workflows to return")
	cmd.Flags().BoolVar(&flagLocal, "local", false, "Skip the live semantic search and use only the offline mirror")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// rdSemanticSearch runs the community API's own q= ranking.
func rdSemanticSearch(ctx context.Context, flags *rootFlags, topic string) ([]rdPost, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, "/posts", map[string]string{
		"q":     topic,
		"limit": strconv.Itoa(50),
		"sort":  "top",
	})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Posts []rdPost `json:"posts"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing search response: %w", err)
	}
	return envelope.Posts, nil
}

// rdUseCaseTokenRE mirrors the store's FTS tokenizer so the token count used
// for relevance below matches the terms the MATCH query actually searches for.
var rdUseCaseTokenRE = regexp.MustCompile(`[\pL\pN_]+`)

// rdLocalRelaxedCap bounds how many partial-match posts the loosened local
// search may contribute. The caller re-sorts the merged set by upvotes, so an
// unbounded partial tier would let popular but unrelated posts outrank the
// workflows that actually matched the topic.
const rdLocalRelaxedCap = 12

// rdLocalSearch runs offline full-text search over the mirror.
//
// store.Search space-joins the query terms, which FTS5 reads as an implicit
// AND, so every token has to appear in the same post. That silently breaks the
// common case for this command: the topic is a phrase ("cold email outreach")
// and the workflows that answer it carry most of those words, not all of them.
// Requiring all of them returns nothing, or the one post that happens to
// mention each word in an unrelated context. So a multi-token topic also runs
// a per-token pass and keeps the posts matching all but one term, ordered by
// how many terms each matched.
func rdLocalSearch(db *store.Store, topic string, limit int) ([]rdPost, error) {
	decode := func(raws []json.RawMessage) []rdPost {
		out := make([]rdPost, 0, len(raws))
		for _, raw := range raws {
			var p rdPost
			if err := json.Unmarshal(raw, &p); err != nil {
				continue
			}
			if p.Title == "" {
				continue
			}
			out = append(out, p)
		}
		return out
	}

	// Whole-phrase pass: every term present. These are the strongest matches.
	raws, err := db.Search(topic, limit, "posts")
	if err != nil {
		return nil, err
	}
	full := decode(raws)

	tokens := rdUseCaseTokenRE.FindAllString(topic, -1)
	if len(tokens) < 2 {
		return full, nil
	}

	hits := make(map[string]int, len(full))
	byID := make(map[string]rdPost, len(full))
	for _, p := range full {
		hits[p.ID] = len(tokens)
		byID[p.ID] = p
	}

	// Per-term pass: count how many of the topic's terms each post carries, and
	// score each match by how rare that term is across the mirror. A term the
	// corpus uses constantly ("email", in 35 of 290 posts) says far less about
	// relevance than a rare one ("outreach", in 5), so weighting by inverse
	// document frequency keeps the loosened search from drifting off topic.
	score := make(map[string]float64, len(full))
	for _, p := range full {
		score[p.ID] = float64(len(tokens))
	}
	for _, token := range tokens {
		raws, err := db.Search(token, limit, "posts")
		if err != nil {
			return nil, err
		}
		matches := decode(raws)
		if len(matches) == 0 {
			continue
		}
		weight := 1 / float64(len(matches))
		for _, p := range matches {
			if _, seen := byID[p.ID]; !seen {
				byID[p.ID] = p
				hits[p.ID] = 0
			}
			if hits[p.ID] >= len(tokens) {
				continue // already a whole-phrase match
			}
			hits[p.ID]++
			score[p.ID] += weight
		}
	}

	// Allow one missing term. Requiring every term is what produced the empty
	// and off-topic results; accepting any single term would let one common
	// word drag in most of the corpus, so the loosened tier is also capped and
	// ordered by the rarity score above.
	want := len(tokens) - 1
	relaxed := make([]rdPost, 0, len(byID))
	for id, matched := range hits {
		if matched >= want && matched < len(tokens) {
			relaxed = append(relaxed, byID[id])
		}
	}
	sort.SliceStable(relaxed, func(i, j int) bool {
		return score[relaxed[i].ID] > score[relaxed[j].ID]
	})

	// Whole-phrase matches always lead; the loosened tier only tops the list up.
	out := full
	maxRows := rdLocalRelaxedCap
	if limit < maxRows {
		maxRows = limit
	}
	for _, p := range relaxed {
		if len(out) >= maxRows {
			break
		}
		out = append(out, p)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// rdSnippet returns a short window of body text around the first query term,
// so a caller can see why a workflow matched without reading the whole post.
func rdSnippet(body, topic string) string {
	body = rdCleanBody(strings.Join(strings.Fields(body), " "))
	if body == "" {
		return ""
	}
	lower := strings.ToLower(body)
	best := -1
	for _, term := range strings.Fields(strings.ToLower(topic)) {
		if len(term) < 4 {
			continue
		}
		if idx := strings.Index(lower, term); idx >= 0 && (best < 0 || idx < best) {
			best = idx
		}
	}
	const width = 150
	if best < 0 {
		return truncate(body, width)
	}
	start := best - 60
	if start < 0 {
		start = 0
	}
	end := start + width
	if end > len(body) {
		end = len(body)
	}
	snippet := strings.TrimSpace(body[start:end])
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(body) {
		snippet += "..."
	}
	return snippet
}
