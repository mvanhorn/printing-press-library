// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(digg-rankings-and-min-starrers): library-side new file. The
// /ai/x/rankings/companies page surfaces three sub-views (Emerging
// Startups, Movers up/down, the full company ranking) inside one
// HTML page's RSC stream. The CLI exposes each as a sibling subcommand
// so consumers get a stable typed schema per command rather than a
// polymorphic --section flag.
//
// All three commands hit the same URL and parse different sections of
// the response. A per-process cache (sync.Once around the fetch)
// avoids redundant HTTP work if a caller runs more than one in a
// single invocation.

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
	"github.com/spf13/cobra"
)

// rankingsCompaniesURL is the live endpoint. Exposed as a package var
// (not const) so tests can swap it for an httptest server URL. The
// short host (di.gg) matches the existing convention in feed.go and
// github.go; both di.gg and digg.com resolve to the same RSC payload.
var rankingsCompaniesURL = "https://di.gg/ai/x/rankings/companies"

// defaultMaxSkipRatio is the schema-drift tolerance for rankings
// commands. 10% feels tight enough to catch a rename of any key
// field (username, rank) — every entry decoding without that field
// raises SkipRatio to 1.0 — while loose enough that one bad entry
// in a snapshot of ten doesn't trip the alarm. Overridable per
// command via --max-skip-ratio (added in commit 5 of this PR).
const defaultMaxSkipRatio = 0.10

func newRankingsCmd(flags *rootFlags) *cobra.Command {
	// One fetcher per command tree. The fetcher itself no longer
	// caches across calls (see rankingsFetcher comment) so this is
	// scoped per-tree for isolation, not deduplication: each subcommand
	// dispatched on this tree issues its own HTTP fetch.
	fetcher := &rankingsFetcher{}

	cmd := &cobra.Command{
		Use:   "rankings",
		Short: "Rankings views Digg publishes alongside the AI 1000 leaderboard",
		Long: `Rankings views Digg publishes alongside the AI 1000 leaderboard.

Today the CLI exposes three sub-views of the /ai/x/rankings/companies
page, each a stable typed schema:

  emerging   Curated list of small AI companies (the "EMERGING STARTUPS
             — CURATED THIS SNAPSHOT" block). 10 rows. Mixes
             AI-judge-flagged emerging startups (IsEmergingStartup=true)
             with adjacent new entrants.

  movers     Companies whose follower count shifted most since the last
             snapshot. Two halves: --direction up returns the gainers,
             --direction down the losers. Default returns both,
             tagged on each row.

  list       The full company ranking (paginated server-side; this
             returns the initial-HTML slice — pass --limit to cap).`,
	}
	cmd.AddCommand(newRankingsEmergingCmd(flags, fetcher))
	cmd.AddCommand(newRankingsMoversCmd(flags, fetcher))
	cmd.AddCommand(newRankingsListCmd(flags, fetcher))
	return cmd
}

// rankingsFetcher fetches the rankings/companies HTML on demand.
//
// Originally this cached the decoded RSC behind a sync.Once so two
// sibling commands invoked on the same Cobra tree could share the
// fetch. That optimization is removed: a long-lived embedder (one
// holding the tree across multiple `rankings *` invocations) would
// pin stale data, while the actual production callers — the CLI
// binary and the MCP server's subprocess shell-out — build a fresh
// tree per invocation anyway, so the cache never helped them.
// Always fetch.
type rankingsFetcher struct{}

func (f *rankingsFetcher) get(cmd *cobra.Command) (string, error) {
	html, err := fetchURL(cmd.Context(), rankingsCompaniesURL)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", rankingsCompaniesURL, err)
	}
	decoded := diggparse.DecodeRSC(html)
	if decoded == "" {
		return "", fmt.Errorf(
			"no RSC pushes found in %s (%d bytes); page shape may have changed",
			rankingsCompaniesURL, len(html))
	}
	return decoded, nil
}

