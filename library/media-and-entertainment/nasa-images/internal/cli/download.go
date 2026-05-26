// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written by the Printing Press operator on top of generated scaffolding.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/nasa-images/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/nasa-images/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/nasa-images/internal/store"

	"github.com/spf13/cobra"
)

func newDownloadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "download",
		Short: "Bulk download NASA assets with byte-range resume",
		Long: `Download NASA assets to local disk with a per-file ledger and byte-range
resume — re-runs of the same command skip completed files and resume in-flight
transfers from the last completed byte.`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newDownloadAlbumCmd(flags))
	return cmd
}

func newDownloadAlbumCmd(flags *rootFlags) *cobra.Command {
	var (
		out, variant, dbPath string
		maxItems             int
		// resume is accepted for documentation clarity; the ledger always tracks
		// per-file progress and resumes interrupted transfers automatically.
		// Passing --resume=false is not honored — set --out to a fresh directory
		// if you want a clean download.
		resume bool
	)
	cmd := &cobra.Command{
		Use:   "album [album_name]",
		Short: "Bulk-download every asset in a curated NASA album with byte-range resume",
		Long: `Walk a NASA album (e.g. Apollo-at-50, Mars-Perseverance), fetch each
asset's rendition manifest, pick the chosen --variant (default: large),
and download to --out with byte-range resume.

The downloads ledger lives in the local SQLite store; re-runs of the same
command skip files marked 'completed' and resume any 'in_flight' transfers
from the last completed byte (via HTTP Range requests).`,
		Example:     "  nasa-images-pp-cli download album Apollo-at-50 --variant orig --out ./apollo",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			album := args[0]
			if variant == "" {
				variant = "large"
			}
			if out == "" {
				out = filepath.Join(".", strings.ReplaceAll(album, string(os.PathSeparator), "_"))
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would download album %q (variant=%s, resume=%t) to %s\n", album, variant, resume, out)
				return nil
			}
			// Live-dogfood guard: bound work in dogfood by curtailing maxItems.
			if cliutil.IsDogfoodEnv() && (maxItems == 0 || maxItems > 2) {
				maxItems = 2
			}

			ctx := cmd.Context()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := openNasaStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			if err := os.MkdirAll(out, 0o755); err != nil {
				return fmt.Errorf("creating output dir: %w", err)
			}

			// Walk the album to collect nasa_ids.
			ids, perr := walkAlbumIDs(ctx, c, album, maxItems)
			if perr != nil {
				return perr
			}
			if len(ids) == 0 {
				return fmt.Errorf("album %q returned no items (check the case-sensitive name)", album)
			}

			downloaded := 0
			skipped := 0
			errored := 0
			for _, nasaID := range ids {
				url, vresolved, perr := pickVariantURL(ctx, c, nasaID, variant)
				if perr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "WARN %s: %v\n", nasaID, perr)
					errored++
					continue
				}
				localPath := filepath.Join(out, fmt.Sprintf("%s.%s", sanitizeFilename(nasaID), urlExt(url)))
				status, bytesDL, dlErr := resumeOrDownload(ctx, flags, s, nasaID, vresolved, url, localPath)
				switch status {
				case "completed":
					downloaded++
					fmt.Fprintf(cmd.OutOrStdout(), "OK   %s -> %s (%d bytes)\n", nasaID, localPath, bytesDL)
				case "skipped":
					skipped++
					fmt.Fprintf(cmd.OutOrStdout(), "SKIP %s (already downloaded)\n", nasaID)
				default:
					errored++
					fmt.Fprintf(cmd.ErrOrStderr(), "ERR  %s: %v\n", nasaID, dlErr)
				}
			}

			summary := map[string]any{
				"album":      album,
				"variant":    variant,
				"out":        out,
				"requested":  len(ids),
				"downloaded": downloaded,
				"skipped":    skipped,
				"errored":    errored,
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.quiet) {
				return flags.printJSON(cmd, summary)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"\nAlbum %q: %d downloaded, %d skipped, %d errored (out of %d).\n",
				album, downloaded, skipped, errored, len(ids))
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "Output directory (default: ./<album_name>)")
	cmd.Flags().StringVar(&variant, "variant", "large", "Rendition variant to download (orig, large, medium, small, thumb)")
	cmd.Flags().IntVar(&maxItems, "max-items", 0, "Maximum items to download (0 = entire album)")
	cmd.Flags().BoolVar(&resume, "resume", true, "Resume from the local downloads ledger (skip completed, byte-range-resume in-flight). Always on; flag retained for clarity.")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nasa-images-pp-cli/data.db)")
	return cmd
}

