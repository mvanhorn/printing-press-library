// Honest CF-gated stubs.
//
// Commands in this file correspond to features in the absorb manifest that
// would require Product Hunt's HTML surface (post detail pages, historical
// leaderboards, topic feeds, user profiles, collections, newsletter archive).
// Cloudflare blocks those routes for automated HTTP clients, so we do not
// implement them in this runtime. Each command runs — exit code 3
// (notFound) — and emits a JSON explanation naming the gate and the best
// Atom-backed alternative. Never a silent no-op, never a fake success.
//
// When the CLI gains `auth login --chrome` + browser-matched TLS, these
// commands can be upgraded in place.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// gatedCommand describes a CF-gated feature: what the user asked for, why we
// can't deliver it, and what they can do instead.
type gatedCommand struct {
	use         string
	short       string
	long        string
	example     string
	alternative string
}

// emitGated prints the explanation and returns a notFoundErr so the exit
// code is typed (3). Agents piping output can branch on exit code.
func emitGated(cmd *cobra.Command, flags *rootFlags, feature string, reason string, alt string, context map[string]any) error {
	payload := map[string]any{
		"feature":      feature,
		"status":       "not_available_in_this_build",
		"reason":       reason,
		"alternative":  alt,
		"cf_gated":     true,
		"upgrade_hint": "A future 'auth login --chrome' pass could import Cloudflare clearance to unlock this.",
	}
	for k, v := range context {
		payload[k] = v
	}
	buf, _ := json.Marshal(payload)
	_ = printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
	return notFoundErr(fmt.Errorf("%s is CF-gated; see JSON output for alternatives", feature))
}

func newPostCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "post <slug>",
		Short:   "(stub) Full post detail — CF-gated, use 'get <slug>' instead",
		Example: `  producthunt-pp-cli post seeknal --json`,
		Long: `Product Hunt's post detail HTML pages at /posts/<slug> are blocked by
Cloudflare for automated HTTP clients. This build does not fetch them.

For /feed-level metadata on a slug (id, title, tagline, author, published,
discussion URL, external URL), use 'get <slug>'. To read the real page
in a browser, use 'open <slug>'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return emitGated(cmd, flags, "post <slug>",
				"producthunt.com/posts/<slug> is Cloudflare-gated and not reachable from a Go HTTP client.",
				"Use 'get <slug>' (reads /feed), or 'open <slug>' to view the HTML page in your browser.",
				map[string]any{"requested_slug": args[0]})
		},
	}
}

func newCommentsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "comments <slug>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "(stub) Comments on a post — CF-gated, requires browser clearance",
		Example: `  producthunt-pp-cli comments seeknal --json`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return emitGated(cmd, flags, "comments <slug>",
				"Comments live on /posts/<slug>, which is Cloudflare-gated. The Atom feed does not carry comments.",
				"Read comments on the website via 'open <slug>'.",
				map[string]any{"requested_slug": args[0]})
		},
	}
}

func newLeaderboardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "leaderboard",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "(stub) Daily/weekly/monthly leaderboards — CF-gated, use local snapshots instead",
		Long: `Historical leaderboard pages at producthunt.com/leaderboard/... are
Cloudflare-gated. This build does not fetch them.

Alternative: run 'sync' on a schedule. Each sync captures a dated ranked
snapshot of the live /feed. Then query 'list --since <window>' or 'trend
<slug>' to read your own historical record.`,
		Example: `  producthunt-pp-cli leaderboard daily --json
  producthunt-pp-cli leaderboard weekly --json`,
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return emitGated(cmd, flags, "leaderboard",
			"producthunt.com/leaderboard/<period>/<date> is Cloudflare-gated.",
			"Run 'sync' regularly to build your own daily leaderboard, then use 'list --since' or 'trend <slug>'.",
			nil)
	}
	for _, period := range []string{"daily", "weekly", "monthly", "yearly"} {
		p := period
		cmd.AddCommand(&cobra.Command{
			Use:     p,
		Annotations: map[string]string{"mcp:read-only": "true"},
			Short:   fmt.Sprintf("(stub) %s leaderboard — CF-gated", p),
			Example: fmt.Sprintf("  producthunt-pp-cli leaderboard %s --json", p),
			RunE: func(c *cobra.Command, args []string) error {
				return emitGated(c, flags, "leaderboard "+p,
					fmt.Sprintf("producthunt.com/leaderboard/%s is Cloudflare-gated.", p),
					"Sync regularly to build your own ranked history.",
					map[string]any{"period": p})
			},
		})
	}
	return cmd
}

func newTopicCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "topic <slug>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "(stub) Topic feed — CF-gated, no Atom alternative",
		Example: `  producthunt-pp-cli topic artificial-intelligence --json`,
		Long: `Topic pages at /topics/<slug> are Cloudflare-gated, and the /feed?category=
query parameter is ignored by Product Hunt server-side (verified during
browser-sniff — the response is identical to unfiltered /feed).

No Atom fallback exists. This stub documents the gap explicitly.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return emitGated(cmd, flags, "topic <slug>",
				"/topics/<slug> is CF-gated and /feed?category= is ignored server-side.",
				"None. Product Hunt does not expose per-topic Atom feeds.",
				map[string]any{"requested_topic": args[0]})
		},
	}
}

func newUserCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "user <handle>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "(stub) User/maker profile — CF-gated",
		Example: `  producthunt-pp-cli user rrhoover --json`,
		Long: `Profile pages at /@<handle> are Cloudflare-gated.

Alternative: 'list --author "Display Name"' searches by the author name as
it appeared in Atom feed entries you've synced.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return emitGated(cmd, flags, "user <handle>",
				"/@<handle> is CF-gated.",
				"Use 'list --author \"Display Name\"' to search author-named posts in your local store.",
				map[string]any{"requested_handle": args[0]})
		},
	}
}

func newCollectionCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "collection <slug>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "(stub) Curated collections — CF-gated, no Atom alternative",
		Example: `  producthunt-pp-cli collection launch-week --json`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return emitGated(cmd, flags, "collection <slug>",
				"/collections/<slug> is CF-gated.",
				"None. Product Hunt does not expose collection data via Atom.",
				map[string]any{"requested_slug": args[0]})
		},
	}
}

func newNewsletterCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "newsletter",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "(stub) Newsletter archive — CF-gated, no Atom alternative",
		Example: `  producthunt-pp-cli newsletter --json`,
		Long: `Newsletter archive pages at /newsletters/archive/... are Cloudflare-gated.
The /feed Atom endpoint does not include newsletter content.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return emitGated(cmd, flags, "newsletter",
				"/newsletters/archive/... is CF-gated.",
				"None. Product Hunt does not expose newsletter issues via Atom.",
				nil)
		},
	}
	return cmd
}
