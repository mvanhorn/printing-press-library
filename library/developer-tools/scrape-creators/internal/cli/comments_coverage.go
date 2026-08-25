// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: reply-gap audit over the synced comment corpus.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/store"
)

type coverageRow struct {
	PostURL     string  `json:"post_url"`
	Handle      string  `json:"handle,omitempty"`
	Reported    int64   `json:"reported_comment_count"`
	Stored      int64   `json:"stored_rows"`
	Gap         int64   `json:"gap"`
	CoveragePct float64 `json:"coverage_pct"`
}

func newNovelCommentsCoverageCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "coverage [handle]",
		Short: "Rank synced posts by missing-thread gap (reported vs stored comments)",
		Long: strings.Trim(`
Use this command to find synced posts with incomplete comment threads: it joins
the API-reported comment counts captured at sync time against the rows actually
stored locally, which is ground truth where the API's child_comment_count is
measurably unreliable. Do NOT use it to fetch the missing replies; use
'comments thread' on the flagged posts.`, "\n"),
		Example: strings.Trim(`
  scrape-creators-pp-cli comments coverage --agent
  scrape-creators-pp-cli comments coverage bracken.design --limit 10`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would audit synced posts for missing comment threads")
				return nil
			}
			if flags.dataSource == "live" {
				return fmt.Errorf("no live equivalent for this command (coverage audits the local corpus); use --data-source local or auto")
			}
			handle := ""
			if len(args) > 0 {
				handle = args[0]
			}

			if dbPath == "" {
				dbPath = defaultDBPath("scrape-creators-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: scrape-creators-pp-cli comments sweep <handle> --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			if err := store.EnsureCommentCorpus(ctx, db.DB()); err != nil {
				return fmt.Errorf("comment corpus migration: %w", err)
			}

			q := `
				SELECT m.post_url, m.handle, m.reported_comment_count,
				       (SELECT COUNT(*) FROM sc_comments c WHERE c.post_url = m.post_url) AS stored
				FROM sc_post_meta m`
			params := []any{}
			if handle != "" {
				q += ` WHERE m.handle = ?`
				params = append(params, handle)
			}
			q += ` ORDER BY (m.reported_comment_count - stored) DESC LIMIT ?`
			params = append(params, limit)

			rows, err := db.DB().QueryContext(ctx, q, params...)
			if err != nil {
				return fmt.Errorf("coverage query: %w", err)
			}
			out := make([]coverageRow, 0, limit)
			for rows.Next() {
				var r coverageRow
				if err := rows.Scan(&r.PostURL, &r.Handle, &r.Reported, &r.Stored); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan coverage row: %w", err)
				}
				r.Gap = r.Reported - r.Stored
				if r.Gap < 0 {
					r.Gap = 0
				}
				if r.Reported > 0 {
					r.CoveragePct = float64(r.Stored) / float64(r.Reported) * 100
					if r.CoveragePct > 100 {
						r.CoveragePct = 100
					}
				} else {
					r.CoveragePct = 100
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate coverage rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return err
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no post metadata in the local store yet; run comments sweep or comments thread first")
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum posts to report")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
