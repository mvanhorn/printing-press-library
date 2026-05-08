// Download command — download files from war.gov medialink URLs.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/store"

	"github.com/spf13/cobra"
)

func newDownloadCmd(flags *rootFlags) *cobra.Command {
	var flagAgency string
	var flagType string
	var outputDir string
	var resume bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download declassified UAP files from war.gov",
		Long: `Download files from war.gov medialink URLs to your local machine.
Supports filtering by agency and type, resume for interrupted downloads,
and progress tracking.

Note: war.gov is behind Akamai CDN and may return 403 on direct HTTP requests.
If you encounter 403 errors, you may need to import browser cookies.`,
		Example: `  # Download all files
  ufo-goat-pp-cli download

  # Download only FBI PDFs
  ufo-goat-pp-cli download --agency FBI --type PDF

  # Download to a specific directory
  ufo-goat-pp-cli download --output-dir ~/ufo-archive

  # Resume interrupted downloads
  ufo-goat-pp-cli download --resume`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("ufo-goat-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ufo-goat-pp-cli sync' first.", err)
			}
			defer db.Close()
			_ = db.EnsureUFOSchema()

			count, _ := db.GetFileCount()
			if count == 0 {
				return fmt.Errorf("no files in local store. Run 'ufo-goat-pp-cli sync' first")
			}

			// Resolve output directory
			if outputDir == "" {
				home, _ := os.UserHomeDir()
				outputDir = filepath.Join(home, "ufo-files")
			}
			if err := os.MkdirAll(outputDir, 0o755); err != nil {
				return fmt.Errorf("creating output directory: %w", err)
			}

			// Get files matching filter
			filter := store.FileFilter{
				Agency: flagAgency,
				Type:   flagType,
			}
			files, err := db.ListUFOFiles(filter)
			if err != nil {
				return err
			}

			// Filter to files with download URLs
			var downloadable []store.UFOFile
			for _, f := range files {
				if f.DownloadURL == "" {
					continue
				}
				if resume && f.Downloaded {
					continue
				}
				downloadable = append(downloadable, f)
			}

			if len(downloadable) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No files to download.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Downloading %d files to %s\n\n", len(downloadable), outputDir)

			var downloaded, failed, skipped int
			started := time.Now()

			for i, f := range downloadable {
				// Build filename from title
				filename := sanitizeFilename(f.Title)
				ext := ".pdf"
				switch f.Type {
				case "VID":
					ext = ".mp4"
				case "IMG":
					ext = ".jpg"
				}
				if !strings.HasSuffix(strings.ToLower(filename), ext) {
					filename += ext
				}

				destPath := filepath.Join(outputDir, f.Agency, filename)

				// Skip if already exists and resume mode
				if resume {
					if info, err := os.Stat(destPath); err == nil && info.Size() > 0 {
						skipped++
						continue
					}
				}

				fmt.Fprintf(os.Stderr, "[%d/%d] %s... ", i+1, len(downloadable), truncate(f.Title, 40))

				err := downloadFile(cmd.Context(), f.DownloadURL, destPath)
				if err != nil {
					if strings.Contains(err.Error(), "403") {
						fmt.Fprintf(os.Stderr, "403 Forbidden (Akamai CDN block)\n")
						fmt.Fprintf(os.Stderr, "  hint: war.gov is behind Akamai CDN. Try importing browser cookies.\n")
					} else {
						fmt.Fprintf(os.Stderr, "error: %v\n", err)
					}
					failed++
					continue
				}

				fmt.Fprintf(os.Stderr, "done\n")
				_ = db.MarkDownloaded(f.ID)
				downloaded++
			}

			elapsed := time.Since(started)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"downloaded": downloaded,
					"failed":     failed,
					"skipped":    skipped,
					"output_dir": outputDir,
					"duration_s": elapsed.Seconds(),
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nDownload complete: %d downloaded, %d failed, %d skipped (%.1fs)\n",
				downloaded, failed, skipped, elapsed.Seconds())
			if failed > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "hint: %d files failed. war.gov may block direct downloads. Try importing browser cookies.\n", failed)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagAgency, "agency", "", "Filter by agency (DoD, FBI, NASA, State)")
	cmd.Flags().StringVar(&flagType, "type", "", "Filter by file type (PDF, VID, IMG)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory (default: ~/ufo-files/)")
	cmd.Flags().BoolVar(&resume, "resume", false, "Skip files that already exist on disk")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}

func downloadFile(ctx context.Context, url, destPath string) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func sanitizeFilename(s string) string {
	// Replace characters that aren't safe in filenames
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	s = replacer.Replace(s)
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
