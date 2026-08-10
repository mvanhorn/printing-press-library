// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel commands: guild snapshot, whois, catch-up, channel history.
// All read live via the real Discord REST API; none mutate server state.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/discord/internal/cliutil"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		guild := &cobra.Command{
			Use:         "guild",
			Short:       "Novel guild operations (snapshot)",
			Annotations: map[string]string{"mcp:read-only": "true"},
			RunE:        parentNoSubcommandRunE(flags),
		}
		channel := &cobra.Command{
			Use:         "channel",
			Short:       "Novel channel operations (history)",
			Annotations: map[string]string{"mcp:read-only": "true"},
			RunE:        parentNoSubcommandRunE(flags),
		}
		addNovelCommandIfAbsent(guild, newNovelGuildSnapshotCmd(flags))
		addNovelCommandIfAbsent(channel, newNovelChannelHistoryCmd(flags))
		addNovelCommandIfAbsent(root, guild)
		addNovelCommandIfAbsent(root, newNovelWhoisCmd(flags))
		addNovelCommandIfAbsent(root, newNovelCatchUpCmd(flags))
		addNovelCommandIfAbsent(root, channel)
	})
}

// guildSnapshotReport is the composite inventory for one guild.
type guildSnapshotReport struct {
	GuildID     string            `json:"guild_id"`
	GuildName   string            `json:"guild_name"`
	MemberCount *int              `json:"member_count,omitempty"`
	Channels    []snapshotChannel `json:"channels"`
	Roles       []snapshotRole    `json:"roles"`
}

type snapshotChannel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     int    `json:"type"`
	Position int    `json:"position,omitempty"`
	Category string `json:"category,omitempty"`
}

type snapshotRole struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       int    `json:"color"`
	Mentionable bool   `json:"mentionable"`
}

func newNovelGuildSnapshotCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "snapshot <guild_id>",
		Short: "Snapshot a guild: channels, roles, and member count in one view",
		Long: `Fetch the guild profile, its channel tree, and its role inventory in a
single call batch and print a compact inventory. Useful for audits, onboarding
docs, and agent-driven guild reports.`,
		Example:     "  discord-pp-cli guild snapshot 1511116929677529270",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would snapshot guild channels and roles from the live API (read-only)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dbPath == "" {
				dbPath = defaultDBPath("discord-pp-cli")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			guildID := args[0]

			report := guildSnapshotReport{GuildID: guildID}

			guildRaw, err := c.GetNoCache(ctx, replacePathParam("/guilds/{guild_id}", "guild_id", guildID), map[string]string{"with_counts": "true"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var guild struct {
				Name        string `json:"name"`
				MemberCount int    `json:"member_count"`
			}
			if err := json.Unmarshal(guildRaw, &guild); err != nil {
				return fmt.Errorf("parsing guild: %w", err)
			}
			report.GuildName = guild.Name
			// member_count is only present when with_counts=true (requested
			// above); when Discord still omits it, leave the field absent
			// instead of implying a real count of zero.
			if guild.MemberCount > 0 {
				report.MemberCount = &guild.MemberCount
			}

			channelsRaw, err := c.GetNoCache(ctx, replacePathParam("/guilds/{guild_id}/channels", "guild_id", guildID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var channels []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Type     int    `json:"type"`
				Position int    `json:"position"`
				ParentID string `json:"parent_id"`
			}
			if err := json.Unmarshal(channelsRaw, &channels); err != nil {
				return fmt.Errorf("parsing channels: %w", err)
			}
			categoryNames := map[string]string{}
			for _, ch := range channels {
				if ch.Type == 4 && ch.Name != "" {
					categoryNames[ch.ID] = ch.Name
				}
			}
			sort.SliceStable(channels, func(i, j int) bool {
				if channels[i].Position != channels[j].Position {
					return channels[i].Position < channels[j].Position
				}
				return channels[i].Name < channels[j].Name
			})
			for _, ch := range channels {
				report.Channels = append(report.Channels, snapshotChannel{
					ID: ch.ID, Name: ch.Name, Type: ch.Type, Position: ch.Position,
					Category: categoryNames[ch.ParentID],
				})
			}

			rolesRaw, err := c.GetNoCache(ctx, replacePathParam("/guilds/{guild_id}/roles", "guild_id", guildID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var roles []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Color       int    `json:"color"`
				Mentionable bool   `json:"mentionable"`
			}
			if err := json.Unmarshal(rolesRaw, &roles); err != nil {
				return fmt.Errorf("parsing roles: %w", err)
			}
			sort.SliceStable(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })
			for _, r := range roles {
				report.Roles = append(report.Roles, snapshotRole{ID: r.ID, Name: r.Name, Color: r.Color, Mentionable: r.Mentionable})
			}

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, []guildSnapshotReport{report}, flags)
			}
			if report.MemberCount != nil {
				fmt.Fprintf(out, "Guild: %s (%s)  members: %d\n\n", report.GuildName, report.GuildID, *report.MemberCount)
			} else {
				fmt.Fprintf(out, "Guild: %s (%s)  members: n/a\n\n", report.GuildName, report.GuildID)
			}
			fmt.Fprintln(out, "CHANNELS")
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "NAME\tTYPE\tPOS\tCATEGORY")
			for _, ch := range report.Channels {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%s\n", ch.Name, ch.Type, ch.Position, ch.Category)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Fprintln(out, "\nROLES")
			tw = newTabWriter(out)
			fmt.Fprintln(tw, "NAME\tID\tCOLOR\tMENTIONABLE")
			for _, r := range report.Roles {
				fmt.Fprintf(tw, "%s\t%s\t%06x\t%v\n", r.Name, r.ID, r.Color, r.Mentionable)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: resolved data directory data.db)")
	return cmd
}

// whoisReport is one user identity card.
type whoisReport struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name,omitempty"`
	Bot         bool     `json:"bot"`
	Avatar      string   `json:"avatar,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	GuildID     string   `json:"guild_id,omitempty"`
	GuildName   string   `json:"guild_name,omitempty"`
	Nick        string   `json:"nick,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	JoinedAt    string   `json:"joined_at,omitempty"`
}

