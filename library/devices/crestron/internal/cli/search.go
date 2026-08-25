// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Reads the local mirror only. Release notes live behind a sign-in on
// per-version pages, so cross-version search is only possible against synced
// text; there is no live equivalent.

package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronstore"

	"github.com/spf13/cobra"
)

type searchView struct {
	Releases []crestronstore.ReleaseHit `json:"releases,omitempty"`
	Products []crestronstore.Product    `json:"products,omitempty"`
	Count    int                        `json:"count"`
	Query    string                     `json:"query"`
	Note     string                     `json:"note,omitempty"`
}

func newNovelSearchCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var flagLimit int
	var flagDB string

	cmd := &cobra.Command{
		Use:   "search <text>",
		Short: "Search every firmware release note and change log at once for a term.",
		Long: strings.Trim(`
Full-text search the local mirror.

With --type firmware_release this searches release notes and change logs across
every synced version at once. Crestron serves those notes on per-version pages
behind a sign-in and offers no search over them, so this view does not exist
anywhere else. Run 'sync --notes' with a signed-in session first to populate it.

Use this command to find which firmware version mentions a term across all
models. Do NOT use it to compare two specific versions; use 'firmware diff'.
Do NOT use it for fleet-wide currency checks; use 'fleet status'.
`, "\n"),
		Example: strings.Trim(`
  crestron-pp-cli search "HDCP" --type firmware_release --agent
  crestron-pp-cli search "Dante" --type firmware_release --limit 10
  crestron-pp-cli search "encoder" --type product
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			// A search term that matches nothing is a valid empty result, not a
			// usage error, so there is no error path to probe.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search term is required"))
			}
			query := strings.Join(args, " ")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if flagDB == "" {
				flagDB = defaultDBPath("crestron-pp-cli")
			}
			if _, statErr := os.Stat(flagDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no local mirror at %s\nrun: crestron-pp-cli sync --notes --db %s\n", flagDB, flagDB)
				view := searchView{Query: query, Note: "no local mirror; run sync first"}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				return nil
			}

			st, err := crestronstore.Open(ctx, flagDB)
			if err != nil {
				return fmt.Errorf("opening local mirror: %w", err)
			}
			defer func() { _ = st.Close() }()

			view := searchView{Query: query}
			kind := strings.ToLower(strings.TrimSpace(flagType))
			switch kind {
			case "", "firmware_release", "release", "firmware":
				hits, err := st.SearchReleases(ctx, query, flagLimit)
				if err != nil {
					return fmt.Errorf("searching releases: %w", err)
				}
				view.Releases = hits
				view.Count = len(hits)
				if len(hits) == 0 {
					counts, _ := st.Counts(ctx)
					if counts["releases"] == 0 {
						fmt.Fprintln(cmd.ErrOrStderr(), "run: crestron-pp-cli sync")
						view.Note = "the local mirror has no releases to search; run 'crestron-pp-cli sync' first"
					} else {
						view.Note = fmt.Sprintf(
							"no release text matched %q across %d synced releases; release notes need 'sync --notes' with a signed-in session",
							query, counts["releases"])
					}
				}
			case "product", "products":
				hits, err := st.SearchProducts(ctx, query, flagLimit)
				if err != nil {
					return fmt.Errorf("searching products: %w", err)
				}
				view.Products = hits
				view.Count = len(hits)
				if len(hits) == 0 {
					counts, _ := st.Counts(ctx)
					if counts["products"] == 0 {
						fmt.Fprintln(cmd.ErrOrStderr(), "run: crestron-pp-cli sync")
						view.Note = "the local mirror has no products to search; run 'crestron-pp-cli sync' first"
					}
					view.Note = fmt.Sprintf("no product matched %q across %d synced products", query, counts["products"])
				}
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown --type %q: use firmware_release or product", flagType))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			if view.Count == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if len(view.Releases) > 0 {
				fmt.Fprintln(w, "VERSION\tDATE\tTITLE\tMATCH")
				for _, h := range view.Releases {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						dashIfEmpty(h.Version), dashIfEmpty(h.Date),
						truncateText(h.Title, 40), truncateText(h.Snippet, 50))
				}
			} else {
				fmt.Fprintln(w, "MODEL\tDESCRIPTION")
				for _, p := range view.Products {
					fmt.Fprintf(w, "%s\t%s\n", p.Model, truncateText(p.Description, 60))
				}
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "firmware_release", "What to search: firmware_release or product")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum results")
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	return cmd
}

func truncateText(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