// walkAlbumIDs walks /album/{name} pages and returns the nasa_id list (capped
// by maxItems when > 0).
func walkAlbumIDs(ctx context.Context, c *client.Client, album string, maxItems int) ([]string, error) {
	_ = ctx
	path := "/album/" + album
	params := map[string]string{}
	var ids []string
	for {
		raw, err := c.Get(path, params)
		if err != nil {
			return nil, fmt.Errorf("calling %s: %w", path, err)
		}
		coll, err := parseNasaCollection(raw)
		if err != nil {
			return nil, err
		}
		for _, item := range coll.Collection.Items {
			if len(item.Data) > 0 && item.Data[0].NasaID != "" {
				ids = append(ids, item.Data[0].NasaID)
				if maxItems > 0 && len(ids) >= maxItems {
					return ids, nil
				}
			}
		}
		next := nextPageFromLinks(coll.Collection.Links)
		if next == "" {
			break
		}
		newPath, newParams, perr := hrefPathAndParams(next)
		if perr != nil {
			return nil, perr
		}
		path = newPath
		params = newParams
	}
	return ids, nil
}

// pickVariantURL fetches /asset/{nasa_id}, classifies each entry, and returns
// the URL of the requested variant. Falls back to the closest larger variant
// when the exact match isn't present.
func pickVariantURL(ctx context.Context, c *client.Client, nasaID, want string) (string, string, error) {
	_ = ctx
	raw, err := c.Get("/asset/"+nasaID, nil)
	if err != nil {
		return "", "", fmt.Errorf("calling /asset/%s: %w", nasaID, err)
	}
	coll, err := parseNasaCollection(raw)
	if err != nil {
		return "", "", err
	}
	byVariant := make(map[string]string)
	for _, item := range coll.Collection.Items {
		if item.Href == "" {
			continue
		}
		kind := classifyVariant(item.Href)
		if _, dup := byVariant[kind]; dup {
			continue
		}
		byVariant[kind] = upgradeToHTTPS(item.Href)
	}
	if url, ok := byVariant[want]; ok {
		return url, want, nil
	}
	// Fallback order from requested toward larger (orig is highest).
	fallback := []string{"orig", "large", "medium", "small", "thumb", "mobile", "preview"}
	for _, v := range fallback {
		if url, ok := byVariant[v]; ok {
			return url, v, nil
		}
	}
	return "", "", fmt.Errorf("no usable variant in manifest")
}

