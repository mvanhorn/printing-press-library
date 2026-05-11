package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/productivity/chainels/internal/store"
	"github.com/spf13/cobra"
)

type alarmDiffResult struct {
	A          string   `json:"a"`
	B          string   `json:"b"`
	OnlyInA    []string `json:"only_in_a"`
	OnlyInB    []string `json:"only_in_b"`
	Common     []string `json:"common"`
	TotalA     int      `json:"total_a"`
	TotalB     int      `json:"total_b"`
}

func newAlarmsDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "diff <community-a> <community-b>",
		Short:   "Diff alarm recipient sets between two communities",
		Long:    "Reads the local store synced by `chainels-pp-cli sync`. Compares alarm-configuration recipient ids between community A and community B; output is a three-way split (only_in_a, only_in_b, common). Requires `send.alarm` scope at sync time to populate.",
		Example: "  chainels-pp-cli alarms diff <community-a-id> <community-b-id> --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				return fmt.Errorf("usage: alarms diff <community-a> <community-b>")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("chainels-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			a, err := alarmRecipientsForCommunity(cmd.Context(), db.DB(), args[0])
			if err != nil {
				return fmt.Errorf("loading community a: %w", err)
			}
			b, err := alarmRecipientsForCommunity(cmd.Context(), db.DB(), args[1])
			if err != nil {
				return fmt.Errorf("loading community b: %w", err)
			}
			result := alarmDiffResult{
				A:       args[0],
				B:       args[1],
				OnlyInA: setDiff(a, b),
				OnlyInB: setDiff(b, a),
				Common:  setIntersect(a, b),
				TotalA:  len(a),
				TotalB:  len(b),
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path")
	return cmd
}

func alarmRecipientsForCommunity(ctx interface{}, db *sql.DB, community string) (map[string]struct{}, error) {
	// Recipients are stored as a JSON array inside the alarm row payload; the
	// generator does not surface them as a column, so we walk alarms tied to
	// the requested community and union the ids.
	rows, err := db.Query(`
		SELECT data FROM alarms
		WHERE json_extract(data,'$.community_id') = ?
		   OR json_extract(data,'$.company_id')   = ?`, community, community)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var payload struct {
			Recipients []struct {
				AccountID string `json:"account_id"`
				ID        string `json:"id"`
			} `json:"recipients"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			continue
		}
		for _, r := range payload.Recipients {
			id := r.AccountID
			if id == "" {
				id = r.ID
			}
			if id != "" {
				out[id] = struct{}{}
			}
		}
	}
	return out, nil
}

func setDiff(a, b map[string]struct{}) []string {
	out := make([]string, 0)
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func setIntersect(a, b map[string]struct{}) []string {
	out := make([]string, 0)
	for k := range a {
		if _, ok := b[k]; ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
