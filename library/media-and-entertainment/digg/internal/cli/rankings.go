// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(digg-rankings-and-min-starrers): library-side new file. The
// rankings page (formerly /ai/x/rankings/companies, now
// https://digg.com/tech/x/rankings) carries the full ranking of people
// and companies inside its RSC stream; `rankings list` parses it.

package cli

import (
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/diggparse"
	"github.com/spf13/cobra"
)

// rankingsCompaniesURL is the live endpoint. Exposed as a package var
// (not const) so tests can swap it for an httptest server URL. The old
// /ai/x/rankings/companies URL 308-redirects here, so the CLI reads the
// current page directly. The page carries one view, the full ranking.
var rankingsCompaniesURL = "https://digg.com/tech/x/rankings"

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

Reads https://digg.com/tech/x/rankings:

  list       The full ranking of people and companies (the
             initial-HTML slice; pass --limit to cap).`,
		// A group command with no RunE prints help and exits 0 for any
		// argument, so a subcommand Digg no longer backs would look like
		// it worked. Name the unknown subcommand instead.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErr(fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath()))
			}
			return cmd.Help()
		},
	}
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

func newRankingsListCmd(flags *rootFlags, fetcher *rankingsFetcher) *cobra.Command {
	var limit int
	maxSkip := defaultMaxSkipRatio
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Full people-and-companies ranking from Digg's rankings page",
		Long: `Returns the ranking embedded in https://digg.com/tech/x/rankings,
which ranks both people and companies (the page shipped the top 1000
on 2026-10-21; the count varies by deploy). Use --limit to cap.`,
		Example: "  digg-pp-cli rankings list --limit 20 --json",
		Annotations: map[string]string{
			// No pp:endpoint annotation: the MCP walker skips pp:endpoint-tagged
			// commands as covered by a typed tool, and none exists for rankings.
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