// reportDriftOrEmit applies the schema-drift threshold to ParseStats
// and either:
//   - emits a stderr warning (non-zero Skipped, below threshold), or
//   - returns a typed error wrapping the threshold violation.
//
// Returns nil on the clean path. The error message includes a
// suggested next-higher --max-skip-ratio value so an operator hit by
// a transient schema bump can relax the gate from the command line.
func reportDriftOrEmit(cmd *cobra.Command, section string, stats diggparse.ParseStats, maxSkip float64) error {
	if err := stats.Threshold(maxSkip); err != nil {
		var te *diggparse.ThresholdError
		if errors.As(err, &te) {
			// At 100% failure no --max-skip-ratio value rescues the
			// caller (the gate trips at SkipRatio >= maxRatio and 1.0
			// is the ceiling — 1.0 >= 1.0 still trips). Suggesting a
			// flag value would be actively misleading; just point at
			// the schema.
			if te.Stats.SkipRatio() >= 1.0 {
				return fmt.Errorf(
					"%s: %w — every entry failed to decode; check digg.com for schema changes",
					section, te)
			}
			// Otherwise suggest a ratio strictly greater than the
			// observed so retrying with the suggestion clears the
			// threshold. +0.05 chosen as a small but visible bump.
			relaxTo := te.Stats.SkipRatio() + 0.05
			if relaxTo > 1.0 {
				relaxTo = 1.0
			}
			return fmt.Errorf(
				"%s: %w — pass --max-skip-ratio %.2f to relax, or check digg.com for schema changes",
				section, te, relaxTo)
		}
		return fmt.Errorf("%s: %w", section, err)
	}
	if stats.Skipped > 0 {
		first := ""
		if len(stats.Errors) > 0 {
			first = stats.Errors[0].Error()
		}
		fmt.Fprintf(cmd.ErrOrStderr(),
			"warn: %s parsed %d/%d entries; %d skipped (%.0f%%). First skip: %s\n",
			section, stats.Decoded, stats.Attempted, stats.Skipped, stats.SkipRatio()*100, first)
	}
	return nil
}

// validateMaxSkipRatio is the PreRunE guard shared by all rankings
// commands. The flag is a float in [0, 1]; out-of-range values would
// be silently ignored by ParseStats.Threshold, so we reject them at
// flag parse time instead of letting the user think the gate is
// active when it isn't.
func validateMaxSkipRatio(ratio float64) error {
	if ratio < 0 || ratio > 1 {
		return fmt.Errorf("--max-skip-ratio must be in [0, 1], got %v", ratio)
	}
	return nil
}

// maxSkipRatioFlagHelp is the help text shared by all three rankings
// commands. Centralized so help stays consistent if we tune defaults.
const maxSkipRatioFlagHelp = "Fraction of entries that may fail to decode before the command exits non-zero. " +
	"Schema-drift detector — if upstream renames a key, every row registers as Skipped and SkipRatio becomes 1.0. " +
	"Default 0.10 (10%) tolerates one bad entry in ten. Range [0, 1]; 0 means \"no tolerance\" (any skip trips) " +
	"and 1 only trips when every entry fails to decode."

func newRankingsEmergingCmd(flags *rootFlags, fetcher *rankingsFetcher) *cobra.Command {
	maxSkip := defaultMaxSkipRatio
	cmd := &cobra.Command{
		Use:   "emerging",
		Short: "Curated list of small AI companies from the rankings/companies snapshot",
		Long: `Emerging startups: the "EMERGING STARTUPS — CURATED THIS SNAPSHOT"
section on /ai/x/rankings/companies. ~10 rows per snapshot, refreshed
daily. Mixes AI-judge-flagged emerging startups (IsEmergingStartup=true)
with adjacent new entrants the curator surfaced; consumers wanting
only the flagged subset can filter on .isEmergingStartup.`,
		Example: "  digg-pp-cli rankings emerging --json",
		// NOTE: deliberately no pp:endpoint annotation. The MCP cobratree
		// walker treats pp:endpoint-tagged commands as "covered by a
		// typed MCP tool" and skips registering a shell-out tool for
		// them. Until the upstream MCP layer ships typed rankings tools,
		// we want the rankings commands exposed as shell-out tools.
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return validateMaxSkipRatio(maxSkip)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			decoded, err := fetcher.get(cmd)
			if err != nil {
				return err
			}
			entries, stats, err := diggparse.ExtractEmerging(decoded)
			if err != nil {
				return fmt.Errorf("emerging: %w", err)
			}
			if err := reportDriftOrEmit(cmd, "rankings.emerging", stats, maxSkip); err != nil {
				return err
			}
			return emitGithub(cmd, flags, entries, 0)
		},
	}
	cmd.Flags().Float64Var(&maxSkip, "max-skip-ratio", defaultMaxSkipRatio, maxSkipRatioFlagHelp)
	return cmd
}

