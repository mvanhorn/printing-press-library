// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `instagram-pp-cli search "<query>"` — local FTS over downloaded posts.
// Reads-only against media_files / media_files_fts; --rerank optionally
// re-orders results using whichever LLM env is configured.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

// searchHit is the row shape returned by the FTS query. Defined at package
// scope so rerank can pass it through without losing type info.
type searchHit struct {
	Shortcode      string  `json:"shortcode"`
	OwnerUsername  string  `json:"owner_username"`
	CaptionSnippet string  `json:"caption_snippet"`
	AltText        string  `json:"alt_text"`
	TakenAt        int64   `json:"taken_at"`
	LocalPath      string  `json:"local_path"`
	Score          float64 `json:"score,omitempty"`
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		owner     string
		since     string
		kind      string
		alt       bool
		limit     int
		rerank    bool
		rerankTop int
		dbPath    string
	)
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Full-text search across local media (captions, alt-text, owners)",
		Long: `Search the local SQLite catalog (no API calls). Indexes the captions,
IG-generated accessibility_caption (alt-text), owner usernames, and shortcodes
of every post you've fetched into the local store.

Use --rerank to re-order the top-N FTS results with an LLM. Set one of
ANTHROPIC_API_KEY, OPENAI_API_KEY, or OLLAMA_HOST to enable.`,
		Example: `  instagram-pp-cli search "sunset"
  instagram-pp-cli search "two people sunglasses" --alt
  instagram-pp-cli search "machine learning" --rerank --rerank-top 50`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2,4",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := args[0]
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []any{}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "(dry-run) would search for %q\n", query)
				return nil
			}

			if rerank && !rerankConfigured() {
				return authErr(fmt.Errorf("--rerank requires one of ANTHROPIC_API_KEY, OPENAI_API_KEY, or OLLAMA_HOST to be set"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			st, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			if err := store.ApplyExtraMigrations(cmd.Context(), st.DB()); err != nil {
				return err
			}

			searchCol := "caption"
			if alt {
				searchCol = "alt_text"
			}

			// Build the FTS query. We restrict on the column inside the MATCH
			// expression so the FTS index can use a single tokenized lookup.
			fts := fmt.Sprintf("%s:%s", searchCol, query)

			topN := limit
			if rerank && rerankTop > topN {
				topN = rerankTop
			}
			if topN <= 0 {
				topN = 50
			}

			// Build SQL with optional owner / since / kind filters. The MATCH
			// clause uses the bare table name; FTS5 doesn't accept aliases on
			// the LHS of MATCH.
			where := []string{"media_files_fts MATCH ?"}
			argsSQL := []any{fts}
			if owner != "" {
				where = append(where, "m.owner_username = ?")
				argsSQL = append(argsSQL, owner)
			}
			if since != "" {
				ts, perr := parseSinceLike(since)
				if perr == nil {
					where = append(where, "m.taken_at >= ?")
					argsSQL = append(argsSQL, ts.Unix())
				}
			}
			if kind != "" {
				switch kind {
				case "reel", "reels", "video":
					where = append(where, "m.kind = 'video'")
				case "image", "post":
					where = append(where, "m.kind = 'image'")
				}
			}
			argsSQL = append(argsSQL, topN)
			sqlText := fmt.Sprintf(`SELECT m.shortcode, COALESCE(m.owner_username, ''),
				COALESCE(SUBSTR(m.caption,1,200), ''), COALESCE(m.alt_text, ''),
				COALESCE(m.taken_at, 0), COALESCE(m.local_path, '')
			FROM media_files_fts f
			JOIN media_files m ON m.id = f.rowid
			WHERE %s
			ORDER BY rank
			LIMIT ?`, strings.Join(where, " AND "))

			rows, err := st.DB().Query(sqlText, argsSQL...)
			if err != nil {
				// FTS5 may reject odd queries — surface a usage hint.
				return usageErr(fmt.Errorf("search query rejected: %w (try simpler terms; FTS5 syntax docs: https://sqlite.org/fts5.html)", err))
			}
			defer rows.Close()
			var hits []searchHit
			for rows.Next() {
				var h searchHit
				if err := rows.Scan(&h.Shortcode, &h.OwnerUsername, &h.CaptionSnippet, &h.AltText, &h.TakenAt, &h.LocalPath); err != nil {
					continue
				}
				hits = append(hits, h)
			}
			if rerank && len(hits) > 0 {
				boxed := make([]any, len(hits))
				for i, h := range hits {
					boxed[i] = h
				}
				ordered, err := rerankHits(cmd.Context(), query, boxed)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: rerank failed: %v\n", err)
				} else {
					unboxed := make([]searchHit, 0, len(ordered))
					for _, x := range ordered {
						if h, ok := x.(searchHit); ok {
							unboxed = append(unboxed, h)
						}
					}
					if len(unboxed) > 0 {
						hits = unboxed
					}
				}
			}
			// Trim to --limit after rerank.
			if limit > 0 && len(hits) > limit {
				hits = hits[:limit]
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
			}
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", h.Shortcode, h.OwnerUsername, h.CaptionSnippet)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&owner, "owner", "", "Restrict to posts by this owner username")
	cmd.Flags().StringVar(&since, "since", "", "Only posts taken since this duration (7d, 24h) or RFC3339 timestamp")
	cmd.Flags().StringVar(&kind, "kind", "", "Restrict to kind: post (image) | reel (video)")
	cmd.Flags().BoolVar(&alt, "alt", false, "Search the alt-text column instead of captions")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results")
	cmd.Flags().BoolVar(&rerank, "rerank", false, "Re-order top results with an LLM (requires API key env)")
	cmd.Flags().IntVar(&rerankTop, "rerank-top", 50, "How many FTS candidates to send to the reranker")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// parseSinceLike accepts either parseSinceDuration's relative form (7d) or a
// raw RFC3339 timestamp. Centralizing here keeps the search command generous
// about input shapes without growing the duration parser.
func parseSinceLike(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return parseSinceDuration(s)
}

func rerankConfigured() bool {
	return os.Getenv("ANTHROPIC_API_KEY") != "" ||
		os.Getenv("OPENAI_API_KEY") != "" ||
		os.Getenv("OLLAMA_HOST") != ""
}

// rerankHits is shared across LLM providers. It serializes the candidates,
// asks for a JSON-array of indices ordered MOST relevant first, then
// re-arranges hits accordingly. Index out of range or non-int entries are
// dropped.
func rerankHits(ctx context.Context, query string, hits []any) ([]any, error) {
	// Generic over hit shape via interface{} so the search command can pass
	// its own typed slice without the package needing to know the schema.
	// Convert to a JSON byte slice for the prompt.
	candidates, err := json.Marshal(hits)
	if err != nil {
		return hits, err
	}
	prompt := fmt.Sprintf(
		"User query: %q. Below are %d Instagram posts as JSON. Return a JSON array of the input array indices (0-based), ordered by relevance to the query, MOST relevant first. Output ONLY the JSON array, no prose.\n\n%s",
		query, len(hits), string(candidates),
	)

	respText, err := callLLM(ctx, prompt)
	if err != nil {
		return hits, err
	}

	// Pull a JSON array out of whatever the LLM returned (some models wrap
	// in code fences despite "ONLY the JSON array").
	jsonText := strings.TrimSpace(respText)
	if i := strings.Index(jsonText, "["); i >= 0 {
		if j := strings.LastIndex(jsonText, "]"); j > i {
			jsonText = jsonText[i : j+1]
		}
	}
	var indices []int
	if err := json.Unmarshal([]byte(jsonText), &indices); err != nil {
		return hits, fmt.Errorf("rerank response not a JSON array: %w", err)
	}
	seen := map[int]bool{}
	out := make([]any, 0, len(hits))
	for _, idx := range indices {
		if idx < 0 || idx >= len(hits) || seen[idx] {
			continue
		}
		seen[idx] = true
		out = append(out, hits[idx])
	}
	// Append any candidates the LLM forgot — never silently lose data.
	for i := range hits {
		if !seen[i] {
			out = append(out, hits[i])
		}
	}
	return out, nil
}

// callLLM picks a provider in priority order: Anthropic, OpenAI, Ollama.
// Returns the assistant's text response. Each branch builds its own request
// body to keep wire formats explicit.
func callLLM(ctx context.Context, prompt string) (string, error) {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return callAnthropic(ctx, key, prompt)
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return callOpenAI(ctx, key, prompt)
	}
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	return callOllama(ctx, host, prompt)
}

func callAnthropic(ctx context.Context, apiKey, prompt string) (string, error) {
	body := map[string]any{
		"model":      "claude-haiku-4-5-20251001",
		"max_tokens": 1024,
		"messages":   []map[string]any{{"role": "user", "content": prompt}},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(b))
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Anthropic HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	for _, c := range parsed.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("Anthropic response had no text content")
}

func callOpenAI(ctx context.Context, apiKey, prompt string) (string, error) {
	body := map[string]any{
		"model":    "gpt-4o-mini",
		"messages": []map[string]any{{"role": "user", "content": prompt}},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("OpenAI HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) > 0 {
		return parsed.Choices[0].Message.Content, nil
	}
	return "", fmt.Errorf("OpenAI response had no choices")
}

func callOllama(ctx context.Context, host, prompt string) (string, error) {
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "llama3.1"
	}
	body := map[string]any{
		"model":    model,
		"messages": []map[string]any{{"role": "user", "content": prompt}},
		"stream":   false,
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, host+"/api/chat", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Ollama HTTP %d", resp.StatusCode)
	}
	var parsed struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	return parsed.Message.Content, nil
}
