// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Shared local-mirror query layer for the archive-oriented commands
// (recall, catchup, archive coverage, threads stale, health, users
// activity, users whois).
//
// Resource-type literals below are taken from the syncer, not guessed:
// sync.go's defaultSyncResources()/syncResourcePath() store rows under the
// hyphenated resource name ("conversations-history", "conversations-replies",
// "users-list", …), while the write-through cache in data_source.go stores
// live reads under the coarser type each generated command passes to
// resolveRead ("conversations" for both conversations.history and
// conversations.replies, "users" for users.info/users.list). Underscored
// variants are accepted too so a mirror produced by a differently-configured
// syncer still resolves. Because "conversations" holds BOTH channel objects
// and message objects, every query below also discriminates on payload shape
// (a message has $.ts and $.text; a channel does not).

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/slackanalytics"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/store"
)

// localMessageResourceTypes are every resource_type under which a Slack
// message can land in the local mirror.
var localMessageResourceTypes = []string{
	"messages", "message",
	"conversations_history", "conversations-history",
	"conversations_replies", "conversations-replies",
	"conversations",
}

// localChannelResourceTypes are every resource_type under which a Slack
// conversation (channel/group/IM) can land.
var localChannelResourceTypes = []string{
	"channels", "conversations",
	"conversations_list", "conversations-list",
	"conversations_info", "conversations-info",
}

// localUserResourceTypes are every resource_type under which a Slack user
// record can land.
var localUserResourceTypes = []string{
	"users", "members",
	"users_list", "users-list",
	"users_info", "users-info",
	"users_lookup_by_email", "users-lookup-by-email",
}

// localIdentityResourceTypes hold the authenticated identity: users.identity
// first, then the auth.test envelope.
var localIdentityResourceTypes = []string{
	"users_identity", "users-identity", "auth_api", "auth-test", "auth_test",
}

// localUsergroupResourceTypes hold usergroup definitions and memberships.
var localUsergroupResourceTypes = []string{
	"usergroups", "usergroups_list", "usergroups-list",
	"usergroups_users_list", "usergroups-users-list",
	"usergroups_usergroups_users_list", "usergroups-usergroups-users-list",
}

// localDNDResourceTypes hold do-not-disturb state.
var localDNDResourceTypes = []string{
	"dnd", "dnd_info", "dnd-info", "dnd_team_info", "dnd-team-info",
}

// localMessage is one mirrored Slack message, already decoded and
// timestamp-parsed so callers never re-parse.
type localMessage struct {
	ResourceType string
	StoreID      string
	Channel      string
	User         string
	UserName     string
	Text         string
	TS           string
	ThreadTS     string
	Subtype      string
	ReplyCount   int
	Reactions    int
	At           time.Time
	HasTime      bool
}

// IsThreadReply reports whether the message is a reply rather than the
// thread parent. Slack stamps both with thread_ts; only the parent's ts
// equals it.
func (m localMessage) IsThreadReply() bool {
	return m.ThreadTS != "" && m.ThreadTS != m.TS
}

// localChannel is one mirrored conversation.
type localChannel struct {
	ID         string
	Name       string
	IsArchived bool
	IsPrivate  bool
	NumMembers int
}

// Label renders "#name" when a name is mirrored, else the bare ID.
func (c localChannel) Label() string {
	if strings.TrimSpace(c.Name) == "" {
		return c.ID
	}
	return "#" + c.Name
}

// localUser is one mirrored user record.
type localUser struct {
	ID          string
	Handle      string
	DisplayName string
	RealName    string
	Email       string
	TZ          string
	TZLabel     string
	TZOffset    int
	IsBot       bool
	Deleted     bool
}

// Identity projects the record into the matcher used by users activity and
// users whois.
func (u localUser) Identity() slackanalytics.UserIdentity {
	return slackanalytics.UserIdentity{
		ID:          u.ID,
		Handle:      u.Handle,
		DisplayName: u.DisplayName,
		RealName:    u.RealName,
		Email:       u.Email,
	}
}

// localDND is mirrored do-not-disturb state for one user.
type localDND struct {
	UserID          string `json:"user_id"`
	DNDEnabled      bool   `json:"dnd_enabled"`
	SnoozeEnabled   bool   `json:"snooze_enabled"`
	NextDNDStartTS  int64  `json:"next_dnd_start_ts,omitempty"`
	NextDNDEndTS    int64  `json:"next_dnd_end_ts,omitempty"`
	SnoozeRemaining int64  `json:"snooze_remaining,omitempty"`
}

