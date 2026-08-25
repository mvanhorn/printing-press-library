// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source/page"
	"github.com/spf13/cobra"
)

// pp:data-source live
// read fetches a single Medium article by URL or id and renders its body as
// Markdown, sourced directly from the article page's embedded JSON
// (window.__APOLLO_STATE__) — no API key, no GraphQL, no cookies required. With
// an optional Medium session cookie (Tier 1) the full member body unlocks;
// without one, a locked post returns Medium's preview and is flagged
// IsPreviewOnly. Served through the Resolver so later sources can extend read
// coverage without touching this command.
func newReadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read <url|id>",
		Short: "Read a Medium article as Markdown (no key, no cookies).",
		Long: strings.Trim(`
Fetch a single Medium article and render its body as Markdown, sourced directly
from the article page with no API key and no cookies.

Accepts either a full Medium URL or a bare article id:
  read 818e7841df9c
  read https://medium.com/p/818e7841df9c
  read https://author.medium.com/some-title-818e7841df9c

Locked (member-only) posts return Medium's truncated preview when read
anonymously; the output is flagged as a preview. Supplying your own Medium
session cookie unlocks the full body (see 'auth' once Tier-1 auth is wired).`, "\n"),
		Example: strings.Trim(`
  medium-reader-pp-cli read 818e7841df9c
  medium-reader-pp-cli read https://medium.com/p/818e7841df9c --json
  medium-reader-pp-cli read 818e7841df9c --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				ref := "<url|id>"
				if len(args) > 0 {
					ref = args[0]
				}
				id := page.ExtractID(ref)
				if id == "" {
					id = ref
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch article %q (https://medium.com/p/%s)\n", ref, id)
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<url|id> is required"))
			}
			ref := strings.TrimSpace(args[0])
			if ref == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<url|id> is required"))
			}
			if page.ExtractID(ref) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("could not extract an article id from %q (expected a Medium URL or a hex article id)", ref))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			resolver := flags.newResolver()
			art, err := resolver.ReadArticle(ctx, ref)
			if err != nil {
				// The page path is keyless; a fetch/parse failure is an
				// upstream source error in v2 terms (no API-key hint to emit).
				return apiErr(fmt.Errorf("read %q: %w", ref, err))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printArticleHuman(cmd, art)
			}
			return printJSONFiltered(cmd.OutOrStdout(), art, flags)
		},
	}
	return cmd
}

// printArticleHuman renders the article as a readable header block followed by
// the Markdown body — the default (non-JSON) presentation for the read command.
func printArticleHuman(cmd *cobra.Command, art *source.Article) error {
	w := cmd.OutOrStdout()
	if art.Title != "" {
		fmt.Fprintf(w, "# %s\n", art.Title)
	}
	meta := make([]string, 0, 3)
	if art.Author != "" {
		meta = append(meta, "by "+art.Author)
	}
	if !art.PublishedAt.IsZero() {
		meta = append(meta, art.PublishedAt.Format("2006-01-02"))
	}
	if art.URL != "" {
		meta = append(meta, art.URL)
	}
	if len(meta) > 0 {
		fmt.Fprintf(w, "%s\n", strings.Join(meta, "  ·  "))
	}
	if art.IsPreviewOnly {
		fmt.Fprintln(w, "(preview only — locked member post; supply a Medium cookie to unlock the full body)")
	}
	fmt.Fprintln(w)
	if art.Markdown != "" {
		fmt.Fprintln(w, art.Markdown)
	} else {
		fmt.Fprintln(w, "(no body available)")
	}
	return nil
}
