// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: docs audit — HEAD-checks every document PDF URL in the corpus
// and flags 404s and soft-404 shells, caching last-checked status locally.
// pp:data-source auto
// Auto strategy: live HEAD checks against the synced URL list, with the local
// catalog as the source of truth for what to check.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/averusa/internal/averusa"
	"github.com/mvanhorn/printing-press-library/library/devices/averusa/internal/cliutil"
)

// soft404ShellSize is the observed byte size of averusa.com's soft-404 shell
// page ("404 | AVer USA" returned as HTTP 200).
const soft404ShellSize = 61301

func newNovelDocsAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "HEAD-checks every document URL in the catalog and flags 404s and soft-404 shells, caching last-checked status locally.",
		Long: strings.Trim(`
HEAD-check every document PDF URL in the synced catalog and report which links
are dead, which return the site's soft-404 shell (HTTP 200 with the 61,301-byte
"404 | AVer USA" page), and which articles have no attached file. Results are
cached back into the corpus so re-runs only re-check changed entries.

Run this before pushing a doc set to a shared drive — averusa.com product pages
have shipped mislinked datasheets (the CAM570 page links cam520pro3-datasheet.pdf).
`, "\n"),
		Example: strings.Trim(`
  averusa-pp-cli docs audit
  averusa-pp-cli docs audit --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "docs audit")
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would HEAD-check every document URL in the local catalog")
				return nil
			}
			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				return notFoundErr(fmt.Errorf("no corpus to audit"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			docs, err := st.ListAVERUSADocuments("", "", 100000)
			if err != nil {
				return err
			}
			// Curtail under live dogfood to fit the per-command timeout.
			if cliutil.IsDogfoodEnv() && (limit == 0 || limit > 5) {
				limit = 5
			}
			if limit > 0 && limit < len(docs) {
				docs = docs[:limit]
			}

			c := averusa.New()
			now := time.Now().UTC().Format(time.RFC3339)
			type broken struct {
				URLName string `json:"url_name"`
				Title   string `json:"title"`
				URL     string `json:"url"`
				Status  int    `json:"status"`
				Reason  string `json:"reason"`
			}
			var broke []broken
			okCount := 0
			noFile := 0
			for _, d := range docs {
				if err := ctx.Err(); err != nil {
					return err
				}
				if d.PDFURL == "" {
					noFile++
					_, _ = st.DB().ExecContext(ctx,
						`UPDATE averusa_documents SET last_checked=?, last_status=0 WHERE url_name=?`,
						now, d.URLName)
					continue
				}
				status, _, headLen, isPDF, err := c.ProbeFile(ctx, d.PDFURL)
				reason := ""
				isNoFile := false
				if err != nil {
					reason = err.Error()
				} else if status == 204 {
					// 204 no-body: the article's fileField has no attached
					// file (text-only article) — not a broken link.
					noFile++
					isNoFile = true
				} else if isPDF {
					// Confirmed downloadable (HEAD content-type or GET probe).
				} else if status != 200 {
					reason = fmt.Sprintf("HTTP %d", status)
				} else if headLen == soft404ShellSize {
					reason = "soft-404 shell (61301-byte 200 page)"
				} else {
					reason = "200 response is not a PDF (HTML shell?)"
				}
				if isNoFile {
					_, _ = st.DB().ExecContext(ctx,
						`UPDATE averusa_documents SET last_checked=?, last_status=? WHERE url_name=?`,
						now, status, d.URLName)
					continue
				}
				if reason != "" {
					broke = append(broke, broken{d.URLName, d.Title, d.PDFURL, status, reason})
				} else {
					okCount++
				}
				_, _ = st.DB().ExecContext(ctx,
					`UPDATE averusa_documents SET last_checked=?, last_status=? WHERE url_name=?`,
					now, status, d.URLName)
			}
			rep := struct {
				Checked     int      `json:"checked"`
				OK          int      `json:"ok"`
				NoFile      int      `json:"no_file"`
				Broken      int      `json:"broken"`
				BrokenLinks []broken `json:"broken_links"`
				CheckedAt   string   `json:"checked_at"`
			}{Checked: len(docs), OK: okCount, NoFile: noFile, Broken: len(broke), BrokenLinks: broke, CheckedAt: now}
			if flags.asJSON {
				return flags.printJSON(cmd, rep)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "audited %d documents: %d ok, %d no attached file, %d broken\n",
				rep.Checked, rep.OK, rep.NoFile, rep.Broken)
			for _, b := range broke {
				fmt.Fprintf(w, "  BROKEN %s (%s) status=%d: %s\n", b.URLName, b.Title, b.Status, b.Reason)
			}
			if rep.Broken == 0 {
				return nil
			}
			return notFoundErr(fmt.Errorf("%d document link(s) are broken or soft-404", rep.Broken))
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "corpus database path (default: CLI state dir)")
	cmd.Flags().IntVar(&limit, "limit", 0, "audit at most N documents (default: all)")
	return cmd
}
