// Copyright 2026 Chris Hatton and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written support code for the novel feature commands.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/sigma-computing/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/sigma-computing/internal/store"
	"github.com/spf13/cobra"
)

// wantJSON reports whether a read-only command should emit JSON rather than a
// human table. JSON when --json is set, or when stdout is not a TTY and the
// user hasn't asked for human-friendly output (agent-safe default).
func wantJSON(flags *rootFlags, cmd *cobra.Command) bool {
	if flags != nil && flags.asJSON {
		return true
	}
	return !isTerminal(cmd.OutOrStdout()) && !humanFriendly
}

// openStore opens the local synced SQLite store read/write (SELECT-only usage).
func openStore(cmd *cobra.Command) (*store.Store, error) {
	db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("sigma-computing-pp-cli"))
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'sigma-computing-pp-cli sync' first.", err)
	}
	return db, nil
}

// memberRef is a resolved member identity from the local store.
type memberRef struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// resolveMemberByEmail looks up a member id by email (case-insensitive) from the
// local members table. Returns ("", false) when not found.
func resolveMemberByEmail(db *sql.DB, email string) (memberRef, bool, error) {
	row := db.QueryRow(
		`SELECT COALESCE(NULLIF(member_id,''), id), email FROM members WHERE LOWER(email) = LOWER(?) LIMIT 1`,
		strings.TrimSpace(email),
	)
	var ref memberRef
	switch err := row.Scan(&ref.ID, &ref.Email); err {
	case nil:
		return ref, true, nil
	case sql.ErrNoRows:
		return memberRef{}, false, nil
	default:
		return memberRef{}, false, err
	}
}

// memberEmailByID resolves a memberId (matching either member_id or id) back to
// an email. Returns "" when unknown.
func memberEmailByID(db *sql.DB, memberID string) string {
	if memberID == "" {
		return ""
	}
	row := db.QueryRow(
		`SELECT email FROM members WHERE member_id = ? OR id = ? LIMIT 1`,
		memberID, memberID,
	)
	var email string
	if err := row.Scan(&email); err != nil {
		return ""
	}
	return email
}

// teamNameByID resolves a teamId (matching team_id or id) to a team name.
func teamNameByID(db *sql.DB, teamID string) string {
	if teamID == "" {
		return ""
	}
	row := db.QueryRow(
		`SELECT name FROM teams WHERE team_id = ? OR id = ? LIMIT 1`,
		teamID, teamID,
	)
	var name string
	if err := row.Scan(&name); err != nil {
		return ""
	}
	return name
}

// teamMember is one member of a team resolved from the local teams_members
// join table (the related member object lives in the `data` JSON blob).
type teamMember struct {
	ID    string
	Email string
}

