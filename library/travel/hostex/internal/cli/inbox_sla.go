// Hand-authored transcendence command. generate --force preserves this file.
// pp:data-source local
package cli

import (
	"math"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/hostex/internal/cliutil"
)

func newNovelInboxSlaCmd(flags *rootFlags) *cobra.Command {
	var flagBreach string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "inbox-sla",
		Short: "Rank open guest conversations by age since the last message and flag threads past your SLA.",
		Long: "Ranks locally synced conversations by how long since their last message and\n" +
			"flags those older than the SLA threshold. Hostex's conversation list has no\n" +
			"age clock; this derives it locally so you can triage what is about to breach.\n\n" +
			"Note: the list endpoint does not expose message sender or unread state, so age\n" +
			"is measured from last_message_at regardless of who sent last. Reads the local\n" +
			"mirror; run `hostex-pp-cli sync --resources conversations` first.",
		Example:     "  hostex-pp-cli inbox-sla --breach 6h --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLiveDataSource(flags); err != nil {
				return err
			}
			breach, err := cliutil.ParseDurationLoose(flagBreach)
			if err != nil {
				return usageErr(err)
			}

			db, done, err := openLocalMirror(cmd, flags, flagDB)
			if done {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "conversations") {
				hintIfStale(cmd, db, "conversations", flags.maxAge)
			}

			conversations, err := listObjs(db, "conversations")
			if err != nil {
				return err
			}

			now := nowUTC()

			type convRow struct {
				ID            any     `json:"id"`
				ChannelType   string  `json:"channel_type,omitempty"`
				PropertyTitle string  `json:"property_title,omitempty"`
				GuestName     string  `json:"guest_name,omitempty"`
				LastMessageAt string  `json:"last_message_at,omitempty"`
				AgeHours      float64 `json:"age_hours"`
				Breached      bool    `json:"breached"`
			}

			rows := make([]convRow, 0)
			breachedCount := 0
			for _, c := range conversations {
				lm, ok := novTime(c["last_message_at"])
				ageHours := 0.0
				breached := false
				if ok {
					ageHours = math.Round(now.Sub(lm).Hours()*10) / 10
					breached = now.Sub(lm) >= breach
				}
				if breached {
					breachedCount++
				}
				guestName := ""
				if g := novMap(c, "guest"); g != nil {
					guestName = novStr(g, "name")
				}
				rows = append(rows, convRow{
					ID:            c["id"],
					ChannelType:   novStr(c, "channel_type"),
					PropertyTitle: novStr(c, "property_title"),
					GuestName:     guestName,
					LastMessageAt: novStr(c, "last_message_at"),
					AgeHours:      ageHours,
					Breached:      breached,
				})
			}

			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].AgeHours > rows[j].AgeHours
			})

			view := struct {
				Breach        string    `json:"breach"`
				Scanned       int       `json:"scanned_conversations"`
				BreachedCount int       `json:"breached"`
				Conversations []convRow `json:"conversations"`
			}{
				Breach:        flagBreach,
				Scanned:       len(conversations),
				BreachedCount: breachedCount,
				Conversations: rows,
			}
			return novEmit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagBreach, "breach", "24h", "Flag conversations whose last message is older than this (e.g. 6h, 2h, 1d)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Path to the local SQLite mirror (default: per-user data dir)")
	return cmd
}
