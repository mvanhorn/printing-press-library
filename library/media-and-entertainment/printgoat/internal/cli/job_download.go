// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: `job download` batch-downloads files for a set of models
// across all three sources as one crash-safe job, with real HTTP
// range-resume shared with `job resume`.
//
// File URL availability is source-dependent: only Thingiverse's
// ThingiverseFile schema (as researched for this CLI) confirms a direct
// download_url/direct_url. Printables and Cults3D expose no such field via
// the endpoints this CLI is built against, so their files are still
// recorded (for visibility and future `library doctor`/`snapshot` use) but
// marked "unsupported_source" instead of guessing at a URL.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

// sizeOrNull stores a known file size, or NULL when the source's file
// listing didn't report one (0 is ambiguous with "unknown" so it's not
// stored as a literal 0 that downloadFileResumable would then treat as an
// expected empty file).
func sizeOrNull(size int64) sql.NullInt64 {
	if size <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: size, Valid: true}
}

// jobPlannedFile is one file queued for a job, before it has a job id.
type jobPlannedFile struct {
	Source   string
	ModelID  string
	FileName string
	FileURL  string
	Size     int64  // known size from the source's file listing, 0 if unknown
	Status   string // "pending" (has a URL) or "unsupported_source"
}

type jobFileResult struct {
	Source          string `json:"source"`
	ModelID         string `json:"model_id"`
	FileName        string `json:"file_name"`
	Status          string `json:"status"`
	DownloadedBytes int64  `json:"downloaded_bytes"`
	TotalBytes      int64  `json:"total_bytes,omitempty"`
	Error           string `json:"error,omitempty"`
}

// planJobFiles resolves each ref (URL or model-key) to a model, fetches its
// file listing, and returns one jobPlannedFile per file. A ref that fails to
// parse or resolve is reported in notes rather than aborting the whole plan
// — a batch job across independent sources should download what it can.
func planJobFiles(ctx context.Context, c *client.Client, refs []string) ([]jobPlannedFile, []map[string]any) {
	var planned []jobPlannedFile
	var notes []map[string]any
	for _, ref := range refs {
		source, id, perr := parseModelRef(ref)
		if perr != nil {
			notes = append(notes, map[string]any{"ref": ref, "error": perr.Error()})
			continue
		}
		detail, ferr := fetchModelDetail(ctx, c, source, id)
		if ferr != nil {
			notes = append(notes, map[string]any{"ref": ref, "source": source, "model_id": id, "error": ferr.Error()})
			continue
		}
		if !detail.Found {
			notes = append(notes, map[string]any{"ref": ref, "source": source, "model_id": id, "error": "model not found (delisted or deleted)"})
			continue
		}
		if len(detail.Files) == 0 {
			notes = append(notes, map[string]any{"ref": ref, "source": source, "model_id": id, "note": "no files listed for this model"})
			continue
		}
		for _, f := range detail.Files {
			status := "pending"
			if f.URL == "" {
				status = "unsupported_source"
			}
			planned = append(planned, jobPlannedFile{
				Source: source, ModelID: id, FileName: f.Name, FileURL: f.URL, Size: f.Size, Status: status,
			})
		}
	}
	return planned, notes
}