// validMoverDirections is the closed set of values accepted by
// `rankings movers --direction`. Centralized so help text and
// PreRunE validation can't drift.
var validMoverDirections = []string{"up", "down", "both"}

func newRankingsMoversCmd(flags *rootFlags, fetcher *rankingsFetcher) *cobra.Command {
	var direction string
	maxSkip := defaultMaxSkipRatio
	cmd := &cobra.Command{
		Use:   "movers",
		Short: "Companies that climbed or fell most since the last rankings/companies snapshot",
		Long: `Movers since the last rankings snapshot.

  --direction up    return only the gainers (positive followCountChange)
  --direction down  return only the losers
  --direction both  return both, with .direction tagged on each row (default)

Each side is curated by Digg to ~10 entries; the section is intended
as a "what changed today" view, not an exhaustive delta feed.`,
		Example: "  digg-pp-cli rankings movers --direction up --json",
		Annotations: map[string]string{
			// See newRankingsEmergingCmd note on omitting pp:endpoint.
			"mcp:read-only": "true",
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := validateMaxSkipRatio(maxSkip); err != nil {
				return err
			}
			for _, ok := range validMoverDirections {
				if direction == ok {
					return nil
				}
			}
			return fmt.Errorf(
				"invalid --direction %q: must be one of %s",
				direction, strings.Join(validMoverDirections, ", "))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			decoded, err := fetcher.get(cmd)
			if err != nil {
				return err
			}
			up, down, stats, err := diggparse.ExtractMovers(decoded)
			if err != nil {
				return fmt.Errorf("movers: %w", err)
			}
			if err := reportDriftOrEmit(cmd, "rankings.movers", stats, maxSkip); err != nil {
				return err
			}
			var out []diggparse.CompanyEntry
			switch direction {
			case "up":
				out = up
			case "down":
				out = down
			case "both":
				out = make([]diggparse.CompanyEntry, 0, len(up)+len(down))
				out = append(out, up...)
				out = append(out, down...)
			}
			return emitGithub(cmd, flags, out, 0)
		},
	}
	cmd.Flags().StringVar(&direction, "direction", "both",
		"Movers direction: up | down | both")
	cmd.Flags().Float64Var(&maxSkip, "max-skip-ratio", defaultMaxSkipRatio, maxSkipRatioFlagHelp)
	return cmd
}

func newRankingsListCmd(flags *rootFlags, fetcher *rankingsFetcher) *cobra.Command {
	var limit int
	maxSkip := defaultMaxSkipRatio
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Full company ranking (initial-HTML slice; paginated server-side)",
		Long: `Returns the initial-HTML slice of the main "Companies followed by
the AI 2K" ranking. The full ranking is paginated server-side; what
ships in the first response is the top N (varies by deploy — typically
~50–200 entries). Use --limit to cap further.

For surface coverage of an account, prefer 'rankings emerging' or
'rankings movers' which return Digg's curated sub-slices.`,
		Example: "  digg-pp-cli rankings list --limit 20 --json",
		Annotations: map[string]string{
			// See newRankingsEmergingCmd note on omitting pp:endpoint.
			"mcp:read-only": "true",
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return validateMaxSkipRatio(maxSkip)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			decoded, err := fetcher.get(cmd)
			if err != nil {
				return err
			}
			entries, stats, err := diggparse.ExtractMainRanking(decoded)
			if err != nil {
				return fmt.Errorf("list: %w", err)
			}
			if err := reportDriftOrEmit(cmd, "rankings.list", stats, maxSkip); err != nil {
				return err
			}
			return emitGithub(cmd, flags, entries, limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows to return (0 = all rows in the initial-HTML slice)")
	cmd.Flags().Float64Var(&maxSkip, "max-skip-ratio", defaultMaxSkipRatio, maxSkipRatioFlagHelp)
	return cmd
}
