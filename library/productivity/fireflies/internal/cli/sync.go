// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/gql"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/store"

	"github.com/spf13/cobra"
)

// transcriptListQuery fetches transcript metadata for sync.
// NOTE: from_date/toDate API params are documented as unreliable in Fireflies —
// the API frequently ignores them. We always fetch without date filters and
// apply date constraints client-side using the epoch-ms `date` field.
const transcriptListQuery = `
query Transcripts($limit: Int, $skip: Int) {
  transcripts(limit: $limit, skip: $skip) {
    id
    title
    date
    dateString
    duration
    privacy
    transcript_url
    organizer_email
    host_email
    participants
    speakers { id name }
    meeting_info { fred_joined silent_meeting summary_status }
    summary {
      action_items
      keywords
      overview
      shorthand_bullet
      gist
      topics_discussed
    }
    meeting_attendees { displayName email }
    channels { id title }
  }
}`

const channelsQuery = `query { channels { id title is_private created_by created_at updated_at } }`
const usersQuery = `query { users { user_id email name num_transcripts is_admin integrations } }`

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var full bool
	var dbPath string
	var resources []string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Fireflies transcripts to local SQLite for offline search and analysis",
		Long: `Pull transcripts, channels, and users from the Fireflies API into a local SQLite
database. Once synced, the search, find, action-items, topics, decisions, and
person commands work offline without consuming API quota.

The Fireflies API ignores the from_date filter. This CLI always fetches without
a date filter and applies incremental cutoffs client-side using the epoch-ms
date field on each transcript.`,
		Example: strings.Trim(`
  fireflies-pp-cli sync
  fireflies-pp-cli sync --full
  fireflies-pp-cli sync --resources transcripts,channels`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"event":"sync_complete","total":0}`)
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			client, err := gql.New(cfg)
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			syncAll := len(resources) == 0
			doResource := func(name string) bool {
				if syncAll {
					return true
				}
				for _, r := range resources {
					if r == name {
						return true
					}
				}
				return false
			}

			totalSynced := 0

			if doResource("transcripts") {
				n, err := syncTranscripts(cmd, client, db, full)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), `{"event":"sync_error","resource":"transcripts","error":%q}`+"\n", err.Error())
				} else {
					totalSynced += n
					fmt.Fprintf(cmd.OutOrStdout(), `{"event":"sync_complete","resource":"transcripts","total":%d}`+"\n", n)
				}
			}

			if doResource("channels") {
				n, err := syncChannels(cmd, client, db)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), `{"event":"sync_error","resource":"channels","error":%q}`+"\n", err.Error())
				} else {
					totalSynced += n
					fmt.Fprintf(cmd.OutOrStdout(), `{"event":"sync_complete","resource":"channels","total":%d}`+"\n", n)
				}
			}

			if doResource("users") {
				n, err := syncUsers(cmd, client, db)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), `{"event":"sync_error","resource":"users","error":%q}`+"\n", err.Error())
				} else {
					totalSynced += n
					fmt.Fprintf(cmd.OutOrStdout(), `{"event":"sync_complete","resource":"users","total":%d}`+"\n", n)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), `{"event":"sync_summary","total":%d}`+"\n", totalSynced)
			return nil
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "Full resync — ignore incremental cursor and re-fetch everything")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/fireflies-pp-cli/data.db)")
	cmd.Flags().StringSliceVar(&resources, "resources", nil, "Resources to sync: transcripts,channels,users (default: all)")
	return cmd
}

func syncTranscripts(cmd *cobra.Command, client *gql.Client, db *store.Store, full bool) (int, error) {
	// Get incremental cursor: timestamp of newest transcript already stored.
	// Client-side filtering only — from_date API param is unreliable.
	var sinceEpochMs float64
	if !full {
		_, lastSynced, _, _ := db.GetSyncState("transcripts")
		if !lastSynced.IsZero() {
			sinceEpochMs = float64(lastSynced.UnixMilli())
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), `{"event":"sync_start","resource":"transcripts"}`+"\n")

	const pageSize = 50
	skip := 0
	total := 0
	var newestDate float64
	done := false

	for !done {
		data, err := client.Query(cmd.Context(), transcriptListQuery, map[string]any{
			"limit": pageSize,
			"skip":  skip,
		}, "transcripts")
		if err != nil {
			return total, fmt.Errorf("fetching transcripts (skip=%d): %w", skip, err)
		}

		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			return total, fmt.Errorf("parsing transcripts: %w", err)
		}
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			var meta struct {
				ID   string  `json:"id"`
				Date float64 `json:"date"`
			}
			if err := json.Unmarshal(item, &meta); err != nil {
				continue
			}

			// Incremental: API returns transcripts sorted by date desc.
			// Stop paging when we reach items older than the last sync.
			if sinceEpochMs > 0 && meta.Date > 0 && meta.Date < sinceEpochMs {
				done = true
				break
			}

			if err := db.UpsertTranscripts(item); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), `{"event":"upsert_error","id":%q,"error":%q}`+"\n", meta.ID, err.Error())
				continue
			}
			total++
			if meta.Date > newestDate {
				newestDate = meta.Date
			}
		}

		if len(items) < pageSize {
			break
		}
		skip += pageSize

		if total > 0 && total%200 == 0 {
			fmt.Fprintf(cmd.OutOrStdout(), `{"event":"sync_progress","resource":"transcripts","fetched":%d}`+"\n", total)
		}
	}

	cursor := ""
	if newestDate > 0 {
		t := time.UnixMilli(int64(newestDate))
		cursor = t.UTC().Format(time.RFC3339)
	}
	_ = db.SaveSyncState("transcripts", cursor, total)
	return total, nil
}

func syncChannels(cmd *cobra.Command, client *gql.Client, db *store.Store) (int, error) {
	data, err := client.Query(cmd.Context(), channelsQuery, nil, "channels")
	if err != nil {
		return 0, fmt.Errorf("fetching channels: %w", err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return 0, fmt.Errorf("parsing channels: %w", err)
	}
	total := 0
	for _, item := range items {
		if err := db.UpsertChannels(item); err != nil {
			continue
		}
		total++
	}
	_ = db.SaveSyncState("channels", "", total)
	return total, nil
}

func syncUsers(cmd *cobra.Command, client *gql.Client, db *store.Store) (int, error) {
	data, err := client.Query(cmd.Context(), usersQuery, nil, "users")
	if err != nil {
		return 0, fmt.Errorf("fetching users: %w", err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return 0, fmt.Errorf("parsing users: %w", err)
	}
	total := 0
	for _, item := range items {
		if err := db.UpsertUsers(item); err != nil {
			continue
		}
		total++
	}
	_ = db.SaveSyncState("users", "", total)
	return total, nil
}