// nextJobID generates "job-<YYYYMMDD>-<NN>" by counting existing job ids for
// today's date, per spec — not time.Now()-seeded randomness beyond the date.
func nextJobID(ctx context.Context, db *sql.DB, now time.Time) (string, error) {
	prefix := "job-" + now.UTC().Format("20060102") + "-"
	rows, err := db.QueryContext(ctx, `SELECT id FROM printgoat_print_jobs WHERE id LIKE ?`, prefix+"%")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	maxN := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		if n, cerr := strconv.Atoi(strings.TrimPrefix(id, prefix)); cerr == nil && n > maxN {
			maxN = n
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%02d", prefix, maxN+1), nil
}

// downloadFileResumable GETs fileURL with a Range header set to resume from
// resumeFrom, streaming into destPath+".partial" and renaming to destPath on
// completion. Returns the total bytes now on disk (whether or not the
// transfer fully completed) so the caller can persist progress either way.
func downloadFileResumable(ctx context.Context, c *client.Client, httpClient *http.Client, fileURL, destPath string, resumeFrom, expectedSize int64) (int64, bool, error) {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
		return resumeFrom, false, fmt.Errorf("creating download directory: %w", err)
	}
	partialPath := destPath + ".partial"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return resumeFrom, false, err
	}
	startAt := resumeFrom
	if startAt > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startAt))
	}
	// Some file URLs (e.g. Thingiverse's proxied download_url, as opposed to
	// its unauthenticated CDN direct_url) require the same per-host
	// credential this client attaches to ordinary API calls.
	if c != nil {
		if authHeader, aerr := c.AuthHeaderForURL(ctx, fileURL); aerr == nil && authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return resumeFrom, false, err
	}
	defer resp.Body.Close()

	openFlags := os.O_CREATE | os.O_WRONLY
	switch resp.StatusCode {
	case http.StatusOK:
		// Server ignored (or there was nothing to resume from): start clean.
		startAt = 0
		openFlags |= os.O_TRUNC
	case http.StatusPartialContent:
		openFlags |= os.O_APPEND
	default:
		return resumeFrom, false, fmt.Errorf("%s returned HTTP %d", fileURL, resp.StatusCode)
	}

	f, err := os.OpenFile(partialPath, openFlags, 0o600) // #nosec G304 -- app-derived download path from a confirmed source file listing.
	if err != nil {
		return resumeFrom, false, fmt.Errorf("opening partial file: %w", err)
	}
	written, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	total := startAt + written
	if copyErr != nil {
		return total, false, copyErr
	}
	if closeErr != nil {
		return total, false, closeErr
	}
	// A clean connection close before the full body arrives (no explicit
	// copy error — e.g. a proxy hiccup that ends the stream early) must not
	// be reported as complete when the server told us how big the file
	// should be. Without this check the file gets marked "done" while
	// truncated, and the byte-range resume logic never gets a chance to
	// finish it because "done" rows are skipped on the next `job resume`.
	if expectedSize > 0 && total < expectedSize {
		return total, false, fmt.Errorf("incomplete transfer: got %d of %d expected bytes", total, expectedSize)
	}
	if err := os.Rename(partialPath, destPath); err != nil {
		return total, false, fmt.Errorf("finalizing download: %w", err)
	}
	return total, true, nil
}

// processJobFiles processes every not-yet-done row for jobID: downloads
// files that have a URL (resuming from downloaded_bytes), and marks
// URL-less rows unsupported_source. Shared by `job download` (right after
// planning) and `job resume` (picking up an existing job).
func processJobFiles(ctx context.Context, c *client.Client, httpClient *http.Client, db *sql.DB, jobID string) ([]jobFileResult, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, source, model_id, file_name, file_url, local_path, total_bytes, downloaded_bytes, status
		 FROM printgoat_print_job_files WHERE job_id = ? ORDER BY id`, jobID)
	if err != nil {
		return nil, err
	}
	type fileRow struct {
		id                    int64
		source, modelID, name string
		fileURL, localPath    string
		total, downloaded     int64
		status                string
	}
	var toProcess []fileRow
	for rows.Next() {
		var r fileRow
		var fileURL, localPath sql.NullString
		var total sql.NullInt64
		if err := rows.Scan(&r.id, &r.source, &r.modelID, &r.name, &fileURL, &localPath, &total, &r.downloaded, &r.status); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.fileURL = fileURL.String
		r.localPath = localPath.String
		if total.Valid {
			r.total = total.Int64
		}
		toProcess = append(toProcess, r)
	}
	closeErr := rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}

	results := make([]jobFileResult, 0, len(toProcess))
	for _, r := range toProcess {
		res := jobFileResult{Source: r.source, ModelID: r.modelID, FileName: r.name, TotalBytes: r.total, DownloadedBytes: r.downloaded}

		if r.status == "done" || r.status == "unsupported_source" {
			res.Status = r.status
			results = append(results, res)
			continue
		}
		if r.fileURL == "" || r.localPath == "" {
			res.Status = "unsupported_source"
			_, _ = db.ExecContext(ctx, `UPDATE printgoat_print_job_files SET status = 'unsupported_source' WHERE id = ?`, r.id)
			results = append(results, res)
			continue
		}

		_, _ = db.ExecContext(ctx, `UPDATE printgoat_print_job_files SET status = 'in_progress' WHERE id = ?`, r.id)
		total, completed, derr := downloadFileResumable(ctx, c, httpClient, r.fileURL, r.localPath, r.downloaded, r.total)
		status := "in_progress"
		if completed {
			status = "done"
		}
		if derr != nil {
			status = "failed"
			res.Error = derr.Error()
		}
		res.Status = status
		res.DownloadedBytes = total
		_, _ = db.ExecContext(ctx, `UPDATE printgoat_print_job_files SET downloaded_bytes = ?, status = ? WHERE id = ?`, total, status, r.id)
		results = append(results, res)
	}
	return results, nil
}

// finalizeJobStatus rolls a job's own status up from its files: any file
// still pending/in_progress/failed keeps the job "in_progress" so `job
// resume` knows there's more to do; otherwise the job is "completed" (files
// left unsupported_source do not block completion — nothing more can be
// done for them by this CLI).
func finalizeJobStatus(ctx context.Context, db *sql.DB, jobID string) error {
	var incomplete int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM printgoat_print_job_files WHERE job_id = ? AND status NOT IN ('done', 'unsupported_source')`,
		jobID,
	).Scan(&incomplete); err != nil {
		return err
	}
	status := "completed"
	if incomplete > 0 {
		status = "in_progress"
	}
	_, err := db.ExecContext(ctx, `UPDATE printgoat_print_jobs SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().UTC().Format(time.RFC3339), jobID)
	return err
}

