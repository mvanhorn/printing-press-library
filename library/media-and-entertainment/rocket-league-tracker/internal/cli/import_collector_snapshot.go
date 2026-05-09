// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/rocket-league-tracker/internal/store"

	"github.com/spf13/cobra"
)

// collectorSnapshot is the JSON shape exported by the rocket-league-stats
// `apps/collector` process. We tolerate a few field-name variants because
// the collector's wire format has evolved.
type collectorSnapshot struct {
	PlayerID    string                      `json:"playerId"`
	Platform    string                      `json:"platform"`
	DisplayName string                      `json:"displayName"`
	Playlists   []collectorPlaylistSnapshot `json:"playlists"`
	CapturedAt  string                      `json:"capturedAt,omitempty"`
}

type collectorPlaylistSnapshot struct {
	Key        string `json:"key"`
	Tier       string `json:"tier"`
	Division   string `json:"division"`
	MMR        int    `json:"mmr"`
	Games      int    `json:"games"`
	Streak     int    `json:"streak"`
	CapturedAt string `json:"captured_at,omitempty"`
}

// newImportCollectorSnapshotCmd implements `import-collector-snapshot <path>` —
// reads a JSON snapshot exported by the collector and upserts player + rank
// rows into the local SQLite store, keyed by storage_key (platform:playerId).
func newImportCollectorSnapshotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "import-collector-snapshot <path>",
		Short:       "Import a JSON snapshot exported by rocket-league-stats apps/collector.",
		Long:        "Reads the JSON file at <path> and upserts player + rank rows into the local SQLite store. Each playlist becomes its own rank row keyed by `<storage_key>:<playlist>` so trend/peek/etc. can read multiple snapshots over time.",
		Example:     "  rocket-league-tracker-pp-cli import-collector-snapshot ./snapshot.json",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			path := args[0]
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}
			var snap collectorSnapshot
			if err := json.Unmarshal(raw, &snap); err != nil {
				return fmt.Errorf("parsing snapshot: %w", err)
			}
			if snap.PlayerID == "" {
				return usageErr(fmt.Errorf("snapshot missing playerId"))
			}
			canonical := normalizePlatform(snap.Platform)
			if canonical == "" {
				canonical = "epic"
			}
			key := storageKey(canonical, snap.PlayerID)

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("rocket-league-tracker-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()

			// Player row.
			playerObj := map[string]any{
				"id":        key,
				"player_id": snap.PlayerID,
				"name":      snap.DisplayName,
				"tag":       snap.Platform,
				"presence":  "",
			}
			playerJSON, _ := json.Marshal(playerObj)
			if err := db.UpsertPlayer(playerJSON); err != nil {
				return fmt.Errorf("upsert player: %w", err)
			}

			// Determine the snapshot timestamp. Per-playlist captured_at takes
			// precedence; otherwise the snapshot-level value; otherwise now.
			fallback := time.Now().UTC().Format(time.RFC3339)
			if snap.CapturedAt != "" {
				fallback = snap.CapturedAt
			}

			imported := 0
			for _, p := range snap.Playlists {
				captured := p.CapturedAt
				if captured == "" {
					captured = fallback
				}
				rankID := fmt.Sprintf("%s:%s:%s", key, strings.ToLower(p.Key), captured)
				rankObj := map[string]any{
					"id":          rankID,
					"player_id":   snap.PlayerID,
					"name":        snap.DisplayName,
					"playlist":    p.Key,
					"tier":        p.Tier,
					"division":    p.Division,
					"mmr":         p.MMR,
					"games":       p.Games,
					"streak":      p.Streak,
					"captured_at": captured,
				}
				rankJSON, _ := json.Marshal(rankObj)
				if err := db.UpsertRank(rankJSON); err != nil {
					return fmt.Errorf("upsert rank %s: %w", rankID, err)
				}
				imported++
			}

			out := map[string]any{
				"player_id":   snap.PlayerID,
				"platform":    canonical,
				"storage_key": key,
				"imported":    imported,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported %d playlists for %s (%s)\n", imported, snap.DisplayName, key)
			return nil
		},
	}
	return cmd
}
