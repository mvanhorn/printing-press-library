// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newTspCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tsp",
		Short: "The Trading System Project — the Trading Tribe's collaborative mechanical-system spec, section by section",
		Long: `The Trading System Project (TSP) is the Trading Tribe's open build of a
mechanical trend-following system: data verification, continuous contracts,
an exponential-crossover system (EA), a support-and-resistance system (SR),
trend definition, diversification, skid/trading-frequency, core position
sizing, and more. 'tsp list' shows the sections; 'tsp show <slug>' prints
one section's notes, rules, and links.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTspList(cmd, flags, "", "")
		},
	}
	var dbPath, sortBy string
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().StringVar(&sortBy, "sort", "doc", "Sort order: doc (document order) | slug | updated")
	cmd.RunE = func(cmd *cobra.Command, args []string) error { return runTspList(cmd, flags, dbPath, sortBy) }
	cmd.AddCommand(newTspListCmd(flags))
	cmd.AddCommand(newTspShowCmd(flags))
	return cmd
}

func newTspListCmd(flags *rootFlags) *cobra.Command {
	var dbPath, sortBy string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the Trading System Project sections (with their last-updated dates)",
		Example:     strings.Trim("\n  seykota-pp-cli tsp list\n  seykota-pp-cli tsp list --sort updated\n", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTspList(cmd, flags, dbPath, sortBy)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().StringVar(&sortBy, "sort", "doc", "Sort order: doc (document order) | slug | updated")
	return cmd
}

type tspSectionView struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Updated string `json:"updated,omitempty"`
	URL     string `json:"url"`
	Chars   int    `json:"chars"`
}

func runTspList(cmd *cobra.Command, flags *rootFlags, dbPath, sortBy string) error {
	s, err := openCorpus(cmd.Context(), dbPath)
	if err != nil {
		return err
	}
	defer s.Close()
	docs, err := s.ListDocs("tsp")
	if err != nil {
		return err
	}
	views := make([]tspSectionView, 0, len(docs))
	for _, d := range docs {
		views = append(views, tspSectionView{Slug: d.Slug, Title: d.Title, Updated: d.Updated, URL: d.URL, Chars: len(d.Body)})
	}
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "slug":
		sort.Slice(views, func(i, j int) bool { return strings.ToLower(views[i].Slug) < strings.ToLower(views[j].Slug) })
	case "updated":
		sort.SliceStable(views, func(i, j int) bool { return views[i].Updated > views[j].Updated })
	case "", "doc":
		// keep document order
	default:
		return usageErr(fmt.Errorf("--sort must be doc, slug, or updated (got %q)", sortBy))
	}
	if wantsJSON(cmd, flags) {
		return emitJSON(cmd, flags, map[string]any{"count": len(views), "sections": views})
	}
	if len(views) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No TSP sections in the local archive. Run 'seykota-pp-cli index build'.")
		return nil
	}
	rows := make([][]string, 0, len(views))
	for _, v := range views {
		u := v.Updated
		if u == "" {
			u = "—"
		}
		rows = append(rows, []string{v.Slug, clip(v.Title, 36), u, v.URL})
	}
	_ = printRows(cmd, []string{"SLUG", "TITLE", "UPDATED", "URL"}, rows)
	fmt.Fprintf(cmd.OutOrStdout(), "\n%d section(s). Read one: seykota-pp-cli tsp show <slug>\n", len(views))
	return nil
}

func newTspShowCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxChars int
	cmd := &cobra.Command{
		Use:   "show [slug]",
		Short: "Print one Trading System Project section (its notes, rules and links)",
		Long: `Print the cleaned text of a TSP section from the local archive. Slug is the
section folder name (e.g. EA, SR, Trends, Diversify, Continuous,
Data_Verification, Skid, Core) — also matched against the section title.`,
		Example: strings.Trim("\n  seykota-pp-cli tsp show EA\n  seykota-pp-cli tsp show SR\n  seykota-pp-cli tsp show Continuous --max 5000\n", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			slug := strings.TrimSpace(args[0])
			s, err := openCorpus(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			d, err := s.TSPBySlug(slug)
			if err != nil {
				if strings.Contains(err.Error(), "no rows") {
					return notFoundErr(fmt.Errorf("no TSP section %q in the local archive — run 'seykota-pp-cli tsp list' for the available slugs", slug))
				}
				return err
			}
			body := d.Body
			if maxChars > 0 && len(body) > maxChars {
				body = body[:maxChars] + "\n…(truncated; use --max 0 for the full text)…"
			}
			if wantsJSON(cmd, flags) {
				return emitJSON(cmd, flags, map[string]any{
					"slug": d.Slug, "title": d.Title, "updated": d.Updated, "url": d.URL, "body": body,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n%s\n", d.Label(), d.URL)
			if d.Updated != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Updated: %s\n", d.Updated)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", body)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Archive DB path (default: the standard data dir)")
	cmd.Flags().IntVar(&maxChars, "max", 0, "Truncate the body to this many characters (0 = full text)")
	return cmd
}
