// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature support for sybill-pp-cli. Shared helpers for the
// local-store cross-entity commands (deals dark, digest, crm-autofill,
// account rollup, activity, patterns).

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sybill/internal/store"
)

// novelStoreLimit is the per-resource ceiling for store reads in novel
// commands. Large enough to cover a real workspace; bounded so a pathological
// store can't exhaust memory.
const novelStoreLimit = 100000

// loadRecords reads every synced record of a resource type from the local
// store as decoded JSON objects. Returns an empty slice (not an error) when
// nothing has been synced yet, so callers can emit an honest "run sync" hint.
func loadRecords(db *store.Store, resourceType string) ([]map[string]any, error) {
	raws, err := db.List(resourceType, novelStoreLimit)
	if err != nil {
		return nil, fmt.Errorf("reading %s from local store: %w", resourceType, err)
	}
	out := make([]map[string]any, 0, len(raws))
	for _, r := range raws {
		var obj map[string]any
		if err := json.Unmarshal(r, &obj); err != nil {
			continue // skip a corrupt blob rather than fail the whole command
		}
		out = append(out, obj)
	}
	return out, nil
}

// novelMachineOutput reports whether the command should emit a machine format
// (JSON/CSV/etc.) rather than a human table. Mirrors the convention used by the
// generated promoted commands: any explicit format flag or a non-terminal
// stdout selects machine output.
func novelMachineOutput(w io.Writer, flags *rootFlags) bool {
	if flags.asJSON || flags.csv || flags.compact || flags.quiet || flags.plain || flags.selectFields != "" {
		return true
	}
	return !isTerminal(w)
}

// str returns the string value at key, tolerating nil and non-string scalars.
func str(obj map[string]any, key string) string {
	if obj == nil {
		return ""
	}
	v, ok := obj[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// Render integers without a trailing .0.
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return ""
	}
}

// firstStr returns the first non-empty value across key variants.
func firstStr(obj map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := str(obj, k); s != "" {
			return s
		}
	}
	return ""
}

// boolField reads a boolean field, tolerating "true"/"false" strings.
func boolField(obj map[string]any, key string) bool {
	if obj == nil {
		return false
	}
	switch t := obj[key].(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true")
	default:
		return false
	}
}

// floatField reads a numeric field, tolerating numeric strings.
func floatField(obj map[string]any, key string) (float64, bool) {
	if obj == nil {
		return 0, false
	}
	switch t := obj[key].(type) {
	case float64:
		return t, true
	case string:
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// nestedObj returns the object value at key, or nil.
func nestedObj(obj map[string]any, key string) map[string]any {
	if obj == nil {
		return nil
	}
	if m, ok := obj[key].(map[string]any); ok {
		return m
	}
	return nil
}

// timeLayouts are the date/time shapes Sybill returns (RFC3339 with and
// without fractional seconds, plus date-only).
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

// parseTime parses a Sybill timestamp string. The second return is false when
// the value is empty or unparseable.
func parseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// parseSince converts a window like "7d", "48h", "30m", or an absolute
// RFC3339 timestamp into an absolute cutoff relative to now.
func parseSince(since string, now time.Time) (time.Time, error) {
	since = strings.TrimSpace(since)
	if since == "" {
		return time.Time{}, fmt.Errorf("empty --since value")
	}
	// Bare day suffix: Go's ParseDuration doesn't understand "d".
	if strings.HasSuffix(since, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(since, "d"))
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid --since %q: expected a form like 7d, 48h, or an RFC3339 timestamp", since)
		}
		return now.Add(-time.Duration(n) * 24 * time.Hour), nil
	}
	if d, err := time.ParseDuration(since); err == nil {
		return now.Add(-d), nil
	}
	if t, ok := parseTime(since); ok {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid --since %q: expected a form like 7d, 48h, or an RFC3339 timestamp", since)
}

// ---- conversation <-> deal / account linkage ----

// convCRM extracts the (id, name, type) linkage from a conversation's crm
// field. type is lowercased. Missing fields come back empty.
func convCRM(conv map[string]any) (id, name, ctype string) {
	crm := nestedObj(conv, "crm")
	if crm == nil {
		return "", "", ""
	}
	return str(crm, "id"), str(crm, "name"), strings.ToLower(str(crm, "type"))
}

// dealName / dealID / etc. read the canonical camelCase API fields, with a
// couple of snake_case fallbacks in case a record was sourced differently.
func dealID(d map[string]any) string   { return firstStr(d, "dealId", "deal_id", "id") }
func dealName(d map[string]any) string { return firstStr(d, "name") }
func dealAccount(d map[string]any) string {
	return firstStr(d, "accountName", "account_name")
}
func dealStage(d map[string]any) string { return firstStr(d, "stage") }
func dealClosed(d map[string]any) bool  { return boolField(d, "closed") }
func dealOwner(d map[string]any) string {
	if o := nestedObj(d, "owner"); o != nil {
		if n := firstStr(o, "name", "email"); n != "" {
			return n
		}
	}
	return firstStr(d, "owner")
}

func convID(c map[string]any) string {
	return firstStr(c, "conversationId", "conversation_id", "id")
}
func convTitle(c map[string]any) string { return firstStr(c, "title") }
func convType(c map[string]any) string  { return strings.ToUpper(firstStr(c, "type")) }
func convStart(c map[string]any) (time.Time, bool) {
	return parseTime(firstStr(c, "startTime", "start_time"))
}

// convMatchesDeal reports whether a conversation is linked to a deal. The link
// is the conversation's crm field pointing at the opportunity by id or name.
func convMatchesDeal(conv, deal map[string]any) bool {
	id, name, ctype := convCRM(conv)
	if ctype != "opportunity" {
		return false
	}
	if id != "" && id == dealID(deal) {
		return true
	}
	if name != "" && dealName(deal) != "" && strings.EqualFold(name, dealName(deal)) {
		return true
	}
	return false
}

// convMatchesAccount reports whether a conversation is linked to an account by
// name, either directly (crm.type=account) or through an opportunity whose
// deal belongs to the account.
func convMatchesAccount(conv map[string]any, accountName string, dealsForAccount []map[string]any) bool {
	id, name, ctype := convCRM(conv)
	if accountName == "" {
		return false
	}
	if ctype == "account" && name != "" && strings.EqualFold(name, accountName) {
		return true
	}
	if ctype == "opportunity" {
		for _, d := range dealsForAccount {
			if (id != "" && id == dealID(d)) || (name != "" && strings.EqualFold(name, dealName(d))) {
				return true
			}
		}
	}
	return false
}

// lastCallForDeal returns the most recent conversation start time linked to a
// deal, and the matching conversation title. ok is false when no linked call
// exists in the store.
func lastCallForDeal(deal map[string]any, convs []map[string]any) (latest time.Time, title string, ok bool) {
	for _, c := range convs {
		if !convMatchesDeal(c, deal) {
			continue
		}
		t, has := convStart(c)
		if !has {
			continue
		}
		if !ok || t.After(latest) {
			latest, title, ok = t, convTitle(c), true
		}
	}
	return latest, title, ok
}

// daysAgo renders a whole-day count between then and now for human output.
func daysAgo(then, now time.Time) int {
	d := now.Sub(then)
	if d < 0 {
		return 0
	}
	return int(d.Hours() / 24)
}
