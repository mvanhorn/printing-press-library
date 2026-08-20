// Copyright 2026 QuantumGlitch and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	moba "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/internal/mobalytics"
	"github.com/spf13/cobra"
)

// newChampionCmd is the umbrella for per-champion lookups.
func newChampionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "champion",
		Short:       "Per-champion lookups: build, counters, matchups, runes, aram, arena, combos, synergies, mained.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newChampionBuildCmd(flags))
	cmd.AddCommand(newChampionCountersCmd(flags))
	cmd.AddCommand(newChampionMatchupsCmd(flags))
	cmd.AddCommand(newChampionRunesCmd(flags))
	cmd.AddCommand(newChampionAramCmd(flags))
	cmd.AddCommand(newChampionSynergiesCmd(flags))
	cmd.AddCommand(newChampionArenaCmd(flags))
	cmd.AddCommand(newChampionCombosCmd(flags))
	cmd.AddCommand(newChampionMainedCmd(flags))
	return cmd
}

// requireSlug pulls the slug from args[0] or returns a friendly error.
func requireSlug(args []string) (string, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return "", fmt.Errorf("champion slug required (e.g. 'jinx', 'leesin')")
	}
	return strings.ToLower(strings.TrimSpace(args[0])), nil
}

// pickBuildType selects a build from a list by type/queue. type="" returns
// the first MOST_POPULAR ranked-solo build; otherwise the first match.
func pickBuildType(builds []moba.ChampionBuild, want string) (moba.ChampionBuild, bool) {
	want = strings.ToUpper(want)
	for _, b := range builds {
		if want != "" {
			if strings.EqualFold(b.Type, want) {
				return b, true
			}
			continue
		}
		if b.Type == "MOST_POPULAR" && b.Queue == "RANKED_SOLO" {
			return b, true
		}
	}
	if len(builds) > 0 {
		return builds[0], true
	}
	return moba.ChampionBuild{}, false
}

func newChampionBuildCmd(flags *rootFlags) *cobra.Command {
	var buildType string
	var mainedOnly bool
	cmd := &cobra.Command{
		Use:         "build <slug>",
		Short:       "Recommended build (runes, items, skill order, summoner spells) for a champion.",
		Example:     `  mobalytics-lol-pp-cli champion build jinx --build-type MOST_POPULAR`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := requireSlug(args)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			html, err := client.Fetch(moba.ChampionPath(slug, "build"))
			if err != nil {
				return err
			}
			page := moba.ChampionBuildPage{
				Slug:      slug,
				Stats:     moba.ParseChampionStats(html, slug),
				Builds:    moba.ParseBuilds(html),
				Synergies: moba.ParseSynergies(html, slug),
			}
			if buildType != "" {
				if b, ok := pickBuildType(page.Builds, buildType); ok {
					page.Builds = []moba.ChampionBuild{b}
				}
			}
			_ = mainedOnly // reserved for future signal once the data surface stabilizes
			return flags.printJSON(cmd, page)
		},
	}
	cmd.Flags().StringVar(&buildType, "build-type", "", "Filter builds by type: MOST_POPULAR, OPTIONAL, ALTERNATIVE, OFF_META, MATCHUP_SPECIFIC.")
	cmd.Flags().BoolVar(&mainedOnly, "mained-only", false, "(Reserved) Restrict to high-mastery players when Mobalytics surfaces the signal.")
	return cmd
}

func newChampionCountersCmd(flags *rootFlags) *cobra.Command {
	var top int
	var minSample int64
	var worst bool
	cmd := &cobra.Command{
		Use:         "counters <slug>",
		Short:       "Best (or worst) counters for a champion, ranked by Mobalytics matchup delta.",
		Example:     `  mobalytics-lol-pp-cli champion counters jinx --top 10`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := requireSlug(args)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			html, err := client.Fetch(moba.ChampionPath(slug, "counters"))
			if err != nil {
				return err
			}
			rows := moba.ParseCounters(html, slug)
			if minSample > 0 {
				kept := rows[:0]
				for _, r := range rows {
					if r.Sample >= minSample {
						kept = append(kept, r)
					}
				}
				rows = kept
			}
			moba.SortCountersByDelta(rows, !worst)
			if top > 0 && top < len(rows) {
				rows = rows[:top]
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().IntVar(&top, "top", 10, "Return top-N rows after sort.")
	cmd.Flags().Int64Var(&minSample, "min-sample", 0, "Drop rows with fewer than N games of matchup data.")
	cmd.Flags().BoolVar(&worst, "worst", false, "Show worst matchups instead of best counters.")
	return cmd
}

func newChampionMatchupsCmd(flags *rootFlags) *cobra.Command {
	var minSample int64
	cmd := &cobra.Command{
		Use:         "matchups <slug>",
		Short:       "Full matchup table for a champion (broader than --counters).",
		Example:     `  mobalytics-lol-pp-cli champion matchups jinx --min-sample 500`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := requireSlug(args)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			html, err := client.Fetch(moba.ChampionPath(slug, "counters"))
			if err != nil {
				return err
			}
			rows := moba.ParseCounters(html, slug)
			if minSample > 0 {
				kept := rows[:0]
				for _, r := range rows {
					if r.Sample >= minSample {
						kept = append(kept, r)
					}
				}
				rows = kept
			}
			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].Sample > rows[j].Sample
			})
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().Int64Var(&minSample, "min-sample", 0, "Drop matchups with fewer than N games.")
	return cmd
}

