// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/mail"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/agentmail/internal/store"
	"github.com/spf13/cobra"
)

const workflowScanLimit = 20000

type workflowResource struct {
	ID, Type string
	Data     map[string]any
	SyncedAt time.Time
}

func openWorkflowStore(ctx context.Context, cmd *cobra.Command, dbPath string) (*store.Store, error) {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = defaultDBPath("agentmail")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("open local mirror: %w", err)
	}
	return db, nil
}
func loadWorkflowResources(ctx context.Context, db *store.Store) ([]workflowResource, error) {
	if db == nil {
		return []workflowResource{}, nil
	}
	rows, err := db.DB().QueryContext(ctx, `SELECT id, resource_type, data, synced_at FROM resources ORDER BY updated_at DESC LIMIT ?`, workflowScanLimit)
	if err != nil {
		return nil, err
	}
	resources := make([]workflowResource, 0, 128)
	for rows.Next() {
		var id, typ, raw, synced sql.NullString
		if err := rows.Scan(&id, &typ, &raw, &synced); err != nil {
			rows.Close()
			return nil, err
		}
		data := map[string]any{}
		if raw.Valid && strings.TrimSpace(raw.String) != "" {
			if err := json.Unmarshal([]byte(raw.String), &data); err != nil {
				rows.Close()
				return nil, fmt.Errorf("invalid local resource %s/%s: %w", typ.String, id.String, err)
			}
		}
		if data == nil {
			data = map[string]any{}
		}
		resources = append(resources, workflowResource{ID: id.String, Type: typ.String, Data: data, SyncedAt: parseWorkflowTime(synced.String)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return resources, nil
}
func workflowType(r workflowResource) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(r.Type, "-", "_"), " ", "_"))
}
func workflowFamily(r workflowResource, names ...string) bool {
	typ := workflowType(r)
	for _, name := range names {
		n := strings.ToLower(strings.ReplaceAll(name, "-", "_"))
		if typ == n || strings.HasSuffix(typ, "_"+n) {
			return true
		}
	}
	kind := strings.ToLower(wfString(r.Data, "resource_type", "type", "kind"))
	for _, name := range names {
		n := strings.ToLower(strings.ReplaceAll(name, "-", "_"))
		if kind == n || strings.HasSuffix(kind, "_"+n) {
			return true
		}
	}
	return false
}
func wfKey(s string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(s))
}
func wfAny(m map[string]any, keys ...string) any {
	for _, key := range keys {
		wanted := wfKey(key)
		for actual, value := range m {
			if wfKey(actual) == wanted && value != nil {
				return value
			}
		}
	}
	return nil
}
func wfString(m map[string]any, keys ...string) string {
	v := wfAny(m, keys...)
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	case map[string]any:
		return wfString(x, "email", "address", "id", "name", "value")
	}
	return ""
}
func wfBool(m map[string]any, keys ...string) bool {
	b, _ := strconv.ParseBool(strings.ToLower(wfString(m, keys...)))
	return b
}
func wfID(r workflowResource) string {
	for _, key := range []string{"message_id", "draft_id", "thread_id", "id", "key"} {
		if id := wfString(r.Data, key); id != "" {
			return store.BareResourceID(id)
		}
	}
	return store.BareResourceID(r.ID)
}
func wfList(m map[string]any, keys ...string) []any {
	v := wfAny(m, keys...)
	if v == nil {
		return []any{}
	}
	switch x := v.(type) {
	case []any:
		return x
	case map[string]any:
		return []any{x}
	case string:
		if x != "" {
			return []any{x}
		}
	}
	return []any{}
}
func wfStringList(m map[string]any, keys ...string) []string {
	out := []string{}
	for _, v := range wfList(m, keys...) {
		switch x := v.(type) {
		case string:
			if strings.TrimSpace(x) != "" {
				out = append(out, strings.TrimSpace(x))
			}
		case map[string]any:
			if s := wfString(x, "email", "address", "name", "label", "id"); s != "" {
				out = append(out, s)
			}
		}
	}
	return uniqueStrings(out)
}
func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		k := strings.ToLower(s)
		if !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	return out
}
func parseWorkflowTimeStrict(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05-07:00", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n > 1e12 {
			return time.UnixMilli(n), nil
		}
		return time.Unix(n, 0), nil
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", s)
}
func parseWorkflowTime(s string) time.Time      { t, _ := parseWorkflowTimeStrict(s); return t }
func workflowDirection(m map[string]any) string { return workflowDirectionFor(m, "") }
func workflowDirectionFor(m map[string]any, inboxAddress string) string {
	d := strings.ToLower(wfString(m, "direction", "message_direction"))
	if d == "inbound" || d == "incoming" || d == "received" {
		return "inbound"
	}
	if d == "outbound" || d == "outgoing" || d == "sent" {
		return "outbound"
	}
	if wfBool(m, "is_outbound", "outbound") {
		return "outbound"
	}
	if wfBool(m, "is_inbound", "inbound") {
		return "inbound"
	}
	from := normalizeMailboxAddress(wfString(m, "from", "sender", "author"))
	inboxAddress = normalizeMailboxAddress(inboxAddress)
	if inboxAddress != "" && from != "" && from == inboxAddress {
		return "outbound"
	}
	for _, to := range workflowRecipientStrings(m) {
		if inboxAddress != "" && normalizeMailboxAddress(to) == inboxAddress {
			return "inbound"
		}
	}
	if strings.Contains(strings.ToLower(wfString(m, "status", "state")), "sent") {
		return "outbound"
	}
	return "unknown"
}