// resumeOrDownload downloads url to localPath with byte-range resume.
// Uses an *http.Client honoring flags.timeout so a stalled CDN connection
// respects --timeout instead of hanging indefinitely.
// Returns ("completed"|"skipped"|"errored", bytes_downloaded, err).
func resumeOrDownload(ctx context.Context, flags *rootFlags, s *store.Store, nasaID, variant, url, localPath string) (string, int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	// Inspect the existing ledger row. ErrNoRows is expected on the first
	// run for a given (nasa_id, variant); any other Scan error is a real DB
	// problem and must surface, not silently cause a redownload.
	var existingStatus sql.NullString
	var existingBytes sql.NullInt64
	row := s.DB().QueryRowContext(ctx,
		`SELECT status, bytes_downloaded FROM downloads WHERE nasa_id = ? AND variant = ?`,
		nasaID, variant)
	if scanErr := row.Scan(&existingStatus, &existingBytes); scanErr != nil && scanErr != sql.ErrNoRows {
		return "errored", 0, fmt.Errorf("reading downloads ledger: %w", scanErr)
	}

	if existingStatus.Valid && existingStatus.String == "completed" {
		if fi, err := os.Stat(localPath); err == nil && fi.Size() > 0 {
			return "skipped", fi.Size(), nil
		}
		// File was removed externally; re-download.
	}

	// Insert or move to in_flight.
	if _, err := s.DB().ExecContext(ctx,
		`INSERT INTO downloads (nasa_id, variant, url, local_path, bytes_downloaded, bytes_total, status, started_at)
		 VALUES (?, ?, ?, ?, 0, 0, 'in_flight', ?)
		 ON CONFLICT(nasa_id, variant) DO UPDATE SET status='in_flight', url=excluded.url, local_path=excluded.local_path`,
		nasaID, variant, url, localPath, now); err != nil {
		return "errored", 0, fmt.Errorf("recording in_flight: %w", err)
	}

	// Probe existing file size for resume.
	var startBytes int64
	if fi, err := os.Stat(localPath); err == nil {
		startBytes = fi.Size()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "errored", startBytes, err
	}
	if startBytes > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", startBytes))
	}
	resp, err := httpClientForFlags(flags).Do(req)
	if err != nil {
		return "errored", startBytes, err
	}
	defer resp.Body.Close()

	// 206 = partial content (range honored); 200 = full response (range ignored).
	openFlag := os.O_CREATE | os.O_WRONLY
	switch resp.StatusCode {
	case http.StatusPartialContent:
		openFlag |= os.O_APPEND
	case http.StatusOK:
		openFlag |= os.O_TRUNC
		startBytes = 0
	case http.StatusRequestedRangeNotSatisfiable:
		// Existing file is already complete on server's side.
		_, _ = s.DB().ExecContext(ctx,
			`UPDATE downloads SET status='completed', bytes_total=bytes_downloaded, completed_at=? WHERE nasa_id=? AND variant=?`,
			now, nasaID, variant)
		return "completed", startBytes, nil
	default:
		return "errored", startBytes, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}

	f, err := os.OpenFile(localPath, openFlag, 0o644)
	if err != nil {
		return "errored", startBytes, fmt.Errorf("opening local file: %w", err)
	}
	// Explicit Close() below — must not defer too, or double-close.
	n, err := io.Copy(f, resp.Body)
	totalBytes := startBytes + n
	if err != nil {
		_ = f.Close()
		_, _ = s.DB().ExecContext(ctx,
			`UPDATE downloads SET bytes_downloaded=?, status='in_flight' WHERE nasa_id=? AND variant=?`,
			totalBytes, nasaID, variant)
		return "errored", totalBytes, fmt.Errorf("writing body: %w", err)
	}
	// Verify the server didn't truncate mid-stream. For 206 (partial content)
	// the Content-Length is the remaining bytes, not the total; for 200 it's
	// the full file size. A truncated body would leave the ledger marked
	// completed with a partial file — exactly the failure mode the resume
	// path was meant to prevent.
	expectedDelta := resp.ContentLength
	if expectedDelta > 0 && n != expectedDelta {
		_, _ = s.DB().ExecContext(ctx,
			`UPDATE downloads SET bytes_downloaded=?, status='in_flight' WHERE nasa_id=? AND variant=?`,
			totalBytes, nasaID, variant)
		return "errored", totalBytes, fmt.Errorf("truncated response: server reported Content-Length=%d, got %d", expectedDelta, n)
	}
	if closeErr := f.Close(); closeErr != nil {
		_, _ = s.DB().ExecContext(ctx,
			`UPDATE downloads SET bytes_downloaded=?, status='in_flight' WHERE nasa_id=? AND variant=?`,
			totalBytes, nasaID, variant)
		return "errored", totalBytes, fmt.Errorf("closing local file: %w", closeErr)
	}
	_, _ = s.DB().ExecContext(ctx,
		`UPDATE downloads SET bytes_downloaded=?, bytes_total=?, status='completed', completed_at=? WHERE nasa_id=? AND variant=?`,
		totalBytes, totalBytes, now, nasaID, variant)
	return "completed", totalBytes, nil
}

// sanitizeFilename strips path separators and reserved characters from a
// NASA ID so it's safe to use as a local filename component.
func sanitizeFilename(name string) string {
	const bad = `<>:"/\|?*` + "\x00"
	out := make([]rune, 0, len(name))
	for _, r := range name {
		if strings.ContainsRune(bad, r) {
			out = append(out, '_')
		} else {
			out = append(out, r)
		}
	}
	return strings.TrimSpace(string(out))
}

// urlExt returns the lowercase file extension from a URL path ("jpg", "mp4",
// "srt", "json"); returns "bin" when no extension is found.
func urlExt(rawURL string) string {
	// Strip query string.
	if i := strings.Index(rawURL, "?"); i > 0 {
		rawURL = rawURL[:i]
	}
	if i := strings.LastIndex(rawURL, "."); i > 0 && i < len(rawURL)-1 {
		ext := strings.ToLower(rawURL[i+1:])
		if len(ext) >= 2 && len(ext) <= 5 {
			return ext
		}
	}
	return "bin"
}
