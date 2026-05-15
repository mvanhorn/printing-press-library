// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It adds the real
// Slack mirror sync engine — `sync mirror` — which populates the
// hand-built mirror schema in internal/store/mirror.go. It is a sibling
// subcommand of the generated generic `sync`; the generated sync command
// tree is left untouched.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

// slackResp is the common Slack Web API envelope: every method returns
// {"ok": true/false, "error": "..."}. The cursor lives in
// response_metadata.next_cursor.
type slackResp struct {
	OK               bool   `json:"ok"`
	Error            string `json:"error"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// mirrorClient is the slice of *client.Client the mirror sync needs.
// Declared as an interface so the engine can be exercised in tests with a
// fake without standing up an HTTP server.
type mirrorClient interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

// mirrorSyncStats accumulates per-run counts for the final summary.
type mirrorSyncStats struct {
	Channels   int `json:"channels"`
	Users      int `json:"users"`
	Usergroups int `json:"usergroups"`
	Messages   int `json:"messages"`
	Threads    int `json:"threads"`
	Reactions  int `json:"reactions"`
	Files      int `json:"files"`
	// Warnings collects non-fatal stage failures (e.g. a resource the token
	// lacks scope for). The sync continues; the warning is surfaced at the end.
	Warnings []string `json:"warnings,omitempty"`
}

// isPermissionErr reports whether err is a Slack authorization/scope failure
// for a single resource. Such errors must not abort the whole mirror sync —
// a token without `usergroups:read` should still mirror channels and history.
func isPermissionErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	for _, marker := range []string{
		"missing_scope", "not_allowed_token_type", "no_permission",
		"not_in_channel", "channel_not_found", "access_denied", "restricted_action",
	} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

func newSyncMirrorCmd(flags *rootFlags) *cobra.Command {
	var full bool
	var since string
	var channelsCSV string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "mirror",
		Short: "Sync Slack channels, users, messages, threads and reactions into the local mirror",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Sync the real Slack workspace into the local SQLite mirror that the
novel verbs (drift, who-said, customer-intel, reactions summarize, ...)
query. Unlike the generic 'sync' command — which snapshots raw API
responses — 'sync mirror' builds typed channel/user/message/thread/
reaction tables with a per-channel high-water-mark cursor.

What it does:
  1. conversations.list  -> m_channels   (public, private, im, mpim)
  2. users.list          -> m_users
  3. usergroups.list     -> m_usergroups
  4. conversations.history (since each channel's cursor) for member
     channels -> m_messages + m_reactions + m_files
  5. conversations.replies for threaded messages -> thread replies +
     m_threads.last_reply_ts

Channels listed in ~/.config/slack-pp-cli/skip.yaml (skip_channels: [...])
are skipped during history sync. Every DM/MPIM read is recorded in the
append-only audit log.`,
		Example: `  # Full mirror sync
  slack-pp-cli sync mirror --full

  # Incremental: only history newer than each channel's cursor
  slack-pp-cli sync mirror

  # Only the last 7 days, two named channels
  slack-pp-cli sync mirror --since 7d --channels C0123,C0456

  # Preview without network IO
  slack-pp-cli sync mirror --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Help-only invocation falls through to Help() — no required
			// args, so nothing to validate here.
			if dryRunOK(flags) {
				if humanFriendly {
					fmt.Fprintln(os.Stderr, "(dry run) would sync channels, users, usergroups, history, threads into the local mirror — no request sent")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), `{"event":"dry_run","command":"sync mirror"}`)
				}
				return nil
			}

			var sinceTS string
			if since != "" {
				t, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
				}
				// Slack ts is decimal seconds since epoch.
				sinceTS = fmt.Sprintf("%d.000000", t.Unix())
			}

			var onlyChannels []string
			for _, c := range strings.Split(channelsCSV, ",") {
				c = strings.TrimSpace(c)
				if c != "" {
					onlyChannels = append(onlyChannels, c)
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureMirrorSchema(cmd.Context()); err != nil {
				return err
			}

			skip, err := loadSkipChannels()
			if err != nil {
				return err
			}

			stats, err := runMirrorSync(cmd.Context(), c, db, mirrorSyncOpts{
				full:         full,
				sinceTS:      sinceTS,
				onlyChannels: onlyChannels,
				skip:         skip,
			})
			if err != nil {
				return err
			}

			if humanFriendly {
				fmt.Fprintf(os.Stderr,
					"mirror sync complete: %d channels, %d users, %d usergroups, %d messages, %d threads, %d reactions, %d files\n",
					stats.Channels, stats.Users, stats.Usergroups, stats.Messages,
					stats.Threads, stats.Reactions, stats.Files)
				for _, w := range stats.Warnings {
					fmt.Fprintln(os.Stderr, "  warning:", w)
				}
				return nil
			}
			out, _ := json.Marshal(map[string]any{"event": "mirror_sync_complete", "stats": stats})
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "Ignore per-channel cursors and re-sync all history")
	cmd.Flags().StringVar(&since, "since", "", "Only sync history newer than this duration (e.g. 7d, 24h, 1w)")
	cmd.Flags().StringVar(&channelsCSV, "channels", "", "Comma-separated channel IDs to sync (default: all member channels)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/slack-pp-cli/data.db)")

	return cmd
}

// mirrorSyncOpts carries the resolved flags into the engine.
type mirrorSyncOpts struct {
	full         bool
	sinceTS      string
	onlyChannels []string
	skip         map[string]bool
	// teamDomain is the workspace subdomain (e.g. "atom-chat") resolved from
	// auth.test at sync start. Used to synthesize message permalinks, since
	// conversations.history does not return them.
	teamDomain string
}

// resolveTeamDomain calls auth.test and extracts the workspace subdomain from
// the returned URL (e.g. "https://atom-chat.slack.com/" -> "atom-chat").
// Returns "" on any failure — permalink synthesis is then skipped, not fatal.
func resolveTeamDomain(c mirrorClient) string {
	data, err := c.Get("/auth.test", nil)
	if err != nil {
		return ""
	}
	var body struct {
		OK  bool   `json:"ok"`
		URL string `json:"url"`
	}
	if json.Unmarshal(data, &body) != nil || !body.OK {
		return ""
	}
	host := body.URL
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	if i := strings.IndexByte(host, '.'); i > 0 {
		return host[:i]
	}
	return ""
}

// messagePermalink synthesizes a Slack archive permalink for a message.
// Slack permalinks are deterministic: the ts with its dot removed, prefixed
// with 'p'. Returns "" when the team domain is unknown.
func messagePermalink(teamDomain, channelID, ts string) string {
	if teamDomain == "" || channelID == "" || ts == "" {
		return ""
	}
	return fmt.Sprintf("https://%s.slack.com/archives/%s/p%s",
		teamDomain, channelID, strings.ReplaceAll(ts, ".", ""))
}

// runMirrorSync executes the four-stage mirror sync. Extracted from RunE
// so it is testable against a fake mirrorClient.
func runMirrorSync(ctx context.Context, c mirrorClient, db *store.Store, opts mirrorSyncOpts) (mirrorSyncStats, error) {
	var stats mirrorSyncStats

	// Resolve the workspace subdomain so history pages can be given permalinks.
	if opts.teamDomain == "" {
		opts.teamDomain = resolveTeamDomain(c)
	}

	// Stage 1 — channels.
	channels, err := syncChannels(ctx, c, db)
	if err != nil {
		return stats, err
	}
	stats.Channels = len(channels)

	// Stage 2 — users.
	n, err := syncUsers(ctx, c, db)
	if err != nil {
		return stats, err
	}
	stats.Users = n

	// Stage 3 — usergroups. A token without `usergroups:read` must not abort
	// the run; warn and continue with the rest of the mirror.
	n, err = syncUsergroups(ctx, c, db)
	if err != nil {
		if !isPermissionErr(err) {
			return stats, err
		}
		stats.Warnings = append(stats.Warnings, fmt.Sprintf("usergroups skipped: %v", err))
	} else {
		stats.Usergroups = n
	}

	// Stage 4 — history + threads, per member channel.
	onlySet := map[string]bool{}
	for _, id := range opts.onlyChannels {
		onlySet[id] = true
	}
	for _, ch := range channels {
		if len(onlySet) > 0 && !onlySet[ch.ID] {
			continue
		}
		if !ch.IsMember {
			continue
		}
		if opts.skip[ch.ID] || opts.skip[ch.Name] {
			continue
		}
		// Reading a DM/MPIM is an audited event.
		if ch.IsIM || ch.IsMPIM {
			if err := db.AppendAuditLog(ctx, "sync", "sync mirror", ch.ID,
				fmt.Sprintf("history read of %s channel", dmKind(ch))); err != nil {
				return stats, err
			}
		}

		cs, err := syncChannelHistory(ctx, c, db, ch, opts)
		if err != nil {
			if !isPermissionErr(err) {
				return stats, err
			}
			stats.Warnings = append(stats.Warnings,
				fmt.Sprintf("channel %s (%s) skipped: %v", ch.Name, ch.ID, err))
			continue
		}
		stats.Messages += cs.messages
		stats.Threads += cs.threads
		stats.Reactions += cs.reactions
		stats.Files += cs.files
	}

	return stats, nil
}

func dmKind(ch store.Channel) string {
	if ch.IsMPIM {
		return "mpim"
	}
	return "im"
}

// slackCursorGet calls a Slack method and surfaces ok:false as an error.
// Slack 429s are already retried inside client.do; if a 429 still escapes
// it arrives here wrapped as a *client.APIError and is converted to a
// *cliutil.RateLimitError, so callers see a typed throttle error rather
// than an empty result (empty-on-throttle silently corrupts the mirror).
func slackCursorGet(c mirrorClient, path string, params map[string]string) (json.RawMessage, slackResp, error) {
	data, err := c.Get(path, params)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 429 {
			return nil, slackResp{}, &cliutil.RateLimitError{
				URL:  path,
				Body: apiErr.Body,
			}
		}
		return nil, slackResp{}, fmt.Errorf("%s: %w", path, err)
	}
	var env slackResp
	if uerr := json.Unmarshal(data, &env); uerr != nil {
		return nil, slackResp{}, fmt.Errorf("%s: decoding response: %w", path, uerr)
	}
	if !env.OK {
		// Slack also signals throttling via an ok:false body
		// (error:"ratelimited") on some endpoints — surface it as the
		// same typed error so callers never treat throttle as no-data.
		if env.Error == "ratelimited" || env.Error == "rate_limited" {
			return nil, env, &cliutil.RateLimitError{URL: path, Body: env.Error}
		}
		return nil, env, fmt.Errorf("%s: slack error: %s", path, env.Error)
	}
	return data, env, nil
}

// syncChannels paginates conversations.list and upserts every channel.
func syncChannels(ctx context.Context, c mirrorClient, db *store.Store) ([]store.Channel, error) {
	var all []store.Channel
	cursor := ""
	for {
		params := map[string]string{
			"types": "public_channel,private_channel,im,mpim",
			"limit": "200",
		}
		if cursor != "" {
			params["cursor"] = cursor
		}
		data, env, err := slackCursorGet(c, "/conversations.list", params)
		if err != nil {
			return nil, err
		}
		var body struct {
			Channels []rawChannel `json:"channels"`
		}
		if err := json.Unmarshal(data, &body); err != nil {
			return nil, fmt.Errorf("conversations.list: decoding channels: %w", err)
		}
		for _, rc := range body.Channels {
			ch := rc.toChannel()
			if err := db.UpsertChannel(ctx, ch); err != nil {
				return nil, err
			}
			all = append(all, ch)
		}
		if env.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = env.ResponseMetadata.NextCursor
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return all, nil
}

// syncUsers paginates users.list and upserts every user.
func syncUsers(ctx context.Context, c mirrorClient, db *store.Store) (int, error) {
	count := 0
	cursor := ""
	for {
		params := map[string]string{"limit": "200"}
		if cursor != "" {
			params["cursor"] = cursor
		}
		data, env, err := slackCursorGet(c, "/users.list", params)
		if err != nil {
			return count, err
		}
		var body struct {
			Members []rawUser `json:"members"`
		}
		if err := json.Unmarshal(data, &body); err != nil {
			return count, fmt.Errorf("users.list: decoding members: %w", err)
		}
		for _, ru := range body.Members {
			if err := db.UpsertUser(ctx, ru.toUser()); err != nil {
				return count, err
			}
			count++
		}
		if env.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = env.ResponseMetadata.NextCursor
		if err := ctx.Err(); err != nil {
			return count, err
		}
	}
	return count, nil
}

// syncUsergroups fetches usergroups.list (with members) and upserts each.
func syncUsergroups(ctx context.Context, c mirrorClient, db *store.Store) (int, error) {
	data, _, err := slackCursorGet(c, "/usergroups.list", map[string]string{
		"include_users": "true",
	})
	if err != nil {
		return 0, err
	}
	var body struct {
		Usergroups []rawUsergroup `json:"usergroups"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return 0, fmt.Errorf("usergroups.list: decoding usergroups: %w", err)
	}
	count := 0
	for _, rg := range body.Usergroups {
		if err := db.UpsertUsergroup(ctx, rg.toUsergroup()); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// channelHistoryStats counts what one channel's history sync produced.
type channelHistoryStats struct {
	messages  int
	threads   int
	reactions int
	files     int
}

// syncChannelHistory paginates conversations.history for one channel
// (incremental from the channel cursor unless --full or --since override
// it), upserts messages/reactions/files, then walks threaded messages via
// conversations.replies.
func syncChannelHistory(ctx context.Context, c mirrorClient, db *store.Store, ch store.Channel, opts mirrorSyncOpts) (channelHistoryStats, error) {
	var cs channelHistoryStats

	// Resolve the oldest-ts bound: --full ignores the cursor; otherwise
	// the channel high-water-mark, then --since if it is more recent.
	oldest := ""
	if !opts.full {
		cur, err := db.GetChannelCursor(ctx, ch.ID)
		if err != nil {
			return cs, err
		}
		oldest = cur
	}
	if opts.sinceTS != "" && opts.sinceTS > oldest {
		oldest = opts.sinceTS
	}

	const pageLimit = 200
	cursor := ""
	var newestTS string
	for {
		params := map[string]string{
			"channel": ch.ID,
			"limit":   fmt.Sprintf("%d", pageLimit),
		}
		if oldest != "" {
			params["oldest"] = oldest
		}
		if cursor != "" {
			params["cursor"] = cursor
		}
		data, env, err := slackCursorGet(c, "/conversations.history", params)
		if err != nil {
			return cs, err
		}
		var body struct {
			Messages []rawMessage `json:"messages"`
		}
		if err := json.Unmarshal(data, &body); err != nil {
			return cs, fmt.Errorf("conversations.history: decoding messages: %w", err)
		}

		// Silent-truncation warning: a page shorter than the requested
		// limit while a next_cursor still advances is the May-2025
		// page-size cap. Slack's last natural page is also short, hence
		// the next_cursor guard.
		if len(body.Messages) < pageLimit && env.ResponseMetadata.NextCursor != "" {
			fmt.Fprintf(os.Stderr,
				"warning: conversations.history for %s returned %d messages for a requested limit of %d while paginating — Slack's page-size cap may be silently truncating; results remain complete via cursor pagination\n",
				ch.ID, len(body.Messages), pageLimit)
		}

		msgs, reactions, files, threadParents := convertHistoryPage(ch.ID, body.Messages, opts.teamDomain)
		if err := db.UpsertMessages(ctx, msgs); err != nil {
			return cs, err
		}
		if err := db.UpsertReactions(ctx, reactions); err != nil {
			return cs, err
		}
		for _, f := range files {
			if err := db.UpsertFile(ctx, f); err != nil {
				return cs, err
			}
		}
		cs.messages += len(msgs)
		cs.reactions += len(reactions)
		cs.files += len(files)

		for _, m := range msgs {
			if newestTS == "" || m.TS > newestTS {
				newestTS = m.TS
			}
		}

		// Walk threads on this page.
		for _, parentTS := range threadParents {
			tn, rn, fn, err := syncThread(ctx, c, db, ch.ID, parentTS, opts.teamDomain)
			if err != nil {
				return cs, err
			}
			cs.threads++
			cs.messages += tn
			cs.reactions += rn
			cs.files += fn
		}

		if env.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = env.ResponseMetadata.NextCursor
		if err := ctx.Err(); err != nil {
			return cs, err
		}
	}

	// Advance the channel high-water-mark only when we saw new messages.
	if newestTS != "" {
		if err := db.SetChannelCursor(ctx, ch.ID, newestTS); err != nil {
			return cs, err
		}
	}
	return cs, nil
}

// syncThread fetches every reply of one thread via conversations.replies,
// upserts the replies, and records the thread's last_reply_ts. The reply
// counts returned exclude the parent (it was already upserted by the
// history page).
func syncThread(ctx context.Context, c mirrorClient, db *store.Store, channelID, parentTS, teamDomain string) (msgs, reactions, files int, err error) {
	const pageLimit = 200
	cursor := ""
	var lastReplyTS string
	replyCount := 0
	for {
		params := map[string]string{
			"channel": channelID,
			"ts":      parentTS,
			"limit":   fmt.Sprintf("%d", pageLimit),
		}
		if cursor != "" {
			params["cursor"] = cursor
		}
		data, env, gerr := slackCursorGet(c, "/conversations.replies", params)
		if gerr != nil {
			return msgs, reactions, files, gerr
		}
		var body struct {
			Messages []rawMessage `json:"messages"`
		}
		if uerr := json.Unmarshal(data, &body); uerr != nil {
			return msgs, reactions, files, fmt.Errorf("conversations.replies: decoding messages: %w", uerr)
		}
		if len(body.Messages) < pageLimit && env.ResponseMetadata.NextCursor != "" {
			fmt.Fprintf(os.Stderr,
				"warning: conversations.replies for %s/%s returned %d messages for a requested limit of %d while paginating — Slack's page-size cap may be silently truncating; results remain complete via cursor pagination\n",
				channelID, parentTS, len(body.Messages), pageLimit)
		}

		m, r, f, _ := convertHistoryPage(channelID, body.Messages, teamDomain)
		// The first message of the first page is the thread parent — it
		// was already upserted by the history page, but re-upserting is
		// idempotent, so we keep it for completeness and just don't
		// double-count it as a reply below.
		if uerr := db.UpsertMessages(ctx, m); uerr != nil {
			return msgs, reactions, files, uerr
		}
		if uerr := db.UpsertReactions(ctx, r); uerr != nil {
			return msgs, reactions, files, uerr
		}
		for _, file := range f {
			if uerr := db.UpsertFile(ctx, file); uerr != nil {
				return msgs, reactions, files, uerr
			}
		}
		for _, mm := range m {
			if mm.TS != parentTS {
				msgs++
				replyCount++
				if mm.TS > lastReplyTS {
					lastReplyTS = mm.TS
				}
			}
		}
		reactions += len(r)
		files += len(f)

		if env.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = env.ResponseMetadata.NextCursor
		if cerr := ctx.Err(); cerr != nil {
			return msgs, reactions, files, cerr
		}
	}

	if lastReplyTS == "" {
		lastReplyTS = parentTS
	}
	if serr := db.SetThread(ctx, store.Thread{
		ChannelID:   channelID,
		ParentTS:    parentTS,
		LastReplyTS: lastReplyTS,
		ReplyCount:  replyCount,
	}); serr != nil {
		return msgs, reactions, files, serr
	}
	return msgs, reactions, files, nil
}

// ----- raw Slack JSON shapes -> typed store rows -------------------------

type rawChannel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsArchived bool   `json:"is_archived"`
	IsMember   bool   `json:"is_member"`
	IsIM       bool   `json:"is_im"`
	IsMpim     bool   `json:"is_mpim"`
	IsPrivate  bool   `json:"is_private"`
	IsGroup    bool   `json:"is_group"`
	NumMembers int    `json:"num_members"`
	Topic      struct {
		Value string `json:"value"`
	} `json:"topic"`
	Purpose struct {
		Value string `json:"value"`
	} `json:"purpose"`
	User string `json:"user"` // present on IM channels
}