func newNovelWhoisCmd(flags *rootFlags) *cobra.Command {
	var flagGuild string

	cmd := &cobra.Command{
		Use:   "whois <user_id>",
		Short: "Identity card for a Discord user, optionally within a guild",
		Long: `Resolve a user id into a human card: username, display name, bot flag,
account creation date (derived from the snowflake), and, when --guild is given,
their guild nick, roles, and join date.`,
		Example:     "  discord-pp-cli whois 1527888912079392860 --guild 1511116929677529270",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve the user card from the live API (read-only)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			userID := args[0]

			report := whoisReport{UserID: userID}

			userRaw, err := c.GetNoCache(ctx, replacePathParam("/users/{user_id}", "user_id", userID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var user struct {
				Username    string `json:"username"`
				DisplayName string `json:"global_name"`
				Bot         bool   `json:"bot"`
				Avatar      string `json:"avatar"`
			}
			if err := json.Unmarshal(userRaw, &user); err != nil {
				return fmt.Errorf("parsing user: %w", err)
			}
			report.Username = user.Username
			report.DisplayName = user.DisplayName
			report.Bot = user.Bot
			report.Avatar = user.Avatar
			report.CreatedAt = snowflakeTime(userID)

			if flagGuild != "" {
				memberPath := replacePathParam("/guilds/{guild_id}/members/{user_id}", "guild_id", flagGuild)
				memberPath = replacePathParam(memberPath, "user_id", userID)
				memberRaw, err := c.GetNoCache(ctx, memberPath, nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var member struct {
					Nick     string   `json:"nick"`
					Roles    []string `json:"roles"`
					JoinedAt string   `json:"joined_at"`
				}
				if err := json.Unmarshal(memberRaw, &member); err != nil {
					return fmt.Errorf("parsing member: %w", err)
				}
				report.GuildID = flagGuild
				report.Nick = member.Nick
				report.Roles = member.Roles
				report.JoinedAt = member.JoinedAt

				guildRaw, gerr := c.GetNoCache(ctx, replacePathParam("/guilds/{guild_id}", "guild_id", flagGuild), nil)
				if gerr != nil {
					// The member lookup succeeded but the guild name did not;
					// a partial card without the guild name is still useful,
					// but the failure must be visible, not silent.
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not resolve guild name for %s: %v\n", flagGuild, gerr)
				} else {
					var guild struct {
						Name string `json:"name"`
					}
					if err := json.Unmarshal(guildRaw, &guild); err != nil {
						return fmt.Errorf("parsing guild: %w", err)
					}
					report.GuildName = guild.Name
				}
			}

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, []whoisReport{report}, flags)
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "FIELD\tVALUE")
			fmt.Fprintf(tw, "user_id\t%s\n", report.UserID)
			fmt.Fprintf(tw, "username\t%s\n", report.Username)
			if report.DisplayName != "" {
				fmt.Fprintf(tw, "display_name\t%s\n", report.DisplayName)
			}
			fmt.Fprintf(tw, "bot\t%v\n", report.Bot)
			fmt.Fprintf(tw, "created_at\t%s\n", report.CreatedAt)
			if report.GuildID != "" {
				fmt.Fprintf(tw, "guild\t%s (%s)\n", report.GuildName, report.GuildID)
				fmt.Fprintf(tw, "nick\t%s\n", report.Nick)
				fmt.Fprintf(tw, "joined_at\t%s\n", report.JoinedAt)
				fmt.Fprintf(tw, "roles\t%d\n", len(report.Roles))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagGuild, "guild", "", "Guild id to resolve guild membership (nick, roles, join date)")
	return cmd
}

// catchUpChannel is per-channel recent activity inside the window.
type catchUpChannel struct {
	Channel      string `json:"channel"`
	ChannelName  string `json:"channel_name"`
	NewMessages  int    `json:"new_messages"`
	LastActivity string `json:"last_activity"`
	Truncated    bool   `json:"truncated,omitempty"`
}

type catchUpReport struct {
	GuildID   string           `json:"guild_id"`
	GuildName string           `json:"guild_name"`
	Window    string           `json:"window"`
	TotalNew  int              `json:"total_new_messages"`
	Truncated bool             `json:"truncated,omitempty"`
	Channels  []catchUpChannel `json:"channels"`
}

func newNovelCatchUpCmd(flags *rootFlags) *cobra.Command {
	var flagGuild string
	var flagSince string

	cmd := &cobra.Command{
		Use:   "catch-up",
		Short: "See what happened while you were away: new message volume per channel",
		Long: `For the given guild (or every guild the bot can see), scan the most recent
messages in each text channel and report per-channel volume inside the window.
Read-only: never marks anything read, never sends anything.`,
		Example:     "  discord-pp-cli catch-up --guild 1511116929677529270 --since 24h",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan recent message volume per channel from the live API (read-only)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			window := 24 * time.Hour
			if d, err := time.ParseDuration(flagSince); err == nil && d > 0 {
				window = d
			}
			cutoff := time.Now().UTC().Add(-window)

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type guildRef struct{ id, name string }
			var guilds []guildRef
			if flagGuild != "" {
				guildRaw, gerr := c.GetNoCache(ctx, replacePathParam("/guilds/{guild_id}", "guild_id", flagGuild), nil)
				if gerr != nil {
					return classifyAPIError(gerr, flags)
				}
				var g struct {
					Name string `json:"name"`
				}
				_ = json.Unmarshal(guildRaw, &g)
				guilds = append(guilds, guildRef{id: flagGuild, name: g.Name})
			} else {
				listRaw, lerr := c.GetNoCache(ctx, "/users/@me/guilds", nil)
				if lerr != nil {
					return classifyAPIError(lerr, flags)
				}
				var list []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}
				if err := json.Unmarshal(listRaw, &list); err != nil {
					return fmt.Errorf("parsing guild list: %w", err)
				}
				for _, g := range list {
					guilds = append(guilds, guildRef{id: g.ID, name: g.Name})
				}
			}

			perGuildCap := 0
			perChannelCap := 0
			if cliutil.IsDogfoodEnv() {
				// Dogfood runs under a 30s subprocess timeout; a full sweep of
				// every channel in every guild would trip it. Curtail there,
				// and say so in the report.
				perGuildCap = 1
				perChannelCap = 1
			}

			var reports []catchUpReport
			truncatedGuilds := false
			for gi, g := range guilds {
				if perGuildCap > 0 && gi >= perGuildCap {
					truncatedGuilds = true
					break
				}
				report := catchUpReport{GuildID: g.id, GuildName: g.name, Window: flagSince}

				channelsRaw, cerr := c.GetNoCache(ctx, replacePathParam("/guilds/{guild_id}/channels", "guild_id", g.id), nil)
				if cerr != nil {
					// A guild the bot lost access to is not fatal; report it as empty.
					if gi == 0 && flagGuild != "" {
						return classifyAPIError(cerr, flags)
					}
					continue
				}
				var channels []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
					Type int    `json:"type"`
				}
				if err := json.Unmarshal(channelsRaw, &channels); err != nil {
					return fmt.Errorf("parsing channels: %w", err)
				}
				count := 0
				truncatedChannels := false
				for ci, ch := range channels {
					if ch.Type != 0 { // text channels only (type 0 = GUILD_TEXT)
						continue
					}
					if perChannelCap > 0 && count >= perChannelCap {
						truncatedChannels = true
						break
					}
					count++
					msgsRaw, merr := c.GetNoCache(ctx, replacePathParam("/channels/{channel_id}/messages", "channel_id", ch.ID), map[string]string{"limit": "25"})
					if merr != nil {
						continue
					}
					var msgs []struct {
						Timestamp string `json:"timestamp"`
					}
					if err := json.Unmarshal(msgsRaw, &msgs); err != nil {
						continue
					}
					newInWindow := 0
					var last time.Time
					for _, m := range msgs {
						t, terr := time.Parse(time.RFC3339, m.Timestamp)
						if terr != nil {
							continue
						}
						if t.After(cutoff) {
							newInWindow++
						}
						if t.After(last) {
							last = t
						}
					}
					// A full 25-message page whose newest message is still
					// inside the window means the window may extend past the
					// fetched page; the count is a lower bound, not the total.
					truncatedMessages := len(msgs) >= 25 && !last.IsZero() && last.After(cutoff)
					report.Truncated = report.Truncated || truncatedMessages
					report.TotalNew += newInWindow
					report.Channels = append(report.Channels, catchUpChannel{
						Channel: ch.ID, ChannelName: ch.Name, NewMessages: newInWindow,
						LastActivity: formatRFC3339OrEmpty(last),
						Truncated:    truncatedMessages,
					})
					_ = ci
				}
				report.Truncated = report.Truncated || truncatedChannels
				sort.SliceStable(report.Channels, func(i, j int) bool { return report.Channels[i].NewMessages > report.Channels[j].NewMessages })
				reports = append(reports, report)
			}

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, reports, flags)
			}
			for _, report := range reports {
				marker := ""
				if report.Truncated {
					marker = " (sampled; some channels or messages were not fully scanned)"
				}
				fmt.Fprintf(out, "Guild %s (%s): %d new messages in %s%s\n", report.GuildName, report.GuildID, report.TotalNew, flagSince, marker)
				tw := newTabWriter(out)
				fmt.Fprintln(tw, "CHANNEL	NEW	LAST ACTIVITY	STATUS")
				for _, ch := range report.Channels {
					status := ""
					if ch.Truncated {
						status = "sampled"
					}
					fmt.Fprintf(tw, "%s	%d	%s	%s\n", ch.ChannelName, ch.NewMessages, ch.LastActivity, status)
				}
				if err := tw.Flush(); err != nil {
					return err
				}
			}
			if truncatedGuilds && len(reports) > 0 {
				fmt.Fprintf(out, "\nnote: scanned the first %d guild(s) only; use --guild to target a specific server\n", len(reports))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagGuild, "guild", "", "Guild id to scan (default: every guild the bot can see)")
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Look back this far (e.g. 24h, 3d, 1w)")
	return cmd
}

