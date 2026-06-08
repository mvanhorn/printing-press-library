// Copyright 2026 "Chris Carson" and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/monitoring/ninjaone/internal/client"
	"github.com/mvanhorn/printing-press-library/library/monitoring/ninjaone/internal/cliutil"
	"github.com/spf13/cobra"
)

// emitDryRunPreview writes a dry-run/preview short-circuit message. When --json
// (or --agent) is set it emits a JSON object so the output stays machine-parseable
// (json_fidelity); otherwise it prints the plain-text line for humans.
func emitDryRunPreview(cmd *cobra.Command, flags *rootFlags, msg string) error {
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "message": msg}, flags)
	}
	fmt.Fprintln(cmd.OutOrStdout(), msg)
	return nil
}

// pp:data-source live
//
// Shared data shapes and fetch/aggregation helpers for the NinjaOne
// transcendence commands (patch-gaps, patch-sweep, alert-storms, patch-stuck,
// alert-clear, stale-devices, alert-flappers, cf-hygiene). Pure-logic helpers
// here are unit-tested in ninjaone_novel_common_test.go.

// ---- data shapes ----

type njOrg struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type njDevice struct {
	ID             int64       `json:"id"`
	OrganizationID int64       `json:"organizationId"`
	LocationID     int64       `json:"locationId"`
	NodeClass      string      `json:"nodeClass"`
	Offline        bool        `json:"offline"`
	SystemName     string      `json:"systemName"`
	DisplayName    string      `json:"displayName"`
	DNSName        string      `json:"dnsName"`
	LastContact    json.Number `json:"lastContact"`
}

func (d njDevice) lastContactSeconds() float64 {
	f, _ := d.LastContact.Float64()
	return f
}

func (d njDevice) bestName() string {
	if d.SystemName != "" {
		return d.SystemName
	}
	if d.DisplayName != "" {
		return d.DisplayName
	}
	if d.DNSName != "" {
		return d.DNSName
	}
	return strconv.FormatInt(d.ID, 10)
}

type njPatch struct {
	ID          string      `json:"id"`
	DeviceID    int64       `json:"deviceId"`
	Name        string      `json:"name"`
	KBNumber    string      `json:"kbNumber"`
	Severity    string      `json:"severity"`
	Status      string      `json:"status"`
	Type        string      `json:"type"`
	Timestamp   json.Number `json:"timestamp"`
	InstalledAt json.Number `json:"installedAt"`
}

type njAlert struct {
	UID                   string      `json:"uid"`
	DeviceID              int64       `json:"deviceId"`
	Message               string      `json:"message"`
	CreateTime            json.Number `json:"createTime"`
	UpdateTime            json.Number `json:"updateTime"`
	SourceType            string      `json:"sourceType"`
	SourceName            string      `json:"sourceName"`
	Subject               string      `json:"subject"`
	Severity              string      `json:"severity"`
	Priority              string      `json:"priority"`
	ConditionName         string      `json:"conditionName"`
	ConditionHealthStatus string      `json:"conditionHealthStatus"`
}

func (a njAlert) createSeconds() int64 {
	f, _ := a.CreateTime.Float64()
	return int64(f)
}

type njCursor struct {
	Name   string `json:"name"`
	Offset int    `json:"offset"`
	Count  int    `json:"count"`
}

// scoped-custom-fields / custom-fields report row
type njCFRow struct {
	Scope    string                     `json:"scope"`    // NODE|ORGANIZATION|LOCATION|END_USER (scoped report)
	EntityID int64                      `json:"entityId"` // scoped report
	DeviceID int64                      `json:"deviceId"` // device report
	Fields   map[string]json.RawMessage `json:"fields"`
}

// ---- org lookup ----

func fetchOrgs(ctx context.Context, c *client.Client) (map[int64]string, error) {
	raw, err := c.Get(ctx, "/v2/organizations", nil)
	if err != nil {
		return nil, err
	}
	var orgs []njOrg
	if err := json.Unmarshal(raw, &orgs); err != nil {
		return nil, fmt.Errorf("parsing organizations: %w", err)
	}
	m := make(map[int64]string, len(orgs))
	for _, o := range orgs {
		m[o.ID] = o.Name
	}
	return m, nil
}

// ---- device paging ----

// fetchDevices pages /v2/devices using the `after`=last-id cursor convention.
// maxPages caps pages scanned. df is an optional device filter. Returns the
// devices and the number of pages actually scanned.
func fetchDevices(ctx context.Context, c *client.Client, df string, maxPages int) ([]njDevice, int, error) {
	const pageSize = 1000
	out := make([]njDevice, 0)
	var after int64
	pages := 0
	for pages < maxPages {
		params := map[string]string{"pageSize": strconv.Itoa(pageSize)}
		if df != "" {
			params["df"] = df
		}
		if after > 0 {
			params["after"] = strconv.FormatInt(after, 10)
		}
		raw, err := c.Get(ctx, "/v2/devices", params)
		if err != nil {
			return out, pages, err
		}
		var batch []njDevice
		if err := json.Unmarshal(raw, &batch); err != nil {
			return out, pages, fmt.Errorf("parsing devices: %w", err)
		}
		pages++
		out = append(out, batch...)
		if len(batch) < pageSize {
			break
		}
		after = batch[len(batch)-1].ID
	}
	return out, pages, nil
}