func (rc rawChannel) toChannel() store.Channel {
	name := rc.Name
	if name == "" && rc.IsIM && rc.User != "" {
		// IMs have no name; surface the peer user id so ResolveChannel
		// can still find them.
		name = "dm:" + rc.User
	}
	return store.Channel{
		ID:         rc.ID,
		Name:       name,
		IsArchived: rc.IsArchived,
		IsMember:   rc.IsMember || rc.IsIM || rc.IsMpim,
		IsIM:       rc.IsIM,
		IsMPIM:     rc.IsMpim,
		IsPrivate:  rc.IsPrivate || rc.IsGroup,
		NumMembers: rc.NumMembers,
		Topic:      rc.Topic.Value,
		Purpose:    rc.Purpose.Value,
	}
}

type rawUser struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	IsBot   bool   `json:"is_bot"`
	Deleted bool   `json:"deleted"`
	TZ      string `json:"tz"`
	Profile struct {
		RealName    string `json:"real_name"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
	} `json:"profile"`
}

func (ru rawUser) toUser() store.User {
	return store.User{
		ID:          ru.ID,
		Name:        ru.Name,
		RealName:    ru.Profile.RealName,
		DisplayName: ru.Profile.DisplayName,
		Email:       ru.Profile.Email,
		IsBot:       ru.IsBot,
		Deleted:     ru.Deleted,
		TZ:          ru.TZ,
	}
}

type rawUsergroup struct {
	ID     string `json:"id"`
	Handle string `json:"handle"`
	Name   string `json:"name"`
	Users  []string `json:"users"`
}

func (rg rawUsergroup) toUsergroup() store.Usergroup {
	return store.Usergroup{
		ID:      rg.ID,
		Handle:  rg.Handle,
		Name:    rg.Name,
		UserIDs: rg.Users,
	}
}

// rawMessage is the subset of a Slack message object the mirror persists.
type rawMessage struct {
	TS         string          `json:"ts"`
	ThreadTS   string          `json:"thread_ts"`
	User       string          `json:"user"`
	BotID      string          `json:"bot_id"`
	Text       string          `json:"text"`
	Subtype    string          `json:"subtype"`
	ReplyCount int             `json:"reply_count"`
	ReplyUsers []string        `json:"reply_users"`
	Reactions  []rawReaction   `json:"reactions"`
	Files      []rawFile       `json:"files"`
	Permalink  string          `json:"permalink"`
}

type rawReaction struct {
	Name  string   `json:"name"`
	Users []string `json:"users"`
	Count int      `json:"count"`
}

type rawFile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Mimetype   string `json:"mimetype"`
	URLPrivate string `json:"url_private"`
	Permalink  string `json:"permalink"`
	Created    int64  `json:"created"`
}

// convertHistoryPage turns a page of raw Slack messages into typed mirror
// rows. It returns the messages, every reaction across them, every file,
// and the ts values of messages that have a thread to walk.
func convertHistoryPage(channelID string, raws []rawMessage, teamDomain string) (msgs []store.Message, reactions []store.Reaction, files []store.File, threadParents []string) {
	for _, rm := range raws {
		userID := rm.User
		if userID == "" {
			userID = rm.BotID
		}
		permalink := rm.Permalink
		if permalink == "" {
			permalink = messagePermalink(teamDomain, channelID, rm.TS)
		}
		m := store.Message{
			ChannelID:  channelID,
			TS:         rm.TS,
			ThreadTS:   rm.ThreadTS,
			UserID:     userID,
			Text:       rm.Text,
			Subtype:    rm.Subtype,
			ReplyCount: rm.ReplyCount,
			Permalink:  permalink,
		}
		if len(rm.ReplyUsers) > 0 {
			if b, err := json.Marshal(rm.ReplyUsers); err == nil {
				m.ReplyUsers = b
			}
		}
		if len(rm.Reactions) > 0 {
			if b, err := json.Marshal(rm.Reactions); err == nil {
				m.Reactions = b
			}
		}
		if len(rm.Files) > 0 {
			if b, err := json.Marshal(rm.Files); err == nil {
				m.Files = b
			}
		}
		msgs = append(msgs, m)

		for _, rr := range rm.Reactions {
			reactions = append(reactions, store.Reaction{
				MessageChannelID: channelID,
				MessageTS:        rm.TS,
				EmojiName:        rr.Name,
				UserIDs:          rr.Users,
				Count:            rr.Count,
			})
		}
		for _, rf := range rm.Files {
			files = append(files, store.File{
				ID:         rf.ID,
				Name:       rf.Name,
				Mimetype:   rf.Mimetype,
				URLPrivate: rf.URLPrivate,
				Permalink:  rf.Permalink,
				ChannelID:  channelID,
				Created:    rf.Created,
			})
		}

		// A message is a thread root when reply_count>0 and it is its
		// own parent (thread_ts == ts, or thread_ts empty on the root).
		isRoot := rm.ThreadTS == "" || rm.ThreadTS == rm.TS
		if rm.ReplyCount > 0 && isRoot {
			threadParents = append(threadParents, rm.TS)
		}
	}
	return msgs, reactions, files, threadParents
}

// loadSkipChannels reads ~/.config/slack-pp-cli/skip.yaml and returns the
// set of channel ids/names to skip during history sync. An absent file is
// not an error — it means "sync everything".
//
// The file is a one-key YAML doc: `skip_channels: [C01, general, ...]`.
// To avoid pulling a YAML dependency into go.mod for one flat list, the
// (block- or flow-style) list is parsed with a small hand-rolled scanner.
func loadSkipChannels() (map[string]bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}
	path := filepath.Join(home, ".config", "slack-pp-cli", "skip.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("reading skip list %s: %w", path, err)
	}
	return parseSkipYAML(string(data)), nil
}

// parseSkipYAML extracts the skip_channels list from a minimal YAML doc.
// Supports flow style (`skip_channels: [a, b]`) and block style
// (`skip_channels:` then `  - a`). Comments (#) and quotes are stripped.
func parseSkipYAML(doc string) map[string]bool {
	out := map[string]bool{}
	lines := strings.Split(doc, "\n")
	inBlock := false
	for _, raw := range lines {
		line := raw
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if inBlock {
			if strings.HasPrefix(trimmed, "- ") {
				if v := cleanSkipToken(trimmed[2:]); v != "" {
					out[v] = true
				}
				continue
			}
			// A non-list line ends the block.
			inBlock = false
		}
		key, val, ok := strings.Cut(trimmed, ":")
		if !ok || strings.TrimSpace(key) != "skip_channels" {
			continue
		}
		val = strings.TrimSpace(val)
		if val == "" {
			inBlock = true
			continue
		}
		// Flow style: [a, b, c]
		val = strings.TrimPrefix(val, "[")
		val = strings.TrimSuffix(val, "]")
		for _, tok := range strings.Split(val, ",") {
			if v := cleanSkipToken(tok); v != "" {
				out[v] = true
			}
		}
	}
	return out
}

// cleanSkipToken strips surrounding whitespace, quotes, and a leading '#'
// from a channel token.
func cleanSkipToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, `"'`)
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	return s
}

// attachMirrorSyncCmd hangs `mirror` off the generated `sync` command so
// the invocation is `slack-pp-cli sync mirror`. Called from root.go.
func attachMirrorSyncCmd(rootCmd *cobra.Command, flags *rootFlags) {
	for _, child := range rootCmd.Commands() {
		if child.Name() == "sync" {
			child.AddCommand(newSyncMirrorCmd(flags))
			return
		}
	}
	// Defensive fallback: if the generated sync command is ever removed,
	// expose mirror sync at the top level so the verb still works.
	rootCmd.AddCommand(newSyncMirrorCmd(flags))
}