// inClause renders a parameterised IN list plus its arguments.
func inClause(values []string) (string, []any) {
	placeholders := make([]string, 0, len(values))
	args := make([]any, 0, len(values))
	for _, v := range values {
		placeholders = append(placeholders, "?")
		args = append(args, v)
	}
	return strings.Join(placeholders, ", "), args
}

// nullString returns the string value or "" for SQL NULL. Every column that
// comes out of json_extract can be NULL — a bare string scan would error and
// silently drop the row — so every optional column is scanned through one of
// these helpers.
func nullString(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return strings.TrimSpace(v.String)
}

func nullInt(v sql.NullInt64) int {
	if !v.Valid {
		return 0
	}
	return int(v.Int64)
}

func nullBool(v sql.NullInt64) bool {
	return v.Valid && v.Int64 != 0
}

// loadLocalMessages reads every mirrored message in one pass. The caller
// filters in Go: SQLite here is one connection, so a second query issued
// while these rows are open would deadlock the driver. Rows are therefore
// drained fully into structs, checked, and closed before anything else runs.
func loadLocalMessages(ctx context.Context, db *store.Store) ([]localMessage, error) {
	types, args := inClause(localMessageResourceTypes)
	query := `SELECT
		r.resource_type,
		r.id,
		COALESCE(json_extract(r.data, '$.channel.id'), json_extract(r.data, '$.channel'), json_extract(r.data, '$.channel_id')) AS channel,
		COALESCE(json_extract(r.data, '$.user'), json_extract(r.data, '$.user_id'), json_extract(r.data, '$.bot_id')) AS user_id,
		COALESCE(json_extract(r.data, '$.username'), json_extract(r.data, '$.user_profile.display_name')) AS user_name,
		json_extract(r.data, '$.text') AS text,
		json_extract(r.data, '$.ts') AS ts,
		json_extract(r.data, '$.thread_ts') AS thread_ts,
		json_extract(r.data, '$.subtype') AS subtype,
		json_extract(r.data, '$.reply_count') AS reply_count,
		json_extract(r.data, '$.reactions') AS reactions
	FROM resources r
	WHERE r.resource_type IN (` + types + `)
	  AND json_extract(r.data, '$.ts') IS NOT NULL
	  AND json_extract(r.data, '$.text') IS NOT NULL`

	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("reading local messages: %w", err)
	}
	messages := make([]localMessage, 0, 64)
	for rows.Next() {
		var (
			resourceType, storeID               string
			channel, userID, userName, text, ts sql.NullString
			threadTS, subtype, reactions        sql.NullString
			replyCount                          sql.NullInt64
		)
		if scanErr := rows.Scan(&resourceType, &storeID, &channel, &userID, &userName, &text, &ts, &threadTS, &subtype, &replyCount, &reactions); scanErr != nil {
			_ = rows.Close() // error path: the returned error is more specific
			return nil, fmt.Errorf("scanning local message: %w", scanErr)
		}
		msg := localMessage{
			ResourceType: resourceType,
			StoreID:      storeID,
			Channel:      nullString(channel),
			User:         nullString(userID),
			UserName:     nullString(userName),
			Text:         nullString(text),
			TS:           nullString(ts),
			ThreadTS:     nullString(threadTS),
			Subtype:      nullString(subtype),
			ReplyCount:   nullInt(replyCount),
			Reactions:    countReactions(nullString(reactions)),
		}
		if msg.TS == "" {
			continue
		}
		if at, ok := slackanalytics.ParseSlackTS(msg.TS); ok {
			msg.At = at
			msg.HasTime = true
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close() // error path: the returned error is more specific
		return nil, fmt.Errorf("reading local messages: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing local message rows: %w", err)
	}
	sort.SliceStable(messages, func(i, j int) bool { return messages[i].TS < messages[j].TS })
	return messages, nil
}

// countReactions sums the count field of a Slack reactions array. A NULL or
// unparseable array counts as zero reactions rather than failing the row.
func countReactions(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	var reactions []struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(raw), &reactions); err != nil {
		return 0
	}
	total := 0
	for _, r := range reactions {
		total += r.Count
	}
	return total
}

