package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"multimail-pp-cli/internal/store"
)

type trustRow struct {
	Mailbox       string `json:"mailbox"`
	MailboxID     string `json:"mailbox_id"`
	OversightMode string `json:"oversight_mode"`
	TimeAtLevel   string `json:"time_at_level"`
	UpgradeCount  int    `json:"upgrade_count"`
	LastUpgradeAt string `json:"last_upgrade_at,omitempty"`
}

func newTrustStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Fleet-wide view of each mailbox's oversight mode, time-at-level, and upgrade history",
		Long: `Trust status shows where each mailbox sits on the trust ladder and how
long it has been at its current oversight level. It joins mailbox metadata
with upgrade history from the local SQLite cache.

The five oversight modes, from most restrictive to most autonomous:
  read_only → gated_all → gated_send → monitored → autonomous

Requires synced data (run 'multimail-pp-cli sync --full' first).`,
		Example: strings.Trim(`
  multimail-pp-cli trust status --json
  multimail-pp-cli trust status --json --select mailbox,oversight_mode,time_at_level`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("multimail-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Get mailbox info: id, name (address), oversight mode, created_at
			type mbInfo struct {
				ID            string
				Name          string
				OversightMode string
				CreatedAt     string
			}
			var mailboxes []mbInfo
			mbRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT id,
					COALESCE(json_extract(data, '$.address'), id),
					COALESCE(json_extract(data, '$.oversight_mode'), 'unknown'),
					COALESCE(json_extract(data, '$.created_at'), '')
				FROM resources WHERE resource_type = 'mailboxes'`)
			if err != nil {
				return fmt.Errorf("querying mailboxes: %w", err)
			}
			for mbRows.Next() {
				var m mbInfo
				if err := mbRows.Scan(&m.ID, &m.Name, &m.OversightMode, &m.CreatedAt); err != nil {
					continue
				}
				mailboxes = append(mailboxes, m)
			}
			mbRows.Close()

			if len(mailboxes) == 0 {
				return fmt.Errorf("no mailboxes in local cache — run 'multimail-pp-cli sync --full' first")
			}

			var results []trustRow
			for _, mb := range mailboxes {
				// Count upgrades for this mailbox
				var upgradeCount int
				var lastUpgrade string
				upgradeRow := db.DB().QueryRowContext(cmd.Context(),
					`SELECT COUNT(*),
						COALESCE(
							MAX(COALESCE(json_extract(data, '$.upgraded_at'), json_extract(data, '$.created_at'), synced_at)),
							''
						)
					FROM upgrade WHERE mailboxes_id = ?`, mb.ID)
				_ = upgradeRow.Scan(&upgradeCount, &lastUpgrade)

				// Compute time-at-level: time since last upgrade, or since mailbox creation
				var timeAtLevel string
				refTime := lastUpgrade
				if refTime == "" {
					refTime = mb.CreatedAt
				}
				if refTime != "" {
					if t, err := time.Parse(time.RFC3339, refTime); err == nil {
						d := time.Since(t)
						switch {
						case d < 24*time.Hour:
							timeAtLevel = fmt.Sprintf("%dh", int(d.Hours()))
						case d < 30*24*time.Hour:
							timeAtLevel = fmt.Sprintf("%dd", int(d.Hours()/24))
						default:
							timeAtLevel = fmt.Sprintf("%dm", int(d.Hours()/(24*30)))
						}
					}
				}
				if timeAtLevel == "" {
					timeAtLevel = "—"
				}

				results = append(results, trustRow{
					Mailbox:       mb.Name,
					MailboxID:     mb.ID,
					OversightMode: mb.OversightMode,
					TimeAtLevel:   timeAtLevel,
					UpgradeCount:  upgradeCount,
					LastUpgradeAt: lastUpgrade,
				})
			}

			// Sort by oversight mode restrictiveness: most restrictive first
			modeOrder := map[string]int{
				"read_only":  0,
				"gated_all":  1,
				"gated_send": 2,
				"monitored":  3,
				"autonomous": 4,
			}
			sort.Slice(results, func(i, j int) bool {
				oi, ok := modeOrder[results[i].OversightMode]
				if !ok {
					oi = -1
				}
				oj, ok := modeOrder[results[j].OversightMode]
				if !ok {
					oj = -1
				}
				return oi < oj
			})

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
