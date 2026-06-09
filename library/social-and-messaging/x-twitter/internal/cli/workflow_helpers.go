// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/x-twitter/internal/store"
	"github.com/spf13/cobra"
)

type workflowEnvelope struct {
	Meta     map[string]any           `json:"meta"`
	Results  any                      `json:"results,omitempty"`
	Warnings []string                 `json:"warnings,omitempty"`
	Sources  []collectionItemSnapshot `json:"sources,omitempty"`
}

type accountSnapshotProfile struct {
	ID            string         `json:"id"`
	Username      string         `json:"username,omitempty"`
	Name          string         `json:"name,omitempty"`
	Description   string         `json:"description,omitempty"`
	Location      string         `json:"location,omitempty"`
	URL           string         `json:"url,omitempty"`
	ProfileURL    string         `json:"profile_url,omitempty"`
	Verified      any            `json:"verified,omitempty"`
	VerifiedType  string         `json:"verified_type,omitempty"`
	PublicMetrics map[string]any `json:"public_metrics,omitempty"`
	PinnedTweetID string         `json:"pinned_tweet_id,omitempty"`
	Protected     any            `json:"protected,omitempty"`
	Source        string         `json:"source"`
	LocalState    string         `json:"local_state"`
	Raw           map[string]any `json:"raw,omitempty"`
}

type workflowWriter interface {
	Write([]byte) (int, error)
}

func workflowFprintf(w workflowWriter, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format, args...)
	return err
}

func workflowFprintln(w workflowWriter, args ...any) error {
	_, err := fmt.Fprintln(w, args...)
	return err
}

func workflowMeta(command, source string) map[string]any {
	if source == "" {
		source = "local"
	}
	return map[string]any{
		"command":      command,
		"source":       source,
		"generated_at": generatedAt(),
	}
}

