// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(digg-rename-and-github-feeds): library-side new file. The four
// /ai/github/* feeds were sniffed and parsed post-publish; the generator
// has no spec for them yet. Wired into root.go's AddCommand block.
//
// `digg-pp-cli github stars|new|activity|recent` commands.
// Fetches the four /ai/github/* feeds, parses the embedded RSC stream
// via diggparse, and emits structured records.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
	"github.com/spf13/cobra"
)

func newGithubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "github",
		Short: "GitHub feeds Digg surfaces alongside the X-account leaderboard (stars / new / activity / recent)",
		Long: `GitHub feeds Digg surfaces alongside the X-account leaderboard.

Four flavors, each parsed from the page's embedded RSC stream:

  stars      Top AI repos by starring activity from Digg-tracked accounts.
             Returns repo_full_name, language, stargazers_count, recent
             starrer list, breakout/novel/ai_related scores, and the model's
             one-sentence classification.

  new        Recently first-seen repos grouped by the creator/starrer who
             first put them on Digg's radar. Each entry carries
             event_created_at + repos[].

  activity   Top GitHub contributor leaderboard: per-author rank,
             contribution count, and distinct repos count over Digg's
             tracking window.

  recent     Live activity feed: per-event entries with the GitHub URL
             (issue/PR/commit/repo), the user who acted, and a short
             description of the target.`,
	}
	cmd.AddCommand(newGithubStarsCmd(flags))
	cmd.AddCommand(newGithubNewCmd(flags))
	cmd.AddCommand(newGithubActivityCmd(flags))
	cmd.AddCommand(newGithubRecentCmd(flags))
	return cmd
}

// fetchGithubFeed pulls https://di.gg/ai/github/<kind> and returns the
// raw HTML bytes. Uses fetchURL (same as digg_sync) so we get text/html
// without going through the JSON-sanitizing API client.
func fetchGithubFeed(cmd *cobra.Command, _ *rootFlags, kind string) ([]byte, error) {
	url := "https://di.gg/ai/github/" + kind
	return fetchURL(cmd.Context(), url)
}

// emitGithub serializes a slice of records using the standard output
// pipeline: JSON / table / csv / plain / quiet, with --select and
// --compact applied to JSON output.
func emitGithub(cmd *cobra.Command, flags *rootFlags, items any, limit int) error {
	raw, err := json.Marshal(items)
	if err != nil {
		return err
	}
	// Apply --limit by slicing the JSON array. Cheap and avoids re-typing.
	if limit > 0 {
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil && len(arr) > limit {
			arr = arr[:limit]
			trimmed, merr := json.Marshal(arr)
			if merr != nil {
				return fmt.Errorf("trimming output to --limit %d: %w", limit, merr)
			}
			raw = trimmed
		}
	}
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		filtered := json.RawMessage(raw)
		if flags.selectFields != "" {
			filtered = filterFields(filtered, flags.selectFields)
		} else if flags.compact {
			filtered = compactFields(filtered)
		}
		return printOutput(cmd.OutOrStdout(), filtered, true)
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		var rows []map[string]any
		if json.Unmarshal(raw, &rows) == nil && len(rows) > 0 {
			if err := printAutoTable(cmd.OutOrStdout(), rows); err != nil {
				return err
			}
			if len(rows) >= 25 {
				fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(rows))
			}
			return nil
		}
	}
	return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(raw), flags)
}

func newGithubStarsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "stars",
		Short:       "Top AI repos ranked by starring activity from Digg-tracked accounts",
		Example:     "  digg-pp-cli github stars --limit 10 --json",
		Annotations: map[string]string{"pp:endpoint": "github.stars", "pp:method": "GET", "pp:path": "/ai/github/stars", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			html, err := fetchGithubFeed(cmd, flags, "stars")
			if err != nil {
				return err
			}
			repos, err := diggparse.ParseGithubStars(html)
			if err != nil {
				return fmt.Errorf("parsing /ai/github/stars: %w", err)
			}
			return emitGithub(cmd, flags, repos, limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows to return (0 = all)")
	return cmd
}

func newGithubNewCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "new",
		Short:       "Recently first-seen GitHub repos grouped by the creator/starrer who put them on Digg's radar",
		Example:     "  digg-pp-cli github new --json",
		Annotations: map[string]string{"pp:endpoint": "github.new", "pp:method": "GET", "pp:path": "/ai/github/new", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			html, err := fetchGithubFeed(cmd, flags, "new")
			if err != nil {
				return err
			}
			events, err := diggparse.ParseGithubNew(html)
			if err != nil {
				return fmt.Errorf("parsing /ai/github/new: %w", err)
			}
			return emitGithub(cmd, flags, events, limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows to return (0 = all)")
	return cmd
}

func newGithubActivityCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "activity",
		Short:       "Top GitHub contributor leaderboard: rank, contributions, distinct repos",
		Example:     "  digg-pp-cli github activity --limit 25 --json",
		Annotations: map[string]string{"pp:endpoint": "github.activity", "pp:method": "GET", "pp:path": "/ai/github/activity", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			html, err := fetchGithubFeed(cmd, flags, "activity")
			if err != nil {
				return err
			}
			rows, err := diggparse.ParseGithubActivity(html)
			if err != nil {
				return fmt.Errorf("parsing /ai/github/activity: %w", err)
			}
			return emitGithub(cmd, flags, rows, limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows to return (0 = all)")
	return cmd
}

func newGithubRecentCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "recent",
		Short:       "Live GitHub activity feed: per-event entries with the github URL and the user who acted",
		Example:     "  digg-pp-cli github recent --limit 20 --json",
		Annotations: map[string]string{"pp:endpoint": "github.recent", "pp:method": "GET", "pp:path": "/ai/github/recent", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			html, err := fetchGithubFeed(cmd, flags, "recent")
			if err != nil {
				return err
			}
			rows, err := diggparse.ParseGithubRecent(html)
			if err != nil {
				return fmt.Errorf("parsing /ai/github/recent: %w", err)
			}
			return emitGithub(cmd, flags, rows, limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows to return (0 = all)")
	return cmd
}
