// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: local library health check over the parallel download
// command's printgoat_downloads table (read-only access; that table is
// owned by a different task and may not exist yet).

package cli

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

type libraryDoctorIssue struct {
	Source    string `json:"source"`
	ModelID   string `json:"model_id"`
	Name      string `json:"name,omitempty"`
	LocalPath string `json:"local_path,omitempty"`
	Issue     string `json:"issue"`
	Detail    string `json:"detail,omitempty"`
}

type downloadRecord struct {
	Source    string
	ModelID   string
	Name      string
	LocalPath string
	SHA256    string
}

// readDownloadRecords reads the parallel download command's
// printgoat_downloads table. Returns (nil, nil) when the table (or an
// expected column) doesn't exist yet — that is a normal "nothing downloaded
// yet" state, not an error, since that table is populated by a different,
// independently-developed command.
func readDownloadRecords(db *sql.DB) ([]downloadRecord, error) {
	rows, err := db.Query(`SELECT source, model_id, model_name, local_path, sha256 FROM printgoat_downloads`)
	if err != nil {
		if isNoSuchTable(err) || isNoSuchColumn(err) {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()

	var records []downloadRecord
	for rows.Next() {
		var r downloadRecord
		var name, localPath, sha sql.NullString
		if err := rows.Scan(&r.Source, &r.ModelID, &name, &localPath, &sha); err != nil {
			return nil, err
		}
		r.Name = name.String
		r.LocalPath = localPath.String
		r.SHA256 = sha.String
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

// pp:data-source computed
func newNovelLibraryDoctorCmd(flags *rootFlags) *cobra.Command {
	var checkRemote bool
	var sampleSize int

	cmd := &cobra.Command{
		Use:         "doctor",
		Short:       "Find orphaned files, missing files, silent duplicates",
		Example:     "  printgoat-pp-cli library doctor --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("printgoat-pp-cli")
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()

			records, err := readDownloadRecords(s.DB())
			if err != nil {
				return fmt.Errorf("reading local downloads: %w", err)
			}
			if len(records) == 0 {
				out := map[string]any{
					"checked":     0,
					"issue_count": 0,
					"issues":      []libraryDoctorIssue{},
					"message":     "no downloads recorded yet; nothing to check",
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			var issues []libraryDoctorIssue

			// Missing files: local_path recorded but not present on disk.
			for _, r := range records {
				if r.LocalPath == "" {
					continue
				}
				if _, statErr := os.Stat(r.LocalPath); statErr != nil {
					if os.IsNotExist(statErr) {
						issues = append(issues, libraryDoctorIssue{
							Source: r.Source, ModelID: r.ModelID, Name: r.Name, LocalPath: r.LocalPath,
							Issue: "missing_file", Detail: "recorded local file was not found on disk",
						})
					} else {
						issues = append(issues, libraryDoctorIssue{
							Source: r.Source, ModelID: r.ModelID, Name: r.Name, LocalPath: r.LocalPath,
							Issue: "unreadable_file", Detail: statErr.Error(),
						})
					}
				}
			}

			// Silent duplicates: same sha256 across different rows.
			bySHA := map[string][]downloadRecord{}
			for _, r := range records {
				if r.SHA256 == "" {
					continue
				}
				bySHA[r.SHA256] = append(bySHA[r.SHA256], r)
			}
			for sha, group := range bySHA {
				if len(group) < 2 {
					continue
				}
				for _, r := range group {
					issues = append(issues, libraryDoctorIssue{
						Source: r.Source, ModelID: r.ModelID, Name: r.Name, LocalPath: r.LocalPath,
						Issue:  "silent_duplicate",
						Detail: fmt.Sprintf("sha256 %s is shared by %d downloaded files", sha, len(group)),
					})
				}
			}

			// Remote existence check on a bounded sample. Best-effort: an
			// unreachable source (missing credentials, network error) is
			// silently skipped for that row rather than treated as delisted
			// — only a confirmed not-found is reported.
			if checkRemote {
				n := sampleSize
				if n <= 0 || n > len(records) {
					n = len(records)
				}
				if c, cerr := flags.newClient(); cerr == nil {
					for _, r := range records[:n] {
						switch r.Source {
						case "printables", "thingiverse":
							detail, ferr := fetchModelDetail(ctx, c, r.Source, r.ModelID)
							if ferr == nil && detail != nil && !detail.Found {
								issues = append(issues, libraryDoctorIssue{
									Source: r.Source, ModelID: r.ModelID, Name: r.Name, LocalPath: r.LocalPath,
									Issue: "delisted", Detail: "remote model no longer exists at the source",
								})
							}
						default:
							// Cults3D has no confirmed cheap existence probe; skip.
						}
					}
				}
			}

			out := map[string]any{
				"checked":     len(records),
				"issue_count": len(issues),
				"issues":      issues,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&checkRemote, "check-remote", true, "Re-check that each downloaded model still exists at its source (Printables/Thingiverse only)")
	cmd.Flags().IntVar(&sampleSize, "sample", 25, "Maximum number of downloads to remote-check, to bound API cost (0 = check all)")
	return cmd
}