// deviceOrgIndex builds deviceId->device and deviceId->organizationId maps.
func deviceOrgIndex(devices []njDevice) (map[int64]njDevice, map[int64]int64) {
	byID := make(map[int64]njDevice, len(devices))
	devToOrg := make(map[int64]int64, len(devices))
	for _, d := range devices {
		byID[d.ID] = d
		devToOrg[d.ID] = d.OrganizationID
	}
	return byID, devToOrg
}

// ---- patch query paging ----

// fetchPatches pages a /v2/queries/{os,software}-patches endpoint following
// the cursor.name back as the `cursor` param. status/severity are optional
// filters. maxPages caps pages scanned. Returns patches + pages scanned.
func fetchPatches(ctx context.Context, c *client.Client, path, status, severity string, maxPages int) ([]njPatch, int, error) {
	const pageSize = 1000
	out := make([]njPatch, 0)
	cursor := ""
	pages := 0
	for pages < maxPages {
		params := map[string]string{"pageSize": strconv.Itoa(pageSize)}
		if status != "" {
			params["status"] = status
		}
		if severity != "" {
			params["severity"] = severity
		}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.Get(ctx, path, params)
		if err != nil {
			return out, pages, err
		}
		var env struct {
			Cursor  njCursor  `json:"cursor"`
			Results []njPatch `json:"results"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return out, pages, fmt.Errorf("parsing patches: %w", err)
		}
		pages++
		out = append(out, env.Results...)
		if len(env.Results) == 0 || env.Cursor.Name == "" || env.Cursor.Name == cursor {
			break
		}
		cursor = env.Cursor.Name
	}
	return out, pages, nil
}

// fetchAlerts fetches /v2/alerts (bare array).
func fetchAlerts(ctx context.Context, c *client.Client) ([]njAlert, error) {
	raw, err := c.Get(ctx, "/v2/alerts", nil)
	if err != nil {
		return nil, err
	}
	var alerts []njAlert
	if err := json.Unmarshal(raw, &alerts); err != nil {
		return nil, fmt.Errorf("parsing alerts: %w", err)
	}
	return alerts, nil
}

// fetchScopedCustomFields pages /v2/queries/scoped-custom-fields following the
// cursor. Returns all rows + pages scanned.
func fetchScopedCustomFields(ctx context.Context, c *client.Client, scopes string, maxPages int) ([]njCFRow, int, error) {
	const pageSize = 1000
	out := make([]njCFRow, 0)
	cursor := ""
	pages := 0
	for pages < maxPages {
		params := map[string]string{"pageSize": strconv.Itoa(pageSize)}
		if scopes != "" {
			params["scopes"] = scopes
		}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.Get(ctx, "/v2/queries/scoped-custom-fields", params)
		if err != nil {
			return out, pages, err
		}
		var env struct {
			Cursor  njCursor  `json:"cursor"`
			Results []njCFRow `json:"results"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return out, pages, fmt.Errorf("parsing custom fields: %w", err)
		}
		pages++
		out = append(out, env.Results...)
		if len(env.Results) == 0 || env.Cursor.Name == "" || env.Cursor.Name == cursor {
			break
		}
		cursor = env.Cursor.Name
	}
	return out, pages, nil
}

// ---- pure-logic helpers (unit-tested) ----

// effectiveMaxScanPages forces a single page under the dogfood timeout budget.
func effectiveMaxScanPages(requested int) int {
	if requested < 1 {
		requested = 1
	}
	if cliutil.IsDogfoodEnv() && requested > 1 {
		return 1
	}
	return requested
}

// orgMatches reports whether device org (id+name) matches a user --org filter.
// An empty filter matches everything. A numeric filter matches the org id; a
// non-numeric filter is a case-insensitive substring match on the org name.
func orgMatches(filter string, orgID int64, orgName string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true
	}
	if n, err := strconv.ParseInt(filter, 10, 64); err == nil {
		return n == orgID
	}
	return strings.Contains(strings.ToLower(orgName), strings.ToLower(filter))
}

// cfValueEmpty reports whether a custom-field raw value counts as "missing".
// null, "", "[]", "{}", and whitespace-only strings are empty.
func cfValueEmpty(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	switch s {
	case "", "null", `""`, "[]", "{}":
		return true
	}
	// Unquote string values so "  " counts as empty.
	if strings.HasPrefix(s, `"`) {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil && strings.TrimSpace(str) == "" {
			return true
		}
	}
	return false
}

// missingRequiredFields returns which of the required field names are absent
// or empty in the given field map. Field lookup is case-insensitive.
func missingRequiredFields(fields map[string]json.RawMessage, required []string) []string {
	lower := make(map[string]json.RawMessage, len(fields))
	for k, v := range fields {
		lower[strings.ToLower(k)] = v
	}
	missing := make([]string, 0)
	for _, r := range required {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		v, ok := lower[strings.ToLower(r)]
		if !ok || cfValueEmpty(v) {
			missing = append(missing, r)
		}
	}
	return missing
}

// timeBucket floors an epoch-seconds time into a window-sized bucket.
func timeBucket(epochSec int64, windowSec int64) int64 {
	if windowSec <= 0 {
		return epochSec
	}
	return epochSec - (epochSec % windowSec)
}

// parseCSVList splits a comma-separated flag value, trimming and dropping empties.
func parseCSVList(s string) []string {
	out := make([]string, 0)
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// boundLimit truncates a slice length to limit when limit > 0.
func boundLimit(n, limit int) int {
	if limit > 0 && n > limit {
		return limit
	}
	return n
}
