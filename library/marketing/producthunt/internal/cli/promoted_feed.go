// Feed command group.
//
// The spec declares a single `feed` resource; this file wires it up with
// runtime-appropriate semantics. `feed` with no subcommand is an alias for
// `today`. `feed raw` dumps the raw Atom XML (useful for piping into other
// parsers). `feed refresh` is an alias for `sync`.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/atom"
	"github.com/spf13/cobra"
)

func newFeedPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feed",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Fetch or inspect the public Product Hunt Atom feed",
		Long: `The 'feed' group covers direct access to producthunt.com/feed (Atom 1.0,
50 entries, no auth). See the subcommands below. For the default
"today's featured launches" view, use the top-level 'today' command —
it accepts --limit, --live, --select, and the full agent-friendly flag set.`,
		Example: `  producthunt-pp-cli feed raw --validate
  producthunt-pp-cli feed refresh
  producthunt-pp-cli today --limit 10  # the common ask`,
	}
	cmd.AddCommand(newFeedRawCmd(flags))
	cmd.AddCommand(newFeedRefreshCmd(flags))
	return cmd
}

func newFeedRawCmd(flags *rootFlags) *cobra.Command {
	var validate bool

	c := &cobra.Command{
		Use:   "raw",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Print the raw Atom XML body of /feed",
		Long: `Fetch /feed and write the raw XML to stdout without parsing. Useful for
piping into other XML tools or preserving exact-wire content.

Pass --validate to parse the body and fail with exit code 5 if the Atom
structure is broken (useful in health checks).`,
		Example: `  producthunt-pp-cli feed raw > /tmp/ph-feed.xml
  producthunt-pp-cli feed raw --validate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := fetchFeedBody(flags.timeout)
			if err != nil {
				return apiErr(err)
			}
			if validate {
				if _, err := atom.Parse(body); err != nil {
					return apiErr(fmt.Errorf("validation failed: %w", err))
				}
			}
			_, err = cmd.OutOrStdout().Write(body)
			return err
		},
	}
	c.Flags().BoolVar(&validate, "validate", false, "Parse the Atom body and fail on error")
	return c
}

func newFeedRefreshCmd(flags *rootFlags) *cobra.Command {
	refresh := &cobra.Command{
		Use:     "refresh",
		Short:   "Alias for 'sync': fetch /feed and persist a snapshot",
		Example: `  producthunt-pp-cli feed refresh --json`,
		Long:    `Alias for the top-level 'sync' command. Kept here for discoverability within the feed group.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return newSyncCmd(flags).RunE(cmd, args)
		},
	}
	refresh.Flags().String("db", "", "Path to local SQLite store")
	return refresh
}