// loadLocalChannels reads mirrored conversations, keyed by channel ID.
func loadLocalChannels(ctx context.Context, db *store.Store) (map[string]localChannel, error) {
	types, args := inClause(localChannelResourceTypes)
	query := `SELECT
		COALESCE(json_extract(r.data, '$.id'), json_extract(r.data, '$.channel.id'), r.id) AS channel_id,
		COALESCE(json_extract(r.data, '$.name'), json_extract(r.data, '$.name_normalized'), json_extract(r.data, '$.channel.name')) AS name,
		json_extract(r.data, '$.is_archived') AS is_archived,
		json_extract(r.data, '$.is_private') AS is_private,
		json_extract(r.data, '$.num_members') AS num_members
	FROM resources r
	WHERE r.resource_type IN (` + types + `)
	  AND json_extract(r.data, '$.ts') IS NULL
	  AND (json_extract(r.data, '$.name') IS NOT NULL
	       OR json_extract(r.data, '$.is_channel') IS NOT NULL
	       OR json_extract(r.data, '$.is_im') IS NOT NULL
	       OR json_extract(r.data, '$.channel.id') IS NOT NULL)`

	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("reading local channels: %w", err)
	}
	channels := map[string]localChannel{}
	for rows.Next() {
		var (
			id, name                          sql.NullString
			isArchived, isPrivate, numMembers sql.NullInt64
		)
		if scanErr := rows.Scan(&id, &name, &isArchived, &isPrivate, &numMembers); scanErr != nil {
			_ = rows.Close() // error path: the returned error is more specific
			return nil, fmt.Errorf("scanning local channel: %w", scanErr)
		}
		channelID := nullString(id)
		if channelID == "" {
			continue
		}
		existing, seen := channels[channelID]
		candidate := localChannel{
			ID:         channelID,
			Name:       nullString(name),
			IsArchived: nullBool(isArchived),
			IsPrivate:  nullBool(isPrivate),
			NumMembers: nullInt(numMembers),
		}
		// A later row with no name must never overwrite a named record.
		if seen && candidate.Name == "" {
			continue
		}
		_ = existing
		channels[channelID] = candidate
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close() // error path: the returned error is more specific
		return nil, fmt.Errorf("reading local channels: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing local channel rows: %w", err)
	}
	return channels, nil
}

// loadLocalUsers reads mirrored users, keyed by user ID. Both the flat
// users.list member shape and the users.info {"user":{…}} envelope resolve.
func loadLocalUsers(ctx context.Context, db *store.Store) (map[string]localUser, error) {
	types, args := inClause(localUserResourceTypes)
	query := `SELECT
		COALESCE(json_extract(r.data, '$.id'), json_extract(r.data, '$.user.id'), r.id) AS user_id,
		COALESCE(json_extract(r.data, '$.name'), json_extract(r.data, '$.user.name')) AS handle,
		COALESCE(json_extract(r.data, '$.profile.display_name'), json_extract(r.data, '$.user.profile.display_name')) AS display_name,
		COALESCE(json_extract(r.data, '$.real_name'), json_extract(r.data, '$.profile.real_name'), json_extract(r.data, '$.user.real_name'), json_extract(r.data, '$.user.profile.real_name')) AS real_name,
		COALESCE(json_extract(r.data, '$.profile.email'), json_extract(r.data, '$.email'), json_extract(r.data, '$.user.profile.email')) AS email,
		COALESCE(json_extract(r.data, '$.tz'), json_extract(r.data, '$.user.tz')) AS tz,
		COALESCE(json_extract(r.data, '$.tz_label'), json_extract(r.data, '$.user.tz_label')) AS tz_label,
		COALESCE(json_extract(r.data, '$.tz_offset'), json_extract(r.data, '$.user.tz_offset')) AS tz_offset,
		COALESCE(json_extract(r.data, '$.is_bot'), json_extract(r.data, '$.user.is_bot')) AS is_bot,
		COALESCE(json_extract(r.data, '$.deleted'), json_extract(r.data, '$.user.deleted')) AS deleted
	FROM resources r
	WHERE r.resource_type IN (` + types + `)
	  AND json_extract(r.data, '$.ts') IS NULL
	  AND (json_extract(r.data, '$.name') IS NOT NULL
	       OR json_extract(r.data, '$.user.name') IS NOT NULL
	       OR json_extract(r.data, '$.profile') IS NOT NULL)`

	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("reading local users: %w", err)
	}
	users := map[string]localUser{}
	for rows.Next() {
		var (
			id, handle, displayName, realName, email sql.NullString
			tz, tzLabel                              sql.NullString
			tzOffset, isBot, deleted                 sql.NullInt64
		)
		if scanErr := rows.Scan(&id, &handle, &displayName, &realName, &email, &tz, &tzLabel, &tzOffset, &isBot, &deleted); scanErr != nil {
			_ = rows.Close() // error path: the returned error is more specific
			return nil, fmt.Errorf("scanning local user: %w", scanErr)
		}
		userID := nullString(id)
		if userID == "" {
			continue
		}
		candidate := localUser{
			ID:          userID,
			Handle:      nullString(handle),
			DisplayName: nullString(displayName),
			RealName:    nullString(realName),
			Email:       nullString(email),
			TZ:          nullString(tz),
			TZLabel:     nullString(tzLabel),
			TZOffset:    nullInt(tzOffset),
			IsBot:       nullBool(isBot),
			Deleted:     nullBool(deleted),
		}
		if existing, seen := users[userID]; seen && candidate.Handle == "" && existing.Handle != "" {
			continue
		}
		users[userID] = candidate
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close() // error path: the returned error is more specific
		return nil, fmt.Errorf("reading local users: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing local user rows: %w", err)
	}
	return users, nil
}

