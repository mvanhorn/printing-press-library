// pp:data-source local
package cli

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/spf13/cobra"
)

type organizeCollision struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}
type organizeResult struct {
	planOutput
	IndexMissing        bool                `json:"index_missing,omitempty"`
	Note                string              `json:"note,omitempty"`
	Matched             int                 `json:"matched"`
	PlannedMoves        int                 `json:"planned_moves"`
	Mkdirs              []string            `json:"mkdirs"`
	SkippedAlreadyThere int                 `json:"skipped_already_there"`
	SkippedNoDate       []string            `json:"skipped_no_date"`
	Collisions          []organizeCollision `json:"collisions"`
	OpsPreview          []dropbox.Op        `json:"ops_preview"`
	ExcludedDevDirs     excludedDevDirs     `json:"excluded_dev_dirs"`
}

func newNovelOrganizeCmd(flags *rootFlags) *cobra.Command {
	var match, under, to, planPath, dbPath, tz string
	var limit int
	var includeDevDirs, force, printPlan bool
	cmd := &cobra.Command{Use: "organize", Short: "Build a move plan from a filename glob and folder template", Long: "Select indexed files by case-insensitive filename glob, expand metadata tokens in the destination folder, and write a reviewable plan.", Example: strings.Trim(`
  dropbox-pp-cli organize --match '*.jpg' --under '/Camera Uploads' --to '/Photos/{year}/{month}' --plan photos.json --agent`, "\n"), Args: cobra.NoArgs, Annotations: map[string]string{"mcp:read-only": "true", "mcp:write-flags": "plan", "pp:data-source": "local"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "organize")
		}
		if match == "" || under == "" || to == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("--match, --under, and --to are required"))
		}
		if err := localSourceOnly(flags); err != nil {
			return err
		}
		if limit < 0 {
			return usageErr(fmt.Errorf("--limit must be nonnegative"))
		}
		if _, err := path.Match(strings.ToLower(match), "x"); err != nil {
			return usageErr(err)
		}
		if !strings.HasPrefix(under, "/") {
			return usageErr(fmt.Errorf("--under must be an absolute Dropbox path"))
		}
		if _, err := dropbox.ExpandFolderTemplate(to, "sample.txt", time.Now()); err != nil {
			return usageErr(err)
		}
		location, err := time.LoadLocation(tz)
		if err != nil {
			return usageErr(fmt.Errorf("invalid --tz %q: %w", tz, err))
		}
		result := organizeResult{Mkdirs: make([]string, 0), Collisions: make([]organizeCollision, 0), OpsPreview: make([]dropbox.Op, 0), SkippedNoDate: make([]string, 0)}
		db, found, err := openIndex(cmd, flags, dbPath)
		if err != nil {
			return err
		}
		if !found {
			result.IndexMissing = true
			result.Note = missingIndexNote
			if planPath != "" {
				return usageErr(fmt.Errorf("no local index; run: dropbox-pp-cli index"))
			}
			result.planOutput, err = outputPlan(planPath, "organize", nil, force, printPlan)
			if err != nil {
				return err
			}
			return printJSONFilteredKeep(cmd.OutOrStdout(), result, flags, planOpKeepFields...)
		}
		defer db.Close()
		// --under bounds the scan in SQL. --match stays in Go because SQLite
		// lower() folds only ASCII, and path.Match on Go-lowered names is the
		// documented contract.
		query := `SELECT ` + dropboxRowColumns + ` FROM dbx_files WHERE tag='file'`
		var queryArgs []any
		if underKey := strings.ToLower(strings.TrimRight(under, "/")); underKey != "" {
			lower, upper := descendantRange(underKey)
			query += ` AND (path_lower=? OR (path_lower>=? AND path_lower<?))`
			queryArgs = append(queryArgs, underKey, lower, upper)
		}
		rows, err := db.DB().QueryContext(cmd.Context(), query+` ORDER BY path_lower`, queryArgs...)
		if err != nil {
			return err
		}
		pattern := strings.ToLower(match)
		type candidate struct{ from, to, rev string }
		candidates := make([]candidate, 0)
		destCounts := map[string]int{}
		for rows.Next() {
			r, err := scanDropboxRow(rows)
			if err != nil {
				_ = rows.Close()
				return err
			}
			ok, err := path.Match(pattern, strings.ToLower(r.Name))
			if err != nil {
				_ = rows.Close()
				return err
			}
			if !ok {
				continue
			}
			if r.DevKind != "" && !includeDevDirs {
				result.ExcludedDevDirs.Files++
				result.ExcludedDevDirs.Bytes += r.Size
				continue
			}
			result.Matched++
			modified, err := time.Parse(time.RFC3339, r.ClientModified)
			if err != nil || modified.IsZero() {
				result.SkippedNoDate = append(result.SkippedNoDate, r.PathDisplay)
				continue
			}
			modified = modified.In(location)
			folder, err := dropbox.ExpandFolderTemplate(to, r.Name, modified)
			if err != nil {
				_ = rows.Close()
				return usageErr(err)
			}
			dest := folder + "/" + r.Name
			if strings.EqualFold(dest, r.PathLower) {
				result.SkippedAlreadyThere++
				continue
			}
			candidates = append(candidates, candidate{r.PathDisplay, dest, r.Rev})
			destCounts[strings.ToLower(dest)]++
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return err
		}
		lookup, err := db.DB().PrepareContext(cmd.Context(), `SELECT EXISTS(SELECT 1 FROM dbx_files WHERE path_lower=?)`)
		if err != nil {
			return err
		}
		defer lookup.Close()
		known := map[string]bool{}
		indexed := func(key string) (bool, error) {
			if found, ok := known[key]; ok {
				return found, nil
			}
			var found bool
			if err := lookup.QueryRowContext(cmd.Context(), key).Scan(&found); err != nil {
				return false, err
			}
			known[key] = found
			return found, nil
		}
		moves := make([]dropbox.Op, 0)
		needed := map[string]string{}
		for _, c := range candidates {
			key := strings.ToLower(c.to)
			destExists, err := indexed(key)
			if err != nil {
				return err
			}
			if destExists || destCounts[key] > 1 {
				reason := "destination exists"
				if destCounts[key] > 1 {
					reason = "multiple sources share destination"
				}
				result.Collisions = append(result.Collisions, organizeCollision{c.from, c.to, reason})
				continue
			}
			moves = append(moves, dropbox.Op{Op: "move", From: c.from, To: c.to, Rev: c.rev})
			folder, _ := dropbox.ParentBase(c.to)
			for folder != "" {
				lower := strings.ToLower(folder)
				folderExists, err := indexed(lower)
				if err != nil {
					return err
				}
				if folderExists {
					break
				}
				needed[lower] = folder
				folder, _ = dropbox.ParentBase(folder)
			}
		}
		for _, folder := range needed {
			result.Mkdirs = append(result.Mkdirs, folder)
		}
		sort.Slice(result.Mkdirs, func(i, j int) bool {
			a, b := result.Mkdirs[i], result.Mkdirs[j]
			if strings.Count(a, "/") != strings.Count(b, "/") {
				return strings.Count(a, "/") < strings.Count(b, "/")
			}
			return strings.ToLower(a) < strings.ToLower(b)
		})
		ops := make([]dropbox.Op, 0, len(result.Mkdirs)+len(moves))
		for _, folder := range result.Mkdirs {
			ops = append(ops, dropbox.Op{Op: "mkdir", Path: folder})
		}
		ops = append(ops, moves...)
		result.PlannedMoves = len(moves)
		result.planOutput, err = outputPlan(planPath, "organize", ops, force, printPlan)
		if err != nil {
			return err
		}
		preview := limit
		if preview > len(ops) {
			preview = len(ops)
		}
		result.OpsPreview = append(result.OpsPreview, ops[:preview]...)
		if !wantsHumanTable(cmd.OutOrStdout(), flags) {
			return printJSONFilteredKeep(cmd.OutOrStdout(), result, flags, planOpKeepFields...)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%d matched, %d moves, %d folders, %d collisions\n", result.Matched, result.PlannedMoves, len(result.Mkdirs), len(result.Collisions))
		for _, op := range result.OpsPreview {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", op.Op, op.From, op.To)
		}
		return nil
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().StringVar(&match, "match", "", "Case-insensitive filename glob")
	cmd.Flags().StringVar(&under, "under", "", "Source folder (recursive)")
	cmd.Flags().StringVar(&to, "to", "", "Destination folder template")
	cmd.Flags().StringVar(&planPath, "plan", "", "Write the full plan to this file")
	cmd.Flags().StringVar(&tz, "tz", "Local", "Time zone for client_modified (IANA name or Local)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum operations in preview")
	cmd.Flags().BoolVar(&includeDevDirs, "include-dev-dirs", false, "Include development dependency folders")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite a file that is not a valid plan")
	cmd.Flags().BoolVar(&printPlan, "print-plan", false, "Include full plan operations in JSON output")
	return cmd
}
