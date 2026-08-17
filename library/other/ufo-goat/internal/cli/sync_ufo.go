// Custom sync command that fetches the CSV manifest from GitHub.
package cli

import (
	"context"
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
	var sourceName string
	var manifestURL string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync the UAP file manifest to local SQLite (configurable source)",
		Long: `Fetch the CSV manifest of declassified UAP files from the PURSUE initiative
and store them locally for offline search, filtering, and analysis.

The source defaults to the community mirror that tracks every release tranche
(release_1, release_02, release_03, ...). Override it with --source <name>,
--manifest-url <url>, or the UFO_SOURCE / UFO_MANIFEST_URL environment vars.
Run 'ufo-goat-pp-cli sources' to list known sources.

Incremental by default — re-running updates only changed records.
Use --full to clear and re-download everything.`,
		Example: `  # Sync from the default (all-releases) source
  ufo-goat-pp-cli sync

  # Sync from a specific named source
  ufo-goat-pp-cli sync --source legacy

  # Sync from a custom mirror URL
  ufo-goat-pp-cli sync --manifest-url https://example.com/uap.csv

  # Full resync (re-download everything)
  ufo-goat-pp-cli sync --full`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("ufo-goat-pp-cli")
			}

			resolvedURL, resolvedSource, err := manifest.ResolveSource(manifestURL, sourceName, "")
			if err != nil {
				return usageErr(err)
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
					fmt.Fprintf(os.Stderr, "Full resync requested. Fetching all files, then replacing the local store...\n")
				}
			}

			started := time.Now()

			// Fetch manifest from the resolved source.
			if humanFriendly {
				fmt.Fprintf(os.Stderr, "Fetching manifest from %s source (%s)...\n", resolvedSource, resolvedURL)
			}
			files, stored, err := fetchAndStoreManifest(cmd.Context(), db, resolvedURL, full)
			if err != nil {
				return err
			}

			if humanFriendly {
				fmt.Fprintf(os.Stderr, "Parsed %d files from CSV manifest\n", len(files))
			}

			// Detect newly-landed release tranches. The government publishes
			// files in batches (release_1, release_2, ...); compare the batches
			// present now against the set we recorded on a previous sync so we
			// can report "Release N just dropped" independent of sync timing.
			newBatches := detectNewBatches(db)

			elapsed := time.Since(started)

			// Build agency breakdown
			agencyCounts := map[string]int{}
			for _, f := range files {
				agencyCounts[f.Agency]++
			}

			latestBatch, _ := db.GetMaxReleaseBatch()

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"event":        "sync_complete",
					"total_files":  stored,
					"agencies":     agencyCounts,
					"latest_batch": latestBatch,
					"new_batches":  newBatches,
					"duration_ms":  elapsed.Milliseconds(),
					"source":       resolvedSource,
					"source_url":   resolvedURL,
				})
			}

			// Build human-friendly summary
			summary := formatAgencySummary(agencyCounts)
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d files (%s) in %.1fs\n", stored, summary, elapsed.Seconds())
			if len(newBatches) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n🛸 New release tranche detected: %s\n", formatBatchList(newBatches))
				fmt.Fprintf(cmd.OutOrStdout(), "   Run 'ufo-goat-pp-cli new' or 'ufo-goat-pp-cli releases' to see what landed.\n")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "Full resync (re-download everything)")
	cmd.Flags().StringVar(&sourceName, "source", "", "Named manifest source (see 'sources'); overrides the default. Env: UFO_SOURCE")
	cmd.Flags().StringVar(&manifestURL, "manifest-url", "", "Custom manifest CSV URL; overrides --source and the default. Env: UFO_MANIFEST_URL")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/ufo-goat-pp-cli/data.db)")

	return cmd
}

// fetchAndStoreManifest downloads the CSV manifest from manifestURL, upserts
// every file into the local store, and rebuilds the FTS index. Shared by 'sync'
// and 'releases check'. Pass "" to use the default source.
func fetchAndStoreManifest(ctx context.Context, db *store.Store, manifestURL string, replace bool) ([]manifest.File, int, error) {
	files, err := manifest.FetchManifest(ctx, manifestURL)
	if err != nil {
		return nil, 0, fmt.Errorf("fetching manifest: %w", err)
	}
	if len(files) == 0 {
		return nil, 0, fmt.Errorf("manifest returned 0 files — check the CSV URL")
	}

	// Full resync: clear the store only AFTER a successful fetch, so a network
	// failure never leaves the user with an empty store.
	if replace {
		if err := db.ClearFiles(); err != nil {
			return nil, 0, fmt.Errorf("clearing store for full resync: %w", err)
		}
	}

	storeFiles := make([]store.UFOFile, len(files))
	for i, f := range files {
		storeFiles[i] = store.UFOFile{
			ID:               f.ID,
			Title:            f.Title,
			Type:             f.Type,
			Agency:           f.Agency,
			ReleaseDate:      f.ReleaseDate,
			ReleaseBatch:     f.ReleaseBatch,
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
		return nil, 0, fmt.Errorf("storing files: %w", err)
	}

	if err := db.RebuildFTS(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: FTS index rebuild failed: %v\n", err)
	}

	_ = db.SaveSyncState("files", "", stored)

	return files, stored, nil
}

// detectNewBatches compares the release tranches present after this sync
// against the set recorded on a previous sync, records the updated set, and
// returns the tranches that are newly present. On the first tracked sync the
// seen set is empty, so all current tranches are reported once.
func detectNewBatches(db *store.Store) []int {
	current, err := db.GetReleaseBatches()
	if err != nil {
		return nil
	}

	seenCursor, _, _, _ := db.GetSyncState("releases_seen")
	seen := map[int]bool{}
	if seenCursor != "" {
		var arr []int
		if json.Unmarshal([]byte(seenCursor), &arr) == nil {
			for _, b := range arr {
				seen[b] = true
			}
		}
	}

	var fresh []int
	for _, b := range current {
		if !seen[b] {
			fresh = append(fresh, b)
			seen[b] = true
		}
	}

	// Persist the updated seen set.
	union := make([]int, 0, len(seen))
	for b := range seen {
		union = append(union, b)
	}
	sort.Ints(union)
	if data, err := json.Marshal(union); err == nil {
		_ = db.SaveSyncState("releases_seen", string(data), len(union))
	}

	return fresh
}

func formatBatchList(batches []int) string {
	parts := make([]string, len(batches))
	for i, b := range batches {
		parts[i] = fmt.Sprintf("Release %d", b)
	}
	return strings.Join(parts, ", ")
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
