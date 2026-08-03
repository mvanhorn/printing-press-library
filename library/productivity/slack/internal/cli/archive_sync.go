// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: mirror Slack message history into the local store.
//
// The generated `sync` command walks flat list endpoints only. Slack's message
// history hangs off conversations.history?channel=<id>, a query-param-keyed
// dependent the generator's path-nested dependent model cannot express, so
// nothing else in this CLI ever writes messages to the mirror. Every
// archive-backed command (archive recall, catchup, threads stale, health,
// users activity) reads what this command writes.

// pp:data-source live

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent := findNovelParent(root, []string{"archive"})
		if parent == nil {
			return
		}
		addNovelCommandIfAbsent(parent, newNovelArchiveSyncCmd(flags))
	})
}

// slackEnvelope is the outer shape every Slack Web API method returns.
// Slack signals failure with HTTP 200 plus ok:false, so a transport-level
// error check alone will happily store an error object as if it were data.
type slackEnvelope struct {
	OK               bool              `json:"ok"`
	Error            string            `json:"error"`
	Warning          string            `json:"warning"`
	Messages         []json.RawMessage `json:"messages"`
	HasMore          bool              `json:"has_more"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// decodeSlackEnvelope unmarshals data and converts an ok:false body into a
// real error. Callers must never store a payload this rejects.
func decodeSlackEnvelope(data json.RawMessage) (*slackEnvelope, error) {
	var env slackEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decoding Slack response: %w", err)
	}
	if !env.OK {
		if env.Error == "" {
			return nil, fmt.Errorf("slack returned ok:false with no error code")
		}
		return nil, fmt.Errorf("slack error: %s", env.Error)
	}
	return &env, nil
}

// slackEnvelopeError reports a Slack ok:false body as an error, and nil for
// anything else. It is deliberately conservative: it fires only when the
// payload is a JSON object carrying an explicit boolean "ok" field set to
// false, so non-Slack shapes, arrays, and the dry-run sentinel pass through
// untouched. The generated sync loop calls this before extraction (see the
// pp:patch slack-ok-false-detection marker in sync.go).
func slackEnvelopeError(data json.RawMessage) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil // not a JSON object; nothing to assert about it
	}
	rawOK, present := probe["ok"]
	if !present {
		return nil
	}
	var ok bool
	if err := json.Unmarshal(rawOK, &ok); err != nil {
		return nil // "ok" present but not boolean; not the Slack envelope shape
	}
	if ok {
		return nil
	}
	code := ""
	if rawErr, hasErr := probe["error"]; hasErr {
		_ = json.Unmarshal(rawErr, &code)
	}
	if code == "" {
		return fmt.Errorf("slack returned ok:false with no error code")
	}
	return fmt.Errorf("slack error: %s", code)
}

// slackEnvelopeErrorCode returns the raw Slack error code from an ok:false
// body, or "" when the payload is not a failed Slack envelope.
func slackEnvelopeErrorCode(data json.RawMessage) string {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return ""
	}
	rawOK, present := probe["ok"]
	if !present {
		return ""
	}
	var ok bool
	if err := json.Unmarshal(rawOK, &ok); err != nil || ok {
		return ""
	}
	code := ""
	if rawErr, hasErr := probe["error"]; hasErr {
		_ = json.Unmarshal(rawErr, &code)
	}
	return code
}

// slackAccessDeniedCodes are per-resource capability limits, not broken
// credentials. Slack splits its surface across bot and user tokens, and no
// scope grant crosses that line — `search`, `stars`, and `reminders` simply
// cannot be reached with a bot token. Failing the whole sync run because two
// of eight resources are out of reach would make a perfectly good credential
// look broken, so these downgrade to per-resource warnings. Genuine
// credential failures (not_authed, invalid_auth, token_revoked,
// account_inactive) stay hard errors.
var slackAccessDeniedCodes = map[string]bool{
	"not_allowed_token_type":  true,
	"missing_scope":           true,
	"no_permission":           true,
	"not_in_channel":          true,
	"restricted_action":       true,
	"team_access_not_granted": true,
}

// resumeCursorResourceType stores per-channel history pagination state in the
// ordinary resources table, so a capped run can continue where it stopped
// instead of re-downloading the newest pages forever.
const resumeCursorResourceType = "archive_sync_cursor"

func loadResumeCursor(ctx context.Context, db *store.Store, channelID string) string {
	var raw sql.NullString
	err := db.DB().QueryRowContext(ctx,
		`SELECT json_extract(data, '$.cursor') FROM resources WHERE resource_type = ? AND id = ?`,
		resumeCursorResourceType, channelID).Scan(&raw)
	if err != nil || !raw.Valid {
		return ""
	}
	return raw.String
}

func saveResumeCursor(db *store.Store, channelID, cursor string) {
	payload, err := json.Marshal(map[string]string{"cursor": cursor, "channel": channelID})
	if err != nil {
		return
	}
	// Best-effort: a failed bookmark costs re-download, never correctness.
	_ = db.Upsert(resumeCursorResourceType, channelID, payload)
}

// injectChannel stamps the owning channel id onto a history message.
// conversations.history omits it (the channel is in the request, not the
// response), but every local reader resolves a message's channel from
// $.channel, so a message stored without it is unattributable.
func injectChannel(raw json.RawMessage, channelID string) (json.RawMessage, string, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, "", fmt.Errorf("decoding message: %w", err)
	}
	ts, _ := m["ts"].(string)
	if strings.TrimSpace(ts) == "" {
		return nil, "", fmt.Errorf("message has no ts")
	}
	m["channel"] = channelID
	out, err := json.Marshal(m)
	if err != nil {
		return nil, "", fmt.Errorf("re-encoding message: %w", err)
	}
	return out, ts, nil
}

type archiveSyncChannelResult struct {
	Channel      string `json:"channel"`
	Name         string `json:"channel_name"`
	Stored       int    `json:"messages_stored"`
	Pages        int    `json:"pages_fetched"`
	Threads      int    `json:"thread_replies_stored"`
	Capped       bool   `json:"page_cap_reached"`
	Resumed      bool   `json:"resumed_from_saved_cursor,omitempty"`
	NextCursor   string `json:"next_cursor,omitempty"`
	ThreadErrors int    `json:"thread_errors,omitempty"`
	ThreadError  string `json:"thread_error,omitempty"`
	Error        string `json:"error,omitempty"`
}

type archiveSyncReport struct {
	Channels       []archiveSyncChannelResult `json:"channels"`
	TotalStored    int                        `json:"total_messages_stored"`
	TotalThreads   int                        `json:"total_thread_replies_stored"`
	ChannelsOK     int                        `json:"channels_succeeded"`
	ChannelsFailed int                        `json:"channels_failed"`
	Since          string                     `json:"since,omitempty"`
	Note           string                     `json:"note,omitempty"`
}

func newNovelArchiveSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath       string
		channelsFlag string
		sinceFlag    string
		maxPages     int
		pageSize     int
		withThreads  bool
		restart      bool
	)

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Mirror Slack message history into the local archive",
		Long: "Use this command to populate the local message archive that 'archive recall', 'catchup', " +
			"'threads stale', and 'health' read from. It pages conversations.history per channel and stores " +
			"each message locally. Run the top-level 'sync' first so the channel list exists. " +
			"Do NOT use this command to refresh channels, users, or files; use the top-level 'sync' instead.",
		Example: strings.Trim(`
  slack-pp-cli archive sync --since 30d
  slack-pp-cli archive sync --channels C01234567 --with-threads --json
`, "\n"),
		Annotations: map[string]string{
			// mcp:local-write, not mcp:read-only: this creates nothing in Slack,
			// but it does write - it pages conversations.history and stores every
			// message in the local mirror. read-only would set readOnlyHint=true,
			// telling MCP hosts to skip the prompt for a command that mutates the
			// store and can issue a large number of calls against Slack's stricter
			// conversations.history rate-limit tier. local-write keeps
			// destructiveHint=false while leaving readOnlyHint unset, matching the
			// convention the teach/playbook commands already use for local-store
			// writes. The top-level 'sync' is likewise not marked read-only.
			"mcp:local-write": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would page conversations.history per channel and store messages in the local mirror")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var oldest string
			if strings.TrimSpace(sinceFlag) != "" {
				d, err := cliutil.ParseDurationLoose(sinceFlag)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--since %q: %w", sinceFlag, err))
				}
				oldest = strconv.FormatInt(time.Now().Add(-d).Unix(), 10)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: slack-pp-cli sync --resources conversations,users --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), archiveSyncReport{Channels: make([]archiveSyncChannelResult, 0)}, flags)
				}
				return nil
			}

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			channels, err := loadLocalChannels(ctx, db)
			if err != nil {
				return err
			}
			targets := selectSyncChannels(channels, channelsFlag)
			if len(targets) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no channels in the local mirror\nrun: slack-pp-cli sync --resources conversations --db %s\n", dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), archiveSyncReport{Channels: make([]archiveSyncChannelResult, 0)}, flags)
				}
				return nil
			}

			// Live dogfood runs under a flat per-command timeout; one page per
			// channel is enough to prove the path without blowing it.
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			report := archiveSyncReport{Channels: make([]archiveSyncChannelResult, 0, len(targets)), Since: sinceFlag}
			for _, ch := range targets {
				res := syncChannelHistory(ctx, c, db, ch, oldest, pageSize, maxPages, withThreads, restart)
				report.Channels = append(report.Channels, res)
				report.TotalStored += res.Stored
				report.TotalThreads += res.Threads
				if res.Error != "" {
					report.ChannelsFailed++
					// Surface per-channel failure immediately; a silent skip
					// is indistinguishable from an empty channel downstream.
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %s\n", ch.Label(), res.Error)
					continue
				}
				report.ChannelsOK++
			}
			if report.ChannelsFailed > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d channels failed; totals cover the remaining %d\n",
					report.ChannelsFailed, len(targets), report.ChannelsOK)
			}
			if report.TotalStored == 0 && report.ChannelsFailed == 0 {
				report.Note = "no messages returned; the bot may not be a member of these channels — invite it, or widen --since"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			for _, r := range report.Channels {
				status := fmt.Sprintf("%d msgs", r.Stored)
				if r.Threads > 0 {
					status += fmt.Sprintf(", %d replies", r.Threads)
				}
				if r.Error != "" {
					status = "ERROR: " + r.Error
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %s\n", r.Name, status)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d messages stored across %d channels\n", report.TotalStored, report.ChannelsOK)
			if report.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), report.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: resolved data directory data.db)")
	cmd.Flags().StringVar(&channelsFlag, "channels", "", "Comma-separated channel IDs or #names (default: every mirrored channel)")
	cmd.Flags().StringVar(&sinceFlag, "since", "", "Only fetch messages newer than this window (e.g. 24h, 7d, 4w)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum history pages to fetch per channel")
	cmd.Flags().IntVar(&pageSize, "page-size", 200, "Messages per API page (Slack caps this at 1000)")
	cmd.Flags().BoolVar(&withThreads, "with-threads", false, "Also fetch replies for messages that have a thread")
	cmd.Flags().BoolVar(&restart, "restart", false, "Ignore saved pagination bookmarks and start from the newest page")
	return cmd
}

// selectSyncChannels resolves the --channels filter against mirrored channels.
// An empty filter means every non-archived channel, ordered for stable output.
func selectSyncChannels(channels map[string]localChannel, filter string) []localChannel {
	wanted := map[string]bool{}
	for _, tok := range strings.Split(filter, ",") {
		tok = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tok), "#"))
		if tok != "" {
			wanted[strings.ToLower(tok)] = true
		}
	}
	out := make([]localChannel, 0, len(channels))
	for _, ch := range channels {
		if len(wanted) > 0 {
			if !wanted[strings.ToLower(ch.ID)] && !wanted[strings.ToLower(ch.Name)] {
				continue
			}
		} else if ch.IsArchived {
			continue
		}
		out = append(out, ch)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// syncChannelHistory pages one channel's history into the store. Errors are
// returned on the result rather than aborting the whole run so one
// inaccessible channel cannot hide every other channel's data.
func syncChannelHistory(ctx context.Context, c *client.Client, db *store.Store, ch localChannel, oldest string, pageSize, maxPages int, withThreads, restart bool) archiveSyncChannelResult {
	res := archiveSyncChannelResult{Channel: ch.ID, Name: ch.Label()}
	cursor := ""
	if !restart && oldest == "" {
		// Resume a previously capped run rather than restarting at the newest
		// page; otherwise repeated syncs never reach older history.
		cursor = loadResumeCursor(ctx, db, ch.ID)
		res.Resumed = cursor != ""
	}
	for page := 0; page < maxPages; page++ {
		params := map[string]string{
			"channel": ch.ID,
			"limit":   strconv.Itoa(pageSize),
		}
		if oldest != "" {
			params["oldest"] = oldest
		}
		if cursor != "" {
			// Slack's pagination key is `cursor`. The generator's default
			// (`after`) silently restarts at page 1 on every request.
			params["cursor"] = cursor
		}
		raw, err := c.Get(ctx, "/conversations.history", params)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		env, err := decodeSlackEnvelope(raw)
		if err != nil {
			res.Error = err.Error()
			return res
		}
		res.Pages++
		for _, m := range env.Messages {
			stamped, ts, err := injectChannel(m, ch.ID)
			if err != nil {
				continue
			}
			if upErr := db.Upsert("conversations_history", ch.ID+":"+ts, stamped); upErr != nil {
				res.Error = fmt.Sprintf("storing message %s: %v", ts, upErr)
				return res
			}
			res.Stored++
			if withThreads {
				n, threadErr := syncThreadReplies(ctx, c, db, ch.ID, stamped, pageSize, maxPages)
				res.Threads += n
				if threadErr != nil {
					// Surface rather than swallow: a thread that failed to
					// mirror is missing data, not an empty thread.
					res.ThreadErrors++
					if res.ThreadError == "" {
						res.ThreadError = threadErr.Error()
					}
				}
			}
		}
		cursor = env.ResponseMetadata.NextCursor
		if cursor == "" || !env.HasMore {
			saveResumeCursor(db, ch.ID, "") // history exhausted; clear the bookmark
			return res
		}
	}
	res.Capped = true
	res.NextCursor = cursor
	saveResumeCursor(db, ch.ID, cursor)
	return res
}

// syncThreadReplies stores every reply page for a parent that has a thread.
// It returns the number stored and any error, so the caller can report a
// partially-mirrored thread instead of reporting the channel as clean.
func syncThreadReplies(ctx context.Context, c *client.Client, db *store.Store, channelID string, parent json.RawMessage, pageSize, maxPages int) (int, error) {
	var p struct {
		TS         string `json:"ts"`
		ThreadTS   string `json:"thread_ts"`
		ReplyCount int    `json:"reply_count"`
	}
	if err := json.Unmarshal(parent, &p); err != nil || p.ReplyCount == 0 {
		return 0, nil
	}
	threadTS := p.ThreadTS
	if threadTS == "" {
		threadTS = p.TS
	}
	stored := 0
	cursor := ""
	for page := 0; page < maxPages; page++ {
		params := map[string]string{
			"channel": channelID,
			"ts":      threadTS,
			"limit":   strconv.Itoa(pageSize),
		}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.Get(ctx, "/conversations.replies", params)
		if err != nil {
			return stored, fmt.Errorf("thread %s: %w", threadTS, err)
		}
		env, err := decodeSlackEnvelope(raw)
		if err != nil {
			return stored, fmt.Errorf("thread %s: %w", threadTS, err)
		}
		for _, m := range env.Messages {
			stamped, ts, err := injectChannel(m, channelID)
			if err != nil || ts == threadTS {
				continue // skip the parent, already stored by the caller
			}
			if upErr := db.Upsert("conversations_replies", channelID+":"+ts, stamped); upErr != nil {
				return stored, fmt.Errorf("thread %s: storing reply %s: %w", threadTS, ts, upErr)
			}
			stored++
		}
		cursor = env.ResponseMetadata.NextCursor
		if cursor == "" || !env.HasMore {
			return stored, nil
		}
	}
	return stored, nil
}
