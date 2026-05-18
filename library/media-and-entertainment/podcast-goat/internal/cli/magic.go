// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 `magic` — bundle top-N FTS5 cached episodes into one MD file.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newMagicCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLimit int
		flagOut   string
	)
	cmd := &cobra.Command{
		Use:         "magic [topic]",
		Short:       "Bundle top-N cached transcripts about a topic into one markdown prompt file",
		Example:     `  podcast-goat-pp-cli magic "training compute" --limit 5`,
		Annotations: map[string]string{"pp:endpoint": "magic.bundle", "pp:method": "GET", "mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			topic := strings.Join(args, " ")
			ps, err := openPodcastStore(cmd.Context())
			if err != nil {
				return err
			}
			rows, err := ps.SearchEpisodes(cmd.Context(), topic, flagLimit)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no cached episodes match %q. Try `episode get <url>` to seed the cache first.\n", topic)
				return nil
			}

			var b strings.Builder
			b.WriteString("---\n")
			b.WriteString(fmt.Sprintf("topic: %q\n", topic))
			b.WriteString(fmt.Sprintf("generated_at: %s\n", time.Now().UTC().Format(time.RFC3339)))
			b.WriteString(fmt.Sprintf("episode_count: %d\n", len(rows)))
			b.WriteString("---\n\n")
			b.WriteString(fmt.Sprintf("# %s — bundled transcripts (%d episodes)\n\n", topic, len(rows)))

			for _, r := range rows {
				full, _ := ps.GetTranscript(cmd.Context(), r.URL)
				b.WriteString("---\n")
				b.WriteString(fmt.Sprintf("source: %s\n", r.Source))
				b.WriteString(fmt.Sprintf("show: %s\n", r.Show))
				b.WriteString(fmt.Sprintf("host: %s\n", r.Host))
				if len(r.Guests) > 0 {
					b.WriteString(fmt.Sprintf("guests: [%s]\n", strings.Join(r.Guests, ", ")))
				}
				if r.PublishedAt != "" {
					b.WriteString(fmt.Sprintf("date: %s\n", r.PublishedAt))
				}
				b.WriteString(fmt.Sprintf("provider: %s\n", r.Provider))
				b.WriteString(fmt.Sprintf("cost_credits: %.2f\n", r.CostCredits))
				b.WriteString(fmt.Sprintf("cookie_hit: %v\n", r.Tier == "cookie"))
				b.WriteString(fmt.Sprintf("url: %s\n", r.URL))
				b.WriteString("---\n\n")
				if full != nil && full.ContentMD != "" {
					b.WriteString(full.ContentMD)
				} else {
					b.WriteString(fmt.Sprintf("(no transcript body cached for %s)\n", r.URL))
				}
				b.WriteString("\n\n")
			}

			outPath := flagOut
			if outPath == "" {
				slug := topicSlug(topic)
				_ = os.MkdirAll(podcastMagicDir(), 0o755)
				outPath = filepath.Join(podcastMagicDir(), fmt.Sprintf("%s-%d.md", slug, time.Now().Unix()))
			}
			if err := writeFile(outPath, b.String()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), outPath)
			return nil
		},
	}
	cmd.Flags().IntVarP(&flagLimit, "limit", "n", 5, "Number of episodes to include")
	cmd.Flags().StringVar(&flagOut, "out", "", "Output file (default: ~/.config/podcast-goat/magic/<slug>-<unix>.md)")
	return cmd
}

var slugReplaceRE = regexp.MustCompile(`[^a-z0-9]+`)

func topicSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugReplaceRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "topic"
	}
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}
