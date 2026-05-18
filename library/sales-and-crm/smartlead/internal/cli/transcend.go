// Copyright 2026 bossriceshark. Licensed under Apache-2.0. See LICENSE.

// Package-local helpers shared by the hand-built transcendence commands
// (health, silent, dupes, sender-health, warmup-gate, drift). These are the
// features that exist only because smartlead-pp-cli keeps a local mirror and
// computes cross-entity views the SmartLead API cannot answer in one call.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/smartlead/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/smartlead/internal/store"
)

// emitEmpty handles a store-backed command whose mirror has nothing to show
// because it has not been synced. Offline analytics commands exit 0 with an
// empty result rather than a non-zero error — an empty array composes cleanly
// for agents piping to jq, and the stderr hint guides humans to run sync.
func emitEmpty[T any](cmd *cobra.Command, flags *rootFlags, what string) error {
	fmt.Fprintf(os.Stderr, "note: no synced %s — run 'smartlead-pp-cli sync' first\n", what)
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), []T{}, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "No synced %s yet. Run 'smartlead-pp-cli sync' first.\n", what)
	return nil
}

// fetchArray GETs a list endpoint and returns its elements, tolerating both a
// bare JSON array and a {"data": [...]} envelope.
func fetchArray(c *client.Client, path string, params map[string]string) ([]json.RawMessage, error) {
	raw, err := c.Get(path, params)
	if err != nil {
		return nil, err
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		return arr, nil
	}
	var wrapped struct {
		Data []json.RawMessage `json:"data"`
	}
	if json.Unmarshal(raw, &wrapped) == nil && wrapped.Data != nil {
		return wrapped.Data, nil
	}
	return nil, fmt.Errorf("unexpected response shape from %s", path)
}

// fetchAllPaged walks a list endpoint with offset/limit pagination and
// returns every element. SmartLead caps the limit query parameter at 100, so
// pageSize must not exceed it.
func fetchAllPaged(c *client.Client, path string, pageSize int) ([]json.RawMessage, error) {
	if pageSize < 1 || pageSize > 100 {
		pageSize = 100
	}
	var all []json.RawMessage
	for offset := 0; offset < 100000; offset += pageSize {
		page, err := fetchArray(c, path, map[string]string{
			"offset": strconv.Itoa(offset),
			"limit":  strconv.Itoa(pageSize),
		})
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < pageSize {
			break
		}
	}
	return all, nil
}

// asInt coerces a JSON-decoded value to an int. SmartLead returns many counts
// as quoted strings ("193"), so string parsing is the common path.
func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	}
	return 0
}

// asBool coerces a JSON-decoded value to a bool.
func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return truthy(t)
	case float64:
		return t != 0
	}
	return false
}

// asString coerces a JSON-decoded value to a trimmed string.
func asString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	}
	return ""
}

// nowUTC is the clock used by every time-windowed transcendence command.
// It is a variable so tests can pin it to a fixed instant.
var nowUTC = func() time.Time { return time.Now().UTC() }

// campaignMeta is the name/status pair loaded from the synced campaigns table.
type campaignMeta struct {
	name   string
	status string
}

// loadCampaignMeta reads campaign id -> name/status from the local mirror.
// When onlyID is non-empty the map is restricted to that campaign.
func loadCampaignMeta(ctx context.Context, conn *sql.DB, onlyID string) (map[string]campaignMeta, error) {
	q := `SELECT id, json_extract(data,'$.name'), json_extract(data,'$.status')
		FROM resources WHERE resource_type = 'campaigns'`
	args := []any{}
	if onlyID != "" {
		q += " AND id = ?"
		args = append(args, onlyID)
	}
	rows, err := conn.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, apiErr(fmt.Errorf("reading campaigns: %w", err))
	}
	defer rows.Close()
	out := map[string]campaignMeta{}
	for rows.Next() {
		var id string
		var name, status sql.NullString
		if rows.Scan(&id, &name, &status) != nil {
			continue
		}
		out[id] = campaignMeta{name: name.String, status: status.String}
	}
	return out, rows.Err()
}

// openMirror opens the local SQLite mirror, defaulting the path the same way
// sync and search do. The caller owns Close.
func openMirror(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("smartlead-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, configErr(fmt.Errorf("opening local store: %w", err))
	}
	return db, nil
}

// domainOf returns the lowercased domain portion of an email address, or ""
// when the input is not a usable address.
func domainOf(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(email[at+1:]))
}

// parseSLTime parses a SmartLead timestamp, tolerating the formats the API
// returns across endpoints (campaign objects, statistics rows, warmup dates).
// Returns the zero time and false when the string is empty or unparseable.
func parseSLTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// warmupInboxRate computes the inbox landing rate from warmup inbox/spam
// counts. Returns 0 when there is no warmup volume yet.
func warmupInboxRate(inbox, spam int) float64 {
	total := inbox + spam
	if total == 0 {
		return 0
	}
	return float64(inbox) / float64(total)
}

// rate divides numerator by denominator, rounded to 4 decimal places, and
// returns 0 when the denominator is 0. Used for every open/reply/bounce ratio.
func rate(num, den int) float64 {
	if den == 0 {
		return 0
	}
	r := float64(num) / float64(den)
	return float64(int(r*10000+0.5)) / 10000
}

// truthy reports whether a json_extract'd value represents a JSON true.
// SQLite returns 1 for JSON booleans, but mock fixtures may surface "true".
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "t", "yes":
		return true
	}
	return false
}
