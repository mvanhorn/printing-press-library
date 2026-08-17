// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/printgoat/internal/store"
	"github.com/spf13/cobra"
)

// downloadResultRow is one file's outcome, mirroring printgoat_downloads'
// columns closely enough that recordDownload can insert it almost as-is.
type downloadResultRow struct {
	Source          string `json:"source"`
	ModelID         string `json:"model_id"`
	ModelName       string `json:"model_name,omitempty"`
	FileName        string `json:"file_name"`
	FileURL         string `json:"file_url,omitempty"`
	LocalPath       string `json:"local_path"`
	FileSize        int64  `json:"file_size,omitempty"`
	BytesDownloaded int64  `json:"bytes_downloaded"`
	Status          string `json:"status"`
	SHA256          string `json:"sha256,omitempty"`
	DownloadedAt    string `json:"downloaded_at"`
}

func newDownloadCmd(flags *rootFlags) *cobra.Command {
	var all bool
	var formatsList string
	var outputDir string
	var openBrowser bool

	cmd := &cobra.Command{
		Use:   "download <source>:<id>",
		Short: "Download a model's files with real resumable byte-range downloads",
		Long: `Downloads Printables or Thingiverse files to a local directory.

Downloads are resumable for real: an interrupted transfer writes to
"<file>.partial" and, on retry, resumes with an HTTP Range request from the
number of bytes already on disk, instead of restarting from scratch or
silently treating an existing partial file as already complete. The file is
renamed to its final name only once the transfer completes. Every
completed download is recorded in the local SQLite ledger
(printgoat_downloads).

Cults3D cannot be downloaded through this command: its API explicitly
excludes third-party file access by design, permanently, for legal reasons.
Pass --open to open the listing's Cults3D page in a browser instead of just
printing its URL.`,
		Example: `  printgoat-pp-cli download printables:12345 --all -o ./models
  printgoat-pp-cli download printables:12345 --formats stl,3mf --dry-run
  printgoat-pp-cli download thingiverse:763622 --all
  printgoat-pp-cli download cults3d:12345 --open`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			source, id, perr := parseSourceID(args[0])
			if perr != nil {
				return usageErr(perr)
			}
			formats := parseFormatsList(formatsList)
			if source != "cults3d" && !all && len(formats) == 0 {
				return usageErr(fmt.Errorf("pass --all or --formats <ext,ext,...> to select files to download"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dryRunOK(flags) {
				if source == "cults3d" {
					fmt.Fprintf(cmd.ErrOrStderr(), "dry run: would %s the Cults3D listing for %s (no file download is possible via the API)\n", openVerb(openBrowser), id)
					return nil
				}
				dir := resolveDownloadDir(ctx, outputDir)
				fmt.Fprintf(cmd.ErrOrStderr(), "dry run: would resolve and download %s files for %s:%s into %s\n", downloadSelectionDescription(all, formats), source, id, dir)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if source == "cults3d" {
				return downloadCults3D(cmd, ctx, c, id, openBrowser)
			}

			dir := resolveDownloadDir(ctx, outputDir)
			dbPath := defaultDBPath("printgoat-pp-cli")
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database for download bookkeeping: %w", err)
			}
			defer db.Close()
			if err := store.EnsurePrintgoatSchema(db.DB()); err != nil {
				return fmt.Errorf("preparing local download ledger: %w", err)
			}

			var results []downloadResultRow
			switch source {
			case "printables":
				results, err = downloadPrintables(cmd, ctx, c, db, id, all, formats, dir)
			case "thingiverse":
				results, err = downloadThingiverse(cmd, ctx, c, db, id, all, formats, dir)
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return outputDownloadResults(cmd, flags, results)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Download every file for the model")
	cmd.Flags().StringVar(&formatsList, "formats", "", "Comma-separated file formats/extensions to download (e.g. stl,3mf,gcode)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "", "Destination directory (default: config's download_dir, or the current directory)")
	cmd.Flags().BoolVar(&openBrowser, "open", false, "For Cults3D: open the listing in a browser instead of just printing its URL")
	return cmd
}

func parseFormatsList(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		f = strings.ToUpper(strings.TrimSpace(f))
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func downloadSelectionDescription(all bool, formats []string) string {
	if all {
		return "all"
	}
	return strings.Join(formats, ",")
}

func openVerb(openBrowser bool) string {
	if openBrowser {
		return "open"
	}
	return "print the URL for"
}

// resolveDownloadDir honors an explicit -o/--output flag first, then falls
// back to the config-store's download_dir preference (best-effort: a
// missing database or unset key is silently treated as "not configured",
// since this is only ever a display/default convenience, never a hard
// requirement), then finally "." .
func resolveDownloadDir(ctx context.Context, explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	dbPath := defaultDBPath("printgoat-pp-cli")
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return "."
	}
	defer db.Close()
	var value string
	if err := db.DB().QueryRowContext(ctx, `SELECT value FROM printgoat_config_kv WHERE key = 'download_dir'`).Scan(&value); err != nil {
		return "."
	}
	if strings.TrimSpace(value) == "" {
		return "."
	}
	return value
}

// selectPrintablesFiles filters a model's file list down to the requested
// set: everything when --all was passed, otherwise anything whose inferred
// extension (Format) matches one of --formats.
func selectPrintablesFiles(files []printablesModelFile, all bool, formats []string) []printablesModelFile {
	if all {
		return files
	}
	want := map[string]bool{}
	for _, f := range formats {
		want[f] = true
	}
	var out []printablesModelFile
	for _, f := range files {
		if want[strings.ToUpper(f.Format)] {
			out = append(out, f)
		}
	}
	return out
}

func downloadPrintables(cmd *cobra.Command, ctx context.Context, c *client.Client, db *store.Store, id string, all bool, formats []string, dir string) ([]downloadResultRow, error) {
	files, err := printablesFiles(ctx, c, id)
	if err != nil {
		return nil, err
	}
	selected := selectPrintablesFiles(files, all, formats)
	if len(selected) == 0 {
		return nil, fmt.Errorf("no files matched for printables:%s (all=%v formats=%v); run 'files printables:%s' to see what's available", id, all, formats, id)
	}
	link, err := printablesGetDownloadLink(ctx, c, id, groupPrintablesFilesByType(selected))
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{}
	results := make([]downloadResultRow, 0, len(selected))
	for _, f := range selected {
		fileURL := findPrintablesFileLink(link, f.ID)
		if fileURL == "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: printables did not return a link for %s (%s); skipping\n", f.Name, f.ID)
			continue
		}
		dest := filepath.Join(dir, sanitizeFilename(f.Name))
		row := downloadResultRow{
			Source:       "printables",
			ModelID:      id,
			FileName:     f.Name,
			FileURL:      fileURL,
			LocalPath:    dest,
			FileSize:     f.SizeBytes,
			DownloadedAt: time.Now().UTC().Format(time.RFC3339),
		}
		n, sum, derr := downloadResumable(ctx, cmd.ErrOrStderr(), c, httpClient, fileURL, dest, f.SizeBytes)
		row.BytesDownloaded = n
		row.SHA256 = sum
		if derr != nil {
			row.Status = "failed"
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: download failed for %s: %v\n", f.Name, derr)
		} else {
			row.Status = "complete"
			if rerr := recordDownload(ctx, db, row); rerr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not record download in local ledger: %v\n", rerr)
			}
		}
		results = append(results, row)
	}
	return results, nil
}

func downloadThingiverse(cmd *cobra.Command, ctx context.Context, c *client.Client, db *store.Store, id string, all bool, formats []string, dir string) ([]downloadResultRow, error) {
	entries, err := thingiverseFiles(ctx, c, id)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, f := range formats {
		want[f] = true
	}
	httpClient := &http.Client{}
	var results []downloadResultRow
	for _, e := range entries {
		format := inferFormatFromName(e.Name)
		if !all && !want[format] {
			continue
		}
		fileURL := thingiverseFileURL(e)
		if fileURL == "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: no download URL for %s (format %s not available as a direct file); skipping\n", e.Name, format)
			continue
		}
		dest := filepath.Join(dir, sanitizeFilename(e.Name))
		row := downloadResultRow{
			Source:       "thingiverse",
			ModelID:      id,
			FileName:     e.Name,
			FileURL:      fileURL,
			LocalPath:    dest,
			FileSize:     e.Size,
			DownloadedAt: time.Now().UTC().Format(time.RFC3339),
		}
		n, sum, derr := downloadResumable(ctx, cmd.ErrOrStderr(), c, httpClient, fileURL, dest, e.Size)
		row.BytesDownloaded = n
		row.SHA256 = sum
		if derr != nil {
			row.Status = "failed"
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: download failed for %s: %v\n", e.Name, derr)
		} else {
			row.Status = "complete"
			if rerr := recordDownload(ctx, db, row); rerr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not record download in local ledger: %v\n", rerr)
			}
		}
		results = append(results, row)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no files matched for thingiverse:%s (all=%v formats=%v); run 'files thingiverse:%s' to see what's available", id, all, formats, id)
	}
	return results, nil
}

// downloadCults3D never fetches a file — Cults3D's API has no download
// capability for other users' files by design. It best-effort resolves the
// listing's real page URL (via the same creation-by-slug query the generated
// `creations get` command uses) so --open has something real to open, and
// falls back to a Cults3D search URL if that lookup fails.
func downloadCults3D(cmd *cobra.Command, ctx context.Context, c *client.Client, id string, openBrowser bool) error {
	fmt.Fprintln(cmd.ErrOrStderr(), "Cults3D's API excludes third-party file downloads by design (their docs are explicit about this); this command cannot fetch Cults3D files.")

	pageURL, err := cults3DPageURL(ctx, c, id)
	if err != nil || pageURL == "" {
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "note: could not resolve a direct listing URL for %q (%v); falling back to a search link.\n", id, err)
		}
		pageURL = "https://cults3d.com/en/search?q=" + url.QueryEscape(id)
	}

	if !openBrowser {
		fmt.Fprintf(cmd.OutOrStdout(), "listing: %s\n", pageURL)
		return nil
	}
	if cliutil.IsVerifyEnv() {
		fmt.Fprintf(cmd.OutOrStdout(), "would open: %s\n", pageURL)
		return nil
	}
	if runtime.GOOS == "darwin" {
		if err := exec.Command("open", pageURL).Start(); err != nil { // #nosec G204 -- fixed "open" argv0, url is the only variable argument.
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not launch browser (%v); listing URL: %s\n", err, pageURL)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "opened: %s\n", pageURL)
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), pageURL)
	return nil
}

