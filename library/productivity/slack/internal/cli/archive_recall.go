// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: full-text search over the local Slack mirror.

// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/slackanalytics"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	// Registered under the `archive` parent, not at root: the framework's
	// learn loop already owns a top-level `recall` (see newRecallCmd in
	// teach.go). Sitting beside `archive coverage` also reads correctly —
	// both operate on the local mirror.
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent := findNovelParent(root, []string{"archive"})
		if parent == nil {
			return
		}
		addNovelCommandIfAbsent(parent, newNovelArchiveRecallCmd(flags))
	})
}

// recallThreadMsg is one sibling message attached as thread context.
type recallThreadMsg struct {
	TS        string `json:"ts"`
	Timestamp string `json:"timestamp"`
	User      string `json:"user"`
	UserName  string `json:"user_name"`
	Text      string `json:"text"`
	IsHit     bool   `json:"is_hit"`
	IsParent  bool   `json:"is_parent"`
}

// recallHit is one matching message with its resolved names, thread context,
// and retention verdict.
type recallHit struct {
	Channel              string            `json:"channel"`
	ChannelName          string            `json:"channel_name"`
	User                 string            `json:"user"`
	UserName             string            `json:"user_name"`
	Text                 string            `json:"text"`
	TS                   string            `json:"ts"`
	Timestamp            string            `json:"timestamp"`
	AgeDays              int               `json:"age_days"`
	Reactions            int               `json:"reactions"`
	ThreadTS             string            `json:"thread_ts"`
	ThreadSize           int               `json:"thread_size"`
	ThreadContext        []recallThreadMsg `json:"thread_context"`
	BeyondSlackRetention bool              `json:"beyond_slack_retention"`
}

func newNovelArchiveRecallCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagChannel string
	var flagFrom string
	var flagSince string
	var flagMinReactions int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "recall <query>",
		Short: "Find messages in your local archive, including ones Slack has already hidden behind the 90-day retention wall.",
		Long: "Use this command to find messages in the local archive, including messages older than Slack's 90-day retention wall, with thread context and resolved display names. " +
			"Do NOT use this command for a quick single-resource lookup of channels or users; use 'search' with --type instead.",
		Example: strings.Trim(`
  # Full-text search the local mirror
  slack-pp-cli archive recall "deploy rollback"

  # Narrow to one channel, one author, and the last week
  slack-pp-cli archive recall "incident" --channel C0GENERAL --from @alice --since 7d

  # Only well-received messages, as JSON for an agent
  slack-pp-cli archive recall "postmortem" --min-reactions 3 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search the local Slack mirror for matching messages (no API call, no writes)")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required: slack-pp-cli archive recall \"<query>\""))
			}
			var since time.Duration
			if strings.TrimSpace(flagSince) != "" {
				parsed, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				since = parsed
			}
			if dbPath == "" {
				dbPath = defaultDBPath("slack-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: slack-pp-cli sync --resources conversations,users && slack-pp-cli archive sync --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]recallHit, 0), flags)
				}
				return nil
			}

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "") {
				hintIfStale(cmd, db, "", flags.maxAge)
			}

			ranked, err := recallFTSCandidates(ctx, db, query)
			if err != nil {
				// FTS is an index over the same rows; a missing or damaged
				// index degrades to an unranked scan rather than failing.
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: full-text index unavailable (%v); falling back to a linear scan\n", err)
				ranked = nil
			}
			messages, err := loadLocalMessages(ctx, db)
			if err != nil {
				return err
			}
			channels, err := loadLocalChannels(ctx, db)
			if err != nil {
				return err
			}
			users, err := loadLocalUsers(ctx, db)
			if err != nil {
				return err
			}

			fromID := strings.TrimSpace(flagFrom)
			if fromID != "" {
				if match, ok := matchLocalUser(users, slackanalytics.ParseUserRef(fromID)); ok {
					fromID = match.ID
				}
			}

			warnUnmatchedChannelFilter(cmd.ErrOrStderr(), flagChannel, channels, messages)

			now := time.Now().UTC()
			// Slack markup is de-rendered on the way out only; the filters
			// below still match against the raw body Slack sent.
			renderer := newTextRenderer(users, channels)
			byKey := map[string]localMessage{}
			threadIndex := map[string][]localMessage{}
			for _, m := range messages {
				byKey[m.ResourceType+"\x00"+m.StoreID] = m
				if m.ThreadTS != "" {
					key := threadKey(m.Channel, m.ThreadTS)
					threadIndex[key] = append(threadIndex[key], m)
				}
			}

			// Rank order comes from FTS; the scan fallback keeps
			// newest-first ordering.
			ordered := make([]localMessage, 0, len(messages))
			if len(ranked) > 0 {
				for _, key := range ranked {
					if m, ok := byKey[key]; ok {
						ordered = append(ordered, m)
					}
				}
			} else {
				for i := len(messages) - 1; i >= 0; i-- {
					ordered = append(ordered, messages[i])
				}
			}

			limit := flagLimit
			if limit <= 0 {
				limit = 20
			}
			hits := make([]recallHit, 0, limit)
			for _, m := range ordered {
				if !slackanalytics.MatchesAllTokens(m.Text, query) {
					continue
				}
				if !recallChannelMatches(flagChannel, m.Channel, channels) {
					continue
				}
				if fromID != "" && !strings.EqualFold(m.User, fromID) {
					continue
				}
				if since > 0 && (!m.HasTime || now.Sub(m.At) > since) {
					continue
				}
				if m.Reactions < flagMinReactions {
					continue
				}
				hit := recallHit{
					Channel:              m.Channel,
					ChannelName:          channelLabel(channels, m.Channel),
					User:                 m.User,
					UserName:             userLabel(users, m.User),
					Text:                 renderer.Render(m.Text),
					TS:                   m.TS,
					Timestamp:            rfc3339(m.At, m.HasTime),
					AgeDays:              slackanalytics.AgeDays(m.At, now),
					Reactions:            m.Reactions,
					ThreadTS:             m.ThreadTS,
					ThreadContext:        make([]recallThreadMsg, 0, 4),
					BeyondSlackRetention: m.HasTime && slackanalytics.BeyondRetention(m.At, now, slackanalytics.RetentionWall),
				}
				if m.ThreadTS != "" {
					siblings := threadIndex[threadKey(m.Channel, m.ThreadTS)]
					hit.ThreadSize = len(siblings)
					for _, sib := range siblings {
						hit.ThreadContext = append(hit.ThreadContext, recallThreadMsg{
							TS:        sib.TS,
							Timestamp: rfc3339(sib.At, sib.HasTime),
							User:      sib.User,
							UserName:  userLabel(users, sib.User),
							Text:      renderer.RenderSnippet(sib.Text, 160),
							IsHit:     sib.TS == m.TS,
							IsParent:  sib.TS == m.ThreadTS,
						})
					}
				}
				hits = append(hits, hit)
				if len(hits) >= limit {
					break
				}
			}

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				return printJSONFiltered(out, hits, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintf(out, "No messages in the local mirror match %q.\n", query)
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "WHEN\tCHANNEL\tWHO\tRETENTION\tTEXT")
			for _, h := range hits {
				retention := "in-slack"
				if h.BeyondSlackRetention {
					retention = "archive-only"
				}
				when := h.Timestamp
				if when == "" {
					when = h.TS
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", when, h.ChannelName, h.UserName, retention, slackanalytics.Snippet(h.Text, 80))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum number of matching messages to return")
	cmd.Flags().StringVar(&flagChannel, "channel", "", "Restrict to one channel (ID or #name)")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Restrict to one author (user ID, @handle, or email)")
	cmd.Flags().StringVar(&flagSince, "since", "", "Only messages newer than this window (e.g. 24h, 7d, 4w)")
	cmd.Flags().IntVar(&flagMinReactions, "min-reactions", 0, "Only messages with at least this many reactions")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: resolved data directory data.db)")
	return cmd
}

// recallChannelMatches applies the --channel filter, accepting a channel ID,
// a #name, or a bare name.
func recallChannelMatches(filter, channelID string, channels map[string]localChannel) bool {
	want := strings.TrimSpace(filter)
	if want == "" {
		return true
	}
	if strings.EqualFold(want, channelID) {
		return true
	}
	bare := strings.TrimPrefix(want, "#")
	if ch, ok := channels[channelID]; ok {
		return strings.EqualFold(ch.Name, bare)
	}
	return false
}

// recallFTSCandidates returns store keys ("<resource_type>\x00<id>") for
// messages matching query, in FTS5 rank order. The rows are drained fully
// and closed before the caller issues any follow-up query: SQLite here is a
// single connection, so an open parent cursor would block the next read.
func recallFTSCandidates(ctx context.Context, db *store.Store, query string) ([]string, error) {
	match := slackanalytics.FTSMatchQuery(query)
	if match == "" {
		return nil, nil
	}
	types, args := inClause(localMessageResourceTypes)
	sqlText := `SELECT r.resource_type, r.id
		FROM resources r
		JOIN resources_fts f ON r.id = f.id AND r.resource_type = f.resource_type
		WHERE resources_fts MATCH ?
		  AND r.resource_type IN (` + types + `)
		  AND json_extract(r.data, '$.ts') IS NOT NULL
		  AND json_extract(r.data, '$.text') IS NOT NULL
		ORDER BY f.rank
		LIMIT 2000`
	queryArgs := append([]any{match}, args...)
	rows, err := db.DB().QueryContext(ctx, sqlText, queryArgs...)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, 32)
	for rows.Next() {
		var resourceType string
		var id sql.NullString
		if scanErr := rows.Scan(&resourceType, &id); scanErr != nil {
			_ = rows.Close() // error path: the returned error is more specific
			return nil, scanErr
		}
		keys = append(keys, resourceType+"\x00"+nullString(id))
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close() // error path: the returned error is more specific
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return keys, nil
}