// resolveSelfUserID returns the authenticated user's Slack ID from the local
// mirror, preferring users.identity over the auth.test envelope. An empty
// return is not an error: the mirror simply never captured an identity, and
// callers degrade (no mentions section, every thread counts as unanswered).
func resolveSelfUserID(ctx context.Context, db *store.Store) (string, string, error) {
	types, args := inClause(localIdentityResourceTypes)
	query := `SELECT
		r.resource_type,
		COALESCE(json_extract(r.data, '$.user.id'), json_extract(r.data, '$.user_id'), json_extract(r.data, '$.id')) AS user_id
	FROM resources r
	WHERE r.resource_type IN (` + types + `)`

	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return "", "", fmt.Errorf("reading local identity: %w", err)
	}
	type candidate struct{ resourceType, userID string }
	found := make([]candidate, 0, 2)
	for rows.Next() {
		var resourceType string
		var userID sql.NullString
		if scanErr := rows.Scan(&resourceType, &userID); scanErr != nil {
			_ = rows.Close() // error path: the returned error is more specific
			return "", "", fmt.Errorf("scanning local identity: %w", scanErr)
		}
		if id := nullString(userID); id != "" {
			found = append(found, candidate{resourceType, id})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close() // error path: the returned error is more specific
		return "", "", fmt.Errorf("reading local identity: %w", err)
	}
	if err := rows.Close(); err != nil {
		return "", "", fmt.Errorf("closing local identity rows: %w", err)
	}
	for _, want := range localIdentityResourceTypes {
		for _, c := range found {
			if c.resourceType == want {
				return c.userID, c.resourceType, nil
			}
		}
	}
	return "", "", nil
}

