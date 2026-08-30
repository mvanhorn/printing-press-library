// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: `snapshot verify` re-hashes the local files pinned by
// `snapshot create` and reports drift, plus a best-effort upstream check.

package cli

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

type snapshotVerifyResult struct {
	snapshotFileEntry
	Status        string `json:"status"` // ok, drifted, missing, unreadable, unhashed
	CurrentSHA256 string `json:"current_sha256,omitempty"`
}

// pp:data-source computed
func newNovelSnapshotVerifyCmd(flags *rootFlags) *cobra.Command {
	var checkUpstream bool

	cmd := &cobra.Command{
		Use:         "verify <name>",
		Short:       "Prove exactly which file version you used for a past print job, even if the upstream model has since changed.",
		Example:     "  printgoat-pp-cli snapshot verify batch-march-orders --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("missing required argument <name>\nUsage: %s <name>", cmd.CommandPath()))
			}
			name := args[0]

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("printgoat-pp-cli")
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()
			if err := store.EnsurePrintgoatNovelSchema(s.DB()); err != nil {
				return fmt.Errorf("preparing local schema: %w", err)
			}

			var filesJSON string
			scanErr := s.DB().QueryRowContext(ctx, `SELECT files_json FROM printgoat_job_snapshots WHERE name = ?`, name).Scan(&filesJSON)
			if scanErr == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("no snapshot named %q; run 'snapshot create %s <model-keys...>' first", name, name))
			}
			if scanErr != nil {
				return fmt.Errorf("reading snapshot: %w", scanErr)
			}

			var files []snapshotFileEntry
			if uerr := json.Unmarshal([]byte(filesJSON), &files); uerr != nil {
				return fmt.Errorf("parsing snapshot: %w", uerr)
			}

			results := make([]snapshotVerifyResult, 0, len(files))
			counts := map[string]int{}
			for _, f := range files {
				vr := snapshotVerifyResult{snapshotFileEntry: f}
				if f.LocalPath == "" {
					vr.Status = "unhashed"
					counts[vr.Status]++
					results = append(results, vr)
					continue
				}
				data, rerr := os.ReadFile(f.LocalPath)
				if rerr != nil {
					if os.IsNotExist(rerr) {
						vr.Status = "missing"
					} else {
						vr.Status = "unreadable"
					}
					counts[vr.Status]++
					results = append(results, vr)
					continue
				}
				sum := sha256.Sum256(data)
				vr.CurrentSHA256 = hex.EncodeToString(sum[:])
				switch {
				case f.SHA256 == "":
					vr.Status = "unhashed"
				case vr.CurrentSHA256 == f.SHA256:
					vr.Status = "ok"
				default:
					vr.Status = "drifted"
				}
				counts[vr.Status]++
				results = append(results, vr)
			}

			out := map[string]any{
				"name":           name,
				"file_count":     len(files),
				"ok_count":       counts["ok"],
				"drifted_count":  counts["drifted"],
				"missing_count":  counts["missing"],
				"unhashed_count": counts["unhashed"],
				"results":        results,
			}

			// Best-effort upstream drift check: does the source still have the
			// same file listing? A fetch failure here (auth/network) is
			// silently skipped for that model rather than failing the whole
			// verification, since the pinned local hashes are the authoritative
			// answer this command exists to give.
			if checkUpstream {
				if c, cerr := flags.newClient(); cerr == nil {
					seen := map[string]bool{}
					var upstream []map[string]any
					for _, f := range files {
						key := modelKey(f.Source, f.ModelID)
						if seen[key] {
							continue
						}
						seen[key] = true
						detail, ferr := fetchModelDetail(ctx, c, f.Source, f.ModelID)
						if ferr != nil {
							continue
						}
						entry := map[string]any{"source": f.Source, "model_id": f.ModelID}
						if !detail.Found {
							entry["upstream_changed"] = true
							entry["note"] = "model no longer exists upstream"
						} else {
							entry["current_file_count"] = len(detail.Files)
						}
						upstream = append(upstream, entry)
					}
					if len(upstream) > 0 {
						out["upstream"] = upstream
					}
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&checkUpstream, "check-upstream", true, "Also best-effort re-fetch each model's current file list to note upstream drift")
	return cmd
}
