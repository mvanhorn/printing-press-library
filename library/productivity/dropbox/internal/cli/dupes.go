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

type duplicateFile struct {
	Path           string `json:"path"`
	Rev            string `json:"rev"`
	ClientModified string `json:"client_modified"`
}
type duplicateGroup struct {
	ContentHash      string          `json:"content_hash"`
	Size             int64           `json:"size"`
	Keeper           string          `json:"keeper"`
	Duplicates       []duplicateFile `json:"duplicates"`
	ReclaimableBytes int64           `json:"reclaimable_bytes"`
	rows             []store.DropboxRow
}
type dupesResult struct {
	planOutput
	IndexMissing          bool             `json:"index_missing,omitempty"`
	Note                  string           `json:"note,omitempty"`
	Groups                []duplicateGroup `json:"groups"`
	TotalGroups           int              `json:"total_groups"`
	TotalReclaimableBytes int64            `json:"total_reclaimable_bytes"`
	ExcludedDevDirs       excludedDevDirs  `json:"excluded_dev_dirs"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) { addNovelCommandIfAbsent(root, newDupesCmd(flags)) })
}

func newDupesCmd(flags *rootFlags) *cobra.Command {
	var dbPath, under, keep, planPath string
	var minSize int64
	var limit int
	var includeDevDirs, force, printPlan bool
	cmd := &cobra.Command{
		Use: "dupes", Short: "Find byte-identical files in the local index",
		Long: "Use this command for byte-identical files anywhere in the Dropbox. Do NOT use it for Dropbox conflicted copies; use 'conflicts' instead.",
		Example: strings.Trim(`
  dropbox-pp-cli dupes --under "/Camera Uploads" --agent
  dropbox-pp-cli dupes --keep in:/Documents/Taxes --plan dupes.json --agent`, "\n"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:write-flags": "plan", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "dupes")
			}
			if err := localSourceOnly(flags); err != nil {
				return err
			}
			if minSize < 0 || limit < 0 {
				return usageErr(fmt.Errorf("--min-size and --limit must be nonnegative"))
			}
			if keep != "oldest" && keep != "newest" && keep != "shortest-path" && !strings.HasPrefix(keep, "in:/") {
				return usageErr(fmt.Errorf("--keep must be oldest, newest, shortest-path, or in:/folder"))
			}
			db, found, err := openIndex(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			result := dupesResult{Groups: make([]duplicateGroup, 0)}
			if !found {
				result.IndexMissing = true
				result.Note = missingIndexNote
				if planPath != "" {
					return usageErr(fmt.Errorf("no local index; run: dropbox-pp-cli index"))
				}
				result.planOutput, err = outputPlan(planPath, "dupes", nil, force, printPlan)
				if err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			defer db.Close()
			if !includeDevDirs {
				result.ExcludedDevDirs, err = countExcludedDuplicateFiles(cmd.Context(), db, minSize, under)
				if err != nil {
					return err
				}
			}
			rows, err := duplicateRows(cmd.Context(), db, minSize, under, includeDevDirs)
			if err != nil {
				return err
			}
			excluded := result.ExcludedDevDirs
			result, all := analyzeDupes(rows, minSize, under, keep, limit)
			result.ExcludedDevDirs = excluded
			ops := make([]dropbox.Op, 0)
			for _, group := range all {
				for _, row := range group.rows {
					if row.PathDisplay != group.Keeper {
						ops = append(ops, dropbox.Op{Op: "delete", Path: row.PathDisplay, Rev: row.Rev, Reason: "duplicate of " + group.Keeper, Keeper: group.Keeper, ContentHash: group.ContentHash})
					}
				}
			}
			result.planOutput, err = outputPlan(planPath, "dupes", ops, force, printPlan)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d duplicate groups; %d reclaimable bytes\n", result.TotalGroups, result.TotalReclaimableBytes)
			for _, g := range result.Groups {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %d copies, keep %s\n", g.ContentHash, len(g.Duplicates), g.Keeper)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().Int64Var(&minSize, "min-size", 1, "Minimum file size in bytes")
	cmd.Flags().StringVar(&under, "under", "", "Only include files under this path")
	cmd.Flags().StringVar(&keep, "keep", "oldest", "Keeper rule: oldest, newest, shortest-path, or in:/folder")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum groups to show")
	cmd.Flags().StringVar(&planPath, "plan", "", "Write a soft-delete plan for all non-keepers")
	cmd.Flags().BoolVar(&includeDevDirs, "include-dev-dirs", false, "Include development dependency folders")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite a file that is not a valid plan")
	cmd.Flags().BoolVar(&printPlan, "print-plan", false, "Include full plan operations in JSON output")
	return cmd
}

func countExcludedDuplicateFiles(ctx context.Context, db *store.Store, minSize int64, under string) (excludedDevDirs, error) {
	query := `SELECT COALESCE(SUM(dev_files),0),COALESCE(SUM(dev_bytes),0) FROM (SELECT COUNT(*) AS copies,COUNT(*) FILTER (WHERE dev_kind IS NOT NULL) AS dev_files,COALESCE(SUM(size) FILTER (WHERE dev_kind IS NOT NULL),0) AS dev_bytes FROM dbx_files INDEXED BY dbx_files_duplicate_candidates WHERE tag='file' AND content_hash<>'' AND size>=?`
	args := []any{minSize}
	under = strings.ToLower(strings.TrimSuffix(under, "/"))
	if under != "" {
		lower, upper := descendantRange(under)
		query += ` AND (path_lower=? OR (path_lower>=? AND path_lower<?))`
		args = append(args, under, lower, upper)
	}
	query += ` GROUP BY content_hash,size HAVING COUNT(*)>1)`
	var result excludedDevDirs
	err := db.DB().QueryRowContext(ctx, query, args...).Scan(&result.Files, &result.Bytes)
	return result, err
}

func duplicateRows(ctx context.Context, db *store.Store, minSize int64, under string, includeDevDirs bool) ([]store.DropboxRow, error) {
	under = strings.ToLower(strings.TrimSuffix(under, "/"))
	query, args := duplicateCandidateSQL(under, minSize, includeDevDirs)
	groups, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	type key struct {
		hash string
		size int64
	}
	keys := make([]key, 0)
	for groups.Next() {
		var k key
		var copies int
		if err := groups.Scan(&k.hash, &k.size, &copies); err != nil {
			_ = groups.Close()
			return nil, err
		}
		keys = append(keys, k)
	}
	err = groups.Err()
	_ = groups.Close()
	if err != nil {
		return nil, err
	}
	all := make([]store.DropboxRow, 0, len(keys)*2)
	memberQuery := `SELECT ` + dropboxRowColumns + ` FROM dbx_files INDEXED BY dbx_files_duplicate_candidates WHERE tag='file' AND content_hash<>'' AND content_hash=? AND size=?`
	if !includeDevDirs {
		memberQuery += ` AND dev_kind IS NULL`
	}
	if under != "" {
		memberQuery += ` AND (path_lower=? OR (path_lower>=? AND path_lower<?))`
	}
	memberQuery += ` ORDER BY path_lower`
	for _, k := range keys {
		memberArgs := []any{k.hash, k.size}
		if under != "" {
			lower, upper := descendantRange(under)
			memberArgs = append(memberArgs, under, lower, upper)
		}
		members, err := db.DB().QueryContext(ctx, memberQuery, memberArgs...)
		if err != nil {
			return nil, err
		}
		selected, err := scanDropboxRows(members)
		if err != nil {
			return nil, err
		}
		all = append(all, selected...)
	}
	return all, nil
}

func analyzeDupes(rows []store.DropboxRow, minSize int64, under, keep string, limit int) (dupesResult, []duplicateGroup) {
	groups := make(map[string][]store.DropboxRow)
	for _, row := range rows {
		if row.Tag == "file" && row.ContentHash != "" && row.Size >= minSize && dropbox.PathWithin(row.PathLower, under) {
			key := fmt.Sprintf("%s\x00%d", row.ContentHash, row.Size)
			groups[key] = append(groups[key], row)
		}
	}
	all := make([]duplicateGroup, 0)
	result := dupesResult{Groups: make([]duplicateGroup, 0)}
	for _, files := range groups {
		if len(files) < 2 {
			continue
		}
		sort.Slice(files, func(i, j int) bool { return duplicateLess(files[i], files[j], keep) })
		g := duplicateGroup{ContentHash: files[0].ContentHash, Size: files[0].Size, Keeper: files[0].PathDisplay, Duplicates: make([]duplicateFile, 0, len(files)-1), ReclaimableBytes: int64(len(files)-1) * files[0].Size, rows: files}
		for _, row := range files[1:] {
			g.Duplicates = append(g.Duplicates, duplicateFile{row.PathDisplay, row.Rev, row.ClientModified})
		}
		all = append(all, g)
		result.TotalReclaimableBytes += g.ReclaimableBytes
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].ReclaimableBytes != all[j].ReclaimableBytes {
			return all[i].ReclaimableBytes > all[j].ReclaimableBytes
		}
		if all[i].ContentHash != all[j].ContentHash {
			return all[i].ContentHash < all[j].ContentHash
		}
		return all[i].Size < all[j].Size
	})
	result.TotalGroups = len(all)
	if limit > len(all) {
		limit = len(all)
	}
	result.Groups = append(result.Groups, all[:limit]...)
	return result, all
}

func duplicateLess(a, b store.DropboxRow, keep string) bool {
	if strings.HasPrefix(keep, "in:") {
		folder := strings.TrimPrefix(keep, "in:")
		ai, bi := dropbox.PathWithin(a.PathLower, folder), dropbox.PathWithin(b.PathLower, folder)
		if ai != bi {
			return ai
		}
		keep = "oldest"
	}
	if keep == "shortest-path" {
		return pathTieLess(a, b)
	}
	if a.ClientModified != b.ClientModified {
		if keep == "newest" {
			return a.ClientModified > b.ClientModified
		}
		if a.ClientModified == "" {
			return false
		}
		if b.ClientModified == "" {
			return true
		}
		return a.ClientModified < b.ClientModified
	}
	return pathTieLess(a, b)
}

func pathTieLess(a, b store.DropboxRow) bool {
	if len(a.PathLower) != len(b.PathLower) {
		return len(a.PathLower) < len(b.PathLower)
	}
	if a.PathLower != b.PathLower {
		return a.PathLower < b.PathLower
	}
	return a.PathDisplay < b.PathDisplay
}
