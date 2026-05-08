// Compound command: trust ladder status.
// Hand-built transcendence feature — not generated from OpenAPI.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"multimail-pp-cli/internal/store"
	"github.com/spf13/cobra"
)

// Trust ladder levels in ascending order of autonomy.
var trustLadder = []string{
	"gated_all",
	"gated_send",
	"monitored",
	"autonomous",
}

type trustStatusResult struct {
	Mailboxes []mailboxTrust `json:"mailboxes"`
	Summary   trustSummary   `json:"summary"`
}

type mailboxTrust struct {
	MailboxID     string           `json:"mailbox_id"`
	Address       string           `json:"address"`
	DisplayName   string           `json:"display_name"`
	CurrentMode   string           `json:"current_mode"`
	LadderPosition int            `json:"ladder_position"`
	NextLevel     string           `json:"next_level,omitempty"`
	NextSteps     string           `json:"next_steps,omitempty"`
	History       []upgradeEvent   `json:"history,omitempty"`
}

type upgradeEvent struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Date      string `json:"date"`
	Status    string `json:"status"`
}

type trustSummary struct {
	TotalMailboxes    int     `json:"total_mailboxes"`
	FullyAutonomous   int     `json:"fully_autonomous"`
	AvgLadderPosition float64 `json:"avg_ladder_position"`
}

func newTrustStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Trust ladder position per mailbox with progression history",
		Long: `Shows current trust ladder position per mailbox, what's needed for
the next level, and progression history. The agent's autonomy roadmap.

Trust ladder (ascending autonomy):
  1. gated_all    — all emails require operator approval
  2. gated_send   — outbound emails require approval (default)
  3. monitored    — emails send immediately, operator notified
  4. autonomous   — full autonomy, no oversight

Requires synced data — run 'multimail-pp-cli sync' first.`,
		Example: `  # See trust status for all mailboxes
  multimail-pp-cli trust status

  # JSON output
  multimail-pp-cli trust status --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("multimail-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'multimail-pp-cli sync' first.", err)
			}
			defer db.Close()

			result, err := computeTrustStatus(db)
			if err != nil {
				return err
			}

			jsonMode := flags.asJSON || !isTerminal(cmd.OutOrStdout())
			if jsonMode {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if flags.compact {
					type compactTrust struct {
						MailboxID   string `json:"mailbox_id"`
						Address     string `json:"address"`
						CurrentMode string `json:"current_mode"`
						Position    int    `json:"position"`
						NextLevel   string `json:"next_level,omitempty"`
					}
					compact := make([]compactTrust, len(result.Mailboxes))
					for i, m := range result.Mailboxes {
						compact[i] = compactTrust{m.MailboxID, m.Address, m.CurrentMode, m.LadderPosition, m.NextLevel}
					}
					return enc.Encode(compact)
				}
				return enc.Encode(result)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Trust Ladder Status")
			fmt.Fprintln(cmd.OutOrStdout(), "===================")
			fmt.Fprintf(cmd.OutOrStdout(), "Mailboxes: %d | Fully Autonomous: %d | Avg Position: %.1f/4\n\n",
				result.Summary.TotalMailboxes, result.Summary.FullyAutonomous, result.Summary.AvgLadderPosition)

			for _, m := range result.Mailboxes {
				bar := trustBar(m.LadderPosition)
				fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", m.Address, m.DisplayName)
				fmt.Fprintf(cmd.OutOrStdout(), "    Mode: %s  Position: %d/4  %s\n", m.CurrentMode, m.LadderPosition, bar)
				if m.NextLevel != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "    Next: %s — %s\n", m.NextLevel, m.NextSteps)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "    Fully autonomous — maximum trust achieved\n")
				}
				if len(m.History) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "    History: ")
					for i, h := range m.History {
						if i > 0 {
							fmt.Fprintf(cmd.OutOrStdout(), " → ")
						}
						fmt.Fprintf(cmd.OutOrStdout(), "%s→%s (%s)", h.From, h.To, h.Status)
					}
					fmt.Fprintln(cmd.OutOrStdout())
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func trustBar(position int) string {
	filled := position
	empty := 4 - position
	bar := "["
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := 0; i < empty; i++ {
		bar += "░"
	}
	bar += "]"
	return bar
}

func ladderPosition(mode string) int {
	for i, level := range trustLadder {
		if level == mode {
			return i + 1
		}
	}
	return 0 // unknown mode
}

func nextTrustLevel(mode string) (string, string) {
	pos := ladderPosition(mode)
	if pos >= len(trustLadder) || pos == 0 {
		return "", ""
	}
	next := trustLadder[pos]
	steps := ""
	switch next {
	case "gated_send":
		steps = "Demonstrate safe inbound handling. Request upgrade via 'mm mailboxes request-upgrade'."
	case "monitored":
		steps = "Build a track record of approved sends. Request upgrade when approval rate is high."
	case "autonomous":
		steps = "Operate in monitored mode without issues. Operator grants full autonomy."
	}
	return next, steps
}

func computeTrustStatus(db *store.Store) (*trustStatusResult, error) {
	sqlDB := db.DB()

	// Get all mailboxes
	rows, err := sqlDB.Query(`SELECT id, data FROM mailboxes`)
	if err != nil {
		return nil, fmt.Errorf("querying mailboxes: %w", err)
	}
	defer rows.Close()

	mailboxes := make([]mailboxTrust, 0)
	totalPos := 0
	autonomous := 0

	for rows.Next() {
		var id string
		var dataStr string
		if err := rows.Scan(&id, &dataStr); err != nil {
			continue
		}

		var mb map[string]any
		if err := json.Unmarshal([]byte(dataStr), &mb); err != nil {
			continue
		}

		mode, _ := mb["oversight_mode"].(string)
		if mode == "" {
			mode = "gated_send" // default
		}

		address, _ := mb["address"].(string)
		displayName, _ := mb["display_name"].(string)
		pos := ladderPosition(mode)
		nextLevel, nextSteps := nextTrustLevel(mode)

		if mode == "autonomous" {
			autonomous++
		}
		totalPos += pos

		// Get upgrade history for this mailbox
		history := getUpgradeHistory(db, id)

		mailboxes = append(mailboxes, mailboxTrust{
			MailboxID:      id,
			Address:        address,
			DisplayName:    displayName,
			CurrentMode:    mode,
			LadderPosition: pos,
			NextLevel:      nextLevel,
			NextSteps:      nextSteps,
			History:        history,
		})
	}

	avgPos := 0.0
	if len(mailboxes) > 0 {
		avgPos = float64(totalPos) / float64(len(mailboxes))
	}

	return &trustStatusResult{
		Mailboxes: mailboxes,
		Summary: trustSummary{
			TotalMailboxes:    len(mailboxes),
			FullyAutonomous:   autonomous,
			AvgLadderPosition: avgPos,
		},
	}, nil
}

func getUpgradeHistory(db *store.Store, mailboxID string) []upgradeEvent {
	sqlDB := db.DB()

	// Check request_upgrade table
	rows, err := sqlDB.Query(`SELECT data FROM request_upgrade WHERE mailboxes_id = ? ORDER BY synced_at`, mailboxID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var events []upgradeEvent
	for rows.Next() {
		var dataStr string
		if err := rows.Scan(&dataStr); err != nil {
			continue
		}
		var req map[string]any
		if err := json.Unmarshal([]byte(dataStr), &req); err != nil {
			continue
		}

		from, _ := req["from_mode"].(string)
		to, _ := req["to_mode"].(string)
		date, _ := req["created_at"].(string)
		status := "requested"

		events = append(events, upgradeEvent{
			From:   from,
			To:     to,
			Date:   date,
			Status: status,
		})
	}

	// Check upgrade (applied) table
	upgradeRows, err := sqlDB.Query(`SELECT data FROM upgrade WHERE mailboxes_id = ? ORDER BY synced_at`, mailboxID)
	if err != nil {
		return events
	}
	defer upgradeRows.Close()

	for upgradeRows.Next() {
		var dataStr string
		if err := upgradeRows.Scan(&dataStr); err != nil {
			continue
		}
		var upg map[string]any
		if err := json.Unmarshal([]byte(dataStr), &upg); err != nil {
			continue
		}

		from, _ := upg["from_mode"].(string)
		to, _ := upg["to_mode"].(string)
		date, _ := upg["created_at"].(string)

		events = append(events, upgradeEvent{
			From:   from,
			To:     to,
			Date:   date,
			Status: "applied",
		})
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Date < events[j].Date
	})

	return events
}
