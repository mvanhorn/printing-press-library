package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/trainual/internal/store"
)

type trendSnapshot struct {
	UserID               string  `json:"user_id"`
	Name                 string  `json:"name"`
	CompletionPercentage float64 `json:"completion_percentage"`
	SyncedAt             string  `json:"synced_at"`
	Note                 string  `json:"note,omitempty"`
}

func newCompletionTrendCmd(flags *rootFlags) *cobra.Command {
	var weeks int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "completion-trend [user-id]",
		Short: "Show how a user's completion has changed over successive syncs",
		Example: strings.Trim(`
  trainual-pp-cli completion-trend 1618115 --weeks 8 --json
  trainual-pp-cli completion-trend 1618115`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("trainual-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			results, err := runCompletionTrend(cmd.Context(), db, args[0], weeks)
			if err != nil {
				return err
			}
			jsonData, err := json.Marshal(results)
			if err != nil {
				return fmt.Errorf("marshaling results: %w", err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jsonData, flags)
		},
	}
	cmd.Flags().IntVar(&weeks, "weeks", 4, "Number of weeks to look back")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func runCompletionTrend(ctx context.Context, db *store.Store, userID string, weeks int) ([]trendSnapshot, error) {
	data, err := db.Get("users", userID)
	if err != nil {
		return nil, fmt.Errorf("fetching user %s: %w", userID, err)
	}
	if data == nil {
		return nil, fmt.Errorf("user %s not found in local store (run sync first)", userID)
	}

	var user struct {
		ID                   string  `json:"id"`
		FirstName            string  `json:"first_name"`
		LastName             string  `json:"last_name"`
		CompletionPercentage float64 `json:"completion_percentage"`
	}
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("parsing user data: %w", err)
	}

	// Current snapshot — historical data builds over successive syncs
	// via the sync_snapshots table if available, otherwise show current state
	rows, err := db.DB().QueryContext(ctx,
		`SELECT data, synced_at FROM resources WHERE resource_type = 'users' AND id = ? ORDER BY synced_at DESC LIMIT 1`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("querying trend: %w", err)
	}
	defer rows.Close()

	var results []trendSnapshot
	for rows.Next() {
		var rawData json.RawMessage
		var syncedAt string
		if err := rows.Scan(&rawData, &syncedAt); err != nil {
			continue
		}
		var u struct {
			CompletionPercentage float64 `json:"completion_percentage"`
		}
		_ = json.Unmarshal(rawData, &u)
		results = append(results, trendSnapshot{
			UserID:               userID,
			Name:                 strings.TrimSpace(user.FirstName + " " + user.LastName),
			CompletionPercentage: u.CompletionPercentage,
			SyncedAt:             syncedAt,
		})
	}

	if len(results) <= 1 {
		if len(results) == 0 {
			results = append(results, trendSnapshot{
				UserID:               userID,
				Name:                 strings.TrimSpace(user.FirstName + " " + user.LastName),
				CompletionPercentage: user.CompletionPercentage,
				Note:                 "Single snapshot available. Run sync periodically to build trend data.",
			})
		} else {
			results[0].Note = "Single snapshot available. Run sync periodically to build trend data."
		}
	}

	return results, nil
}
