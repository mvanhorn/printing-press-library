package cli

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
)

type overviewFileTotals struct {
	files  int
	bytes  int64
	sample string
}

type overviewDevRootKey struct {
	path string
	kind string
}

// queryOverviewFast derives the wide-table summaries in one rowid-order pass.
// The old SQL queries each revisited the file table and fetched rows through
// several unrelated indexes, which is expensive on a large local index.
func queryOverviewFast(ctx context.Context, db *store.Store, top int) (overviewResult, error) {
	r := newOverviewResult()
	q := db.DB()
	conn, err := q.Conn(ctx)
	if err != nil {
		return r, err
	}
	defer conn.Close()
	// The mapping is virtual address space; SQLite and the OS page in only the
	// ranges read by this command. Keep the setting on this connection only.
	if _, err := conn.ExecContext(ctx, `PRAGMA mmap_size=4000000000`); err != nil {
		return r, err
	}
	rows, err := conn.QueryContext(ctx, `SELECT path_lower,COALESCE(NULLIF(path_display,''),path_lower),name,tag,COALESCE(size,0),COALESCE(client_modified,''),dev_kind,parent_lower FROM dbx_files`)
	if err != nil {
		return r, err
	}
	fileFolders := make(map[string]*overviewFileTotals)
	rootFolders := make(map[string]string)
	byType := make(map[string]*overviewFileTotals)
	byYear := make(map[string]*overviewFileTotals)
	devKinds := make(map[string]*overviewDevDir)
	devRoots := make(map[overviewDevRootKey]*overviewFileTotals)
	for rows.Next() {
		var path, display, modified string
		var name, tag, devKind, parent sql.NullString
		var size int64
		if err := rows.Scan(&path, &display, &name, &tag, &size, &modified, &devKind, &parent); err != nil {
			_ = rows.Close()
			return r, err
		}
		switch tag.String {
		case "file":
			r.Totals.Files++
			r.Totals.Bytes += size
			if folder := overviewRootSegment(path); folder != "" {
				bucket := overviewTotalsBucket(fileFolders, folder)
				bucket.files++
				bucket.bytes += size
				if bucket.sample == "" || display < bucket.sample {
					bucket.sample = display
				}
			}
			ext := overviewExtension(name.String)
			typeBucket := overviewTotalsBucket(byType, ext)
			typeBucket.files++
			typeBucket.bytes += size
			year := overviewModifiedYear(modified)
			yearBucket := overviewTotalsBucket(byYear, year)
			yearBucket.files++
			yearBucket.bytes += size
			if devKind.Valid {
				kind := overviewKindBucket(devKinds, devKind.String)
				kind.Files++
				kind.Bytes += size
				root := overviewDevRoot(path, devKind.String)
				rootBucket := overviewDevRootBucket(devRoots, overviewDevRootKey{root, devKind.String})
				rootBucket.files++
				rootBucket.bytes += size
			}
		case "folder":
			r.Totals.Folders++
			if parent.Valid && parent.String == "" {
				rootFolders[path] = display
			}
			if devKind.Valid {
				root := overviewDevRoot(path, devKind.String)
				if path == root {
					overviewKindBucket(devKinds, devKind.String).Folders++
					overviewDevRootBucket(devRoots, overviewDevRootKey{root, devKind.String})
				}
			}
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return r, err
	}

	folderKeys := make(map[string]bool, len(fileFolders)+len(rootFolders))
	for path := range fileFolders {
		folderKeys[path] = true
	}
	for path := range rootFolders {
		folderKeys[path] = true
	}
	for path := range folderKeys {
		bucket := fileFolders[path]
		item := overviewBucket{}
		if bucket != nil {
			item.Files, item.Bytes = bucket.files, bucket.bytes
			item.Name = overviewRootSegment(bucket.sample)
		}
		if display, ok := rootFolders[path]; ok {
			item.Name = display
		}
		r.TopFolders = append(r.TopFolders, item)
	}
	sort.Slice(r.TopFolders, func(i, j int) bool {
		if r.TopFolders[i].Bytes != r.TopFolders[j].Bytes {
			return r.TopFolders[i].Bytes > r.TopFolders[j].Bytes
		}
		return r.TopFolders[i].Name < r.TopFolders[j].Name
	})
	if len(r.TopFolders) > top {
		r.TopFolders = r.TopFolders[:top]
	}
	for ext, bucket := range byType {
		r.ByType = append(r.ByType, overviewBucket{ext, bucket.files, bucket.bytes})
	}
	sort.Slice(r.ByType, func(i, j int) bool {
		if r.ByType[i].Bytes != r.ByType[j].Bytes {
			return r.ByType[i].Bytes > r.ByType[j].Bytes
		}
		return r.ByType[i].Name < r.ByType[j].Name
	})
	if len(r.ByType) > top {
		r.ByType = r.ByType[:top]
	}
	for year, bucket := range byYear {
		r.ByYear = append(r.ByYear, overviewYear{year, bucket.files, bucket.bytes})
	}
	sort.Slice(r.ByYear, func(i, j int) bool { return r.ByYear[i].Year < r.ByYear[j].Year })
	for _, kind := range devKinds {
		r.DevDirs = append(r.DevDirs, *kind)
	}
	sort.Slice(r.DevDirs, func(i, j int) bool {
		if r.DevDirs[i].Bytes != r.DevDirs[j].Bytes {
			return r.DevDirs[i].Bytes > r.DevDirs[j].Bytes
		}
		return r.DevDirs[i].Kind < r.DevDirs[j].Kind
	})
	devKeys := make([]overviewDevRootKey, 0, len(devRoots))
	for key := range devRoots {
		devKeys = append(devKeys, key)
	}
	sort.Slice(devKeys, func(i, j int) bool {
		a, b := devRoots[devKeys[i]], devRoots[devKeys[j]]
		if a.bytes != b.bytes {
			return a.bytes > b.bytes
		}
		if devKeys[i].path != devKeys[j].path {
			return devKeys[i].path < devKeys[j].path
		}
		return devKeys[i].kind < devKeys[j].kind
	})
	for _, key := range devKeys[:min(10, len(devKeys))] {
		bucket := devRoots[key]
		path := key.path
		err := q.QueryRowContext(ctx, `SELECT COALESCE(NULLIF(path_display,''),path_lower) FROM dbx_files WHERE path_lower=?`, key.path).Scan(&path)
		if err != nil && err != sql.ErrNoRows {
			return r, err
		}
		r.TopDevFolders = append(r.TopDevFolders, overviewDevFolder{path, key.kind, bucket.bytes, bucket.files})
	}
	rows, err = q.QueryContext(ctx, `SELECT COALESCE(NULLIF(path_display,''),path_lower),COALESCE(size,0) FROM dbx_files WHERE tag='file' ORDER BY size DESC,path_display ASC LIMIT 10`)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var item overviewLargeFile
		if err := rows.Scan(&item.PathDisplay, &item.Size); err != nil {
			_ = rows.Close()
			return r, err
		}
		r.LargestFiles = append(r.LargestFiles, item)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return r, err
	}
	candidate, args := duplicateCandidateSQLWithIndex("", 1, false, "dbx_files_visible_duplicates")
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(copies),0),COALESCE(SUM((copies-1)*size),0) FROM (`+candidate+`)`, args...).Scan(&r.Duplicates.Groups, &r.Duplicates.Files, &r.Duplicates.ReclaimableBytes); err != nil {
		return r, err
	}
	rows, err = q.QueryContext(ctx, `SELECT name FROM dbx_files WHERE dev_kind IS NULL AND conflict_candidate=1`)
	if err != nil {
		return r, err
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return r, err
		}
		if _, _, _, ok := dropbox.ConflictedCopy(name); ok {
			r.ConflictedCopies++
		}
	}
	err = rows.Err()
	_ = rows.Close()
	return r, err
}

func overviewTotalsBucket(buckets map[string]*overviewFileTotals, key string) *overviewFileTotals {
	if buckets[key] == nil {
		buckets[key] = &overviewFileTotals{}
	}
	return buckets[key]
}

func overviewKindBucket(buckets map[string]*overviewDevDir, key string) *overviewDevDir {
	if buckets[key] == nil {
		buckets[key] = &overviewDevDir{Kind: key}
	}
	return buckets[key]
}

func overviewDevRootBucket(buckets map[overviewDevRootKey]*overviewFileTotals, key overviewDevRootKey) *overviewFileTotals {
	if buckets[key] == nil {
		buckets[key] = &overviewFileTotals{}
	}
	return buckets[key]
}

func overviewRootSegment(path string) string {
	if len(path) < 2 {
		return ""
	}
	i := strings.IndexByte(path[1:], '/')
	if i < 0 {
		return ""
	}
	return path[:i+1]
}

func overviewExtension(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i < 0 {
		return "(none)"
	}
	return overviewASCIILower(name[i:])
}

func overviewModifiedYear(modified string) string {
	if len(modified) >= 4 {
		for i := 0; i < 4; i++ {
			if modified[i] < '0' || modified[i] > '9' {
				return "(unknown)"
			}
		}
		return modified[:4]
	}
	return "(unknown)"
}

func overviewDevRoot(path, kind string) string {
	needle := "/" + overviewASCIILower(kind) + "/"
	i := strings.Index(path+"/", needle)
	length := i + 1 + len(kind)
	if i < 0 {
		length = len(kind)
	}
	if length < 0 {
		length = 0
	}
	if length > len(path) {
		length = len(path)
	}
	return path[:length]
}

func overviewASCIILower(v string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}, v)
}
