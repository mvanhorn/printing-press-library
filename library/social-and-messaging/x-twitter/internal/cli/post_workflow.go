// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/store"
	"github.com/spf13/cobra"
)

var (
	rawPostIDRE = regexp.MustCompile(`^[0-9]{5,}$`)
	urlPostIDRE = regexp.MustCompile(`/(?:status|statuses)/([0-9]+)(?:[/?#]|$)|/i/web/status/([0-9]+)(?:[/?#]|$)`)
)

type postAuthorSummary struct {
	ID       string `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
	Name     string `json:"name,omitempty"`
}

type resolvedPostRecord struct {
	Input                 string             `json:"input,omitempty"`
	TweetID               string             `json:"tweet_id"`
	URL                   string             `json:"url"`
	Author                *postAuthorSummary `json:"author,omitempty"`
	CreatedAt             string             `json:"created_at,omitempty"`
	Text                  string             `json:"text,omitempty"`
	ConversationID        string             `json:"conversation_id,omitempty"`
	ReferencedTweets      []tweetReference   `json:"referenced_tweets,omitempty"`
	PublicMetrics         map[string]any     `json:"public_metrics,omitempty"`
	NonPublicMetrics      map[string]any     `json:"non_public_metrics,omitempty"`
	OrganicMetrics        map[string]any     `json:"organic_metrics,omitempty"`
	Entities              map[string]any     `json:"entities,omitempty"`
	Media                 []map[string]any   `json:"media,omitempty"`
	PostType              string             `json:"post_type,omitempty"`
	Source                string             `json:"source"`
	LocalState            string             `json:"local_state"`
	NextSuggestedCommands []string           `json:"next_suggested_commands,omitempty"`
	Raw                   map[string]any     `json:"raw,omitempty"`
}

type tweetReference struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

func newNovelPostCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "post",
		Short:       "Resolve X post URLs or IDs into canonical local/live records",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelPostResolveCmd(flags))
	return cmd
}

func newNovelPostResolveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var include string
	var live bool

	cmd := &cobra.Command{
		Use:   "resolve <url-or-id>",
		Short: "Normalize an X post URL or ID into a canonical structured record",
		Long:  "Resolve accepts x.com/twitter.com post URLs and raw post IDs. It uses the local store first in auto mode, falls back to /2/tweets/:id with public read auth, and records whether the result came from local, live, or mixed data.",
		Example: `  x-twitter-pp-cli post resolve https://x.com/user/status/123 --agent
  x-twitter-pp-cli post resolve 123 --include author,media,links,refs,metrics --agent
  x-twitter-pp-cli post resolve 123 --live --json`,
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
			record, err := resolvePost(cmd, flags, args[0], dbPath, mode, parseIncludeSet(include))
			if err != nil {
				return err
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.printTable(cmd, []string{"ID", "AUTHOR", "SOURCE", "TEXT"}, [][]string{{
					record.TweetID,
					authorDisplay(record.Author),
					record.Source,
					truncatePlain(record.Text, 96),
				}})
			}
			return printJSONFiltered(cmd.OutOrStdout(), record, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local database (defaults to the standard cache location)")
	cmd.Flags().StringVar(&include, "include", "author,refs,metrics", "Comma-separated extras: author,media,links,refs,metrics,raw")
	cmd.Flags().BoolVar(&live, "live", false, "Bypass local lookup and fetch from the live X API")
	return cmd
}

func parseIncludeSet(include string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(include, ",") {
		key := strings.ToLower(strings.TrimSpace(part))
		if key != "" {
			out[key] = true
		}
	}
	return out
}

func extractPostID(input string) (string, error) {
	s := strings.TrimSpace(input)
	if rawPostIDRE.MatchString(s) {
		return s, nil
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", usageErr(fmt.Errorf("could not parse %q as an X post URL or numeric post ID", input))
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "x.com", "twitter.com", "mobile.twitter.com", "www.x.com", "www.twitter.com":
	default:
		return "", usageErr(fmt.Errorf("unsupported post URL host %q; expected x.com or twitter.com", u.Hostname()))
	}
	matches := urlPostIDRE.FindStringSubmatch(u.EscapedPath())
	if len(matches) == 0 {
		return "", usageErr(fmt.Errorf("could not find a /status/<id> segment in %q", input))
	}
	for _, m := range matches[1:] {
		if m != "" {
			return m, nil
		}
	}
	return "", usageErr(fmt.Errorf("could not find a post ID in %q", input))
}

func canonicalPostURL(id string, author *postAuthorSummary) string {
	if author != nil && author.Username != "" {
		return "https://x.com/" + author.Username + "/status/" + id
	}
	return "https://x.com/i/web/status/" + id
}

func resolvePost(cmd *cobra.Command, flags *rootFlags, input, dbPath, mode string, include map[string]bool) (*resolvedPostRecord, error) {
	id, err := extractPostID(input)
	if err != nil {
		return nil, err
	}
	if dbPath == "" {
		dbPath = defaultDBPath("x-twitter-pp-cli")
	}
	var localRecord *resolvedPostRecord
	var localErr error
	if mode != "live" {
		localRecord, localErr = resolvePostLocal(cmd, input, id, dbPath, include)
		if localErr == nil && localRecord != nil {
			return localRecord, nil
		}
		if mode == "local" {
			if localErr != nil {
				return nil, localErr
			}
			return nil, notFoundErr(fmt.Errorf("post %s not found in local store; run sync or retry with --data-source live", id))
		}
	}
	liveRecord, err := resolvePostLive(cmd, flags, input, id, dbPath, include)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	if localErr == nil && localRecord != nil {
		liveRecord.LocalState = "synced"
		liveRecord.Source = "mixed"
	}
	return liveRecord, nil
}

func resolvePostLocal(cmd *cobra.Command, input, id, dbPath string, include map[string]bool) (*resolvedPostRecord, error) {
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	defer db.Close()

	raw, err := localTweetJSON(cmd, db, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying local post %s: %w", id, err)
	}
	rec, err := normalizeTweetRecord(input, raw, nil, "local", "synced", include)
	if err != nil {
		return nil, err
	}
	if rec.Author != nil && rec.Author.Username == "" && rec.Author.ID != "" {
		if author, err := localUserSummary(cmd, db, rec.Author.ID); err == nil && author != nil {
			rec.Author = author
			rec.URL = canonicalPostURL(rec.TweetID, author)
		}
	}
	return rec, nil
}

func localTweetJSON(cmd *cobra.Command, db *store.Store, id string) (json.RawMessage, error) {
	var data string
	err := db.DB().QueryRowContext(cmd.Context(),
		`SELECT data FROM tweets WHERE id = ?
		 UNION ALL
		 SELECT data FROM resources WHERE id = ? AND resource_type IN ('tweets', 'users_tweets', 'liked_tweets', 'bookmarks', 'quote_tweets')
		 LIMIT 1`, id, id).Scan(&data)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func localUserSummary(cmd *cobra.Command, db *store.Store, id string) (*postAuthorSummary, error) {
	var data string
	if err := db.DB().QueryRowContext(cmd.Context(),
		`SELECT data FROM users WHERE id = ?
		 UNION ALL SELECT data FROM resources WHERE id = ? AND resource_type = 'users'
		 LIMIT 1`, id, id).Scan(&data); err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil, err
	}
	return userSummaryFromMap(obj), nil
}

func resolvePostLive(cmd *cobra.Command, flags *rootFlags, input, id, dbPath string, include map[string]bool) (*resolvedPostRecord, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	params := postLookupParams(include)
	data, err := c.Get(cmd.Context(), "/2/tweets/"+url.PathEscape(id), params)
	if err != nil {
		return nil, err
	}
	rec, err := normalizeTweetEnvelope(input, data, include, "live", "not_synced")
	if err != nil {
		return nil, err
	}
	if err := cacheResolvedPost(cmd, dbPath, data); err == nil {
		rec.LocalState = "synced"
	}
	return rec, nil
}

func postLookupParams(include map[string]bool) map[string]string {
	params := map[string]string{
		"tweet.fields": "author_id,conversation_id,created_at,entities,public_metrics,referenced_tweets",
		"expansions":   "author_id,referenced_tweets.id,referenced_tweets.id.author_id,attachments.media_keys",
		"user.fields":  "id,name,username,verified,public_metrics",
	}
	if include["media"] {
		params["media.fields"] = "media_key,type,url,preview_image_url,width,height,alt_text"
	}
	return params
}

func normalizeTweetEnvelope(input string, data json.RawMessage, include map[string]bool, source, localState string) (*resolvedPostRecord, error) {
	var envelope struct {
		Data     json.RawMessage `json:"data"`
		Includes struct {
			Users []map[string]any `json:"users"`
			Media []map[string]any `json:"media"`
		} `json:"includes"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decoding post response: %w", err)
	}
	userByID := map[string]*postAuthorSummary{}
	for _, u := range envelope.Includes.Users {
		user := userSummaryFromMap(u)
		if user.ID != "" {
			userByID[user.ID] = user
		}
	}
	rec, err := normalizeTweetRecord(input, envelope.Data, userByID, source, localState, include)
	if err != nil {
		return nil, err
	}
	if include["media"] {
		rec.Media = envelope.Includes.Media
	}
	return rec, nil
}

