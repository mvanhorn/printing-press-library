// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto
// Reads the local panel and, unless --offline is given, also consults the live
// index so a stale local copy cannot understate what is missing upstream.

package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/pbs/internal/pbsfetch"
)

type revisionRow struct {
	AsOf      string `json:"as_of"`
	Kind      string `json:"kind"`
	Role      string `json:"role"`
	Finding   string `json:"finding"`
	StoredAt  string `json:"stored_url,omitempty"`
	LiveAt    string `json:"live_url,omitempty"`
	StoredSHA string `json:"stored_sha256,omitempty"`
	LiveSHA   string `json:"live_sha256,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

type revisionsEnvelope struct {
	Checked    int                 `json:"files_checked"`
	Findings   []revisionRow       `json:"findings"`
	Unchanged  int                 `json:"unchanged"`
	Collisions map[string][]string `json:"upstream_url_collisions,omitempty"`
	Note       string              `json:"note,omitempty"`
}

func newNovelRevisionsCmd(flags *rootFlags) *cobra.Command {
	var (
		asOf     string
		all      bool
		since    string
		maxFiles int
		dbPath   string
	)
	cmd := &cobra.Command{
		Use:   "revisions",
		Short: "Detect releases the Bureau has rewritten, re-pointed or deleted",
		Long: strings.Trim(`
Compare what is stored locally against what the Bureau serves now.

Use this command to detect what the Bureau has CHANGED, REWRITTEN or DELETED
upstream since capture. Do NOT use it to audit parse correctness; use 'verify'
instead. Do NOT use it to fetch new releases; use 'sync' instead.

This is only meaningful because the local store accrues what the site
overwrites. The live index reaches further back than the Internet Archive does,
the Bureau has already 404'd its entire pre-2023 tree in a migration, and one
monthly annex is served under two different months so one of them is lost.

Four findings are distinguished:
  deleted        a file we hold now returns 404
  rewritten      the same filename now serves different bytes
  re-pointed     the index now points that release at a different filename
  collision      two releases upstream now share one file, so one is lost
`, "\n"),
		Example: strings.Trim(`
  pbs-pp-cli revisions
  pbs-pp-cli revisions --all --agent
  pbs-pp-cli revisions --since 2026-08-01 --json
`, "\n"),
		Annotations: map[string]string{
			// Read-only against upstream and the store: it compares, never writes.
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "revisions")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Live dogfood runs every command inside a flat per-command
			// timeout. Curtail the number of files re-fetched rather than
			// skipping the network: this is a read path and must stay real.
			if cliutil.IsDogfoodEnv() {
				all = false
				if maxFiles == 0 || maxFiles > 2 {
					maxFiles = 2
				}
			}
			if maxFiles == 0 {
				maxFiles = 12
			}

			dbPath = panelDBPath(dbPath)
			db, ok, err := openPanelForRead(ctx, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				noMirror(cmd.ErrOrStderr(), dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), revisionsEnvelope{
						Findings: []revisionRow{}, Note: "no local panel yet"}, flags)
				}
				return nil
			}
			defer db.Close()
			out := revisionsEnvelope{Findings: []revisionRow{}, Collisions: map[string][]string{}}

			fc := pbsfetch.New(flags.timeout, 2)
			idx, _, err := fetchIndex(ctx, fc)
			if err != nil {
				return classifyAPIErrorOnly(err)
			}
			// An upstream collision means two releases now share one file, so
			// one release's data is unrecoverable. Reported whether or not we
			// hold either release.
			for u, keys := range idx.DuplicateURLs {
				out.Collisions[u] = keys
				out.Findings = append(out.Findings, revisionRow{
					Finding: "collision", LiveAt: u,
					Detail: "two releases upstream share this file, so one of them is lost: " + strings.Join(keys, ", "),
				})
			}

			// A release can list BOTH a .pdf and an .xlsx in the same role, so
			// the live side is a SET of URLs per (release, kind, role). Comparing
			// against one arbitrarily chosen member reported a spurious
			// "re-pointed" finding on every release.
			liveSet := map[string]map[string]bool{}
			for _, r := range idx.All() {
				for _, f := range r.Files {
					k := r.AsOfKey() + "|" + string(r.Kind) + "|" + string(f.Role)
					if liveSet[k] == nil {
						liveSet[k] = map[string]bool{}
					}
					liveSet[k][f.URL] = true
				}
			}

			// Candidates: what we hold, newest first.
			// The url recorded on the coverage row is the one actually fetched.
			// Joining pbs_release_file instead would match several rows per role.
			q := `SELECT as_of, kind, role, COALESCE(sha256,''), COALESCE(url,'')
			      FROM pbs_coverage
			      WHERE state = ?`
			qa := []any{covFetched}
			if asOf != "" {
				q += ` AND as_of = ?`
				qa = append(qa, asOf)
			}
			if since != "" {
				q += ` AND as_of >= ?`
				qa = append(qa, since)
			}
			q += ` ORDER BY as_of DESC`
			rows, err := db.DB().QueryContext(ctx, q, qa...)
			if err != nil {
				return fmt.Errorf("query stored files: %w", err)
			}
			type cand struct{ asOf, kind, role, sha, url string }
			var cands []cand
			for rows.Next() {
				var c cand
				if err := rows.Scan(&c.asOf, &c.kind, &c.role, &c.sha, &c.url); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan stored file: %w", err)
				}
				cands = append(cands, c)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			if err := rows.Close(); err != nil {
				return err
			}
			if !all && len(cands) > maxFiles {
				cands = cands[:maxFiles]
				out.Note = fmt.Sprintf("checked the %d most recent stored files; pass --all to check every one", maxFiles)
			}

			for _, c := range cands {
				key := c.asOf + "|" + c.kind + "|" + c.role
				set := liveSet[key]
				if len(set) == 0 {
					out.Findings = append(out.Findings, revisionRow{
						AsOf: c.asOf, Kind: c.kind, Role: c.role, Finding: "re-pointed",
						StoredAt: c.url, Detail: "the index no longer lists any file in this role for this release",
					})
					continue
				}
				if c.url != "" && !set[c.url] {
					var live []string
					for u := range set {
						live = append(live, u)
					}
					sort.Strings(live)
					out.Findings = append(out.Findings, revisionRow{
						AsOf: c.asOf, Kind: c.kind, Role: c.role, Finding: "re-pointed",
						StoredAt: c.url, LiveAt: strings.Join(live, " "),
						Detail: "the file we fetched is no longer listed for this release",
					})
					continue
				}
				if c.sha == "" || c.url == "" {
					continue
				}
				res, ferr := fc.Get(ctx, c.url)
				out.Checked++
				switch {
				case errors.Is(ferr, pbsfetch.ErrNotFound):
					out.Findings = append(out.Findings, revisionRow{
						AsOf: c.asOf, Kind: c.kind, Role: c.role, Finding: "deleted",
						StoredAt: c.url, StoredSHA: c.sha,
						Detail: "a file we hold now returns 404; the local copy is the surviving record",
					})
				case ferr != nil:
					out.Findings = append(out.Findings, revisionRow{
						AsOf: c.asOf, Kind: c.kind, Role: c.role, Finding: "unreachable",
						StoredAt: c.url, Detail: ferr.Error(),
					})
				case res.SHA256 != c.sha:
					out.Findings = append(out.Findings, revisionRow{
						AsOf: c.asOf, Kind: c.kind, Role: c.role, Finding: "rewritten",
						StoredAt: c.url, LiveAt: c.url,
						StoredSHA: c.sha[:12], LiveSHA: res.SHA256[:12],
						Detail: "same filename, different bytes: the Bureau restated this release in place",
					})
				default:
					out.Unchanged++
				}
			}

			sort.SliceStable(out.Findings, func(i, j int) bool {
				if out.Findings[i].AsOf != out.Findings[j].AsOf {
					return out.Findings[i].AsOf > out.Findings[j].AsOf
				}
				return out.Findings[i].Finding < out.Findings[j].Finding
			})

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "re-checked %d stored files; %d unchanged\n\n", out.Checked, out.Unchanged)
			if len(out.Findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No upstream revisions, deletions or collisions found.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "AS OF\tROLE\tFINDING\tDETAIL")
			for _, f := range out.Findings {
				as := f.AsOf
				if as == "" {
					as = "-"
				}
				role := f.Role
				if role == "" {
					role = "-"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", as, role, f.Finding, truncate(f.Detail, 78))
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if out.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nnote: %s\n", out.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&asOf, "as-of", "", "Check one release, YYYY-MM-DD")
	cmd.Flags().BoolVar(&all, "all", false, "Check every stored file instead of the most recent")
	cmd.Flags().StringVar(&since, "since", "", "Check only releases on or after this date, YYYY-MM-DD")
	cmd.Flags().IntVar(&maxFiles, "max-files", 0, "Maximum stored files to re-fetch this run (default 12)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local panel database path")
	return cmd
}

var _ = context.Background
var _ = sql.ErrNoRows
