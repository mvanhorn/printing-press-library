// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: resolve any identifier form into one identity card.

// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
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
		addNovelCommandIfAbsent(parent, newNovelUsersWhoisCmd(flags))
	})
}

// whoisChannel is one conversation the subject has posted in.
type whoisChannel struct {
	Channel     string `json:"channel"`
	ChannelName string `json:"channel_name"`
	Messages    int    `json:"messages"`
	LastSeen    string `json:"last_seen"`
}

// whoisCard is the single identity card. Emitted as a one-element array so
// the empty-mirror and not-found paths can render [] without changing shape.
type whoisCard struct {
	Query          string         `json:"query"`
	QueryKind      string         `json:"query_kind"`
	Resolved       bool           `json:"resolved"`
	Source         string         `json:"source"`
	UserID         string         `json:"user_id"`
	Handle         string         `json:"handle"`
	DisplayName    string         `json:"display_name"`
	RealName       string         `json:"real_name"`
	Email          string         `json:"email"`
	TZ             string         `json:"tz"`
	TZLabel        string         `json:"tz_label"`
	TZOffset       int            `json:"tz_offset"`
	LocalTime      string         `json:"local_time"`
	DND            *localDND      `json:"dnd"`
	Messages       int            `json:"messages"`
	LastSeen       string         `json:"last_seen"`
	LastSeenTS     string         `json:"last_seen_ts"`
	SharedChannels []whoisChannel `json:"shared_channels"`

	// Pointers, not values: these are assertions about a person, and a
	// zero value is an assertion too. On the not-found path a plain
	// `"is_bot": false, "deleted": false, "days_since_seen": 0` reads as
	// "a live human, seen today" for somebody the mirror never had. Nil
	// drops the key entirely, so absent stays distinguishable from false.
	IsBot         *bool    `json:"is_bot,omitempty"`
	Deleted       *bool    `json:"deleted,omitempty"`
	DaysSinceSeen *float64 `json:"days_since_seen,omitempty"`
}

func newNovelUsersWhoisCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "whois <id|@handle|email>",
		Short: "Turn an opaque Slack ID, handle, or email into one card with shared channels, timezone, DND state, and last-seen.",
		Long: "Use this command to resolve an opaque Slack ID, @handle, or email into one identity card with shared channels, timezone, DND state, and last-seen. " +
			"Do NOT use this command when you need the raw profile payload; use 'users info' instead.",
		Example: strings.Trim(`
  # Who is this ID?
  slack-pp-cli users whois U04AB9XYZ --agent

  # Same person, by handle or email
  slack-pp-cli users whois @alice
  slack-pp-cli users whois alice@example.com

  # Never touch the network
  slack-pp-cli users whois U04AB9XYZ --data-source local
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve the identifier against the local mirror, falling back to users.info / users.lookupByEmail (no writes)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			target := strings.TrimSpace(strings.Join(args, " "))
			if target == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an identifier is required: slack-pp-cli users whois <id|@handle|email>"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: slack-pp-cli sync --resources conversations,users && slack-pp-cli archive sync --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]whoisCard, 0), flags)
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

			ref := slackanalytics.ParseUserRef(target)
			card := whoisCard{
				Query:          target,
				QueryKind:      string(ref.Kind),
				SharedChannels: make([]whoisChannel, 0, 4),
			}

			var resolved localUser
			found := false
			// --data-source live skips the mirror entirely; auto and local
			// both consult it first.
			if flags.dataSource != "live" {
				users, err := loadLocalUsers(ctx, db)
				if err != nil {
					return err
				}
				resolved, found = matchLocalUser(users, ref)
			}
			if found {
				card.Source = "local"
			} else if flags.dataSource != "local" {
				// Unsynced (or unmatched) identity: fall back to the live
				// lookup this command's `auto` data source allows.
				live, liveErr := whoisLiveLookup(ctx, flags, ref)
				if liveErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: live lookup failed: %v\n", liveErr)
				} else if live != nil {
					resolved = *live
					found = true
					card.Source = "live"
				}
			}

			if found {
				now := time.Now().UTC()
				card.Resolved = true
				card.UserID = resolved.ID
				card.Handle = resolved.Handle
				card.DisplayName = resolved.DisplayName
				card.RealName = resolved.RealName
				card.Email = resolved.Email
				card.TZ = resolved.TZ
				card.TZLabel = resolved.TZLabel
				card.TZOffset = resolved.TZOffset
				isBot, deleted := resolved.IsBot, resolved.Deleted
				card.IsBot, card.Deleted = &isBot, &deleted
				if resolved.TZOffset != 0 {
					card.LocalTime = formatLocalTime(now, resolved.TZOffset, resolved.TZLabel)
				}

				dnd, err := loadLocalDND(ctx, db, resolved.ID)
				if err != nil {
					return err
				}
				card.DND = dnd

				messages, err := loadLocalMessages(ctx, db)
				if err != nil {
					return err
				}
				channels, err := loadLocalChannels(ctx, db)
				if err != nil {
					return err
				}
				perChannel := map[string]*whoisChannel{}
				var lastAt time.Time
				for _, m := range messages {
					if !strings.EqualFold(m.User, resolved.ID) {
						continue
					}
					card.Messages++
					if m.TS > card.LastSeenTS {
						card.LastSeenTS = m.TS
						lastAt = m.At
					}
					entry, ok := perChannel[m.Channel]
					if !ok {
						entry = &whoisChannel{
							Channel:     m.Channel,
							ChannelName: channelLabel(channels, m.Channel),
						}
						perChannel[m.Channel] = entry
					}
					entry.Messages++
					if seen := rfc3339(m.At, m.HasTime); seen > entry.LastSeen {
						entry.LastSeen = seen
					}
				}
				card.LastSeen = rfc3339(lastAt, !lastAt.IsZero())
				if !lastAt.IsZero() {
					days := slackanalytics.RoundDays(now.Sub(lastAt))
					card.DaysSinceSeen = &days
				}
				for _, entry := range perChannel {
					card.SharedChannels = append(card.SharedChannels, *entry)
				}
				sort.SliceStable(card.SharedChannels, func(i, j int) bool {
					if card.SharedChannels[i].Messages != card.SharedChannels[j].Messages {
						return card.SharedChannels[i].Messages > card.SharedChannels[j].Messages
					}
					return card.SharedChannels[i].Channel < card.SharedChannels[j].Channel
				})
			}

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				// Machine output stays an array in every path. An
				// unresolved card still carries query/query_kind, so an
				// agent can tell "no such user" from "empty mirror" (which
				// returns []).
				return printJSONFiltered(out, []whoisCard{card}, flags)
			}
			if !card.Resolved {
				fmt.Fprintf(out, "No user matches %q in the local mirror.\n", target)
				return nil
			}
			fmt.Fprintf(out, "%s\n", card.DisplayName)
			tw := newTabWriter(out)
			fmt.Fprintf(tw, "id\t%s\n", card.UserID)
			fmt.Fprintf(tw, "handle\t@%s\n", card.Handle)
			fmt.Fprintf(tw, "real name\t%s\n", card.RealName)
			fmt.Fprintf(tw, "email\t%s\n", card.Email)
			fmt.Fprintf(tw, "timezone\t%s (%s)\n", card.TZ, card.TZLabel)
			if card.LocalTime != "" {
				fmt.Fprintf(tw, "local time\t%s\n", card.LocalTime)
			}
			if card.DND != nil {
				fmt.Fprintf(tw, "dnd\tenabled=%t snoozed=%t\n", card.DND.DNDEnabled, card.DND.SnoozeEnabled)
			} else {
				fmt.Fprintf(tw, "dnd\tunknown (not synced)\n")
			}
			fmt.Fprintf(tw, "messages\t%d\n", card.Messages)
			fmt.Fprintf(tw, "last seen\t%s\n", card.LastSeen)
			fmt.Fprintf(tw, "source\t%s\n", card.Source)
			if err := tw.Flush(); err != nil {
				return err
			}
			if len(card.SharedChannels) == 0 {
				fmt.Fprintln(out, "\nNo shared channels in the local mirror.")
				return nil
			}
			fmt.Fprintln(out, "\nShared channels:")
			ctw := newTabWriter(out)
			fmt.Fprintln(ctw, "CHANNEL\tMESSAGES\tLAST SEEN")
			for _, c := range card.SharedChannels {
				fmt.Fprintf(ctw, "%s\t%d\t%s\n", c.ChannelName, c.Messages, c.LastSeen)
			}
			return ctw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: resolved data directory data.db)")
	return cmd
}

// formatLocalTime renders now in the user's timezone as an RFC3339 stamp whose
// offset matches that zone.
//
// The obvious spelling, now.Add(offset).Format(time.RFC3339), is wrong: now
// carries a UTC location, so Add shifts the instant while Format still writes
// the "Z" suffix. The result reads as the user's wall clock but claims to be
// UTC, so every consumer that parses it lands tz_offset seconds away from the
// real moment. Binding the shift to a FixedZone keeps the instant correct and
// makes the printed offset (e.g. "-04:00") agree with the wall clock.
func formatLocalTime(now time.Time, tzOffset int, tzLabel string) string {
	return now.In(time.FixedZone(tzLabel, tzOffset)).Format(time.RFC3339)
}

// whoisLiveLookup resolves a reference against the live API: users.info for
// an ID, users.lookupByEmail for an email. Handles have no live lookup
// endpoint (users.list would have to be paged), so they return nil and the
// caller reports "unresolved" rather than pretending.
func whoisLiveLookup(ctx context.Context, flags *rootFlags, ref slackanalytics.UserRef) (*localUser, error) {
	var path string
	params := map[string]string{}
	switch ref.Kind {
	case slackanalytics.RefID:
		path = "/users.info"
		params["user"] = ref.Value
	case slackanalytics.RefEmail:
		path = "/users.lookupByEmail"
		params["email"] = ref.Value
	default:
		return nil, nil
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		User  struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			RealName string `json:"real_name"`
			TZ       string `json:"tz"`
			TZLabel  string `json:"tz_label"`
			TZOffset int    `json:"tz_offset"`
			IsBot    bool   `json:"is_bot"`
			Deleted  bool   `json:"deleted"`
			Profile  struct {
				DisplayName string `json:"display_name"`
				RealName    string `json:"real_name"`
				Email       string `json:"email"`
			} `json:"profile"`
		} `json:"user"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decoding %s response: %w", path, err)
	}
	if envelope.Error != "" {
		return nil, fmt.Errorf("%s: %s", strings.TrimPrefix(path, "/"), envelope.Error)
	}
	if envelope.User.ID == "" {
		return nil, nil
	}
	realName := envelope.User.RealName
	if realName == "" {
		realName = envelope.User.Profile.RealName
	}
	return &localUser{
		ID:          envelope.User.ID,
		Handle:      envelope.User.Name,
		DisplayName: envelope.User.Profile.DisplayName,
		RealName:    realName,
		Email:       envelope.User.Profile.Email,
		TZ:          envelope.User.TZ,
		TZLabel:     envelope.User.TZLabel,
		TZOffset:    envelope.User.TZOffset,
		IsBot:       envelope.User.IsBot,
		Deleted:     envelope.User.Deleted,
	}, nil
}
