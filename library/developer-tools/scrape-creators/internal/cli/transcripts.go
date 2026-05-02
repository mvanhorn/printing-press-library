package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newTranscriptsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transcripts",
		Short: "Search transcripts you've synced from TikTok / YouTube / Instagram / Facebook / Twitter",
	}
	cmd.AddCommand(newTranscriptsSearchCmd(flags))
	return cmd
}

// transcriptHit is one search result row.
type transcriptHit struct {
	Platform string `json:"platform"`
	ID       string `json:"id"`
	Snippet  string `json:"snippet"`
	URL      string `json:"url,omitempty"`
}

// platforms with a transcript endpoint.
var transcriptTables = []string{"tiktok", "youtube", "instagram", "facebook", "twitter"}

func newTranscriptsSearchCmd(flags *rootFlags) *cobra.Command {
	var platform string
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across every synced transcript",
		Long: `Searches the local SQLite store for transcript text matching <query>. Run
the per-platform transcript commands first (e.g. tiktok list-video-3, youtube
list-video-3) so transcripts land in the local store.

The search uses LIKE on JSON-extracted transcript text. For best results
sync into the per-platform tables before running this.`,
		Example:     "  scrape-creators-pp-cli transcripts search \"giveaway\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(args[0])
			if dryRunOK(flags) {
				return nil
			}
			s, err := novelOpenStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			tables := transcriptTables
			if platform != "" && platform != "all" {
				tables = []string{platform}
			}

			var hits []transcriptHit
			like := "%" + query + "%"
			for _, t := range tables {
				rows, err := s.DB().QueryContext(cmd.Context(),
					`SELECT id, data FROM `+t+` WHERE data LIKE ? LIMIT ?`,
					like, limit)
				if err != nil {
					// Table may not exist for some platforms - skip silently.
					continue
				}
				for rows.Next() {
					var id, data string
					if err := rows.Scan(&id, &data); err != nil {
						continue
					}
					var d map[string]any
					if json.Unmarshal([]byte(data), &d) != nil {
						continue
					}
					snippet := extractTranscriptSnippet(d, query)
					if snippet == "" {
						continue
					}
					hit := transcriptHit{Platform: t, ID: id, Snippet: snippet}
					if u, ok := d["url"].(string); ok {
						hit.URL = u
					}
					hits = append(hits, hit)
					if len(hits) >= limit {
						break
					}
				}
				rows.Close()
				if len(hits) >= limit {
					break
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no transcript matches in local store — sync first?")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "PLATFORM\tID\tSNIPPET")
			for _, h := range hits {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", h.Platform, truncate(h.ID, 16), truncate(h.Snippet, 80))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "all", "Restrict to one platform (all | tiktok | youtube | instagram | facebook | twitter)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum hits to return")
	return cmd
}

// extractTranscriptSnippet looks for transcript-like text in the JSON blob
// and returns a 120-char window around the first match of `query`. Returns
// "" if no transcript text matches.
func extractTranscriptSnippet(d map[string]any, query string) string {
	candidates := []string{}
	collectStrings(d, &candidates, 4)
	low := strings.ToLower(query)
	for _, s := range candidates {
		if strings.Contains(strings.ToLower(s), low) {
			idx := strings.Index(strings.ToLower(s), low)
			start := idx - 40
			if start < 0 {
				start = 0
			}
			end := idx + len(query) + 80
			if end > len(s) {
				end = len(s)
			}
			out := s[start:end]
			if start > 0 {
				out = "…" + out
			}
			if end < len(s) {
				out = out + "…"
			}
			return strings.ReplaceAll(strings.ReplaceAll(out, "\n", " "), "  ", " ")
		}
	}
	return ""
}

// collectStrings walks a JSON-decoded value and collects every string value
// up to maxDepth levels deep. Strings under 30 chars are skipped (likely
// IDs / handles, not transcript content).
func collectStrings(v any, out *[]string, maxDepth int) {
	if maxDepth <= 0 {
		return
	}
	switch x := v.(type) {
	case string:
		if len(x) >= 30 {
			*out = append(*out, x)
		}
	case map[string]any:
		for _, vv := range x {
			collectStrings(vv, out, maxDepth-1)
		}
	case []any:
		for _, vv := range x {
			collectStrings(vv, out, maxDepth-1)
		}
	}
}
