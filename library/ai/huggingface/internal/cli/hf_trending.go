package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface/internal/hfx"
)

type trendingResponse struct {
	hfx.Envelope
	Filter  trendingFilter   `json:"filter"`
	Results []trendingResult `json:"results"`
	Explain string           `json:"explain,omitempty"`
}

type trendingFilter struct {
	Size      string `json:"size,omitempty"`
	Library   string `json:"library,omitempty"`
	Task      string `json:"task,omitempty"`
	SinceDays int    `json:"since_days,omitempty"`
	Sort      string `json:"sort"`
}

type trendingResult struct {
	ID           string   `json:"id"`
	Author       string   `json:"author"`
	Downloads    int      `json:"downloads"`
	Likes        int      `json:"likes"`
	LastModified string   `json:"last_modified"`
	Tags         []string `json:"tags,omitempty"`
}

func newHFTrendingCmd(flags *rootFlags) *cobra.Command {
	var sizeFilter, libraryFilter, taskFilter, sinceFilter, sortKey string
	cmd := &cobra.Command{
		Use:   "trending",
		Short: "What's trending on Hugging Face (size-class + library + window filters HF doesn't expose).",
		Long: `trending lists models by recent activity. HF only exposes downloads/likes/lastModified
sort keys server-side; size-class and time-window filters are client-side.

Default sort: lastModified (recently active). Pass --sort downloads|likes for
all-time popularity views.`,
		Example: `  huggingface-pp-cli trending
  huggingface-pp-cli trending --size 7b-13b --library gguf
  huggingface-pp-cli trending --since 7d --sort downloads --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			// Validate sort
			s := strings.ToLower(strings.TrimSpace(sortKey))
			switch s {
			case "", "lastmodified", "lastModified":
				s = "lastModified"
			case "downloads", "likes", "createdat", "createdAt":
				// HF accepts these; normalize
				if s == "createdat" {
					s = "createdAt"
				}
			default:
				return hfNotFound("invalid --sort %q (use: lastModified, downloads, likes, createdAt)", sortKey)
			}

			q := url.Values{}
			q.Set("sort", s)
			q.Set("direction", "-1")
			pull := flags.limit * 3
			if pull < 50 {
				pull = 50
			}
			q.Set("limit", strconv.Itoa(pull))
			q.Set("full", "true")
			if libraryFilter != "" {
				q.Set("filter", libraryFilter)
			}
			if taskFilter != "" {
				// HF accepts pipeline_tag via filter as well
				if libraryFilter != "" {
					q.Add("filter", taskFilter)
				} else {
					q.Set("filter", taskFilter)
				}
			}

			ms, status, err := hfListModels(ctx, q, hfTokenForRequests())
			if err != nil {
				if status == 429 {
					return hfRateLimited("rate limited (HTTP 429)")
				}
				return err
			}

			// Client-side filters
			var sinceCutoff time.Time
			sinceDays := 0
			if sinceFilter != "" {
				sinceDays = parseSinceDays(sinceFilter)
				if sinceDays > 0 {
					sinceCutoff = time.Now().UTC().Add(-time.Duration(sinceDays) * 24 * time.Hour)
				}
			}

			results := []trendingResult{}
			for _, m := range ms {
				if sizeFilter != "" && !matchesSizeFilter(m.Tags, sizeFilter) {
					continue
				}
				if !sinceCutoff.IsZero() {
					if t, err := time.Parse(time.RFC3339Nano, m.LastModified); err == nil {
						if t.Before(sinceCutoff) {
							continue
						}
					} else if t, err := time.Parse(time.RFC3339, m.LastModified); err == nil {
						if t.Before(sinceCutoff) {
							continue
						}
					}
				}
				results = append(results, trendingResult{
					ID:           m.ID,
					Author:       m.Author,
					Downloads:    m.Downloads,
					Likes:        m.Likes,
					LastModified: m.LastModified,
					Tags:         hfMaxStrings(m.Tags, 8),
				})
				if len(results) >= flags.limit {
					break
				}
			}

			if len(results) == 0 {
				return hfNotFound("no trending models matched filters (try widening --size / --since / --library)")
			}

			resp := trendingResponse{
				Envelope: hfx.NewEnvelope("trending"),
				Filter: trendingFilter{
					Size:      sizeFilter,
					Library:   libraryFilter,
					Task:      taskFilter,
					SinceDays: sinceDays,
					Sort:      s,
				},
				Results: results,
			}
			if flags.explain {
				resp.Explain = fmt.Sprintf("explain: HF doesn't expose 'trending' as a sort param; this command pulls a larger window via sort=%s direction=-1 (limit=%d), then post-filters client-side. Default --sort=lastModified gives 'recently active'; use --sort=downloads for all-time popularity.",
					s, pull)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "trending (sort=%s, %d results)\n\n", s, len(results))
			fmt.Fprintf(cmd.OutOrStdout(), "  %-50s  %-15s  %-10s  %-8s  %s\n", "ID", "AUTHOR", "DOWNLOADS", "LIKES", "MODIFIED")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-50s  %-15s  %-10d  %-8d  %s\n", r.ID, r.Author, r.Downloads, r.Likes, r.LastModified)
			}
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sizeFilter, "size", "", "Size class (e.g. 7b-13b, 20b-40b) — best-effort tag match")
	cmd.Flags().StringVar(&libraryFilter, "library", "", "Library tag (gguf, transformers, mlx, etc.)")
	cmd.Flags().StringVar(&taskFilter, "task", "", "Pipeline tag (text-generation, image-classification, etc.)")
	cmd.Flags().StringVar(&sinceFilter, "since", "", "Only include models modified within this window (e.g. 7d, 30d)")
	cmd.Flags().StringVar(&sortKey, "sort", "lastModified", "Sort key: lastModified (default), downloads, likes, createdAt")
	return cmd
}

// parseSinceDays parses "7d" / "30d" / "12h" → integer days. Returns 0 on parse fail.
func parseSinceDays(s string) int {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0
	}
	var unit byte = 'd'
	if l := len(s); l > 0 && (s[l-1] == 'd' || s[l-1] == 'h' || s[l-1] == 'w') {
		unit = s[l-1]
		s = s[:l-1]
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	switch unit {
	case 'h':
		if n < 24 {
			return 1 // round up to a day
		}
		return n / 24
	case 'w':
		return n * 7
	default:
		return n
	}
}
