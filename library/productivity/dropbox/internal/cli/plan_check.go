// pp:data-source local
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
	"github.com/spf13/cobra"
)

type indexSnapshot struct {
	entries map[string]dropbox.SnapshotEntry
	links   map[string]string
	roots   map[string]bool
	db      *store.Store
	ctx     context.Context
}

func (s indexSnapshot) RootComplete(p string) bool {
	if complete, ok := s.roots[strings.ToLower(strings.TrimRight(p, "/"))]; ok {
		return complete
	}
	return s.roots[dropbox.IndexRoot(p)]
}

func (s indexSnapshot) Entries() map[string]dropbox.SnapshotEntry {
	return s.entries
}
func (s indexSnapshot) LinkPath(url string) (string, bool) {
	p, ok := s.links[url]
	return p, ok
}
func (s indexSnapshot) HasDevDescendant(p string) (bool, error) {
	lower, upper := descendantRange(strings.ToLower(p))
	var found int
	err := s.db.DB().QueryRowContext(s.ctx, `SELECT 1 FROM dbx_files INDEXED BY dbx_files_dev_path WHERE path_lower>=? AND path_lower<? AND dev_kind IS NOT NULL LIMIT 1`, lower, upper).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func loadIndexSnapshot(ctx context.Context, db *store.Store) (indexSnapshot, error) {
	s := indexSnapshot{entries: map[string]dropbox.SnapshotEntry{}, links: map[string]string{}, db: db, ctx: ctx}
	rows, err := db.DB().QueryContext(ctx, `SELECT path_lower,COALESCE(tag,''),COALESCE(rev,''),COALESCE(id,''),COALESCE(content_hash,''),COALESCE(shared_folder_id,''),COALESCE(parent_shared_folder_id,''),COALESCE(dev_kind,''),COALESCE(size,0) FROM dbx_files`)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var p string
		var e dropbox.SnapshotEntry
		if err := rows.Scan(&p, &e.Tag, &e.Rev, &e.EntryID, &e.ContentHash, &e.SharedFolderID, &e.ParentSharedFolderID, &e.DevKind, &e.Size); err != nil {
			_ = rows.Close()
			return s, err
		}
		s.entries[p] = e
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return s, err
	}
	rows, err = db.DB().QueryContext(ctx, `SELECT url,COALESCE(path_lower,'') FROM dbx_shared_links`)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var u, p string
		if err := rows.Scan(&u, &p); err != nil {
			_ = rows.Close()
			return s, err
		}
		s.links[u] = p
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return s, err
	}
	s.roots, err = completeIndexRoots(ctx, db.DB())
	return s, err
}

func checkPlanAtIndex(ctx context.Context, db *store.Store, p dropbox.Plan, opts dropbox.CheckOptions) (dropbox.CheckReport, error) {
	s, err := loadIndexSnapshot(ctx, db)
	if err != nil {
		return dropbox.CheckReport{}, err
	}
	return dropbox.CheckPlan(p, s, opts), nil
}

func newNovelPlanCheckCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxOps int
	var allowNonemptyDelete, allowUnshare bool
	cmd := &cobra.Command{Use: "check <file>", Short: "Validate a plan against the local index", Long: "Validate source revisions, destinations, parent folders, share boundaries, and restore risks. Exit codes: 0 plan is valid, 2 plan has errors.", Example: strings.Trim(`
  dropbox-pp-cli plan check photos.json --agent
  dropbox-pp-cli plan check taxes.json --db ./dropbox.db --json`, "\n"), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2", "pp:happy-args": "file=photos.json"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "plan check")
		}
		if len(args) != 1 {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("plan check requires one file"))
		}
		if err := localSourceOnly(flags); err != nil {
			return err
		}
		if maxOps <= 0 {
			return usageErr(fmt.Errorf("--max-ops must be positive"))
		}
		p, err := dropbox.ReadPlan(args[0])
		if err != nil {
			return usageErr(err)
		}
		db, found, err := openIndex(cmd, flags, dbPath)
		if err != nil {
			return err
		}
		if !found {
			report := dropbox.CheckReport{OK: false, Results: []dropbox.CheckResult{{Seq: 0, Op: "plan", Status: "error", Code: "index_missing", Message: "no local index; run: dropbox-pp-cli index"}}, Errors: 1}
			if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
				return err
			}
			return usageErr(fmt.Errorf("no local index"))
		}
		defer db.Close()
		report, err := checkPlanAtIndex(cmd.Context(), db, p, dropbox.CheckOptions{MaxOps: maxOps, AllowNonemptyDelete: allowNonemptyDelete, AllowUnshare: allowUnshare})
		if err != nil {
			return err
		}
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%d errors, %d warnings\n", report.Errors, report.Warnings)
			for _, r := range report.Results {
				if r.Status != "ok" {
					fmt.Fprintf(cmd.OutOrStdout(), "%d %s: %s\n", r.Seq, r.Code, r.Message)
				}
			}
		}
		if !report.OK {
			return usageErr(fmt.Errorf("plan has %d errors", report.Errors))
		}
		return nil
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().IntVar(&maxOps, "max-ops", 5000, "Maximum operations in a plan")
	cmd.Flags().BoolVar(&allowNonemptyDelete, "allow-nonempty-delete", false, "Allow deleting folders with indexed file descendants")
	cmd.Flags().BoolVar(&allowUnshare, "allow-unshare", false, "Allow deleting shared folders")
	return cmd
}
