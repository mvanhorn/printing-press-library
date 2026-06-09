// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/store"
	"github.com/spf13/cobra"
)

type threadContextResult struct {
	Meta         map[string]any       `json:"meta"`
	RootTweet    *resolvedPostRecord  `json:"root_tweet,omitempty"`
	FocusTweet   *resolvedPostRecord  `json:"focus_tweet"`
	Ancestors    []resolvedPostRecord `json:"ancestors,omitempty"`
	QuotedPosts  []resolvedPostRecord `json:"quoted_posts,omitempty"`
	Replies      []threadContextReply `json:"replies,omitempty"`
	Participants []postAuthorSummary  `json:"participants,omitempty"`
	Gaps         []string             `json:"gaps,omitempty"`
	Warnings     []string             `json:"warnings,omitempty"`
}

type threadContextReply struct {
	resolvedPostRecord
	InReplyTo string `json:"in_reply_to,omitempty"`
	Depth     int    `json:"depth"`
}

func newNovelThreadContextCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var includeParents, includeQuotes, includeReplies, live bool
	var depth, limit int

	cmd := &cobra.Command{
		Use:   "context <url-or-id>",
		Short: "Resolve a post and reconstruct surrounding conversation context",
		Long:  "thread context accepts an X URL or post ID, resolves the focus post, includes parent/quote context when available, and can include bounded local/live replies. It is read-only toward X.",
		Example: `  x-twitter-pp-cli thread context https://x.com/user/status/123 --agent
  x-twitter-pp-cli thread context 123 --replies --depth 3 --limit 100 --agent`,
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			mode := flags.dataSource
			if live {
				mode = "live"
			}
			result, err := buildThreadContext(cmd, flags, args[0], dbPath, mode, includeParents, includeQuotes, includeReplies, depth, limit)
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				rows := [][]string{{"focus", result.FocusTweet.TweetID, authorDisplay(result.FocusTweet.Author), truncatePlain(result.FocusTweet.Text, 96)}}
				for _, a := range result.Ancestors {
					rows = append(rows, []string{"ancestor", a.TweetID, authorDisplay(a.Author), truncatePlain(a.Text, 96)})
				}
				for _, q := range result.QuotedPosts {
					rows = append(rows, []string{"quote", q.TweetID, authorDisplay(q.Author), truncatePlain(q.Text, 96)})
				}
				for _, r := range result.Replies {
					rows = append(rows, []string{fmt.Sprintf("reply:%d", r.Depth), r.TweetID, authorDisplay(r.Author), truncatePlain(r.Text, 96)})
				}
				return flags.printTable(cmd, []string{"TYPE", "ID", "AUTHOR", "TEXT"}, rows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database (defaults to the standard cache location)")
	cmd.Flags().BoolVar(&includeParents, "parents", true, "Include replied-to parent chain when available")
	cmd.Flags().BoolVar(&includeQuotes, "quotes", true, "Include quoted posts when available")
	cmd.Flags().BoolVar(&includeReplies, "replies", false, "Include bounded replies from local store and/or recent search")
	cmd.Flags().IntVar(&depth, "depth", 2, "Maximum parent/reply depth")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum replies to include")
	cmd.Flags().BoolVar(&live, "live", false, "Bypass local lookup for the focus post")
	return cmd
}

func buildThreadContext(cmd *cobra.Command, flags *rootFlags, input, dbPath, mode string, includeParents, includeQuotes, includeReplies bool, depth, limit int) (*threadContextResult, error) {
	if depth < 1 {
		depth = 1
	}
	if limit <= 0 {
		limit = 100
	}
	include := parseIncludeSet("author,links,refs,metrics")
	focus, err := resolvePost(cmd, flags, input, dbPath, mode, include)
	if err != nil {
		return nil, err
	}
	result := &threadContextResult{
		Meta: map[string]any{
			"source":       focus.Source,
			"command":      "thread context",
			"generated_at": generatedAt(),
		},
		FocusTweet: focus,
		Gaps:       []string{},
	}
	seen := map[string]bool{focus.TweetID: true}
	if includeParents {
		ancestors, gaps := resolveAncestorChain(cmd, flags, focus, dbPath, mode, include, depth, seen)
		result.Ancestors = ancestors
		result.Gaps = append(result.Gaps, gaps...)
	}
	if includeQuotes {
		quotes, gaps := resolveReferencedPosts(cmd, flags, focus, dbPath, mode, include, "quoted", seen)
		result.QuotedPosts = quotes
		result.Gaps = append(result.Gaps, gaps...)
	}
	if focus.ConversationID != "" && focus.ConversationID != focus.TweetID {
		root, err := resolvePost(cmd, flags, focus.ConversationID, dbPath, mode, include)
		if err == nil {
			result.RootTweet = root
			seen[root.TweetID] = true
		} else {
			result.Gaps = append(result.Gaps, "conversation_root_unavailable")
		}
	}
	if includeReplies {
		replies, source, gaps := loadContextReplies(cmd, flags, focus, dbPath, mode, include, limit, depth, seen)
		result.Replies = replies
		result.Gaps = append(result.Gaps, gaps...)
		if source != "" && source != focus.Source {
			result.Meta["source"] = "mixed"
		}
	}
	result.Participants = threadParticipants(result)
	if len(result.Gaps) == 0 {
		result.Gaps = nil
	}
	return result, nil
}

func resolveAncestorChain(cmd *cobra.Command, flags *rootFlags, focus *resolvedPostRecord, dbPath, mode string, include map[string]bool, maxDepth int, seen map[string]bool) ([]resolvedPostRecord, []string) {
	var ancestors []resolvedPostRecord
	var gaps []string
	current := focus
	for i := 0; i < maxDepth; i++ {
		parentID := referencedID(current, "replied_to")
		if parentID == "" {
			break
		}
		if seen[parentID] {
			gaps = append(gaps, "reply_parent_cycle")
			break
		}
		parent, err := resolvePost(cmd, flags, parentID, dbPath, mode, include)
		if err != nil {
			gaps = append(gaps, "reply_parent_unavailable:"+parentID)
			break
		}
		ancestors = append(ancestors, *parent)
		seen[parent.TweetID] = true
		current = parent
	}
	return ancestors, gaps
}

func resolveReferencedPosts(cmd *cobra.Command, flags *rootFlags, focus *resolvedPostRecord, dbPath, mode string, include map[string]bool, refType string, seen map[string]bool) ([]resolvedPostRecord, []string) {
	var out []resolvedPostRecord
	var gaps []string
	for _, ref := range focus.ReferencedTweets {
		if ref.Type != refType || seen[ref.ID] {
			continue
		}
		rec, err := resolvePost(cmd, flags, ref.ID, dbPath, mode, include)
		if err != nil {
			gaps = append(gaps, refType+"_post_unavailable:"+ref.ID)
			continue
		}
		out = append(out, *rec)
		seen[rec.TweetID] = true
	}
	return out, gaps
}

func referencedID(rec *resolvedPostRecord, refType string) string {
	if rec == nil {
		return ""
	}
	for _, ref := range rec.ReferencedTweets {
		if ref.Type == refType {
			return ref.ID
		}
	}
	return ""
}

func loadContextReplies(cmd *cobra.Command, flags *rootFlags, focus *resolvedPostRecord, dbPath, mode string, include map[string]bool, limit, maxDepth int, skipIDs map[string]bool) ([]threadContextReply, string, []string) {
	var all []threadContextReply
	var gaps []string
	if dbPath == "" {
		dbPath = defaultDBPath("x-twitter-pp-cli")
	}
	if mode != "live" {
		local, err := loadLocalContextReplies(cmd, focus, dbPath, include, limit, skipIDs)
		if err == nil {
			all = append(all, local...)
		} else {
			gaps = append(gaps, "local_replies_unavailable")
		}
	}
	if mode != "local" && len(all) < limit && focus.ConversationID != "" {
		live, err := loadLiveContextReplies(cmd, flags, focus, include, limit-len(all), skipIDs)
		if err == nil {
			all = append(all, live...)
			gaps = append(gaps, "recent_search_window_only")
		} else {
			gaps = append(gaps, "live_replies_unavailable")
		}
	}
	all = dedupeContextReplies(all)
	all = assignReplyDepths(all, focus.TweetID, maxDepth)
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].CreatedAt != all[j].CreatedAt {
			return all[i].CreatedAt < all[j].CreatedAt
		}
		return all[i].TweetID < all[j].TweetID
	})
	source := ""
	if len(all) > 0 {
		source = all[0].Source
		for _, item := range all[1:] {
			if item.Source != source {
				source = "mixed"
				break
			}
		}
	}
	return all, source, gaps
}

