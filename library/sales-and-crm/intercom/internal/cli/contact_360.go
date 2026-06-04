// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/intercom/internal/store"
	"github.com/spf13/cobra"
)

type contact360 struct {
	Contact             json.RawMessage   `json:"contact"`
	Companies           []json.RawMessage `json:"companies"`
	OpenConversations   []json.RawMessage `json:"open_conversations"`
	RecentConversations []json.RawMessage `json:"recent_conversations"`
	Tickets             []json.RawMessage `json:"tickets"`
	Notes               []json.RawMessage `json:"notes"`
	Tags                []string          `json:"tags"`
}

func newContact360Cmd(flags *rootFlags) *cobra.Command {
	var by string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "360 <key>",
		Short:       "Cross-entity view of a contact: companies, conversations, tickets, notes, tags",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Look up by email (auto-detected)
  intercom-pp-cli contact 360 alice@example.com

  # Force lookup by external_id
  intercom-pp-cli contact 360 ext-12345 --by external_id

  # By contact id, JSON output
  intercom-pp-cli contact 360 65a1b2c3d4e5f6 --by id --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || len(args) == 0 {
				return cmd.Help()
			}
			key := args[0]
			detect := strings.ToLower(strings.TrimSpace(by))
			if detect == "" {
				detect = detectContactKey(key)
			}
			switch detect {
			case "email", "external_id", "id":
			default:
				return usageErr(fmt.Errorf("--by must be 'email', 'external_id', or 'id', got %q", by))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("intercom-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return notFoundErr(fmt.Errorf("no local data: run 'intercom-pp-cli sync' first (%w)", err))
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "contacts")

			contactID, contactData, err := resolveContactKey(db, key, detect)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return notFoundErr(fmt.Errorf("contact %q (by %s) not found in local store", key, detect))
				}
				return err
			}

			view := contact360{
				Contact:             contactData,
				Companies:           []json.RawMessage{},
				OpenConversations:   []json.RawMessage{},
				RecentConversations: []json.RawMessage{},
				Tickets:             []json.RawMessage{},
				Notes:               []json.RawMessage{},
				Tags:                []string{},
			}

			view.Companies = readContactCompanies(db, contactID)
			openC, recentC, tagsFromC := readContactConversations(db, contactID)
			view.OpenConversations = openC
			view.RecentConversations = recentC
			view.Tickets = readContactTickets(db, contactID)
			view.Notes = readContactNotes(db, contactID)
			view.Tags = mergeContactTags(contactData, tagsFromC)

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}

	cmd.Flags().StringVar(&by, "by", "", "Key kind: 'email', 'external_id', or 'id' (auto-detect when empty)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func detectContactKey(s string) string {
	if strings.Contains(s, "@") {
		return "email"
	}
	allDigits := true
	for _, r := range s {
		if r < '0' || r > '9' {
			allDigits = false
			break
		}
	}
	if allDigits && len(s) > 0 {
		return "id"
	}
	// Intercom contact ids are hex strings (e.g. 65a1b2c3d4e5...) so treat
	// a 24-hex-character token as an id; otherwise external_id.
	if len(s) >= 16 && isHexish(s) {
		return "id"
	}
	return "external_id"
}