// teamMembersFromStore returns the members of a team using the synced
// teams_members join table (parent_id == teamId). The member object is stored
// in the row's `data` JSON. Returns an empty slice (not error) when no rows.
func teamMembersFromStore(db *sql.DB, teamID string) ([]teamMember, error) {
	rows, err := db.Query(
		`SELECT data FROM teams_members WHERE parent_id = ? OR teams_id = ?`,
		teamID, teamID,
	)
	if err != nil {
		return nil, err
	}
	// Collect all rows BEFORE resolving emails. memberEmailByID issues another
	// query on the same *sql.DB; calling it inside this rows.Next() loop while
	// the result set is still open deadlocks SQLite's single-connection pool.
	var out []teamMember
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			rows.Close()
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		tm := teamMember{
			ID:    firstString(obj, "memberId", "id"),
			Email: firstString(obj, "email"),
		}
		if tm.ID != "" || tm.Email != "" {
			out = append(out, tm)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	// Now the result set is closed; safe to issue follow-up queries.
	for i := range out {
		if out[i].Email == "" && out[i].ID != "" {
			out[i].Email = memberEmailByID(db, out[i].ID)
		}
	}
	return out, nil
}

// memberTeamIDs returns the set of team ids a member belongs to, using the
// synced members_teams join table (parent_id == memberId; the related team
// object is in the `data` blob).
func memberTeamIDs(db *sql.DB, memberID string) ([]string, error) {
	rows, err := db.Query(
		`SELECT data FROM members_teams WHERE parent_id = ? OR members_id = ?`,
		memberID, memberID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	seen := map[string]struct{}{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		id := firstString(obj, "teamId", "id")
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, rows.Err()
}

// resolveRecipientID resolves a member reference (email or raw memberId) to a
// memberId. A value without "@" is treated as an already-resolved memberId. An
// email is resolved against the local store first; on a store miss it falls
// back to GET /v2/members and matches by email. This function may perform HTTP
// on the API-fallback path, so callers must short-circuit dry-run/verify first.
func resolveRecipientID(cmd *cobra.Command, flags *rootFlags, c *client.Client, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("empty member reference")
	}
	if !strings.Contains(ref, "@") {
		return ref, nil // already a memberId
	}
	// Try the local store first (offline-friendly).
	if db, err := openStore(cmd); err == nil {
		defer db.Close()
		if m, found, qerr := resolveMemberByEmail(db.DB(), ref); qerr == nil && found {
			return m.ID, nil
		}
	}
	// API fallback: list members, match by email.
	id, err := lookupMemberIDViaAPI(cmd, c, ref)
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("no member found for email %q (checked local store and GET /v2/members)", ref)
	}
	return id, nil
}

// decodeEntries extracts the row objects from a Sigma list response, which is
// either a bare JSON array or an object with an "entries" array (the spec
// declares several list endpoints as oneOf[array, {entries,nextPage}]).
func decodeEntries(raw json.RawMessage) ([]map[string]any, string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "[") {
		var arr []map[string]any
		if err := json.Unmarshal(raw, &arr); err != nil {
			return nil, "", err
		}
		return arr, "", nil
	}
	var page struct {
		Entries  []map[string]any `json:"entries"`
		NextPage string           `json:"nextPage"`
	}
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, "", err
	}
	return page.Entries, page.NextPage, nil
}

// getAllEntries pages a Sigma list endpoint to exhaustion, following the
// nextPage cursor (sent back as the ?page= query param). It caps total pages to
// avoid an unbounded loop if the cursor never clears. baseParams may carry
// server-side filters (e.g. email, limit).
func getAllEntries(cmd *cobra.Command, c *client.Client, path string, baseParams map[string]string) ([]map[string]any, error) {
	var all []map[string]any
	params := map[string]string{}
	for k, v := range baseParams {
		params[k] = v
	}
	const maxPages = 1000
	for page := 0; page < maxPages; page++ {
		raw, err := c.Get(cmd.Context(), path, params)
		if err != nil {
			return nil, err
		}
		entries, next, derr := decodeEntries(raw)
		if derr != nil {
			return nil, derr
		}
		all = append(all, entries...)
		if next == "" {
			break
		}
		params["page"] = next
	}
	return all, nil
}

// lookupMemberIDViaAPI resolves a member's email to its memberId via the API.
// It uses the server-side ?email= filter when available and falls back to a
// paginated scan, so a member on any page is found (not just page one).
func lookupMemberIDViaAPI(cmd *cobra.Command, c *client.Client, email string) (string, error) {
	want := strings.ToLower(strings.TrimSpace(email))
	// Fast path: the members endpoint supports an exact email filter.
	entries, err := getAllEntries(cmd, c, "/v2/members", map[string]string{"email": email, "includeInactive": "true"})
	if err != nil {
		return "", fmt.Errorf("listing members: %w", err)
	}
	for _, e := range entries {
		if strings.ToLower(firstString(e, "email")) == want {
			return firstString(e, "memberId", "id"), nil
		}
	}
	// Defensive fallback: some tenants ignore the filter — full paginated scan.
	all, err := getAllEntries(cmd, c, "/v2/members", map[string]string{"includeInactive": "true"})
	if err != nil {
		return "", fmt.Errorf("listing members: %w", err)
	}
	for _, e := range all {
		if strings.ToLower(firstString(e, "email")) == want {
			return firstString(e, "memberId", "id"), nil
		}
	}
	return "", nil
}

// firstString returns the first non-empty string value among the given keys of
// a decoded JSON object.
func firstString(obj map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := obj[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}
