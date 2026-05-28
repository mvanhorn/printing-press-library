// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// contactProps is the parsed properties block of a HubSpot contact record.
// All numeric and date fields come back from HubSpot as JSON strings; this
// struct normalizes them into typed values once and tracks "value was
// present" via *Set companion booleans so callers can distinguish "score is
// zero" from "score was never set on this contact".
type contactProps struct {
	ID                    string
	Email                 string
	FirstName             string
	LastName              string
	Company               string
	HubspotScore          float64
	HubspotScoreSet       bool
	LifecycleStage        string
	LeadStatus            string
	LastActivityDate      time.Time
	LastActivitySet       bool
	EmailOpen             int64
	EmailClick            int64
	OwnerID               string
	AnalyticsSource       string
	AnalyticsSourceDetail string
	NumAssociatedDeals    int64
	TotalRevenue          float64
	CreateDate            time.Time
	LastModifiedDate      time.Time
}

// hubspotContactEnvelope is the wire shape returned by GET /crm/v3/objects/contacts.
// properties values arrive as strings ("45" not 45); parseContactProps does
// the conversion once for downstream analytics commands.
type hubspotContactEnvelope struct {
	ID         string            `json:"id"`
	Properties map[string]string `json:"properties"`
	CreatedAt  string            `json:"createdAt"`
	UpdatedAt  string            `json:"updatedAt"`
	Archived   bool              `json:"archived"`
}

// parseContactProps decodes a stored crm.data JSON row into a contactProps
// value. id is the row's primary key; it always lands on the returned struct
// even when properties is nil so callers iterating crm rows can still surface
// the contact id in "no data" output. Missing properties are zero-valued
// rather than rejected — most analytics commands handle absence with a
// filter predicate, not an error.
func parseContactProps(id string, data json.RawMessage) (contactProps, error) {
	out := contactProps{ID: id}
	if len(data) == 0 {
		return out, nil
	}
	var env hubspotContactEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return out, err
	}
	if env.ID != "" {
		out.ID = env.ID
	}
	if t, ok := parseHubSpotTime(env.CreatedAt); ok {
		out.CreateDate = t
	}
	if t, ok := parseHubSpotTime(env.UpdatedAt); ok {
		out.LastModifiedDate = t
	}
	if env.Properties == nil {
		return out, nil
	}
	out.Email = trimNullish(env.Properties["email"])
	out.FirstName = trimNullish(env.Properties["firstname"])
	out.LastName = trimNullish(env.Properties["lastname"])
	out.Company = trimNullish(env.Properties["company"])
	if v, ok := parseStringFloat(env.Properties["hubspotscore"]); ok {
		out.HubspotScore = v
		out.HubspotScoreSet = true
	}
	out.LifecycleStage = trimNullish(env.Properties["lifecyclestage"])
	out.LeadStatus = trimNullish(env.Properties["hs_lead_status"])
	if t, ok := parseHubSpotTime(env.Properties["hs_last_activity_date"]); ok {
		out.LastActivityDate = t
		out.LastActivitySet = true
	}
	if v, ok := parseStringInt(env.Properties["hs_email_open"]); ok {
		out.EmailOpen = v
	}
	if v, ok := parseStringInt(env.Properties["hs_email_click"]); ok {
		out.EmailClick = v
	}
	out.OwnerID = trimNullish(env.Properties["hubspot_owner_id"])
	out.AnalyticsSource = trimNullish(env.Properties["hs_analytics_source"])
	out.AnalyticsSourceDetail = trimNullish(env.Properties["hs_analytics_source_data_1"])
	if v, ok := parseStringInt(env.Properties["num_associated_deals"]); ok {
		out.NumAssociatedDeals = v
	}
	if v, ok := parseStringFloat(env.Properties["total_revenue"]); ok {
		out.TotalRevenue = v
	}
	// HubSpot also surfaces createdate/lastmodifieddate inside properties;
	// prefer those when present because the envelope-level fields can lag
	// slightly behind a property-history write.
	if t, ok := parseHubSpotTime(env.Properties["createdate"]); ok {
		out.CreateDate = t
	}
	if t, ok := parseHubSpotTime(env.Properties["lastmodifieddate"]); ok {
		out.LastModifiedDate = t
	}
	return out, nil
}

// trimNullish strips whitespace and treats "null"/"undefined" sentinels as
// empty. HubSpot sometimes returns the literal string "null" for properties
// that were cleared via the API; without this guard those leak into output
// as visible "null" tokens.
func trimNullish(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return ""
	}
	lower := strings.ToLower(t)
	if lower == "null" || lower == "undefined" {
		return ""
	}
	return t
}

// parseStringFloat parses HubSpot's string-encoded numeric properties.
// Returns ok=false on empty input or unparseable values so callers can fall
// back to a "value not set" predicate without distinguishing "0" from "".
func parseStringFloat(s string) (float64, bool) {
	t := trimNullish(s)
	if t == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseStringInt parses HubSpot's string-encoded integer properties. Falls
// back to ParseFloat for values like "12.0" that HubSpot occasionally emits
// for fields that are otherwise integers.
func parseStringInt(s string) (int64, bool) {
	t := trimNullish(s)
	if t == "" {
		return 0, false
	}
	if v, err := strconv.ParseInt(t, 10, 64); err == nil {
		return v, true
	}
	if v, err := strconv.ParseFloat(t, 64); err == nil {
		return int64(v), true
	}
	return 0, false
}

// hubSpotTimeLayouts covers both the RFC3339 form HubSpot uses in v3 API
// responses and the millisecond-precision variant ("2024-01-01T00:00:00.000Z")
// that property values use. time.Parse with a layout list keeps callers free
// of layout-detection boilerplate.
var hubSpotTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.000Z",
	"2006-01-02T15:04:05Z",
	"2006-01-02",
}

// parseHubSpotTime accepts the formats HubSpot emits across endpoints.
// Returns ok=false on empty/unparseable values; callers gate against the
// LastActivitySet flag (or similar) before reading the time.
func parseHubSpotTime(s string) (time.Time, bool) {
	t := trimNullish(s)
	if t == "" {
		return time.Time{}, false
	}
	// HubSpot also sometimes returns numeric epoch-millis strings in
	// property values; try that path before declaring failure.
	if n, err := strconv.ParseInt(t, 10, 64); err == nil && n > 1_000_000_000_000 {
		return time.UnixMilli(n).UTC(), true
	}
	for _, layout := range hubSpotTimeLayouts {
		if parsed, err := time.Parse(layout, t); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// contactDisplayName returns the most identifying short label for a contact:
// "First Last" if any name parts are set, otherwise the email, otherwise the
// HubSpot id. Used everywhere a list row needs a single human-readable
// column.
func contactDisplayName(p contactProps) string {
	first := strings.TrimSpace(p.FirstName)
	last := strings.TrimSpace(p.LastName)
	switch {
	case first != "" && last != "":
		return first + " " + last
	case first != "":
		return first
	case last != "":
		return last
	case p.Email != "":
		return p.Email
	default:
		return p.ID
	}
}