// loadSelfUsergroups returns the usergroup IDs the given user belongs to, so
// a <!subteam^Sxxx> mention of their team counts as a mention of them.
func loadSelfUsergroups(ctx context.Context, db *store.Store, selfID string) ([]string, error) {
	if strings.TrimSpace(selfID) == "" {
		return []string{}, nil
	}
	types, args := inClause(localUsergroupResourceTypes)
	query := `SELECT
		COALESCE(json_extract(r.data, '$.id'), json_extract(r.data, '$.usergroup'), json_extract(r.data, '$.usergroup_id'), r.id) AS group_id,
		COALESCE(json_extract(r.data, '$.users'), json_extract(r.data, '$.prefs.users')) AS users
	FROM resources r
	WHERE r.resource_type IN (` + types + `)`

	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("reading local usergroups: %w", err)
	}
	groups := make([]string, 0, 4)
	seen := map[string]bool{}
	for rows.Next() {
		var groupID, users sql.NullString
		if scanErr := rows.Scan(&groupID, &users); scanErr != nil {
			_ = rows.Close() // error path: the returned error is more specific
			return nil, fmt.Errorf("scanning local usergroup: %w", scanErr)
		}
		id := nullString(groupID)
		if id == "" || seen[id] {
			continue
		}
		var members []string
		if raw := nullString(users); raw != "" {
			if err := json.Unmarshal([]byte(raw), &members); err != nil {
				// usergroups.users.list can return a comma-joined string.
				members = strings.Split(raw, ",")
			}
		}
		for _, m := range members {
			if strings.EqualFold(strings.TrimSpace(m), selfID) {
				seen[id] = true
				groups = append(groups, strings.ToUpper(id))
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close() // error path: the returned error is more specific
		return nil, fmt.Errorf("reading local usergroups: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing local usergroup rows: %w", err)
	}
	sort.Strings(groups)
	return groups, nil
}

// loadLocalDND returns mirrored do-not-disturb state for one user, or nil
// when the mirror carries none. Both the per-user dnd.info shape and the
// dnd.teamInfo {"users":{"U…":{…}}} shape resolve.
func loadLocalDND(ctx context.Context, db *store.Store, userID string) (*localDND, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, nil
	}
	types, args := inClause(localDNDResourceTypes)
	query := `SELECT r.id, r.data FROM resources r WHERE r.resource_type IN (` + types + `)`
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("reading local dnd state: %w", err)
	}
	type rawRow struct {
		id   string
		data []byte
	}
	raws := make([]rawRow, 0, 4)
	for rows.Next() {
		var id sql.NullString
		var data sql.NullString
		if scanErr := rows.Scan(&id, &data); scanErr != nil {
			_ = rows.Close() // error path: the returned error is more specific
			return nil, fmt.Errorf("scanning local dnd state: %w", scanErr)
		}
		raws = append(raws, rawRow{id: nullString(id), data: []byte(nullString(data))})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close() // error path: the returned error is more specific
		return nil, fmt.Errorf("reading local dnd state: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing local dnd rows: %w", err)
	}

	type dndPayload struct {
		DNDEnabled      *bool                 `json:"dnd_enabled"`
		SnoozeEnabled   *bool                 `json:"snooze_enabled"`
		NextDNDStartTS  int64                 `json:"next_dnd_start_ts"`
		NextDNDEndTS    int64                 `json:"next_dnd_end_ts"`
		SnoozeRemaining int64                 `json:"snooze_remaining"`
		Users           map[string]dndPayload `json:"users"`
	}
	toDND := func(id string, p dndPayload) *localDND {
		out := &localDND{
			UserID:          id,
			NextDNDStartTS:  p.NextDNDStartTS,
			NextDNDEndTS:    p.NextDNDEndTS,
			SnoozeRemaining: p.SnoozeRemaining,
		}
		if p.DNDEnabled != nil {
			out.DNDEnabled = *p.DNDEnabled
		}
		if p.SnoozeEnabled != nil {
			out.SnoozeEnabled = *p.SnoozeEnabled
		}
		return out
	}
	for _, raw := range raws {
		if len(raw.data) == 0 {
			continue
		}
		var payload dndPayload
		if err := json.Unmarshal(raw.data, &payload); err != nil {
			continue
		}
		for id, perUser := range payload.Users {
			if strings.EqualFold(id, userID) {
				return toDND(userID, perUser), nil
			}
		}
		if strings.EqualFold(raw.id, userID) && payload.DNDEnabled != nil {
			return toDND(userID, payload), nil
		}
	}
	return nil, nil
}

// channelLabel resolves a channel ID to "#name", falling back to the ID and
// then to a stable placeholder for messages mirrored without channel context.
func channelLabel(channels map[string]localChannel, id string) string {
	if strings.TrimSpace(id) == "" {
		return "(unknown channel)"
	}
	if ch, ok := channels[id]; ok {
		return ch.Label()
	}
	return id
}

// userLabel resolves a user ID to a display name, falling back to the ID.
func userLabel(users map[string]localUser, id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	if u, ok := users[id]; ok {
		return u.Identity().DisplayLabel()
	}
	return id
}

// newTextRenderer wires the mirrored users and channels into the de-markup
// pass applied to every message body on its way to output. Build it once per
// command run and pass it down: Slack's markup is per-message, but the
// lookups behind it are the whole mirror.
//
// Message bodies stay in Slack's wire markup in the store — this renderer
// runs at the presentation boundary only, so mention classification and
// full-text search keep seeing the encoding Slack actually sent.
func newTextRenderer(users map[string]localUser, channels map[string]localChannel) slackanalytics.TextRenderer {
	return slackanalytics.TextRenderer{
		User: func(id string) (string, bool) {
			u, ok := users[id]
			if !ok {
				return "", false
			}
			return u.Identity().DisplayLabel(), true
		},
		Channel: func(id string) (string, bool) {
			ch, ok := channels[id]
			if !ok {
				return "", false
			}
			return ch.Name, true
		},
		// Usergroup names are not in the default sync set; the inline label
		// Slack embeds in <!subteam^S…|@eng> covers the readable case and an
		// unresolvable group degrades to @S…, same as any unknown ID.
	}
}