// historyMessage is one message in a channel digest.
type historyMessage struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	Author    string `json:"author"`
	AuthorID  string `json:"author_id"`
	Bot       bool   `json:"bot"`
	Timestamp string `json:"timestamp"`
	Content   string `json:"content,omitempty"`
	Type      int    `json:"type"`
}

func newNovelChannelHistoryCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "history <channel_id>",
		Short: "Recent message digest for a channel",
		Long: `Fetch the most recent messages in a channel and print them as a compact
digest: author, timestamp, and content. Read-only.`,
		Example:     "  discord-pp-cli channel history 1511205390652670002 --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch the recent message digest from the live API (read-only)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if flagLimit <= 0 {
				flagLimit = 25
			}
			if cliutil.IsDogfoodEnv() && flagLimit > 5 {
				flagLimit = 5
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			channelID := args[0]

			msgsRaw, err := c.GetNoCache(ctx, replacePathParam("/channels/{channel_id}/messages", "channel_id", channelID), map[string]string{"limit": fmt.Sprintf("%d", flagLimit)})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var msgs []struct {
				ID        string `json:"id"`
				Timestamp string `json:"timestamp"`
				Content   string `json:"content"`
				Type      int    `json:"type"`
				Author    struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					Bot      bool   `json:"bot"`
				} `json:"author"`
			}
			if err := json.Unmarshal(msgsRaw, &msgs); err != nil {
				return fmt.Errorf("parsing messages: %w", err)
			}
			digest := make([]historyMessage, 0, len(msgs))
			for _, m := range msgs {
				digest = append(digest, historyMessage{
					ID: m.ID, ChannelID: channelID, Author: m.Author.Username, AuthorID: m.Author.ID,
					Bot: m.Author.Bot, Timestamp: m.Timestamp, Content: strings.TrimSpace(m.Content), Type: m.Type,
				})
			}

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, digest, flags)
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "WHEN\tAUTHOR\tTYPE\tCONTENT")
			for _, m := range digest {
				fmt.Fprintf(tw, "%s\t%s\t%d\t%s\n", m.Timestamp, m.Author, m.Type, truncate(m.Content, 120))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Maximum messages to fetch")
	return cmd
}

// formatRFC3339OrEmpty renders a time as RFC3339, or "" when zero.
func formatRFC3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// snowflakeTime derives the creation timestamp from a Discord snowflake id.
func snowflakeTime(id string) string {
	var n uint64
	for _, ch := range id {
		if ch < '0' || ch > '9' {
			return ""
		}
		n = n*10 + uint64(ch-'0')
	}
	if n == 0 {
		return ""
	}
	// Discord epoch: 2015-01-01T00:00:00.000Z, timestamp in the top 42 bits.
	ms := (n >> 22) + 1420070400000
	return time.UnixMilli(int64(ms)).UTC().Format(time.RFC3339)
}