func isHexish(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// resolveContactKey returns (contactID, contactData, error).
func resolveContactKey(db *store.Store, key, by string) (string, json.RawMessage, error) {
	var (
		id   string
		data string
	)
	switch by {
	case "id":
		err := db.DB().QueryRow(`SELECT id, data FROM contacts WHERE id = ? LIMIT 1`, key).Scan(&id, &data)
		if err != nil {
			return "", nil, err
		}
	case "email":
		err := db.DB().QueryRow(
			`SELECT id, data FROM contacts WHERE LOWER(COALESCE(email, json_extract(data, '$.email'))) = LOWER(?) LIMIT 1`,
			key,
		).Scan(&id, &data)
		if err != nil {
			return "", nil, err
		}
	case "external_id":
		err := db.DB().QueryRow(
			`SELECT id, data FROM contacts WHERE COALESCE(external_id, json_extract(data, '$.external_id')) = ? LIMIT 1`,
			key,
		).Scan(&id, &data)
		if err != nil {
			return "", nil, err
		}
	}
	return id, json.RawMessage(data), nil
}

func readContactCompanies(db *store.Store, contactID string) []json.RawMessage {
	out := []json.RawMessage{}
	// The companies_contacts join table is keyed by companies_id (parent).
	// The contacts_companies child table is keyed by contacts_id; its `data`
	// holds the company payloads attached to that contact. Try the natural
	// fit first.
	rows, err := db.DB().Query(
		`SELECT data FROM contacts_companies WHERE contacts_id = ?`,
		contactID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var raw string
			if err := rows.Scan(&raw); err == nil && strings.TrimSpace(raw) != "" {
				out = appendCompanyItems(out, json.RawMessage(raw))
			}
		}
	}
	return out
}

// appendCompanyItems unwraps the Intercom companies envelope `{ "data": [...] }`
// if present; otherwise appends raw verbatim.
func appendCompanyItems(out []json.RawMessage, raw json.RawMessage) []json.RawMessage {
	var env struct {
		Data []json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &env) == nil && env.Data != nil {
		return append(out, env.Data...)
	}
	return append(out, raw)
}

func readContactConversations(db *store.Store, contactID string) (open, recent []json.RawMessage, tags []string) {
	open = []json.RawMessage{}
	recent = []json.RawMessage{}
	tags = nil
	// PATCH(contact-360-sql-pushdown): the participant contact id lives inside
	// `data.contacts.contacts[].id`. Push the membership test down into SQLite
	// via json_each so the engine eliminates non-matching conversations before
	// we deserialize. Previously the query did `SELECT data FROM conversations`
	// (no WHERE) and the Go loop did the filter, which is O(total conversations)
	// per call — on a 500k-conversation workspace that meant loading and
	// JSON-parsing every blob per `contact 360`.
	rows, err := db.DB().Query(`
		SELECT data, json_extract(data, '$.state') AS state, json_extract(data, '$.updated_at') AS updated_at
		FROM conversations
		WHERE EXISTS (
			SELECT 1
			FROM json_each(json_extract(data, '$.contacts.contacts')) AS c
			WHERE json_extract(c.value, '$.id') = ?
		)
	`, contactID)
	if err != nil {
		return
	}
	defer rows.Close()
	type entry struct {
		raw       json.RawMessage
		updatedAt int64
	}
	var closedRecent []entry
	for rows.Next() {
		var (
			raw       string
			state     sql.NullString
			updatedAt sql.NullInt64
		)
		if err := rows.Scan(&raw, &state, &updatedAt); err != nil {
			continue
		}
		if !conversationHasContact(json.RawMessage(raw), contactID) {
			continue
		}
		tags = collectConversationTags(json.RawMessage(raw), tags)
		if state.Valid && state.String == "closed" {
			ua := int64(0)
			if updatedAt.Valid {
				ua = updatedAt.Int64
			}
			closedRecent = append(closedRecent, entry{json.RawMessage(raw), ua})
		} else {
			open = append(open, json.RawMessage(raw))
		}
	}
	// Sort closed by updated_at desc, take 10.
	for i := 0; i < len(closedRecent); i++ {
		for j := i + 1; j < len(closedRecent); j++ {
			if closedRecent[j].updatedAt > closedRecent[i].updatedAt {
				closedRecent[i], closedRecent[j] = closedRecent[j], closedRecent[i]
			}
		}
	}
	if len(closedRecent) > 10 {
		closedRecent = closedRecent[:10]
	}
	for _, e := range closedRecent {
		recent = append(recent, e.raw)
	}
	return
}

