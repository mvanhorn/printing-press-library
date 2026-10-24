// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSuggestCmd(flags))
	})
}

// suggestResult is the google-play-shaped autocomplete answer.
type suggestResult struct {
	Term        string   `json:"term"`
	Storefront  string   `json:"storefront"`
	Suggestions []string `json:"suggestions"`
	Note        string   `json:"note,omitempty"`
}

func newSuggestCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "suggest <term>",
		Short: "Get App Store search autocomplete suggestions for a partial term",
		Long: strings.Trim(`
Return the App Store's search suggestions (autocomplete hints) for a partial
term in one storefront (--country), in Apple's order: a quick way to mine
keyword seeds for keyword-rank.

Source: the search.itunes.apple.com hints endpoint, an undocumented surface
that robots.txt flags. This command is itself the explicit opt-in: it calls
the endpoint only when you run suggest, once per run, and nothing else in this
CLI (sync, bulk capture) ever calls it. Nothing is stored except the capture
log row.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli suggest insta --json
  app-store-pp-cli suggest "photo ed" --country gb --limit 5`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			// Any text is a valid search term: a nonsense term is an empty or
			// broad search (exit 0), not an invalid argument.
			"pp:no-error-path-probe": "true",
			"pp:happy-args":          "term=insta",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "suggest")
			}
			term := strings.TrimSpace(strings.Join(args, " "))
			if term == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search term is required, e.g. suggest insta"))
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be 0 (all) or more (got %d)", limit))
			}
			cc, err := appStoreCountry(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, st, err := s2bOpenLive(cmd, flags)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			hints, err := c.Hints(ctx, cc, term)
			if err != nil {
				return classifyAppStoreErr(err)
			}
			out := suggestResult{Term: term, Storefront: cc, Suggestions: make([]string, 0, len(hints.Suggestions))}
			out.Suggestions = append(out.Suggestions, hints.Suggestions...)
			if limit > 0 && len(out.Suggestions) > limit {
				out.Suggestions = out.Suggestions[:limit]
			}
			if len(out.Suggestions) == 0 {
				out.Note = fmt.Sprintf("Apple returned no suggestions for %q in storefront %s (verified empty)", term, cc)
			}
			w := cmd.OutOrStdout()
			if !wantsHumanTable(w, flags) {
				return printJSONFiltered(w, out, flags)
			}
			if len(out.Suggestions) == 0 {
				_, _ = fmt.Fprintln(w, "No suggestions.")
				s2bPrintNotes(cmd, []string{out.Note})
				return nil
			}
			for _, s := range out.Suggestions {
				_, _ = fmt.Fprintln(w, cliutil.ScrubTerminal(s))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum suggestions to return (0 = all Apple returns, usually up to 10)")
	s2bDBFlag(cmd)
	return cmd
}
