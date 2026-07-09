// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel-feature support: shared helpers for the GAT-style audit
// commands (audit *, workflow offboard). Pure logic lives here so it can be
// unit-tested without a live Google Workspace; the command files wire flags and
// live API calls on top of these.

package cli

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// Google API host roots. Each Google API lives on its own host; the generated
// client accepts an absolute URL per call, so novel commands target hosts
// directly rather than relying on a single base URL.
const (
	wsDirectoryBase = "https://admin.googleapis.com/admin/directory/v1"
	wsReportsBase   = "https://admin.googleapis.com/admin/reports/v1"
	wsDriveBase     = "https://www.googleapis.com/drive/v3"
)

// emailDomain returns the lowercased domain part of an email address, or "" if
// the address has no usable domain.
// validUserKey reports whether s is a plausible Google user key: an email
// address or an immutable numeric directory ID. Lets commands reject obvious
// garbage (exit 2) instead of round-tripping a bad key to the API.
func validUserKey(s string) bool {
	if strings.Contains(s, "@") {
		return emailDomain(s) != ""
	}
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func emailDomain(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	i := strings.LastIndex(email, "@")
	if i < 0 || i == len(email)-1 {
		return ""
	}
	return email[i+1:]
}

// internalDomainSet builds a lookup set of company-owned domains from a
// comma-separated flag value. Empty entries are ignored.
func internalDomainSet(csv string) map[string]bool {
	set := map[string]bool{}
	for _, d := range strings.Split(csv, ",") {
		d = strings.TrimSpace(strings.ToLower(d))
		if d != "" {
			set[d] = true
		}
	}
	return set
}

// --- OAuth scope risk scoring (the audit app-risk core) ---

// scopeRiskTier classifies a set of OAuth scopes into an overall risk tier and
// reports whether any scope grants full Drive access. The tier is the maximum
// risk across all scopes: "High" > "Moderate" > "Low". The classification is a
// curated static reference table — the Directory API returns only raw scope
// strings with no risk metadata.
func scopeRiskTier(scopes []string) (tier string, fullDrive bool) {
	rank := 0 // 0 low, 1 moderate, 2 high
	for _, s := range scopes {
		r, fd := scopeRisk(s)
		if fd {
			fullDrive = true
		}
		if r > rank {
			rank = r
		}
	}
	switch rank {
	case 2:
		return "High", fullDrive
	case 1:
		return "Moderate", fullDrive
	default:
		return "Low", fullDrive
	}
}

// scopeRisk scores a single scope. Returns (rank, isFullDrive).
func scopeRisk(scope string) (int, bool) {
	s := strings.ToLower(strings.TrimSpace(scope))
	readonly := strings.HasSuffix(s, ".readonly") || strings.Contains(s, "readonly")

	// Full, unrestricted Drive or Gmail mailbox access is the highest risk.
	switch {
	case s == "https://www.googleapis.com/auth/drive":
		return 2, true
	case s == "https://mail.google.com/":
		return 2, false
	case s == "https://www.googleapis.com/auth/cloud-platform":
		return 2, false
	}

	// Admin Directory / Reports write access, Gmail modify/send, and broad
	// content scopes are high risk when not read-only.
	highSubstrings := []string{
		"auth/admin.directory", "auth/apps.", "auth/gmail.modify",
		"auth/gmail.settings", "auth/gmail.send", "auth/gmail.compose",
		"auth/ediscovery", "auth/cloud-identity",
	}
	for _, h := range highSubstrings {
		if strings.Contains(s, h) {
			if readonly && !strings.Contains(s, "settings") {
				return 1, false // read-only admin/content is moderate
			}
			return 2, false
		}
	}

	// Broad read-only content access and drive.file are moderate.
	moderateSubstrings := []string{
		"auth/drive", "auth/gmail", "auth/calendar", "auth/contacts",
		"auth/spreadsheets", "auth/documents", "auth/reports",
	}
	for _, m := range moderateSubstrings {
		if strings.Contains(s, m) {
			return 1, s == "https://www.googleapis.com/auth/drive"
		}
	}

	// openid/email/profile and other narrow scopes are low risk.
	return 0, false
}

// riskRank maps a tier label to its numeric order for --min-risk filtering.
func riskRank(tier string) int {
	switch strings.ToLower(tier) {
	case "high":
		return 2
	case "moderate", "medium":
		return 1
	default:
		return 0
	}
}

// --- External-sharing classification (audit external-shares / domain-graph) ---

// driveExternalShare is one externally-shared file row.
type driveExternalShare struct {
	FileID       string `json:"file_id"`
	Name         string `json:"name"`
	Owner        string `json:"owner"`
	ShareType    string `json:"share_type"` // anyone | external_user | external_domain
	ExternalWith string `json:"external_with,omitempty"`
}

// classifyPermission decides whether a single Drive permission represents
// external exposure given the set of internal (company-owned) domains. It
// returns whether the permission is external, a share-type label, and the
// external counterparty (email/domain) when applicable.
func classifyPermission(permType, emailAddress, domain string, internal map[string]bool) (external bool, shareType, with string) {
	switch strings.ToLower(strings.TrimSpace(permType)) {
	case "anyone":
		return true, "anyone", ""
	case "user", "group":
		d := emailDomain(emailAddress)
		if d == "" {
			return false, "", ""
		}
		if internal[d] {
			return false, "", ""
		}
		return true, "external_" + strings.ToLower(permType), emailAddress
	case "domain":
		d := strings.ToLower(strings.TrimSpace(domain))
		if d == "" || internal[d] {
			return false, "", ""
		}
		return true, "external_domain", d
	default:
		return false, "", ""
	}
}

// domainEdge is a per-external-domain rollup for the domain connection graph.
type domainEdge struct {
	Domain    string `json:"domain"`
	FileCount int    `json:"file_count"`
	UserCount int    `json:"user_count"`
}

// aggregateDomainEdges rolls external shares up into per-domain edges, counting
// distinct files and distinct external counterparties per domain. "anyone"
// shares are grouped under the synthetic domain "(public link)".
func aggregateDomainEdges(shares []driveExternalShare) []domainEdge {
	type acc struct {
		files map[string]bool
		users map[string]bool
	}
	m := map[string]*acc{}
	for _, sh := range shares {
		dom := sh.ExternalWith
		if sh.ShareType == "anyone" {
			dom = "(public link)"
		} else {
			dom = emailDomain(sh.ExternalWith)
			if dom == "" {
				dom = strings.ToLower(sh.ExternalWith)
			}
		}
		if dom == "" {
			continue
		}
		a := m[dom]
		if a == nil {
			a = &acc{files: map[string]bool{}, users: map[string]bool{}}
			m[dom] = a
		}
		a.files[sh.FileID] = true
		// Only count an actual external counterparty (an email) as a distinct
		// user; an external_domain share's ExternalWith is the domain itself.
		if strings.Contains(sh.ExternalWith, "@") {
			a.users[strings.ToLower(sh.ExternalWith)] = true
		}
	}
	out := make([]domainEdge, 0, len(m))
	for dom, a := range m {
		out = append(out, domainEdge{Domain: dom, FileCount: len(a.files), UserCount: len(a.users)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FileCount != out[j].FileCount {
			return out[i].FileCount > out[j].FileCount
		}
		return out[i].Domain < out[j].Domain
	})
	return out
}

// --- Activity timeline (audit reconstruct / logins) ---

// activityEvent is one normalized audit-log event.
type activityEvent struct {
	Time        string `json:"time"`
	Application string `json:"application"`
	Actor       string `json:"actor,omitempty"`
	Name        string `json:"name"`
	Detail      string `json:"detail,omitempty"`
	IP          string `json:"ip,omitempty"`
}

// mergeTimeline sorts events chronologically (ascending by RFC3339 time string,
// which sorts lexically for that format). Events with empty times sort last.
func mergeTimeline(events []activityEvent) []activityEvent {
	out := append([]activityEvent(nil), events...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Time == "" {
			return false
		}
		if out[j].Time == "" {
			return true
		}
		return out[i].Time < out[j].Time
	})
	return out
}

// parseReportsActivities decodes a Reports API activities.list response into
// normalized events. The Reports response shape is
// {"items":[{"id":{"time":..,"applicationName":..},"actor":{"email":..},
// "ipAddress":..,"events":[{"name":..,"parameters":[..]}]}]}.
func parseReportsActivities(data json.RawMessage) ([]activityEvent, error) {
	var env struct {
		Items []struct {
			ID struct {
				Time            string `json:"time"`
				ApplicationName string `json:"applicationName"`
			} `json:"id"`
			Actor struct {
				Email string `json:"email"`
			} `json:"actor"`
			IPAddress string `json:"ipAddress"`
			Events    []struct {
				Name       string `json:"name"`
				Parameters []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"parameters"`
			} `json:"events"`
		} `json:"items"`
	}
	if len(data) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	var out []activityEvent
	for _, it := range env.Items {
		base := activityEvent{
			Time:        it.ID.Time,
			Application: it.ID.ApplicationName,
			Actor:       it.Actor.Email,
			IP:          it.IPAddress,
		}
		if len(it.Events) == 0 {
			out = append(out, base)
			continue
		}
		for _, ev := range it.Events {
			e := base
			e.Name = ev.Name
			var parts []string
			for _, p := range ev.Parameters {
				if p.Value != "" {
					parts = append(parts, p.Name+"="+p.Value)
				}
			}
			e.Detail = strings.Join(parts, " ")
			out = append(out, e)
		}
	}
	return out, nil
}

// isLoginFailure reports whether a login event name denotes a failed/blocked
// sign-in attempt.
func isLoginFailure(eventName string) bool {
	n := strings.ToLower(eventName)
	return strings.Contains(n, "failure") || strings.Contains(n, "fail") ||
		strings.Contains(n, "suspicious") || strings.Contains(n, "blocked")
}

// --- small shared helpers for live calls ---

// emitAudit writes an audit result as JSON when machine output is requested,
// otherwise prints a compact human summary line plus the JSON. Audit commands
// are read-only and agent-facing, so JSON is the primary surface.
func emitAudit(cmd *cobra.Command, flags *rootFlags, v any) error {
	return flags.printJSON(cmd, v)
}