func normalizeTweetRecord(input string, raw json.RawMessage, users map[string]*postAuthorSummary, source, localState string, include map[string]bool) (*resolvedPostRecord, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decoding post object: %w", err)
	}
	id := stringField(obj, "id")
	if id == "" {
		return nil, notFoundErr(fmt.Errorf("post %q was not present in the response", input))
	}
	author := &postAuthorSummary{ID: stringField(obj, "author_id"), Username: stringField(obj, "username")}
	if users != nil && author.ID != "" && users[author.ID] != nil {
		author = users[author.ID]
	}
	if author.ID == "" && author.Username == "" {
		author = nil
	}
	rec := &resolvedPostRecord{
		Input:                 input,
		TweetID:               id,
		Author:                author,
		CreatedAt:             stringField(obj, "created_at"),
		Text:                  stringField(obj, "text"),
		ConversationID:        stringField(obj, "conversation_id"),
		PostType:              postTypeFromObject(obj),
		Source:                source,
		LocalState:            localState,
		NextSuggestedCommands: []string{"thread context " + id, "collection save " + id + " --collection <name>"},
	}
	rec.URL = canonicalPostURL(id, author)
	if include["refs"] {
		rec.ReferencedTweets = tweetRefsFromObject(obj)
	}
	if include["metrics"] {
		rec.PublicMetrics = mapField(obj, "public_metrics")
		rec.NonPublicMetrics = mapField(obj, "non_public_metrics")
		rec.OrganicMetrics = mapField(obj, "organic_metrics")
	}
	if include["links"] {
		rec.Entities = mapField(obj, "entities")
	}
	if include["raw"] {
		rec.Raw = obj
	}
	return rec, nil
}