func normalizeMailboxAddress(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if parsed, err := mail.ParseAddress(value); err == nil {
		return strings.ToLower(strings.TrimSpace(parsed.Address))
	}
	return value
}
func wfTime(r workflowResource, m map[string]any, keys ...string) time.Time {
	if t := parseWorkflowTime(wfString(m, keys...)); !t.IsZero() {
		return t
	}
	return r.SyncedAt
}
func workflowSince(s string) (time.Time, error) {
	if strings.TrimSpace(s) == "" {
		return time.Time{}, nil
	}
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "d"), 64)
		return time.Now().UTC().Add(-time.Duration(n*24) * time.Hour), err
	}
	if strings.HasSuffix(s, "h") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(s, "h"), 64)
		return time.Now().UTC().Add(-time.Duration(n) * time.Hour), err
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().UTC().Add(-d), nil
	}
	if t := parseWorkflowTime(s); !t.IsZero() {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid duration %q (use 7d, 24h, or an RFC3339 timestamp)", s)
}
func workflowInbox(r workflowResource) string {
	return wfString(r.Data, "inbox_id", "inbox", "mailbox_id")
}
func workflowThread(r workflowResource) string {
	return wfString(r.Data, "thread_id", "thread", "conversation_id")
}
func workflowOutboundCount(messages, resources []workflowResource) int {
	count := 0
	for _, r := range messages {
		if workflowDirectionFor(r.Data, workflowInboxAddress(workflowInbox(r), resources)) == "outbound" {
			count++
		}
	}
	return count
}
func workflowInboxAddress(inboxID string, resources []workflowResource) string {
	if inboxID == "" {
		return ""
	}
	for _, r := range resources {
		if workflowFamily(r, "inboxes") && strings.EqualFold(wfID(r), inboxID) {
			return wfString(r.Data, "email", "address", "inbox_email")
		}
	}
	return ""
}
func workflowMessageTime(r workflowResource) time.Time {
	return wfTime(r, r.Data, "timestamp", "sent_at", "received_at", "created_at", "date", "created", "updated_at")
}
func workflowMessageID(r workflowResource) string {
	if id := wfString(r.Data, "message_id"); id != "" {
		return store.BareResourceID(id)
	}
	return store.BareResourceID(r.ID)
}
func workflowDraftPending(r workflowResource) bool {
	s := strings.ToLower(wfString(r.Data, "status", "state"))
	return s != "sent" && s != "cancelled" && s != "canceled" && s != "delivered"
}
func workflowDraftThread(r workflowResource, resources []workflowResource) string {
	if tid := workflowThread(r); tid != "" {
		return tid
	}
	ref := wfString(r.Data, "in_reply_to", "in_reply_to_message_id", "reference", "references")
	if ref == "" {
		return ""
	}
	for _, candidate := range resources {
		if !workflowFamily(candidate, "messages") {
			continue
		}
		if strings.EqualFold(wfID(candidate), store.BareResourceID(ref)) {
			return workflowThread(candidate)
		}
	}
	return ""
}
func workflowAttachmentBelongs(r workflowResource, draftID string) bool {
	if !workflowFamily(r, "attachment", "attachments") {
		return false
	}
	for _, key := range []string{"draft_id", "draft", "parent_id", "resource_id"} {
		if id := wfString(r.Data, key); id != "" && strings.EqualFold(store.BareResourceID(id), store.BareResourceID(draftID)) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(r.ID), strings.ToLower(draftID+"\x00")) || strings.EqualFold(store.BareResourceID(r.ID), store.BareResourceID(draftID))
}
func workflowRecipientStrings(m map[string]any) []string {
	return uniqueStrings(append(append(wfStringList(m, "to", "recipients", "to_emails"), wfStringList(m, "cc", "cc_emails")...), wfStringList(m, "bcc", "bcc_emails")...))
}
func workflowParticipants(m map[string]any) []string {
	return uniqueStrings(append(append(wfStringList(m, "from", "sender", "author"), workflowRecipientStrings(m)...), wfStringList(m, "reply_to", "reply_to_email")...))
}
func workflowLabels(m map[string]any) []string { return wfStringList(m, "labels", "tags") }
func durationAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d/time.Minute))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
}
func closeWorkflowStore(db *store.Store) {
	if db != nil {
		_ = db.Close()
	}
}
func workflowReason(count int, noun string) string {
	if count == 0 {
		return "no matching " + noun + " found in the local mirror"
	}
	return ""
}
func workflowOutput(w io.Writer, value any, flags *rootFlags, table []map[string]any) error {
	if wantsHumanTable(w, flags) && (flags == nil || !flags.agent) {
		if len(table) > 0 {
			return printAutoTable(w, table)
		}
		if envelope, ok := value.(map[string]any); ok {
			reason, _ := envelope["reason"].(string)
			if reason != "" {
				_, err := fmt.Fprintf(w, "No local results: %s\n", reason)
				return err
			}
		}
		return nil
	}
	if flags != nil && flags.selectFields != "" {
		if envelope, ok := value.(map[string]any); ok {
			if items, ok := envelope["items"]; ok {
				itemField := false
				for _, field := range strings.Split(flags.selectFields, ",") {
					field = strings.TrimSpace(field)
					if field != "" && !strings.Contains(field, ".") && field != "count" && field != "scanned" && field != "scan_limit" && field != "reason" {
						itemField = true
					}
				}
				if itemField {
					switch rows := items.(type) {
					case []map[string]any:
						if len(rows) > 0 {
							return printJSONFiltered(w, items, flags)
						}
					case []any:
						if len(rows) > 0 {
							return printJSONFiltered(w, items, flags)
						}
					}
				}
			}
		}
	}
	return printJSONFiltered(w, value, flags)
}
func sortWorkflowByTime(items []map[string]any, field string) {
	sort.SliceStable(items, func(i, j int) bool { a, _ := items[i][field].(string); b, _ := items[j][field].(string); return a < b })
}