func conversationHasContact(raw json.RawMessage, contactID string) bool {
	var conv struct {
		Contacts struct {
			Contacts []struct {
				ID string `json:"id"`
			} `json:"contacts"`
		} `json:"contacts"`
	}
	if json.Unmarshal(raw, &conv) != nil {
		return false
	}
	for _, c := range conv.Contacts.Contacts {
		if c.ID == contactID {
			return true
		}
	}
	return false
}

func collectConversationTags(raw json.RawMessage, acc []string) []string {
	var conv struct {
		Tags struct {
			Tags []struct {
				Name string `json:"name"`
			} `json:"tags"`
		} `json:"tags"`
	}
	if json.Unmarshal(raw, &conv) != nil {
		return acc
	}
	for _, t := range conv.Tags.Tags {
		if t.Name != "" {
			acc = append(acc, t.Name)
		}
	}
	return acc
}

func readContactTickets(db *store.Store, contactID string) []json.RawMessage {
	out := []json.RawMessage{}
	// PATCH(contact-360-sql-pushdown): push the contact-membership filter into
	// SQLite via json_each on `data.contacts.contacts[]`. Previously this
	// scanned every row of resources WHERE resource_type='tickets' and ran
	// ticketHasContact in Go — O(total tickets) per `contact 360` call.
	rows, err := db.DB().Query(`
		SELECT data
		FROM resources
		WHERE resource_type = 'tickets'
		  AND EXISTS (
			SELECT 1
			FROM json_each(json_extract(data, '$.contacts.contacts')) AS c
			WHERE json_extract(c.value, '$.id') = ?
		  )
	`, contactID)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		// The SQL filter already enforced membership; we keep this trivial
		// loop as the cheap deserialize step. ticketHasContact() is no
		// longer load-bearing for filtering but stays as a defensive
		// double-check for callers that bypass this helper.
		out = append(out, json.RawMessage(raw))
	}
	return out
}

func ticketHasContact(raw json.RawMessage, contactID string) bool {
	var t struct {
		Contacts struct {
			Contacts []struct {
				ID string `json:"id"`
			} `json:"contacts"`
		} `json:"contacts"`
		ContactIDs []string `json:"contact_ids"`
	}
	if json.Unmarshal(raw, &t) != nil {
		return false
	}
	for _, c := range t.Contacts.Contacts {
		if c.ID == contactID {
			return true
		}
	}
	for _, id := range t.ContactIDs {
		if id == contactID {
			return true
		}
	}
	return false
}

func readContactNotes(db *store.Store, contactID string) []json.RawMessage {
	out := []json.RawMessage{}
	rows, err := db.DB().Query(
		`SELECT data FROM contacts_notes WHERE contacts_id = ?`,
		contactID,
	)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err == nil && strings.TrimSpace(raw) != "" {
			out = appendNoteItems(out, json.RawMessage(raw))
		}
	}
	return out
}

func appendNoteItems(out []json.RawMessage, raw json.RawMessage) []json.RawMessage {
	var env struct {
		Data []json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &env) == nil && env.Data != nil {
		return append(out, env.Data...)
	}
	return append(out, raw)
}

// mergeContactTags unions contact.tags.tags[].name + tags collected from
// conversations. De-duplicates and returns a non-nil slice.
func mergeContactTags(contactData json.RawMessage, fromConvs []string) []string {
	set := map[string]struct{}{}
	for _, t := range fromConvs {
		set[t] = struct{}{}
	}
	var c struct {
		Tags struct {
			Data []struct {
				Name string `json:"name"`
			} `json:"data"`
			Tags []struct {
				Name string `json:"name"`
			} `json:"tags"`
		} `json:"tags"`
	}
	if json.Unmarshal(contactData, &c) == nil {
		for _, t := range c.Tags.Data {
			if t.Name != "" {
				set[t.Name] = struct{}{}
			}
		}
		for _, t := range c.Tags.Tags {
			if t.Name != "" {
				set[t.Name] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	// Stable order so JSON is reproducible.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
