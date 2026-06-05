// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.
// Hand-written transcendence command. Not generated.

package cli

import (
	"bufio"
	"context"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/store"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const aiVisibilitySchema = `CREATE TABLE IF NOT EXISTS ai_visibility (
  brand TEXT,
  keyword TEXT,
  llm TEXT,
  mentioned INTEGER,
  excerpt TEXT,
  checked_at TIMESTAMP,
  PRIMARY KEY (brand, keyword, llm, checked_at)
)`

type aiVisibilityRow struct {
	Brand     string `json:"brand"`
	Keyword   string `json:"keyword"`
	LLM       string `json:"llm"`
	Mentioned bool   `json:"mentioned"`
	Excerpt   string `json:"excerpt,omitempty"`
	PrevMentioned *bool `json:"prev_mentioned,omitempty"`
	CheckedAt string `json:"checked_at"`
}

var aiVisibilityLLMs = []struct {
	name string
	path string
}{
	{"chat_gpt", "/v3/ai_optimization/chat_gpt/llm_responses/live"},
	{"claude", "/v3/ai_optimization/claude/llm_responses/live"},
	{"gemini", "/v3/ai_optimization/gemini/llm_responses/live"},
	{"perplexity", "/v3/ai_optimization/perplexity/llm_responses/live"},
}

func newAIVisibilityCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai-visibility",
		Short: "Track brand mentions in AI search answers (ChatGPT/Claude/Gemini/Perplexity)",
	}
	cmd.AddCommand(newAIVisibilityTrackCmd(flags))
	return cmd
}

func newAIVisibilityTrackCmd(flags *rootFlags) *cobra.Command {
	var brand string
	var keywordsFile string

	cmd := &cobra.Command{
		Use:   "track",
		Short: "Query each LLM for each keyword, record whether the brand appears, diff vs last run",
		Example: strings.Trim(`
  dataforseo-pp-cli ai-visibility track --brand "Florida Tree Men" --keywords /tmp/kw.txt
  dataforseo-pp-cli ai-visibility track --brand "FSM Stump Grinding" --keywords kw.txt --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would query LLMs and record brand mentions")
				return nil
			}
			if brand == "" {
				return usageErr(fmt.Errorf("--brand is required"))
			}
			if keywordsFile == "" {
				return usageErr(fmt.Errorf("--keywords <file> is required"))
			}

			keywords, err := readKeywordLines(keywordsFile)
			if err != nil {
				return err
			}
			if len(keywords) == 0 {
				return usageErr(fmt.Errorf("%s: no keywords found", keywordsFile))
			}

			ctx := context.Background()
			s, err := store.OpenWithContext(ctx, defaultDBPath("dataforseo-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if _, err := s.DB().Exec(aiVisibilitySchema); err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			out := make([]aiVisibilityRow, 0, len(keywords)*len(aiVisibilityLLMs))
			brandLower := strings.ToLower(brand)
			now := time.Now().UTC().Format(time.RFC3339)
			for _, kw := range keywords {
				query := fmt.Sprintf("best %s company", kw)
				for _, llm := range aiVisibilityLLMs {
					body := []map[string]any{{"user_prompt": query}}
					resp, _, err := c.Post(llm.path, body)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s/%s: %v\n", llm.name, kw, err)
						continue
					}
					text := extractLLMResponseText(resp)
					mentioned := strings.Contains(strings.ToLower(text), brandLower)
					excerpt := ""
					if mentioned {
						excerpt = sliceAroundMatch(text, brandLower, 80)
					}

					prev := loadPrevMention(s, brand, kw, llm.name)
					row := aiVisibilityRow{
						Brand:     brand,
						Keyword:   kw,
						LLM:       llm.name,
						Mentioned: mentioned,
						Excerpt:   excerpt,
						CheckedAt: now,
					}
					if prev != nil {
						row.PrevMentioned = prev
					}
					out = append(out, row)

					mentionedInt := 0
					if mentioned {
						mentionedInt = 1
					}
					_, _ = s.DB().Exec(
						`INSERT OR REPLACE INTO ai_visibility (brand, keyword, llm, mentioned, excerpt, checked_at) VALUES (?, ?, ?, ?, ?, ?)`,
						brand, kw, llm.name, mentionedInt, excerpt, now,
					)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&brand, "brand", "", "Brand name to look for in LLM answers")
	cmd.Flags().StringVar(&keywordsFile, "keywords", "", "File of newline-separated keywords")
	return cmd
}

func readKeywordLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out, scanner.Err()
}

func extractLLMResponseText(raw json.RawMessage) string {
	var resp struct {
		Tasks []struct {
			Result []struct {
				Items []struct {
					Sections []struct {
						Text string `json:"text"`
					} `json:"sections"`
					Text string `json:"text"`
				} `json:"items"`
			} `json:"result"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return ""
	}
	var b strings.Builder
	for _, t := range resp.Tasks {
		for _, r := range t.Result {
			for _, item := range r.Items {
				if item.Text != "" {
					b.WriteString(item.Text)
					b.WriteString(" ")
				}
				for _, sec := range item.Sections {
					b.WriteString(sec.Text)
					b.WriteString(" ")
				}
			}
		}
	}
	return b.String()
}

func sliceAroundMatch(text, needleLower string, radius int) string {
	idx := strings.Index(strings.ToLower(text), needleLower)
	if idx < 0 {
		return ""
	}
	start := idx - radius
	if start < 0 {
		start = 0
	}
	end := idx + len(needleLower) + radius
	if end > len(text) {
		end = len(text)
	}
	return strings.TrimSpace(text[start:end])
}

func loadPrevMention(s *store.Store, brand, keyword, llm string) *bool {
	row := s.DB().QueryRow(
		`SELECT mentioned FROM ai_visibility WHERE brand = ? AND keyword = ? AND llm = ? ORDER BY checked_at DESC LIMIT 1`,
		brand, keyword, llm,
	)
	var mentioned int
	if err := row.Scan(&mentioned); err != nil {
		return nil
	}
	b := mentioned == 1
	return &b
}
