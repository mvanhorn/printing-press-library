// pp:data-source local
package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
	"github.com/spf13/cobra"
)

type messCategory[T any] struct {
	Count int `json:"count"`
	Items []T `json:"items"`
}
type messJunk struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}
type messDeep struct {
	Path  string `json:"path"`
	Depth int    `json:"depth"`
}
type messFolderGroup struct {
	Parent         string   `json:"parent"`
	NormalizedName string   `json:"normalized_name"`
	Folders        []string `json:"folders"`
}
type messResult struct {
	planOutput
	IndexMissing         bool                          `json:"index_missing,omitempty"`
	Note                 string                        `json:"note,omitempty"`
	EmptyFolders         messCategory[string]          `json:"empty_folders"`
	SingleFileFolders    messCategory[string]          `json:"single_file_folders"`
	RootFiles            messCategory[string]          `json:"root_files"`
	JunkNames            messCategory[messJunk]        `json:"junk_names"`
	DeepPaths            messCategory[messDeep]        `json:"deep_paths"`
	NearDuplicateFolders messCategory[messFolderGroup] `json:"near_duplicate_folders"`
	ExcludedDevDirs      excludedDevDirs               `json:"excluded_dev_dirs"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelMessCmd(flags))
	})
}

func newNovelMessCmd(flags *rootFlags) *cobra.Command {
	var dbPath, planPath string
	var limit, maxDepth int
	var includeDevDirs, force, printPlan bool
	cmd := &cobra.Command{
		Use: "mess", Short: "Find structural clutter in the local Dropbox index",
		Long: "Use this command for structural problems: empty or near-duplicate folders, loose root files, junk names. Do NOT use it for conflicted copies; use 'conflicts' instead. Do NOT use it for rule-based filing; use 'organize' instead.",
		Example: strings.Trim(`
  dropbox-pp-cli mess --agent
  dropbox-pp-cli mess --max-depth 6 --plan empty-folders.json --agent`, "\n"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:write-flags": "plan", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "mess")
			}
			if err := localSourceOnly(flags); err != nil {
				return err
			}
			if limit < 0 || maxDepth < 0 {
				return usageErr(fmt.Errorf("--limit and --max-depth must be nonnegative"))
			}
			db, found, err := openIndex(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			result := newMessResult()
			if !found {
				result.IndexMissing = true
				result.Note = missingIndexNote
				if planPath != "" {
					return usageErr(fmt.Errorf("no local index; run: dropbox-pp-cli index"))
				}
				result.planOutput, err = outputPlan(planPath, "mess", nil, force, printPlan)
				if err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			defer db.Close()
			result, empty, incomplete, err := queryMess(cmd.Context(), db, limit, maxDepth, includeDevDirs)
			if err != nil {
				return err
			}
			if planPath != "" && len(incomplete) > 0 {
				return usageErr(fmt.Errorf("index roots are missing or incomplete; run: dropbox-pp-cli index"))
			}
			ops := make([]dropbox.Op, 0)
			if len(empty) > 0 {
				emptySet := make(map[string]bool, len(empty))
				for _, row := range empty {
					emptySet[row.PathLower] = true
				}
				for _, row := range empty {
					if !emptySet[row.ParentLower] {
						files, bytes := 0, int64(0)
						ops = append(ops, dropbox.Op{Op: "delete", Path: row.PathDisplay, Reason: "empty folder", ExpectFiles: &files, ExpectBytes: &bytes})
					}
				}
			}
			result.planOutput, err = outputPlan(planPath, "mess", ops, force, printPlan)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d empty folders, %d single-file folders, %d root files, %d junk names, %d deep paths, %d near-duplicate folder groups\n", result.EmptyFolders.Count, result.SingleFileFolders.Count, result.RootFiles.Count, result.JunkNames.Count, result.DeepPaths.Count, result.NearDuplicateFolders.Count)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum items per category")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 8, "Depth after which a path is considered deep")
	cmd.Flags().StringVar(&planPath, "plan", "", "Write a soft-delete plan for maximal empty folders")
	cmd.Flags().BoolVar(&includeDevDirs, "include-dev-dirs", false, "Include development dependency folders")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite a file that is not a valid plan")
	cmd.Flags().BoolVar(&printPlan, "print-plan", false, "Include full plan operations in JSON output")
	return cmd
}

func newMessResult() messResult {
	return messResult{
		EmptyFolders:         messCategory[string]{Items: make([]string, 0)},
		SingleFileFolders:    messCategory[string]{Items: make([]string, 0)},
		RootFiles:            messCategory[string]{Items: make([]string, 0)},
		JunkNames:            messCategory[messJunk]{Items: make([]messJunk, 0)},
		DeepPaths:            messCategory[messDeep]{Items: make([]messDeep, 0)},
		NearDuplicateFolders: messCategory[messFolderGroup]{Items: make([]messFolderGroup, 0)},
	}
}

func queryMess(ctx context.Context, db *store.Store, limit, maxDepth int, includeDevDirs bool) (messResult, []store.DropboxRow, map[string]bool, error) {
	r := newMessResult()
	q := db.DB()
	completeRoots, err := completeIndexRoots(ctx, q)
	if err != nil {
		return r, nil, nil, err
	}
	incomplete := make(map[string]bool)
	devFilter := ""
	if !includeDevDirs {
		var err error
		r.ExcludedDevDirs, err = countExcludedDevDirs(ctx, db, "")
		if err != nil {
			return r, nil, nil, err
		}
		devFilter = ` AND dev_kind IS NULL`
	}
	folders := make([]store.DropboxRow, 0)
	rows, err := q.QueryContext(ctx, `SELECT path_lower,COALESCE(NULLIF(path_display,''),path_lower),COALESCE(parent_lower,''),name FROM dbx_files WHERE tag='folder'`+devFilter+` ORDER BY path_lower`)
	if err != nil {
		return r, nil, nil, err
	}
	for rows.Next() {
		var f store.DropboxRow
		if err := rows.Scan(&f.PathLower, &f.PathDisplay, &f.ParentLower, &f.Name); err != nil {
			_ = rows.Close()
			return r, nil, nil, err
		}
		folders = append(folders, f)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return r, nil, nil, err
	}
	// Aggregate direct files in SQL, then propagate each bucket through its folder ancestors.
	fileCounts := make(map[string]int)
	fileTable := `dbx_files`
	if !includeDevDirs {
		fileTable += ` INDEXED BY dbx_files_visible_file_parent`
	}
	rows, err = q.QueryContext(ctx, `SELECT COALESCE(parent_lower,''),COUNT(*) FROM `+fileTable+` WHERE tag='file'`+devFilter+` GROUP BY parent_lower`)
	if err != nil {
		return r, nil, nil, err
	}
	for rows.Next() {
		var parent string
		var count int
		if err := rows.Scan(&parent, &count); err != nil {
			_ = rows.Close()
			return r, nil, nil, err
		}
		for parent != "" {
			fileCounts[parent] += count
			i := strings.LastIndex(parent, "/")
			if i <= 0 {
				break
			}
			parent = parent[:i]
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return r, nil, nil, err
	}
	groups := make(map[string][]string)
	for _, f := range folders {
		root := indexRootForEntry(f.PathLower, "folder")
		if !completeRoots[root] {
			continue
		}
		if fileCounts[f.PathLower] == 1 {
			r.SingleFileFolders.Count++
			if len(r.SingleFileFolders.Items) < limit {
				r.SingleFileFolders.Items = append(r.SingleFileFolders.Items, f.PathDisplay)
			}
		}
		key := f.ParentLower + "\x00" + dropbox.NormalizeFolderName(f.Name)
		groups[key] = append(groups[key], f.PathDisplay)
	}
	// An indexed path range makes each NOT EXISTS probe stop at the first file.
	devEmptyFilter := ""
	if !includeDevDirs {
		devEmptyFilter = ` AND folder.dev_kind IS NULL AND NOT EXISTS (SELECT 1 FROM dbx_files AS dev WHERE dev.path_lower>=folder.path_lower||'/' AND dev.path_lower<folder.path_lower||'0' AND dev.dev_kind IS NOT NULL LIMIT 1)`
	}
	rows, err = q.QueryContext(ctx, `SELECT path_lower,COALESCE(NULLIF(path_display,''),path_lower),COALESCE(parent_lower,'') FROM dbx_files AS folder WHERE folder.tag='folder' AND COALESCE(folder.shared_folder_id,'')='' AND NOT EXISTS (SELECT 1 FROM dbx_files AS file INDEXED BY dbx_files_file_path_size WHERE file.tag='file' AND file.path_lower>=folder.path_lower||'/' AND file.path_lower<folder.path_lower||'0' LIMIT 1)`+devEmptyFilter+` ORDER BY folder.path_lower`)
	if err != nil {
		return r, nil, nil, err
	}
	empty := make([]store.DropboxRow, 0)
	for rows.Next() {
		var f store.DropboxRow
		if err := rows.Scan(&f.PathLower, &f.PathDisplay, &f.ParentLower); err != nil {
			_ = rows.Close()
			return r, nil, nil, err
		}
		root := indexRootForEntry(f.PathLower, "folder")
		if !completeRoots[root] {
			incomplete[root] = true
			continue
		}
		empty = append(empty, f)
		r.EmptyFolders.Count++
		if len(r.EmptyFolders.Items) < limit {
			r.EmptyFolders.Items = append(r.EmptyFolders.Items, f.PathDisplay)
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return r, nil, nil, err
	}
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM dbx_files WHERE tag='file' AND parent_lower=''`+devFilter).Scan(&r.RootFiles.Count); err != nil {
		return r, nil, nil, err
	}
	rows, err = q.QueryContext(ctx, `SELECT COALESCE(NULLIF(path_display,''),path_lower) FROM dbx_files WHERE tag='file' AND parent_lower=''`+devFilter+` ORDER BY path_lower LIMIT ?`, limit)
	if err != nil {
		return r, nil, nil, err
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			_ = rows.Close()
			return r, nil, nil, err
		}
		r.RootFiles.Items = append(r.RootFiles.Items, v)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return r, nil, nil, err
	}
	rows, err = q.QueryContext(ctx, `SELECT name,COALESCE(NULLIF(path_display,''),path_lower) FROM dbx_files WHERE (lower(trim(name)) LIKE 'copy of %' OR lower(trim(name)) LIKE 'untitled%' OR lower(trim(name)) LIKE 'new folder%' OR name GLOB '* ([0-9]*)*' OR lower(name) LIKE '% copy%' OR lower(name) LIKE '%-copy%')`+devFilter+` ORDER BY path_lower`)
	if err != nil {
		return r, nil, nil, err
	}
	for rows.Next() {
		var name, display string
		if err := rows.Scan(&name, &display); err != nil {
			_ = rows.Close()
			return r, nil, nil, err
		}
		if kind, ok := dropbox.JunkName(name); ok {
			r.JunkNames.Count++
			if len(r.JunkNames.Items) < limit {
				r.JunkNames.Items = append(r.JunkNames.Items, messJunk{display, kind})
			}
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return r, nil, nil, err
	}
	depthExpr := `length(path_lower)-length(replace(path_lower,'/',''))`
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM dbx_files WHERE `+depthExpr+`>?`+devFilter, maxDepth).Scan(&r.DeepPaths.Count); err != nil {
		return r, nil, nil, err
	}
	rows, err = q.QueryContext(ctx, `SELECT COALESCE(NULLIF(path_display,''),path_lower),`+depthExpr+` FROM dbx_files WHERE `+depthExpr+`>?`+devFilter+` ORDER BY path_lower LIMIT ?`, maxDepth, limit)
	if err != nil {
		return r, nil, nil, err
	}
	for rows.Next() {
		var d messDeep
		if err := rows.Scan(&d.Path, &d.Depth); err != nil {
			_ = rows.Close()
			return r, nil, nil, err
		}
		r.DeepPaths.Items = append(r.DeepPaths.Items, d)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return r, nil, nil, err
	}
	keys := make([]string, 0)
	for key, paths := range groups {
		if len(paths) > 1 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.SplitN(key, "\x00", 2)
		paths := groups[key]
		sort.Strings(paths)
		r.NearDuplicateFolders.Count++
		if len(r.NearDuplicateFolders.Items) < limit {
			r.NearDuplicateFolders.Items = append(r.NearDuplicateFolders.Items, messFolderGroup{Parent: parts[0], NormalizedName: parts[1], Folders: paths})
		}
	}
	return r, empty, incomplete, nil
}