func openWorkflowDB(cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("x-twitter-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	return db, nil
}

func recentSearchRecords(cmd *cobra.Command, flags *rootFlags, query string, limit int, since string, sinceID string, include map[string]bool) ([]*resolvedPostRecord, error) {
	if limit <= 0 {
		limit = 25
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
	params := map[string]string{
		"query":        query,
		"max_results":  strconv.Itoa(maxResults),
		"tweet.fields": "author_id,conversation_id,created_at,entities,public_metrics,referenced_tweets",
		"expansions":   "author_id,attachments.media_keys",
		"user.fields":  "id,name,username,verified,public_metrics",
		"media.fields": "media_key,type,url,preview_image_url,width,height,alt_text",
	}
	if sinceID != "" {
		params["since_id"] = sinceID
	} else if start, ok, err := sinceStartTime(since); err != nil {
		return nil, err
	} else if ok {
		params["start_time"] = start.Format(time.RFC3339)
	}
	data, err := c.Get(cmd.Context(), "/2/tweets/search/recent", params)
	if err != nil {
		return nil, err
	}
	return normalizeRecentSearchEnvelope(data, include, limit)
}

func normalizeRecentSearchEnvelope(data json.RawMessage, include map[string]bool, limit int) ([]*resolvedPostRecord, error) {
	var envelope struct {
		Data     []json.RawMessage `json:"data"`
		Includes struct {
			Users []map[string]any `json:"users"`
		} `json:"includes"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decoding search results: %w", err)
	}
	userByID := map[string]*postAuthorSummary{}
	for _, u := range envelope.Includes.Users {
		user := userSummaryFromMap(u)
		if user.ID != "" {
			userByID[user.ID] = user
		}
	}
	if include == nil {
		include = parseIncludeSet("author,media,links,refs,metrics")
	}
	records := make([]*resolvedPostRecord, 0, minInt(limit, len(envelope.Data)))
	for _, raw := range envelope.Data {
		if limit > 0 && len(records) >= limit {
			break
		}
		rec, err := normalizeSearchTweetRecord(raw, userByID, include)
		if err == nil {
			records = append(records, rec)
		}
	}
	return records, nil
}

func sinceStartTime(value string) (time.Time, bool, error) {
	trimmed := strings.TrimSpace(value)
	lowered := strings.ToLower(trimmed)
	if lowered == "" || lowered == "last" || lowered == "all" {
		return time.Time{}, false, nil
	}
	if dur, ok, err := parseFlexibleDuration(lowered); err != nil {
		return time.Time{}, false, err
	} else if ok {
		return time.Now().UTC().Add(-dur), true, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t.UTC(), true, nil
		}
	}
	return time.Time{}, false, usageErr(fmt.Errorf("invalid --since %q: use last, a duration like 24h/7d, RFC3339, or YYYY-MM-DD", trimmed))
}

func parseFlexibleDuration(value string) (time.Duration, bool, error) {
	if value == "" {
		return 0, false, nil
	}
	unit := value[len(value)-1]
	number := value[:len(value)-1]
	switch unit {
	case 'd':
		n, err := strconv.Atoi(number)
		if err != nil {
			return 0, false, usageErr(fmt.Errorf("invalid day duration %q", value))
		}
		return time.Duration(n) * 24 * time.Hour, true, nil
	case 'w':
		n, err := strconv.Atoi(number)
		if err != nil {
			return 0, false, usageErr(fmt.Errorf("invalid week duration %q", value))
		}
		return time.Duration(n) * 7 * 24 * time.Hour, true, nil
	default:
		dur, err := time.ParseDuration(value)
		if err != nil {
			return 0, false, nil
		}
		return dur, true, nil
	}
}

func normalizeURLMentionQuery(input string) (string, string, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", "", usageErr(fmt.Errorf("URL or domain required"))
	}
	parseValue := raw
	if !strings.Contains(parseValue, "://") {
		parseValue = "https://" + parseValue
	}
	u, err := url.Parse(parseValue)
	if err != nil || u.Hostname() == "" {
		return "", "", usageErr(fmt.Errorf("could not parse %q as a URL or domain", input))
	}
	host := strings.ToLower(u.Hostname())
	path := strings.Trim(u.EscapedPath(), "/")
	target := host
	if path != "" {
		target += "/" + path
	}
	return `url:"` + target + `"`, target, nil
}

func normalizeAccountInput(input string) (string, bool) {
	value := strings.TrimSpace(input)
	value = strings.TrimPrefix(value, "https://x.com/")
	value = strings.TrimPrefix(value, "https://twitter.com/")
	value = strings.Trim(value, "/")
	value = strings.TrimPrefix(value, "@")
	if rawPostIDRE.MatchString(value) {
		return value, true
	}
	return value, false
}

func resolveAccountProfile(cmd *cobra.Command, flags *rootFlags, input, dbPath, mode string, includeRaw bool) (*accountSnapshotProfile, error) {
	value, isID := normalizeAccountInput(input)
	if value == "" {
		return nil, usageErr(fmt.Errorf("username or user ID required"))
	}
	if dbPath == "" {
		dbPath = defaultDBPath("x-twitter-pp-cli")
	}
	if mode != "live" {
		profile, err := resolveAccountProfileLocal(cmd, value, isID, dbPath, includeRaw)
		if err == nil && profile != nil {
			return profile, nil
		}
		if mode == "local" {
			return nil, notFoundErr(fmt.Errorf("account %q not found in local store; retry with --data-source live", input))
		}
	}
	profile, err := resolveAccountProfileLive(cmd, flags, value, isID, dbPath, includeRaw)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	return profile, nil
}

func resolveAccountProfileLocal(cmd *cobra.Command, value string, isID bool, dbPath string, includeRaw bool) (*accountSnapshotProfile, error) {
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	defer db.Close()
	var raw string
	query := `SELECT data FROM users WHERE lower(username) = lower(?) LIMIT 1`
	if isID {
		query = `SELECT data FROM users WHERE id = ? LIMIT 1`
	}
	if err := db.DB().QueryRowContext(cmd.Context(), query, value).Scan(&raw); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return normalizeAccountProfile(json.RawMessage(raw), "local", "synced", includeRaw)
}

func resolveAccountProfileLive(cmd *cobra.Command, flags *rootFlags, value string, isID bool, dbPath string, includeRaw bool) (*accountSnapshotProfile, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	path := "/2/users/by/username/" + url.PathEscape(value)
	if isID {
		path = "/2/users/" + url.PathEscape(value)
	}
	data, err := c.Get(cmd.Context(), path, map[string]string{
		"user.fields": "created_at,description,location,pinned_tweet_id,profile_image_url,protected,public_metrics,url,username,verified,verified_type",
	})
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if db, err := store.OpenWithContext(cmd.Context(), dbPath); err == nil {
		_ = db.UpsertUsers(envelope.Data)
		_ = db.Close()
	}
	profile, err := normalizeAccountProfile(envelope.Data, "live", "not_synced", includeRaw)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func normalizeAccountProfile(raw json.RawMessage, source, localState string, includeRaw bool) (*accountSnapshotProfile, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decoding user object: %w", err)
	}
	id := stringField(obj, "id")
	if id == "" {
		return nil, notFoundErr(fmt.Errorf("account was not present in the response"))
	}
	profile := &accountSnapshotProfile{
		ID:            id,
		Username:      strings.TrimPrefix(stringField(obj, "username"), "@"),
		Name:          stringField(obj, "name"),
		Description:   stringField(obj, "description"),
		Location:      stringField(obj, "location"),
		URL:           stringField(obj, "url"),
		Verified:      obj["verified"],
		VerifiedType:  stringField(obj, "verified_type"),
		PublicMetrics: mapField(obj, "public_metrics"),
		PinnedTweetID: stringField(obj, "pinned_tweet_id"),
		Protected:     obj["protected"],
		Source:        source,
		LocalState:    localState,
	}
	if profile.Username != "" {
		profile.ProfileURL = "https://x.com/" + profile.Username
	} else {
		profile.ProfileURL = "https://x.com/i/user/" + id
	}
	if includeRaw {
		profile.Raw = obj
	}
	return profile, nil
}

func localRecentPostsForAccount(cmd *cobra.Command, dbPath, userID string, limit int, include map[string]bool) ([]*resolvedPostRecord, error) {
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT data FROM tweets WHERE author_id = ? ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*resolvedPostRecord
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		rec, err := normalizeTweetRecordWithOwnID(json.RawMessage(raw), nil, "local", "synced", include)
		if err == nil {
			out = append(out, rec)
		}
	}
	return out, rows.Err()
}

func liveRecentPostsForAccount(cmd *cobra.Command, flags *rootFlags, userID string, limit int, include map[string]bool) ([]*resolvedPostRecord, error) {
	if limit <= 0 {
		return nil, nil
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	maxResults := limit
	if maxResults < 5 {
		maxResults = 5
	}
	if maxResults > 100 {
		maxResults = 100
	}
	data, err := c.Get(cmd.Context(), "/2/users/"+url.PathEscape(userID)+"/tweets", map[string]string{
		"max_results":  strconv.Itoa(maxResults),
		"tweet.fields": "author_id,conversation_id,created_at,entities,public_metrics,referenced_tweets",
		"expansions":   "author_id,attachments.media_keys",
		"user.fields":  "id,name,username,verified,public_metrics",
		"media.fields": "media_key,type,url,preview_image_url,width,height,alt_text",
	})
	if err != nil {
		return nil, err
	}
	return normalizeRecentSearchEnvelope(data, include, limit)
}

func collectionItemFromPost(rec *resolvedPostRecord, savedAt string) collectionItemSnapshot {
	if rec == nil {
		return collectionItemSnapshot{}
	}
	return collectionItemSnapshot{
		TweetID: rec.TweetID,
		URL:     rec.URL,
		Author:  rec.Author,
		Text:    rec.Text,
		SavedAt: savedAt,
		Source:  rec.Source,
	}
}