func normalizeTweetRecordWithOwnID(raw json.RawMessage, users map[string]*postAuthorSummary, source, localState string, include map[string]bool) (*resolvedPostRecord, error) {
	var obj map[string]any
	_ = json.Unmarshal(raw, &obj)
	return normalizeTweetRecord(stringField(obj, "id"), raw, users, source, localState, include)
}

func cacheResolvedPost(cmd *cobra.Command, dbPath string, data json.RawMessage) error {
	if dbPath == "" {
		dbPath = defaultDBPath("x-twitter-pp-cli")
	}
	var envelope struct {
		Data     json.RawMessage `json:"data"`
		Includes struct {
			Users []json.RawMessage `json:"users"`
		} `json:"includes"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || len(envelope.Data) == 0 {
		return nil
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.UpsertTweets(envelope.Data); err != nil {
		return err
	}
	for _, user := range envelope.Includes.Users {
		_ = db.UpsertUsers(user)
	}
	return nil
}

func userSummaryFromMap(obj map[string]any) *postAuthorSummary {
	return &postAuthorSummary{
		ID:       stringField(obj, "id"),
		Username: strings.TrimPrefix(stringField(obj, "username"), "@"),
		Name:     stringField(obj, "name"),
	}
}

func tweetRefsFromObject(obj map[string]any) []tweetReference {
	raw, ok := obj["referenced_tweets"].([]any)
	if !ok {
		return nil
	}
	refs := make([]tweetReference, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ref := tweetReference{Type: stringField(m, "type"), ID: stringField(m, "id")}
		if ref.ID != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}

func postTypeFromObject(obj map[string]any) string {
	refs := tweetRefsFromObject(obj)
	if len(refs) == 0 {
		return "original"
	}
	types := make([]string, 0, len(refs))
	for _, ref := range refs {
		switch ref.Type {
		case "replied_to":
			types = append(types, "reply")
		case "quoted":
			types = append(types, "quote")
		case "retweeted":
			types = append(types, "repost")
		default:
			types = append(types, ref.Type)
		}
	}
	sort.Strings(types)
	return strings.Join(types, ",")
}

func stringField(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	switch v := obj[key].(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case float64:
		return fmt.Sprintf("%.0f", v)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func mapField(obj map[string]any, key string) map[string]any {
	if obj == nil {
		return nil
	}
	if v, ok := obj[key].(map[string]any); ok {
		return v
	}
	return nil
}

func authorDisplay(author *postAuthorSummary) string {
	if author == nil {
		return ""
	}
	if author.Username != "" {
		return "@" + author.Username
	}
	return author.ID
}

func truncatePlain(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func generatedAt() string {
	return time.Now().UTC().Format(time.RFC3339)
}