// cults3DPageURL resolves a Cults3D listing's real page URL via the same
// "creation" GraphQL query the generated `creations get` command uses. Uses
// the "url" field, not "shortUrl": Creation.shortUrl returns a bare numeric
// redirect form (e.g. "https://cults3d.com/:2034048") that 403s when fetched
// directly rather than redirecting — confirmed live. That query takes a
// slug; download's <id> is treated as one, which works for slugs and may not
// for the bare numeric "identifier" search returns — unconfirmed against the
// live API. Failure here is non-fatal to the caller: it falls back to a
// search link.
func cults3DPageURL(ctx context.Context, c *client.Client, slugOrID string) (string, error) {
	body := map[string]any{
		"query": `query GetCreation($slug: String!) { creation(slug: $slug) { identifier name url } }`,
		"variables": map[string]any{
			"slug": slugOrID,
		},
	}
	data, _, err := c.PostQueryWithParams(ctx, cults3dGraphQLURL, nil, body)
	if err != nil {
		return "", err
	}
	var resp struct {
		graphQLErrors
		Data struct {
			Creation *struct {
				URL string `json:"url"`
			} `json:"creation"`
		} `json:"data"`
	}
	if uerr := json.Unmarshal(data, &resp); uerr != nil {
		return "", fmt.Errorf("parsing cults3d creation response: %w", uerr)
	}
	if msg := resp.firstError(); msg != "" {
		return "", fmt.Errorf("cults3d: %s", msg)
	}
	if resp.Data.Creation == nil || resp.Data.Creation.URL == "" {
		return "", fmt.Errorf("cults3d creation %q not found", slugOrID)
	}
	return resp.Data.Creation.URL, nil
}