func loadLocalContextReplies(cmd *cobra.Command, focus *resolvedPostRecord, dbPath string, include map[string]bool, limit int, skipIDs map[string]bool) ([]threadContextReply, error) {
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT data FROM (
			SELECT id, data, COALESCE(created_at, json_extract(data, '$.created_at'), '') AS created_at
			FROM tweets
			WHERE conversation_id = ? OR id = ?
			UNION ALL
			SELECT id, data, COALESCE(json_extract(data, '$.created_at'), '') AS created_at
			FROM resources
			WHERE (json_extract(data, '$.conversation_id') = ? OR id = ?)
			  AND resource_type IN ('tweets', 'users_tweets', 'liked_tweets', 'bookmarks', 'quote_tweets')
		)
		 ORDER BY created_at ASC
		 LIMIT ?`, focus.ConversationID, focus.ConversationID, focus.ConversationID, focus.ConversationID, limit*2)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var replies []threadContextReply
	seen := make(map[string]bool, len(skipIDs))
	for id, skip := range skipIDs {
		seen[id] = skip
	}
	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil || !data.Valid {
			continue
		}
		rec, err := normalizeTweetRecordWithOwnID(json.RawMessage(data.String), nil, "local", "synced", include)
		if err != nil || rec.TweetID == focus.TweetID {
			continue
		}
		if seen[rec.TweetID] {
			continue
		}
		parent := referencedID(rec, "replied_to")
		if parent == "" {
			continue
		}
		reply := threadContextReply{resolvedPostRecord: *rec, InReplyTo: parent, Depth: 1}
		replies = append(replies, reply)
		seen[rec.TweetID] = true
		if len(replies) >= limit {
			break
		}
	}
	return replies, rows.Err()
}

func loadLiveContextReplies(cmd *cobra.Command, flags *rootFlags, focus *resolvedPostRecord, include map[string]bool, limit int, skipIDs map[string]bool) ([]threadContextReply, error) {
	if limit <= 0 {
		return nil, nil
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	maxResults := limit
	if maxResults < 10 {
		maxResults = 10
	}
	if maxResults > 100 {
		maxResults = 100
	}
	data, err := c.Get(cmd.Context(), "/2/tweets/search/recent", map[string]string{
		"query":        "conversation_id:" + focus.ConversationID,
		"max_results":  strconv.Itoa(maxResults),
		"tweet.fields": "author_id,conversation_id,created_at,entities,public_metrics,referenced_tweets",
		"expansions":   "author_id,referenced_tweets.id,referenced_tweets.id.author_id",
		"user.fields":  "id,name,username,verified,public_metrics",
	})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data     []json.RawMessage `json:"data"`
		Includes struct {
			Users []map[string]any `json:"users"`
		} `json:"includes"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	users := map[string]*postAuthorSummary{}
	for _, u := range envelope.Includes.Users {
		user := userSummaryFromMap(u)
		if user.ID != "" {
			users[user.ID] = user
		}
	}
	replies := make([]threadContextReply, 0, minInt(limit, len(envelope.Data)))
	for _, raw := range envelope.Data {
		if len(replies) >= limit {
			break
		}
		rec, err := normalizeTweetRecordWithOwnID(raw, users, "live", "not_synced", include)
		if err != nil || rec.TweetID == focus.TweetID {
			continue
		}
		if skipIDs[rec.TweetID] {
			continue
		}
		parent := referencedID(rec, "replied_to")
		if parent == "" {
			continue
		}
		replies = append(replies, threadContextReply{resolvedPostRecord: *rec, InReplyTo: parent, Depth: 1})
	}
	return replies, nil
}