// pp:data-source live
func newNovelJobDownloadCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "download <urls-or-model-keys...>",
		Short:       "Batch-download across sites as one crash-safe job that actually resumes from where it stopped (real file bytes today: Thingiverse only).",
		Example:     "  printgoat-pp-cli job download thingiverse:763622 thingiverse:2009 --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("job download requires at least one URL or <model-key>\nUsage: %s <urls-or-model-keys...>", cmd.CommandPath()))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
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

			planned, notes := planJobFiles(ctx, c, args)
			if len(planned) == 0 {
				out := map[string]any{"notes": notes, "message": "no downloadable files found for the given references"}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			now := time.Now().UTC()
			jobID, jerr := nextJobID(ctx, s.DB(), now)
			if jerr != nil {
				return fmt.Errorf("allocating job id: %w", jerr)
			}
			nowStr := now.Format(time.RFC3339)
			if _, err := s.DB().ExecContext(ctx,
				`INSERT INTO printgoat_print_jobs (id, status, created_at, updated_at) VALUES (?, 'in_progress', ?, ?)`,
				jobID, nowStr, nowStr,
			); err != nil {
				return fmt.Errorf("creating job: %w", err)
			}

			jobsRoot := filepath.Join(filepath.Dir(dbPath), "jobs", jobID)
			for _, pf := range planned {
				var localPath sql.NullString
				if pf.Status == "pending" {
					// pf.ModelID and pf.FileName both originate from upstream
					// API responses (or user-supplied model refs); sanitizing
					// through the same filepath.Base-based helper download.go
					// uses prevents a crafted "../../etc/passwd"-shaped name
					// from escaping jobsRoot via filepath.Join's lexical
					// ".." resolution.
					localPath = sql.NullString{String: filepath.Join(jobsRoot, pf.Source, sanitizeFilename(pf.ModelID), sanitizeFilename(pf.FileName)), Valid: true}
				}
				if _, err := s.DB().ExecContext(ctx,
					`INSERT OR IGNORE INTO printgoat_print_job_files (job_id, source, model_id, file_name, file_url, local_path, total_bytes, downloaded_bytes, status)
					 VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`,
					jobID, pf.Source, pf.ModelID, pf.FileName, pf.FileURL, localPath, sizeOrNull(pf.Size), pf.Status,
				); err != nil {
					return fmt.Errorf("recording job file: %w", err)
				}
			}

			results, perr := processJobFiles(ctx, c, c.HTTPClient, s.DB(), jobID)
			if perr != nil {
				return fmt.Errorf("processing job files: %w", perr)
			}
			if err := finalizeJobStatus(ctx, s.DB(), jobID); err != nil {
				return fmt.Errorf("finalizing job status: %w", err)
			}

			out := map[string]any{"job_id": jobID, "files": results}
			if len(notes) > 0 {
				out["notes"] = notes
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