// sanitizeFilename strips any directory components from an upstream file
// name before it is joined to the output directory, so a maliciously or
// accidentally path-shaped name (e.g. "../../etc/passwd") can never escape
// the destination directory.
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	if name == "" || name == "." || name == ".." || name == string(filepath.Separator) {
		return "download.bin"
	}
	return name
}

// downloadResumable fetches rawURL to dest using a real HTTP byte-range
// resume, not thingfinder's fake "skip if the filename exists" resume:
//
//  1. Write to dest+".partial".
//  2. Before each attempt, stat the partial file for its current size.
//  3. Send "Range: bytes=<size>-" so the server only sends the remainder.
//  4. Append the response body to the partial file.
//  5. Rename to dest only once the transfer is complete (checked against
//     Content-Length + offset when the server reports a size).
//
// Returns the total bytes on disk and a sha256 hex digest of the completed
// file. On failure after retries it returns the error and the byte count
// written so far so callers can report a partial-progress warning.
func downloadResumable(ctx context.Context, stderr io.Writer, c *client.Client, httpClient *http.Client, rawURL, dest string, expectedSize int64) (int64, string, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil { // #nosec G301 -- destination dir is user-specified via -o/config, not upstream-controlled.
		return 0, "", fmt.Errorf("creating destination directory %s: %w", filepath.Dir(dest), err)
	}
	partial := dest + ".partial"

	const maxAttempts = 5
	var lastErr error
	var total int64
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		offset := int64(0)
		if info, err := os.Stat(partial); err == nil {
			offset = info.Size()
		}
		total = offset
		if expectedSize > 0 && offset >= expectedSize {
			break // a prior attempt already finished writing the bytes
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return offset, "", fmt.Errorf("building download request: %w", err)
		}
		// Printables' file CDN sits behind Cloudflare, which blocks Go's
		// default "Go-http-client/1.1" User-Agent outright (confirmed live:
		// the exact same signed link 200s for curl/a browser UA and 403s
		// with Go's default) even though the request is a plain
		// unauthenticated GET to a public signed link. A realistic browser
		// UA is not bypassing any auth or ToS control here — it just avoids
		// a generic bot-signature block, consistent with internal/client's
		// own use of surf's Chrome impersonation for API calls.
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}
		// Some download URLs (e.g. Thingiverse's proxied download_url, as
		// opposed to its unauthenticated CDN direct_url) require the same
		// per-host credential this client attaches to ordinary API calls.
		// Unauthenticated CDN links simply get no header back (empty string).
		if c != nil {
			if authHeader, aerr := c.AuthHeaderForURL(ctx, rawURL); aerr == nil && authHeader != "" {
				req.Header.Set("Authorization", authHeader)
			}
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return offset, "", ctx.Err()
			}
			fmt.Fprintf(stderr, "warning: download attempt %d/%d failed (%v); will retry\n", attempt, maxAttempts, err)
			if werr := sleepBackoff(ctx, attempt); werr != nil {
				return offset, "", werr
			}
			continue
		}

		if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			// The server disagrees the partial is a valid prefix (e.g. the
			// upstream file changed underneath us) - discard and restart.
			_ = resp.Body.Close()
			_ = os.Remove(partial)
			lastErr = fmt.Errorf("server rejected resume range for offset %d; restarting from scratch", offset)
			continue
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			_ = resp.Body.Close()
			return offset, "", fmt.Errorf("download request returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		if offset > 0 && resp.StatusCode == http.StatusOK {
			// We asked for a range but got the whole file back — the server
			// doesn't support resume. Discard the stale partial and restart
			// clean using this same response body.
			_ = os.Remove(partial)
			offset = 0
		}

		openFlags := os.O_CREATE | os.O_WRONLY
		if offset > 0 {
			openFlags |= os.O_APPEND
		} else {
			openFlags |= os.O_TRUNC
		}
		f, err := os.OpenFile(partial, openFlags, 0o644) // #nosec G302,G304 -- destination path is derived from a user-specified output dir + upstream filename, sanitized by sanitizeFilename.
		if err != nil {
			_ = resp.Body.Close()
			return offset, "", fmt.Errorf("opening partial file %s: %w", partial, err)
		}
		written, copyErr := io.Copy(f, resp.Body)
		closeErr := f.Close()
		_ = resp.Body.Close()
		total = offset + written
		if copyErr != nil {
			lastErr = copyErr
			if ctx.Err() != nil {
				return total, "", ctx.Err()
			}
			fmt.Fprintf(stderr, "warning: download attempt %d/%d interrupted after %d bytes (%v); will resume\n", attempt, maxAttempts, written, copyErr)
			if werr := sleepBackoff(ctx, attempt); werr != nil {
				return total, "", werr
			}
			continue
		}
		if closeErr != nil {
			return total, "", fmt.Errorf("closing partial file %s: %w", partial, closeErr)
		}
		if expectedSize > 0 && total < expectedSize {
			// Connection closed cleanly but short of the advertised size;
			// loop again to request the remainder starting from `total`.
			lastErr = fmt.Errorf("incomplete transfer: got %d of %d expected bytes", total, expectedSize)
			continue
		}

		sum, err := sha256File(partial)
		if err != nil {
			return total, "", fmt.Errorf("hashing downloaded file: %w", err)
		}
		if err := os.Rename(partial, dest); err != nil {
			return total, "", fmt.Errorf("finalizing download to %s: %w", dest, err)
		}
		return total, sum, nil
	}
	return total, "", fmt.Errorf("download failed after %d attempts: %w", maxAttempts, lastErr)
}

