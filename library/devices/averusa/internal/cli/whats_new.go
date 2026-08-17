// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: whats-new — docs/products added or updated since the last
// harvest, filterable by age. averusa.com has no changelog, so only the local
// corpus diff can answer "what changed since I last looked".
// pp:data-source local
// Local-store command: reads the synced corpus only.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelWhatsNewCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var flagSince string
	var limit int

	cmd := &cobra.Command{
		Use:   "whats-new",
		Short: "List documents and products added or updated since the last sync, filterable by age.",
		Long: strings.Trim(`
List documents and products that were added or updated since the cutoff
(default: 7 days). averusa.com publishes no changelog, so this diff is computed
against the local corpus's per-row update timestamps recorded at harvest time.
`, "\n"),
		Example: strings.Trim(`
  averusa-pp-cli whats-new
  averusa-pp-cli whats-new --since 30d --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--since=30d",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "whats-new")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("--data-source live has no live equivalent for whats-new; it diffs the local corpus after `harvest`"))
			}
			cutoff, err := parseSince(flagSince)
			if err != nil {
				return usageErr(err)
			}
			if limit <= 0 {
				limit = 100
			}
			cutoffTime := time.Now().UTC().Add(-cutoff).Format(time.RFC3339)

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				return notFoundErr(fmt.Errorf("no corpus to diff"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			docs, prods, err := st.WhatChangedAVERUSA(cutoffTime, limit)
			if err != nil {
				return err
			}
			if len(docs) == 0 && len(prods) == 0 {
				return notFoundErr(fmt.Errorf("nothing new or updated in the last %s", cutoff))
			}
			rep := struct {
				Since     string `json:"since"`
				Documents []struct {
					Title   string `json:"title"`
					DocType string `json:"doc_type"`
					Model   string `json:"model,omitempty"`
					URLName string `json:"url_name"`
				} `json:"documents"`
				Products []struct {
					Name       string `json:"name"`
					Slug       string `json:"slug"`
					Category   string `json:"category"`
					Discontinued bool `json:"discontinued"`
				} `json:"products"`
			}{Since: formatSince(cutoff)}
			for _, d := range docs {
				rep.Documents = append(rep.Documents, struct {
					Title   string `json:"title"`
					DocType string `json:"doc_type"`
					Model   string `json:"model,omitempty"`
					URLName string `json:"url_name"`
				}{d.Title, d.DocType, d.Model, d.URLName})
			}
			for _, p := range prods {
				rep.Products = append(rep.Products, struct {
					Name       string `json:"name"`
					Slug       string `json:"slug"`
					Category   string `json:"category"`
					Discontinued bool `json:"discontinued"`
				}{p.Name, p.Slug, p.Category, p.Discontinued})
			}
			if flags.asJSON {
				return flags.printJSON(cmd, rep)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "New or updated in the last %s:\n", cutoff)
			for _, d := range rep.Documents {
				fmt.Fprintf(w, "  doc   %s [%s] %s\n", d.Title, d.DocType, d.URLName)
			}
			for _, p := range rep.Products {
				fmt.Fprintf(w, "  prod  %s (%s)%s\n", p.Name, p.Slug, map[bool]string{true: " [discontinued]", false: ""}[p.Discontinued])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "corpus database path (default: CLI state dir)")
	cmd.Flags().StringVar(&flagSince, "since", "", "look back this far (e.g. 30d, 24h, 1w; default 7d)")
	cmd.Flags().IntVar(&limit, "limit", 0, "max rows per section (default 100)")
	return cmd
}

// parseSince accepts Go durations plus day/week suffixes (30d, 1w).
func parseSince(s string) (time.Duration, error) {
	if strings.TrimSpace(s) == "" {
		return 7 * 24 * time.Hour, nil
	}
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(strings.TrimSuffix(s, "d")); err == nil && strings.HasSuffix(s, "d") {
		return time.Duration(n) * 24 * time.Hour, nil
	}
	if n, err := strconv.Atoi(strings.TrimSuffix(s, "w")); err == nil && strings.HasSuffix(s, "w") {
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid --since %q: use forms like 30d, 1w, 24h", s)
	}
	return d, nil
}

// formatSince renders a duration compactly (24h, 7d, 30d).
func formatSince(d time.Duration) string {
	if d%(7*24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	}
	if d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	}
	return d.String()
}
