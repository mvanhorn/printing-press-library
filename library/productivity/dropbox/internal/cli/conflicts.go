// pp:data-source local
package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/spf13/cobra"
)

type conflictPair struct {
	Copy           string   `json:"copy"`
	Original       string   `json:"original"`
	Class          string   `json:"class"`
	CopyRev        string   `json:"copy_rev"`
	Size           int64    `json:"size"`
	Kind           string   `json:"kind"`
	Depth          int      `json:"depth"`
	FileCount      int      `json:"file_count"`
	Bytes          int64    `json:"bytes"`
	DifferingPaths []string `json:"differing_paths,omitempty"`
	DifferingCount int      `json:"differing_count,omitempty"`
}
type conflictCounts struct {
	Identical       int `json:"identical"`
	Different       int `json:"different"`
	Orphan          int `json:"orphan"`
	Folder          int `json:"folder"`
	IdenticalTree   int `json:"identical_tree"`
	SubsetTree      int `json:"subset_tree"`
	DivergedTree    int `json:"diverged_tree"`
	InvalidFiles    int `json:"invalid_files"`
	EmptyCopy       int `json:"empty_copy"`
	IndexIncomplete int `json:"index_incomplete"`
}
type conflictsResult struct {
	planOutput
	IndexMissing    bool            `json:"index_missing,omitempty"`
	Note            string          `json:"note,omitempty"`
	Pairs           []conflictPair  `json:"pairs"`
	Counts          conflictCounts  `json:"counts"`
	ExcludedDevDirs excludedDevDirs `json:"excluded_dev_dirs"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelConflictsCmd(flags))
	})
}

func newNovelConflictsCmd(flags *rootFlags) *cobra.Command {
	var dbPath, planPath string
	var includeDevDirs, force, printPlan bool
	cmd := &cobra.Command{
		Use: "conflicts", Short: "Find Dropbox conflict copies and compare them with originals",
		Long: "Find conflicted copies, selective sync conflicts, case conflicts, and invalid-files folders in the local Dropbox index.",
		Example: strings.Trim(`
  dropbox-pp-cli conflicts --agent
  dropbox-pp-cli conflicts --plan conflicts.json --agent`, "\n"),
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:write-flags": "plan", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "conflicts")
			}
			if err := localSourceOnly(flags); err != nil {
				return err
			}
			db, found, err := openIndex(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			result := conflictsResult{Pairs: make([]conflictPair, 0)}
			if !found {
				result.IndexMissing = true
				result.Note = missingIndexNote
				if planPath != "" {
					return usageErr(fmt.Errorf("no local index; run: dropbox-pp-cli index"))
				}
				result.planOutput, err = outputPlan(planPath, "conflicts", nil, force, printPlan)
				if err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			defer db.Close()
			candidateFilter := `conflict_candidate=1`
			devFilter := ""
			if !includeDevDirs {
				result.ExcludedDevDirs, err = countExcludedDevDirs(cmd.Context(), db, ` AND `+candidateFilter)
				if err != nil {
					return err
				}
				devFilter = ` AND dev_kind IS NULL`
			}
			completeRoots, err := completeIndexRoots(cmd.Context(), db.DB())
			if err != nil {
				return err
			}
			candidateRows, err := db.DB().QueryContext(cmd.Context(), `SELECT `+dropboxRowColumns+` FROM dbx_files WHERE `+candidateFilter+devFilter+` ORDER BY path_lower`)
			if err != nil {
				return err
			}
			rows, err := scanDropboxRows(candidateRows)
			if err != nil {
				return err
			}
			ops := make([]dropbox.Op, 0)
			incomplete := make(map[string]bool)
			for _, copyRow := range rows {
				if !includeDevDirs && copyRow.Tag == "folder" {
					lower, upper := descendantRange(copyRow.PathLower)
					var devCount int
					if err := db.DB().QueryRowContext(cmd.Context(), `SELECT EXISTS(SELECT 1 FROM dbx_files WHERE path_lower>=? AND path_lower<? AND dev_kind IS NOT NULL LIMIT 1)`, lower, upper).Scan(&devCount); err != nil {
						return err
					}
					if devCount > 0 {
						var files int
						var bytes int64
						if err := db.DB().QueryRowContext(cmd.Context(), `SELECT COUNT(*),COALESCE(SUM(size),0) FROM dbx_files WHERE tag='file' AND path_lower>=? AND path_lower<?`, lower, upper).Scan(&files, &bytes); err != nil {
							return err
						}
						result.ExcludedDevDirs.Files += files
						result.ExcludedDevDirs.Bytes += bytes
						continue
					}
				}
				originalName, kind, _, _, depth, ok := dropbox.ConflictMarkerWithDepth(copyRow.Name)
				if !ok {
					continue
				}
				pair := conflictPair{Copy: copyRow.PathDisplay, CopyRev: copyRow.Rev, Size: copyRow.Size, Class: "orphan", Kind: kind, Depth: depth}
				var original struct {
					tag, display, hash, path string
					size                     int64
				}
				lookupErr := db.DB().QueryRowContext(cmd.Context(), `SELECT tag,COALESCE(NULLIF(path_display,''),path_lower),COALESCE(content_hash,''),path_lower,COALESCE(size,0) FROM dbx_files INDEXED BY dbx_files_parent_name_lower WHERE parent_lower=? AND lower(name)=? LIMIT 1`, copyRow.ParentLower, strings.ToLower(originalName)).Scan(&original.tag, &original.display, &original.hash, &original.path, &original.size)
				if lookupErr != nil && lookupErr != sql.ErrNoRows {
					return lookupErr
				}
				if lookupErr == nil {
					pair.Original = original.display
				}
				copyRoot := indexRootForEntry(copyRow.PathLower, copyRow.Tag)
				originalRoot := ""
				if lookupErr == nil {
					originalRoot = indexRootForEntry(original.path, original.tag)
				}
				if !completeRoots[copyRoot] || (lookupErr == nil && !completeRoots[originalRoot]) {
					pair.Class = "index_incomplete"
					couldPlan := kind != "invalid_files" && lookupErr == nil && ((copyRow.Tag == "folder" && original.tag == "folder") || (copyRow.Tag == "file" && original.tag == "file" && copyRow.ContentHash != "" && copyRow.ContentHash == original.hash && copyRow.Size == original.size))
					if couldPlan {
						if !completeRoots[copyRoot] {
							incomplete[copyRoot] = true
						}
						if !completeRoots[originalRoot] {
							incomplete[originalRoot] = true
						}
					}
				} else if kind == "invalid_files" {
					pair.Class = "invalid_files"
					if copyRow.Tag == "folder" {
						files, err := conflictTreeFiles(cmd.Context(), db.DB(), copyRow.PathLower)
						if err != nil {
							return err
						}
						pair.FileCount = len(files)
						for _, file := range files {
							pair.Bytes += file.size
						}
					} else if copyRow.Tag == "file" {
						pair.FileCount, pair.Bytes = 1, copyRow.Size
					}
				} else if lookupErr == nil {
					switch {
					case copyRow.Tag == "folder" && original.tag == "folder":
						copyFiles, err := conflictTreeFiles(cmd.Context(), db.DB(), copyRow.PathLower)
						if err != nil {
							return err
						}
						originalFiles, err := conflictTreeFiles(cmd.Context(), db.DB(), original.path)
						if err != nil {
							return err
						}
						pair.FileCount = len(copyFiles)
						for _, file := range copyFiles {
							pair.Bytes += file.size
						}
						pair.Class, pair.DifferingPaths, pair.DifferingCount = compareConflictTrees(copyFiles, originalFiles)
						if pair.Class == "identical_tree" {
							reason := "conflict copy identical to " + pair.Original
							if kind == "selective_sync_conflict" {
								reason = "selective sync conflict copy identical to " + pair.Original
							} else if kind == "conflicted_copy" {
								reason = "conflicted copy identical to " + pair.Original
							} else if kind == "case_conflict" {
								reason = "case conflict copy identical to " + pair.Original
							}
							files, bytes := pair.FileCount, pair.Bytes
							ops = append(ops, dropbox.Op{Op: "delete", Path: pair.Copy, Reason: reason, ExpectFiles: &files, ExpectBytes: &bytes, Keeper: pair.Original, ExpectTreeHash: conflictTreeHash(originalFiles), ExpectPathTreeHash: conflictTreeHash(copyFiles)})
						} else if pair.Class == "subset_tree" {
							files, bytes := pair.FileCount, pair.Bytes
							ops = append(ops, dropbox.Op{Op: "delete", Path: pair.Copy, Reason: "contains only files already in " + pair.Original, ExpectFiles: &files, ExpectBytes: &bytes, Keeper: pair.Original, ExpectTreeHash: conflictTreeHash(originalFiles), ExpectPathTreeHash: conflictTreeHash(copyFiles)})
						}
					case copyRow.Tag == "folder" || original.tag == "folder":
						pair.Class = "folder"
					case copyRow.ContentHash != "" && copyRow.ContentHash == original.hash && copyRow.Size == original.size:
						pair.Class = "identical"
						ops = append(ops, dropbox.Op{Op: "delete", Path: copyRow.PathDisplay, Rev: copyRow.Rev, Reason: "identical to " + original.display, Keeper: original.display, ContentHash: original.hash})
					default:
						pair.Class = "different"
					}
				}
				result.Counts.add(pair.Class)
				result.Pairs = append(result.Pairs, pair)
			}
			if planPath != "" && len(incomplete) > 0 {
				return usageErr(fmt.Errorf("index roots are missing or incomplete; run: dropbox-pp-cli index"))
			}
			result.planOutput, err = outputPlan(planPath, "conflicts", ops, force, printPlan)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d identical files, %d identical trees, %d subset trees, %d diverged trees, %d invalid-files folders, %d incomplete indexes\n", result.Counts.Identical, result.Counts.IdenticalTree, result.Counts.SubsetTree, result.Counts.DivergedTree, result.Counts.InvalidFiles, result.Counts.IndexIncomplete)
			for _, p := range result.Pairs {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", p.Class, p.Copy, p.Original)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().StringVar(&planPath, "plan", "", "Write a soft-delete plan for identical and subset conflict copies")
	cmd.Flags().BoolVar(&includeDevDirs, "include-dev-dirs", false, "Include development dependency folders")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite a file that is not a valid plan")
	cmd.Flags().BoolVar(&printPlan, "print-plan", false, "Include full plan operations in JSON output")
	return cmd
}

func (c *conflictCounts) add(class string) {
	switch class {
	case "identical":
		c.Identical++
	case "different":
		c.Different++
	case "orphan":
		c.Orphan++
	case "folder":
		c.Folder++
	case "identical_tree":
		c.IdenticalTree++
	case "subset_tree":
		c.SubsetTree++
	case "diverged_tree":
		c.DivergedTree++
	case "invalid_files":
		c.InvalidFiles++
	case "empty_copy":
		c.EmptyCopy++
	case "index_incomplete":
		c.IndexIncomplete++
	}
}

type conflictTreeFile struct {
	relative, hash string
	size           int64
}

func conflictTreeHash(files []conflictTreeFile) string {
	h := sha256.New()
	for _, file := range files {
		fmt.Fprintf(h, "%s|%s|%d\n", file.relative, file.hash, file.size)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func conflictTreeFiles(ctx context.Context, db *sql.DB, folder string) ([]conflictTreeFile, error) {
	lower, upper := descendantRange(folder)
	rows, err := db.QueryContext(ctx, `SELECT path_lower,COALESCE(content_hash,''),COALESCE(size,0) FROM dbx_files INDEXED BY dbx_files_file_path_size WHERE path_lower>=? AND path_lower<? AND tag='file' ORDER BY path_lower`, lower, upper)
	if err != nil {
		return nil, err
	}
	files := make([]conflictTreeFile, 0)
	for rows.Next() {
		var path string
		var file conflictTreeFile
		if err := rows.Scan(&path, &file.hash, &file.size); err != nil {
			_ = rows.Close()
			return nil, err
		}
		file.relative = strings.TrimPrefix(path, lower)
		files = append(files, file)
	}
	err = rows.Err()
	_ = rows.Close()
	return files, err
}

func compareConflictTrees(copyFiles, originalFiles []conflictTreeFile) (class string, differingPaths []string, differingCount int) {
	differingPaths = make([]string, 0)
	if len(copyFiles) == 0 {
		return "empty_copy", differingPaths, 0
	}
	j := 0
	for _, copyFile := range copyFiles {
		for j < len(originalFiles) && originalFiles[j].relative < copyFile.relative {
			j++
		}
		matches := j < len(originalFiles) && originalFiles[j].relative == copyFile.relative
		if !matches || copyFile.hash == "" || originalFiles[j].hash == "" || copyFile.hash != originalFiles[j].hash || copyFile.size != originalFiles[j].size {
			differingCount++
			if len(differingPaths) < 20 {
				differingPaths = append(differingPaths, copyFile.relative)
			}
		}
	}
	if differingCount > 0 {
		return "diverged_tree", differingPaths, differingCount
	}
	if len(copyFiles) == len(originalFiles) {
		return "identical_tree", differingPaths, 0
	}
	return "subset_tree", differingPaths, 0
}