func assignReplyDepths(replies []threadContextReply, focusID string, maxDepth int) []threadContextReply {
	if maxDepth < 1 {
		maxDepth = 1
	}
	byID := make(map[string]threadContextReply, len(replies))
	for _, reply := range replies {
		if reply.TweetID != "" {
			byID[reply.TweetID] = reply
		}
	}
	cache := map[string]int{}
	visiting := map[string]bool{}
	var depthFor func(string) int
	depthFor = func(id string) int {
		if depth, ok := cache[id]; ok {
			return depth
		}
		if visiting[id] {
			return 1
		}
		visiting[id] = true
		defer delete(visiting, id)

		reply, ok := byID[id]
		if !ok || reply.InReplyTo == "" || reply.InReplyTo == focusID {
			cache[id] = 1
			return 1
		}
		depth := 1
		if _, ok := byID[reply.InReplyTo]; ok {
			depth = depthFor(reply.InReplyTo) + 1
		}
		cache[id] = depth
		return depth
	}
	out := replies[:0]
	for _, reply := range replies {
		reply.Depth = depthFor(reply.TweetID)
		if reply.Depth <= maxDepth {
			out = append(out, reply)
		}
	}
	return out
}

func dedupeContextReplies(in []threadContextReply) []threadContextReply {
	seen := map[string]bool{}
	out := in[:0]
	for _, item := range in {
		if item.TweetID == "" || seen[item.TweetID] {
			continue
		}
		seen[item.TweetID] = true
		out = append(out, item)
	}
	return out
}

func threadParticipants(result *threadContextResult) []postAuthorSummary {
	if result == nil {
		return nil
	}
	byID := map[string]postAuthorSummary{}
	add := func(author *postAuthorSummary) {
		if author == nil {
			return
		}
		key := author.ID
		if key == "" {
			key = author.Username
		}
		if key != "" {
			byID[key] = *author
		}
	}
	add(result.FocusTweet.Author)
	if result.RootTweet != nil {
		add(result.RootTweet.Author)
	}
	for _, item := range result.Ancestors {
		add(item.Author)
	}
	for _, item := range result.QuotedPosts {
		add(item.Author)
	}
	for _, item := range result.Replies {
		add(item.Author)
	}
	keys := make([]string, 0, len(byID))
	for k := range byID {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]postAuthorSummary, 0, len(keys))
	for _, k := range keys {
		out = append(out, byID[k])
	}
	return out
}
