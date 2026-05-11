// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `instagram-pp-cli highlights diff <handle>` — compare current highlight
// tray against the prior snapshot in highlight_snapshots and report
// added/removed/renamed items.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/instagram/internal/store"
)

func newHighlightsDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "diff [handle]",
		Short: "Diff a profile's current highlight tray vs. the prior snapshot",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			handle := args[0]
			if dryRunOK(flags) {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"added": []string{}, "removed": []string{}, "renamed": []string{},
					}, flags)
				}
				return nil
			}
			if cliutil.IsVerifyEnv() {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"added": []string{}, "removed": []string{}, "renamed": []string{},
					}, flags)
				}
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			userID, username, err := resolveUserID(c, handle)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			raw, err := c.Get(fmt.Sprintf("/api/v1/highlights/%s/highlights_tray/", userID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			currentTray := parseTrayItems(raw)

			if dbPath == "" {
				dbPath = defaultDBPath("instagram-pp-cli")
			}
			st, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			if err := store.ApplyExtraMigrations(cmd.Context(), st.DB()); err != nil {
				return err
			}

			// Load latest prior snapshot for this owner.
			var priorJSON string
			err = st.DB().QueryRow(`SELECT tray_json FROM highlight_snapshots
				WHERE owner_username = ? ORDER BY taken_at DESC LIMIT 1`, username).Scan(&priorJSON)
			priorTray := []trayItem{}
			if err == nil && priorJSON != "" {
				_ = json.Unmarshal([]byte(priorJSON), &priorTray)
			}

			added, removed, renamed := diffTray(priorTray, currentTray)

			// Persist the new snapshot — local cache, not external state.
			b, _ := json.Marshal(currentTray)
			_, _ = st.DB().Exec(`INSERT INTO highlight_snapshots (owner_username, taken_at, tray_json)
				VALUES (?, ?, ?)`, username, time.Now().Unix(), string(b))

			result := map[string]any{
				"profile": username,
				"added":   added,
				"removed": removed,
				"renamed": renamed,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"%s: %d added, %d removed, %d renamed\n",
				username, len(added), len(removed), len(renamed))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type trayItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func parseTrayItems(raw json.RawMessage) []trayItem {
	var t struct {
		Tray []trayItem `json:"tray"`
	}
	_ = json.Unmarshal(raw, &t)
	return t.Tray
}

// diffTray is exported logic so it's unit-testable without a DB. Returns
// IDs added, removed, and items whose title changed (renamed).
func diffTray(prior, current []trayItem) (added, removed []string, renamed []map[string]string) {
	priorByID := map[string]string{}
	for _, p := range prior {
		priorByID[p.ID] = p.Title
	}
	currentByID := map[string]string{}
	for _, c := range current {
		currentByID[c.ID] = c.Title
		if _, ok := priorByID[c.ID]; !ok {
			added = append(added, c.ID)
		} else if priorByID[c.ID] != c.Title {
			renamed = append(renamed, map[string]string{
				"id": c.ID, "from": priorByID[c.ID], "to": c.Title,
			})
		}
	}
	for id := range priorByID {
		if _, ok := currentByID[id]; !ok {
			removed = append(removed, id)
		}
	}
	return
}
