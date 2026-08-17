// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cross-channel activity rollup for one person.

// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/slackanalytics"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent := findNovelParent(root, []string{"users"})
		if parent == nil {
			return
		}
		addNovelCommandIfAbsent(parent, newNovelUsersActivityCmd(flags))
	})
}

// userActivityRow is one channel's worth of a person's footprint.
type userActivityRow struct {
	User            string  `json:"user"`
	UserName        string  `json:"user_name"`
	Channel         string  `json:"channel"`
	ChannelName     string  `json:"channel_name"`
	Messages        int     `json:"messages"`
	ThreadStarts    int     `json:"thread_starts"`
	ThreadsCarried  int     `json:"threads_carried"`
	ThreadsLastWord int     `json:"threads_last_word"`
	Reactions       int     `json:"reactions_received"`
	FirstSeen       string  `json:"first_seen"`
	LastSeen        string  `json:"last_seen"`
	LastSeenTS      string  `json:"last_seen_ts"`
	DaysSinceSeen   float64 `json:"days_since_seen"`
	WindowDays      int     `json:"window_days"`
}

func newNovelUsersActivityCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "activity <user>",
		Short: "Profile where one person posts, which threads they carry, and when they were last seen.",
		Long: "Use this command to profile where a single named person is active across channels and threads. " +
			"Do NOT use this command for your own since-I-was-away summary; use 'catchup' instead.",
		Example: strings.Trim(`
  # Where has alice been posting?
  slack-pp-cli users activity @alice

  # Last 30 days, by Slack ID, as JSON
  slack-pp-cli users activity U04AB9XYZ --days 30 --json

  # By email
  slack-pp-cli users activity alice@example.com
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would roll up one person's local-mirror activity per channel (no API call, no writes)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			target := strings.TrimSpace(strings.Join(args, " "))
			if target == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a user is required: slack-pp-cli users activity <id|@handle|email>"))
			}
			if flagDays < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--days must be zero or positive, got %d", flagDays))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: slack-pp-cli sync --resources conversations,users && slack-pp-cli archive sync --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]userActivityRow, 0), flags)
				}
				return nil
			}

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "users") {
				hintIfStale(cmd, db, "users", flags.maxAge)
			}

			users, err := loadLocalUsers(ctx, db)
			if err != nil {
				return err
			}
			messages, err := loadLocalMessages(ctx, db)
			if err != nil {
				return err
			}
			channels, err := loadLocalChannels(ctx, db)
			if err != nil {
				return err
			}

			ref := slackanalytics.ParseUserRef(target)
			userID := ""
			userName := target
			if match, ok := matchLocalUser(users, ref); ok {
				userID = match.ID
				userName = match.Identity().DisplayLabel()
			} else if ref.Kind == slackanalytics.RefID {
				// An unmirrored but well-formed ID still profiles: the
				// messages carry the author ID even when users.list was
				// never synced.
				userID = ref.Value
			} else {
				// The caller named a specific person. Returning an empty list
				// here would be indistinguishable from "that person exists but
				// said nothing", so this is a usage error, not an empty result.
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("%q does not resolve to a mirrored user; run: slack-pp-cli sync --resources users --db %s", target, dbPath))
			}

			rows := make([]userActivityRow, 0, 8)
			if userID != "" {
				now := time.Now().UTC()
				var cutoff time.Time
				if flagDays > 0 {
					cutoff = now.Add(-time.Duration(flagDays) * 24 * time.Hour)
				}
				inWindow := func(m localMessage) bool {
					if flagDays <= 0 {
						return true
					}
					return m.HasTime && !m.At.Before(cutoff)
				}

				type agg struct {
					messages     int
					threadStarts int
					reactions    int
					firstTS      string
					lastTS       string
					first, last  time.Time
					carried      map[string]bool
					lastWord     map[string]bool
				}
				aggs := map[string]*agg{}
				get := func(channelID string) *agg {
					a, ok := aggs[channelID]
					if !ok {
						a = &agg{carried: map[string]bool{}, lastWord: map[string]bool{}}
						aggs[channelID] = a
					}
					return a
				}
				for _, m := range messages {
					if !strings.EqualFold(m.User, userID) || !inWindow(m) {
						continue
					}
					a := get(m.Channel)
					a.messages++
					a.reactions += m.Reactions
					if m.ThreadTS != "" && m.ThreadTS == m.TS {
						a.threadStarts++
					}
					if m.ThreadTS != "" {
						a.carried[m.ThreadTS] = true
					}
					if a.firstTS == "" || m.TS < a.firstTS {
						a.firstTS = m.TS
						a.first = m.At
					}
					if m.TS > a.lastTS {
						a.lastTS = m.TS
						a.last = m.At
					}
				}
				// Threads where this person spoke last — the ones they are
				// actually carrying rather than merely appearing in.
				for _, thread := range groupThreads(messages, 2) {
					latest := thread.Latest()
					if !strings.EqualFold(latest.User, userID) || !inWindow(latest) {
						continue
					}
					if a, ok := aggs[thread.Channel]; ok {
						a.lastWord[thread.ThreadTS] = true
					}
				}

				for channelID, a := range aggs {
					row := userActivityRow{
						User:            userID,
						UserName:        userName,
						Channel:         channelID,
						ChannelName:     channelLabel(channels, channelID),
						Messages:        a.messages,
						ThreadStarts:    a.threadStarts,
						ThreadsCarried:  len(a.carried),
						ThreadsLastWord: len(a.lastWord),
						Reactions:       a.reactions,
						FirstSeen:       rfc3339(a.first, !a.first.IsZero()),
						LastSeen:        rfc3339(a.last, !a.last.IsZero()),
						LastSeenTS:      a.lastTS,
						WindowDays:      flagDays,
					}
					if !a.last.IsZero() {
						row.DaysSinceSeen = slackanalytics.RoundDays(now.Sub(a.last))
					}
					rows = append(rows, row)
				}
				sort.SliceStable(rows, func(i, j int) bool {
					if rows[i].Messages != rows[j].Messages {
						return rows[i].Messages > rows[j].Messages
					}
					return rows[i].Channel < rows[j].Channel
				})
				if flagLimit > 0 && len(rows) > flagLimit {
					rows = rows[:flagLimit]
				}
			}

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintf(out, "No mirrored messages from %s.\n", target)
				return nil
			}
			fmt.Fprintf(out, "%s (%s)\n\n", userName, userID)
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "CHANNEL\tMESSAGES\tTHREAD STARTS\tTHREADS\tLAST WORD\tLAST SEEN")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%d\t%s\n", r.ChannelName, r.Messages, r.ThreadStarts, r.ThreadsCarried, r.ThreadsLastWord, r.LastSeen)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 0, "Only activity within this many days (0 = the whole mirror)")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum channels to return (0 for all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: resolved data directory data.db)")
	return cmd
}