func newChampionRunesCmd(flags *rootFlags) *cobra.Command {
	var buildType string
	cmd := &cobra.Command{
		Use:         "runes <slug>",
		Short:       "Recommended runes (primary + secondary) for a champion.",
		Example:     `  mobalytics-lol-pp-cli champion runes jinx`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := requireSlug(args)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			html, err := client.Fetch(moba.ChampionPath(slug, "build"))
			if err != nil {
				return err
			}
			builds := moba.ParseBuilds(html)
			b, ok := pickBuildType(builds, buildType)
			if !ok {
				return fmt.Errorf("no build found for %s", slug)
			}
			return flags.printJSON(cmd, map[string]any{
				"slug":      slug,
				"buildType": b.Type,
				"perks":     b.Perks,
				"spells":    b.Spells,
			})
		},
	}
	cmd.Flags().StringVar(&buildType, "build-type", "", "Filter to a build type (default: MOST_POPULAR RANKED_SOLO).")
	return cmd
}

func newChampionAramCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "aram <slug>",
		Short:       "ARAM build for a champion (Howling Abyss).",
		Example:     `  mobalytics-lol-pp-cli champion aram jinx`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := requireSlug(args)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			html, err := client.Fetch(moba.ChampionPath(slug, "aram-builds"))
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, moba.ChampionBuildPage{
				Slug:   slug,
				Stats:  moba.ParseChampionStats(html, slug),
				Builds: moba.ParseBuilds(html),
			})
		},
	}
	return cmd
}

func newChampionSynergiesCmd(flags *rootFlags) *cobra.Command {
	var top int
	cmd := &cobra.Command{
		Use:         "synergies <slug>",
		Short:       "Team-mate synergy WRs for a champion.",
		Example:     `  mobalytics-lol-pp-cli champion synergies jinx --top 5`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := requireSlug(args)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			html, err := client.Fetch(moba.ChampionPath(slug, "build"))
			if err != nil {
				return err
			}
			rows := moba.ParseSynergies(html, slug)
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].WinRate > rows[j].WinRate })
			if top > 0 && top < len(rows) {
				rows = rows[:top]
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().IntVar(&top, "top", 5, "Top-N synergies by win rate.")
	return cmd
}

func newChampionArenaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "arena <slug>",
		Short:       "Arena (2v2v2v2) build for a champion.",
		Example:     `  mobalytics-lol-pp-cli champion arena jinx`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := requireSlug(args)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			html, err := client.Fetch(moba.ChampionPath(slug, "arena-builds"))
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, moba.ChampionBuildPage{
				Slug:   slug,
				Stats:  moba.ParseChampionStats(html, slug),
				Builds: moba.ParseBuilds(html),
			})
		},
	}
	return cmd
}

func newChampionCombosCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "combos <slug>",
		Short: "Champion combo sequences with move-by-move steps, difficulty, and video URL.",
		Long: `Fetch /lol/champions/<slug>/combos and parse Mobalytics's named
combo records. Each combo carries:

  - slug + championSlug (e.g. "ahri-quick-trade")
  - difficulty (Easy / Average / Hard / Severe)
  - sequence: ordered list of move steps; each step is a list of tokens
    that fire together (typically one like "Q" or "AA", sometimes two
    like "Q" + "Flash")
  - shortDescription + executionText + notes (prose from Mobalytics's
    coaching writers)
  - videoUrl (Vimeo link Mobalytics renders on the page)
  - tags (e.g. "basic", "all-in", "trade")

The combo videos themselves are not downloaded; the videoUrl is included
so callers can open or embed them.`,
		Example: `  mobalytics-lol-pp-cli champion combos ahri
  mobalytics-lol-pp-cli champion combos ahri --agent --select slug,difficulty,sequence`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := requireSlug(args)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			path := moba.ChampionPath(slug, "combos")
			html, err := client.Fetch(path)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{
				"slug":      slug,
				"combosURL": "https://mobalytics.gg" + path,
				"stats":     moba.ParseChampionStats(html, slug),
				"combos":    moba.ParseCombos(html),
			})
		},
	}
	return cmd
}

func newChampionMainedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mained <slug>",
		Short: "(Stub) High-mastery one-trick build view for a champion.",
		Long: `Mobalytics does not expose a "mained-only" filter on its public
HTML pages — LeagueOfGraphs-style one-trick stats are not surfaced. This
command returns the OPTIONAL build (often a niche/expert build) as the
closest proxy until Mobalytics ships the filter.`,
		Example:     `  mobalytics-lol-pp-cli champion mained jinx`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			slug, err := requireSlug(args)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			client := moba.NewClient(flags.timeout)
			html, err := client.Fetch(moba.ChampionPath(slug, "build"))
			if err != nil {
				return err
			}
			builds := moba.ParseBuilds(html)
			b, ok := pickBuildType(builds, "OPTIONAL")
			if !ok {
				b, _ = pickBuildType(builds, "")
			}
			return flags.printJSON(cmd, map[string]any{
				"slug":  slug,
				"note":  "Approximated as OPTIONAL build; Mobalytics does not expose a one-trick filter on public HTML.",
				"build": b,
			})
		},
	}
	return cmd
}