func sleepBackoff(ctx context.Context, attempt int) error {
	wait := time.Duration(attempt) * time.Second
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- app-managed partial-download path under the user-specified output dir.
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// recordDownload upserts one completed (or failed) download into the
// printgoat_downloads ledger, keyed on (source, model_id, file_name) so a
// re-run of the same download overwrites its prior row instead of
// accumulating duplicates.
func recordDownload(ctx context.Context, db *store.Store, row downloadResultRow) error {
	_, err := db.DB().ExecContext(ctx, `
INSERT INTO printgoat_downloads
	(source, model_id, model_name, file_name, file_url, local_path, file_size, bytes_downloaded, status, sha256, downloaded_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(source, model_id, file_name) DO UPDATE SET
	model_name = excluded.model_name,
	file_url = excluded.file_url,
	local_path = excluded.local_path,
	file_size = excluded.file_size,
	bytes_downloaded = excluded.bytes_downloaded,
	status = excluded.status,
	sha256 = excluded.sha256,
	downloaded_at = excluded.downloaded_at
`,
		row.Source, row.ModelID, nullableString(row.ModelName), row.FileName, nullableString(row.FileURL),
		row.LocalPath, row.FileSize, row.BytesDownloaded, row.Status, nullableString(row.SHA256), row.DownloadedAt)
	return err
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func outputDownloadResults(cmd *cobra.Command, flags *rootFlags, results []downloadResultRow) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if len(results) == 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "No files downloaded.")
		} else if err := printAutoTable(cmd.OutOrStdout(), downloadResultsToRows(results)); err != nil {
			return err
		}
	} else {
		raw, err := marshalJSONNoEscape(results)
		if err != nil {
			return err
		}
		if err := printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": "live"}); err != nil {
			return err
		}
	}
	if n := countFailedDownloads(results); n > 0 {
		return apiErr(fmt.Errorf("%d of %d file(s) failed to download; see warnings above", n, len(results)))
	}
	return nil
}

func downloadResultsToRows(results []downloadResultRow) []map[string]any {
	rows := make([]map[string]any, len(results))
	for i, r := range results {
		row := map[string]any{
			"source":           r.Source,
			"model_id":         r.ModelID,
			"file_name":        r.FileName,
			"local_path":       r.LocalPath,
			"bytes_downloaded": r.BytesDownloaded,
			"status":           r.Status,
			"downloaded_at":    r.DownloadedAt,
		}
		if r.FileSize != 0 {
			row["file_size"] = r.FileSize
		}
		if r.SHA256 != "" {
			row["sha256"] = r.SHA256
		}
		rows[i] = row
	}
	return rows
}

func countFailedDownloads(results []downloadResultRow) int {
	n := 0
	for _, r := range results {
		if r.Status != "complete" {
			n++
		}
	}
	return n
}
