// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: ride + FIT-file archive. Not generated.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/wahoo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/wahoo/internal/store"
)

// pp:data-source local

type backupView struct {
	OutDir          string          `json:"out_dir"`
	WorkoutsWritten int             `json:"workouts_written"`
	FilesDownloaded int             `json:"files_downloaded"`
	FilesSkipped    int             `json:"files_skipped"`
	FilesFailed     int             `json:"files_failed"`
	Failures        []backupFailure `json:"failures"`
}

type backupFailure struct {
	WorkoutID string `json:"workout_id"`
	URL       string `json:"url,omitempty"`
	Error     string `json:"error"`
}

type backupItem struct {
	id        string
	starts    time.Time
	hasStarts bool
	fileURL   string
	raw       string
}

func newNovelBackupCmd(flags *rootFlags) *cobra.Command {
	var out string
	var full bool
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Archive every workout record and its FIT file locally",
		Long: "Download every workout record (as JSON) plus its raw FIT file from the local\n" +
			"mirror into a resumable directory tree, so you own a permanent backup the Wahoo\n" +
			"app can't export. Re-running skips files already present unless --full is set.\n" +
			"FIT downloads don't count against the API rate limit. Run 'sync --resources\n" +
			"workouts' first so the mirror knows your rides.",
		Example: "  wahoo-pp-cli backup --out ./wahoo-archive --full",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would archive workouts + FIT files to %q\n", out)
				return nil
			}
			if out == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--out <dir> is required"))
			}
			// Side-effect guard: never write archive files during a verify pass.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would archive workouts + FIT files to %q\n", out)
				return nil
			}
			var cutoff time.Time
			haveCutoff := false
			if since != "" {
				d, err := cliutil.ParseDurationLoose(since)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
				}
				cutoff = time.Now().Add(-d)
				haveCutoff = true
			}
			path := dbPathOrDefault(dbPath)
			if mirrorMissing(cmd, flags, path, "workouts", `{"workouts_written":0}`) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), path)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			items, err := readBackupItems(db)
			if err != nil {
				return fmt.Errorf("reading workouts: %w", err)
			}
			// Private ride data: owner-only dir perms (matches config/cache store).
			if err := os.MkdirAll(out, 0o700); err != nil {
				return fmt.Errorf("creating output dir: %w", err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			httpClient := &http.Client{Timeout: 60 * time.Second}

			// Under live-dogfood, curtail to a single item so the matrix's
			// per-command timeout is respected (real downloads, not mocks).
			if cliutil.IsDogfoodEnv() && len(items) > 1 {
				items = items[:1]
			}

			view := backupView{OutDir: out, Failures: make([]backupFailure, 0)}
			for _, it := range items {
				if haveCutoff && (!it.hasStarts || it.starts.Before(cutoff)) {
					continue
				}
				recPath := filepath.Join(out, it.id+".json")
				if err := os.WriteFile(recPath, []byte(it.raw), 0o600); err != nil {
					view.FilesFailed++
					view.Failures = append(view.Failures, backupFailure{WorkoutID: it.id, Error: err.Error()})
					continue
				}
				view.WorkoutsWritten++
				if it.fileURL == "" {
					continue
				}
				fitPath := filepath.Join(out, it.id+".fit")
				if !full {
					if _, statErr := os.Stat(fitPath); statErr == nil {
						view.FilesSkipped++
						continue
					}
				}
				if err := downloadTo(ctx, httpClient, it.fileURL, fitPath); err != nil {
					view.FilesFailed++
					view.Failures = append(view.Failures, backupFailure{WorkoutID: it.id, URL: it.fileURL, Error: err.Error()})
					continue
				}
				view.FilesDownloaded++
			}

			if view.FilesFailed > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d FIT download(s) failed; %d succeeded\n", view.FilesFailed, view.FilesDownloaded)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "Archived to %s:\n", view.OutDir)
				fmt.Fprintf(out, "  Workout records: %d\n", view.WorkoutsWritten)
				fmt.Fprintf(out, "  FIT downloaded:  %d\n", view.FilesDownloaded)
				fmt.Fprintf(out, "  FIT skipped:     %d\n", view.FilesSkipped)
				if view.FilesFailed > 0 {
					fmt.Fprintf(out, "  FIT failed:      %d\n", view.FilesFailed)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "Output directory for the archive (required)")
	cmd.Flags().BoolVar(&full, "full", false, "Re-download FIT files even if already present")
	cmd.Flags().StringVar(&since, "since", "", "Only archive rides since this ago, e.g. 30d, 1w, 720h")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/wahoo-pp-cli/data.db)")
	return cmd
}

func readBackupItems(db *store.Store) ([]backupItem, error) {
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = 'workouts'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []backupItem
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil || !raw.Valid || raw.String == "" {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(raw.String), &obj) != nil {
			continue
		}
		it := backupItem{id: idString(obj["id"]), raw: raw.String}
		if it.id == "" {
			continue
		}
		if s, ok := obj["starts"].(string); ok {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				it.starts = t
				it.hasStarts = true
			}
		}
		if sum, ok := obj["workout_summary"].(map[string]any); ok && sum != nil {
			if file, ok := sum["file"].(map[string]any); ok && file != nil {
				if u, ok := file["url"].(string); ok {
					it.fileURL = u
				}
			}
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// downloadTo streams url to dest via a temp file + rename so an interrupted
// download never leaves a truncated .fit in place.
func downloadTo(ctx context.Context, c *http.Client, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d downloading FIT", resp.StatusCode)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".fit-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, dest)
}