// warnUnmatchedChannelFilter tells the caller when --channel names a
// conversation the mirror has never seen. Without it a typo ("#genral") and a
// genuinely quiet channel produce the same thing — empty output, exit 0 — and
// the reader concludes there was nothing to find. This mirrors the
// "does not resolve to a mirrored user" error users activity already raises
// for --from, except it stays a warning: an unknown channel is not fatal, and
// a partially synced mirror legitimately holds messages from channels whose
// metadata was never fetched.
//
// The candidate set deliberately spans both the mirrored conversation records
// and the channel IDs actually present in messages, so a valid ID whose
// channel object was never synced does not trip the warning.
func warnUnmatchedChannelFilter(w io.Writer, filter string, channels map[string]localChannel, messages []localMessage) bool {
	want := strings.TrimSpace(filter)
	if want == "" {
		return false
	}
	seen := make(map[string]bool, len(channels)+8)
	for id := range channels {
		seen[id] = true
	}
	for _, m := range messages {
		if m.Channel != "" {
			seen[m.Channel] = true
		}
	}
	for id := range seen {
		if recallChannelMatches(want, id, channels) {
			return false
		}
	}
	fmt.Fprintf(w, "warning: --channel %q matches no channel in the local mirror; results will be empty. Run 'slack-pp-cli archive coverage' to list mirrored channels.\n", want)
	return true
}

// threadKey scopes a thread_ts to its channel; two channels can legitimately
// carry the same parent timestamp.
func threadKey(channel, threadTS string) string {
	return channel + "\x00" + threadTS
}

// localThread is a grouped conversation thread: its parent (when mirrored)
// and every reply, ordered oldest-first.
type localThread struct {
	Channel  string
	ThreadTS string
	Messages []localMessage
}

// Latest returns the newest message in the thread.
func (t localThread) Latest() localMessage {
	return t.Messages[len(t.Messages)-1]
}

// Parent returns the thread-parent message when it is mirrored; the second
// return reports whether it was found.
func (t localThread) Parent() (localMessage, bool) {
	for _, m := range t.Messages {
		if m.TS == t.ThreadTS {
			return m, true
		}
	}
	return localMessage{}, false
}

// Replies returns only the reply messages (everything but the parent).
func (t localThread) Replies() []localMessage {
	replies := make([]localMessage, 0, len(t.Messages))
	for _, m := range t.Messages {
		if m.TS != t.ThreadTS {
			replies = append(replies, m)
		}
	}
	return replies
}

// groupThreads buckets messages by (channel, thread_ts). Only groups with at
// least minSize messages are returned, so a lone parent that never drew a
// reply is not reported as an unanswered thread. Results are ordered by
// latest activity, newest first.
func groupThreads(messages []localMessage, minSize int) []localThread {
	if minSize < 1 {
		minSize = 1
	}
	buckets := map[string]*localThread{}
	order := make([]string, 0, 16)
	for _, m := range messages {
		if m.ThreadTS == "" {
			continue
		}
		key := threadKey(m.Channel, m.ThreadTS)
		if _, ok := buckets[key]; !ok {
			buckets[key] = &localThread{Channel: m.Channel, ThreadTS: m.ThreadTS}
			order = append(order, key)
		}
		buckets[key].Messages = append(buckets[key].Messages, m)
	}
	threads := make([]localThread, 0, len(order))
	for _, key := range order {
		t := buckets[key]
		if len(t.Messages) < minSize {
			continue
		}
		sort.SliceStable(t.Messages, func(i, j int) bool { return t.Messages[i].TS < t.Messages[j].TS })
		threads = append(threads, *t)
	}
	sort.SliceStable(threads, func(i, j int) bool {
		return threads[i].Latest().TS > threads[j].Latest().TS
	})
	return threads
}

// matchLocalUser resolves a typed reference (ID, @handle, display name, real
// name, or email) against the mirrored users. The second return reports
// whether a record matched.
func matchLocalUser(users map[string]localUser, ref slackanalytics.UserRef) (localUser, bool) {
	ids := make([]string, 0, len(users))
	for id := range users {
		ids = append(ids, id)
	}
	sort.Strings(ids) // deterministic winner when two records both match
	for _, id := range ids {
		if users[id].Identity().Matches(ref) {
			return users[id], true
		}
	}
	return localUser{}, false
}

// rfc3339 renders a parsed message time, or "" when the timestamp never
// parsed, so JSON consumers can distinguish "unknown" from the epoch.
func rfc3339(t time.Time, ok bool) string {
	if !ok || t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
