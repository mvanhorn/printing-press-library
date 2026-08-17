// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source/rss"
	"github.com/spf13/cobra"
)

// pp:data-source live
// feed reads a Medium author, publication, or tag RSS feed directly from
// medium.com — no API key, no GraphQL, no cookies. It is the v2 RSS surface,
// served through the Resolver so later sources can extend feed coverage without
// touching this command.
func newFeedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feed <@user|publication|tag>",
		Short: "Read a Medium author, publication, or tag RSS feed (no key, no cookies).",
		Long: strings.Trim(`
Read a Medium RSS feed for an author, publication, or tag, sourced directly from
medium.com with no API key and no cookies.

Reference auto-detection:
  @<handle>     an author        -> medium.com/feed/@<handle>
  tag/<tag>     a topic tag      -> medium.com/feed/tag/<tag>
  <slug>        a publication    -> medium.com/feed/<slug>

To read a tag, prefix it with "tag/" (a bare token is treated as a publication
slug). To read an author, prefix the handle with "@".`, "\n"),
		Example: strings.Trim(`
  medium-reader-pp-cli feed @quincylarson
  medium-reader-pp-cli feed tag/ux --agent
  medium-reader-pp-cli feed uxdesign-cc --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				ref := "<@user|publication|tag>"
				if len(args) > 0 {
					ref = args[0]
				}
				kind := kindLabel(rss.ClassifyRef(ref))
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch %s feed for %q (%s)\n", kind, ref, rss.FeedURL(rss.ClassifyRef(ref), ref))
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<@user|publication|tag> is required"))
			}
			ref := strings.TrimSpace(args[0])
			if ref == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<@user|publication|tag> is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			resolver := flags.newResolver()
			posts, err := resolver.Feed(ctx, ref)
			if err != nil {
				// The RSS path is keyless; a fetch/parse failure is an
				// upstream source error in v2 terms (no API-key hint to emit).
				return apiErr(fmt.Errorf("feed %q: %w", ref, err))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if len(posts) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "no posts found for %q\n", ref)
					return nil
				}
				headers := []string{"DATE", "AUTHOR", "TITLE", "URL"}
				rows := make([][]string, 0, len(posts))
				for _, p := range posts {
					date := ""
					if !p.PublishedAt.IsZero() {
						date = p.PublishedAt.Format("2006-01-02")
					}
					rows = append(rows, []string{date, p.Author, p.Title, p.URL})
				}
				return flags.printTable(cmd, headers, rows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), posts, flags)
		},
	}
	return cmd
}

// kindLabel renders a feed Kind as a short human label for dry-run output.
func kindLabel(k rss.Kind) string {
	switch k {
	case rss.KindUser:
		return "author"
	case rss.KindTag:
		return "tag"
	default:
		return "publication"
	}
}
