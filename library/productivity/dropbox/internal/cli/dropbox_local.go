package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
	"github.com/spf13/cobra"
)

const missingIndexNote = "no local index: run `dropbox-pp-cli index`"

// openIndex returns found=false for a missing database without creating it.
func openIndex(cmd *cobra.Command, flags *rootFlags, dbPath string) (*store.Store, bool, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("dropbox-pp-cli")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local index at %s\nrun: dropbox-pp-cli index\n", dbPath)
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, false, err
	}
	if err := db.EnsureDropboxSchema(cmd.Context()); err != nil {
		_ = db.Close()
		return nil, false, err
	}
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT COALESCE(last_full_at,''), COALESCE(last_incremental_at,''), COALESCE(complete,0) FROM dbx_index_state`)
	if err != nil {
		_ = db.Close()
		return nil, false, err
	}
	incomplete := false
	unknownAge := false
	oldestHours := 0
	for rows.Next() {
		var full, incremental string
		var complete bool
		if err := rows.Scan(&full, &incremental, &complete); err != nil {
			_ = rows.Close()
			_ = db.Close()
			return nil, false, err
		}
		if !complete {
			incomplete = true
		}
		stamp := incremental
		if stamp == "" {
			stamp = full
		}
		at, err := time.Parse(time.RFC3339, stamp)
		if err != nil {
			unknownAge = true
			continue
		}
		hours := int(time.Since(at).Hours())
		if hours > oldestHours {
			oldestHours = hours
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		_ = db.Close()
		return nil, false, err
	}
	if incomplete {
		fmt.Fprintln(cmd.ErrOrStderr(), "index is incomplete; run: dropbox-pp-cli index")
	} else if unknownAge {
		fmt.Fprintln(cmd.ErrOrStderr(), "index age is unknown; run: dropbox-pp-cli index")
	} else if oldestHours > 24 {
		fmt.Fprintf(cmd.ErrOrStderr(), "index is %d hours old; run: dropbox-pp-cli index\n", oldestHours)
	}
	return db, true, nil
}

const dropboxRowColumns = `path_lower, COALESCE(id,''), tag, name, COALESCE(path_display,''), COALESCE(parent_lower,''), COALESCE(rev,''), COALESCE(size,0), COALESCE(content_hash,''), COALESCE(client_modified,''), COALESCE(server_modified,''), COALESCE(shared_folder_id,''), COALESCE(parent_shared_folder_id,''), COALESCE(is_downloadable,0), COALESCE(dev_kind,'')`

func scanDropboxRows(rows *sql.Rows) ([]store.DropboxRow, error) {
	out := make([]store.DropboxRow, 0)
	for rows.Next() {
		row, err := scanDropboxRow(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, row)
	}
	err := rows.Err()
	_ = rows.Close()
	return out, err
}

// scanDropboxRow reads one row selected with dropboxRowColumns.
func scanDropboxRow(rows *sql.Rows) (store.DropboxRow, error) {
	var row store.DropboxRow
	if err := rows.Scan(&row.PathLower, &row.ID, &row.Tag, &row.Name, &row.PathDisplay, &row.ParentLower, &row.Rev, &row.Size, &row.ContentHash, &row.ClientModified, &row.ServerModified, &row.SharedFolderID, &row.ParentSharedFolderID, &row.IsDownloadable, &row.DevKind); err != nil {
		return row, err
	}
	if row.PathDisplay == "" {
		row.PathDisplay = row.PathLower
	}
	return row, nil
}

// descendantRange is a half-open range over path_lower. The slash prevents
// sibling prefixes (such as /docs2) from matching /docs.
func descendantRange(p string) (string, string) {
	return p + "/", p + "0"
}

// duplicateCandidateSQL is shared by dupes and overview so their headlines
// use identical eligibility and path-scope rules.
func duplicateCandidateSQL(under string, minSize int64, includeDevDirs bool) (string, []any) {
	return duplicateCandidateSQLWithIndex(under, minSize, includeDevDirs, "dbx_files_duplicate_candidates")
}

func duplicateCandidateSQLWithIndex(under string, minSize int64, includeDevDirs bool, index string) (string, []any) {
	query := `SELECT content_hash,size,COUNT(*) AS copies FROM dbx_files INDEXED BY ` + index + ` WHERE tag='file' AND content_hash<>'' AND size>=?`
	args := []any{minSize}
	if !includeDevDirs {
		query += ` AND dev_kind IS NULL`
	}
	if under != "" {
		lower, upper := descendantRange(under)
		query += ` AND (path_lower=? OR (path_lower>=? AND path_lower<?))`
		args = append(args, under, lower, upper)
	}
	return query + ` GROUP BY content_hash,size HAVING COUNT(*)>1`, args
}

type excludedDevDirs struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

func countExcludedDevDirs(ctx context.Context, db *store.Store, where string, args ...any) (excludedDevDirs, error) {
	var result excludedDevDirs
	table := `dbx_files`
	if where == "" {
		table += ` INDEXED BY dbx_files_dev_file_size`
	}
	err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(size),0) FROM `+table+` WHERE tag='file' AND dev_kind IS NOT NULL`+where, args...).Scan(&result.Files, &result.Bytes)
	return result, err
}

func localSourceOnly(flags *rootFlags) error {
	if flags.dataSource != "" && flags.dataSource != "auto" && flags.dataSource != "local" {
		return usageErr(fmt.Errorf("command requires local data source"))
	}
	return nil
}

func liveSourceOnly(flags *rootFlags) error {
	if flags.dataSource != "" && flags.dataSource != "auto" && flags.dataSource != "live" {
		return usageErr(fmt.Errorf("command requires live data source"))
	}
	return nil
}

func completeIndexRoots(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT root,COALESCE(complete,0) FROM dbx_index_state`)
	if err != nil {
		return nil, err
	}
	roots := make(map[string]bool)
	for rows.Next() {
		var root string
		var complete bool
		if err := rows.Scan(&root, &complete); err != nil {
			_ = rows.Close()
			return nil, err
		}
		roots[root] = complete
	}
	err = rows.Err()
	_ = rows.Close()
	return roots, err
}

func indexRootForEntry(path, tag string) string {
	root := dropbox.IndexRoot(path)
	if root == "" && tag == "folder" {
		return strings.ToLower(path)
	}
	return root
}
