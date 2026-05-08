// Custom sync command that fetches the CSV manifest from GitHub.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/manifest"
	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/store"

	"github.com/spf13/cobra"
)

func newUFOSyncCmd(flags *rootFlags) *cobra.Command {
	var full bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync the UAP file manifest from GitHub to local SQLite",
		Long: `Fetch the CSV manifest of declassified UAP files from the PURSUE initiative
(GitHub: DenisSergeevitch/UFO-USA) and store them locally for offline search,
filtering, and analysis.

Incremental by default — re-running updates only changed records.
Use --full to clear and re-download everything.`,
		Example: `  # Sync all files
  ufo-goat-pp-cli sync

  # Full resync (re-download everything)
  ufo-goat-pp-cli sync --full`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("ufo-goat-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			// Ensure the extended schema is in place
			if err := db.EnsureUFOSchema(); err != nil {
				return fmt.Errorf("ensuring schema: %w", err)
			}

			// Check if we already have data
			existingCount, _ := db.GetFileCount()
			if existingCount > 0 && !full {
				if humanFriendly {
					fmt.Fprintf(os.Stderr, "Found %d existing files. Fetching updates...\n", existingCount)
				}
			} else if full {
				if humanFriendly {
					fmt.Fprintf(os.Stderr, "Full resync requested. Fetching all files...\n")
				}
			}

			started := time.Now()

			// Fetch manifest from GitHub
			if humanFriendly {
				fmt.Fprintf(os.Stderr, "Fetching manifest from GitHub...\n")
			}
			files, err := manifest.FetchManifest(cmd.Context())
			if err != nil {
				return fmt.Errorf("fetching manifest: %w", err)
			}

			if len(files) == 0 {
				return fmt.Errorf("manifest returned 0 files — check the CSV URL")
			}

			if humanFriendly {
				fmt.Fprintf(os.Stderr, "Parsed %d files from CSV manifest\n", len(files))
			}

			// Convert manifest files to store format and upsert
			storeFiles := make([]store.UFOFile, len(files))
			for i, f := range files {
				storeFiles[i] = store.UFOFile{
					ID:               f.ID,
					Title:            f.Title,
					Type:             f.Type,
					Agency:           f.Agency,
					ReleaseDate:      f.ReleaseDate,
					IncidentDate:     f.IncidentDate,
					ParsedDate:       f.ParsedDate,
					IncidentLocation: f.IncidentLocation,
					Description:      f.Description,
					Redacted:         f.Redacted,
					DownloadURL:      f.DownloadURL,
					ThumbnailURL:     f.ThumbnailURL,
					DVIDSVideoID:     f.DVIDSVideoID,
					VideoTitle:       f.VideoTitle,
					VideoPairing:     f.VideoPairing,
					PDFPairing:       f.PDFPairing,
					ModalImage:       f.ModalImage,
					PDFImageLink:     f.PDFImageLink,
				}
			}

			stored, err := db.UpsertUFOFileBatch(storeFiles)
			if err != nil {
				return fmt.Errorf("storing files: %w", err)
			}

			// Rebuild FTS index
			if err := db.RebuildFTS(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: FTS index rebuild failed: %v\n", err)
			}

			// Save sync state
			_ = db.SaveSyncState("files", "", stored)

			elapsed := time.Since(started)

			// Build agency breakdown
			agencyCounts := map[string]int{}
			for _, f := range files {
				agencyCounts[f.Agency]++
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"event":       "sync_complete",
					"total_files": stored,
					"agencies":    agencyCounts,
					"duration_ms": elapsed.Milliseconds(),
					"source":      manifest.ManifestURL,
				})
			}

			// Build human-friendly summary
			summary := formatAgencySummary(agencyCounts)
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d files (%s) in %.1fs\n", stored, summary, elapsed.Seconds())
			return nil
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "Full resync (re-download everything)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/ufo-goat-pp-cli/data.db)")

	return cmd
}

func formatAgencySummary(counts map[string]int) string {
	type agencyCount struct {
		name  string
		count int
	}
	var sorted []agencyCount
	for name, count := range counts {
		sorted = append(sorted, agencyCount{name, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	var parts []string
	for _, ac := range sorted {
		parts = append(parts, fmt.Sprintf("%d %s", ac.count, ac.name))
	}
	return strings.Join(parts, ", ")
}
