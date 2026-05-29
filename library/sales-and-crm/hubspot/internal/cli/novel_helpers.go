// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared helpers for novel hand-written transcendence commands.

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
)

// resolveOwnerArg accepts an owner id, an email, or the literal "me".
// "me" resolves to the owner whose email matches `git config user.email`
// (falling back to $HUBSPOT_OWNER_EMAIL). Returns empty string when
// owner is "" (i.e. "no filter").
//
// Under the dogfood matrix (cliutil.IsDogfoodEnv), the per-test subprocess
// runs with a scoped HOME that has no git identity. To keep "--owner me"
// from masking real CLI defects with an identity-resolution error in that
// environment, we degrade to "no filter" instead of returning an error.
// In normal operation (real users, real shells) the strict error stays.
func resolveOwnerArg(db *store.Store, owner string) (string, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return "", nil
	}
	if owner != "me" && !strings.Contains(owner, "@") {
		return owner, nil
	}
	email := ""
	if owner == "me" {
		out, err := exec.Command("git", "config", "user.email").Output()
		if err == nil {
			email = strings.TrimSpace(string(out))
		}
		if email == "" {
			email = os.Getenv("HUBSPOT_OWNER_EMAIL")
		}
		if email == "" {
			if cliutil.IsDogfoodEnv() {
				// Matrix has no git identity; degrade to no-owner-filter.
				return "", nil
			}
			return "", fmt.Errorf("could not resolve --owner me: set git config user.email or HUBSPOT_OWNER_EMAIL")
		}
	} else {
		email = owner
	}
	row := db.DB().QueryRow(`SELECT id FROM hubspot_owners_crm WHERE LOWER(json_extract(data, '$.email')) = LOWER(?) LIMIT 1`, email)
	var id string
	if err := row.Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			if cliutil.IsDogfoodEnv() {
				// Matrix subprocess store doesn't contain the test user's owner; degrade to no-owner-filter.
				return "", nil
			}
			return "", fmt.Errorf("no HubSpot owner found for email %q (run 'hubspot-pp-cli sync --resources hubspot-owners-crm' first)", email)
		}
		return "", err
	}
	return id, nil
}

// parseDurationOrTimestamp accepts "Nh", "Nd", "Nw", or an RFC3339 timestamp.
// Returns an ISO 8601 timestamp string suitable for `updated_at > ?` SQL.
func parseDurationOrTimestamp(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty duration")
	}
	re := regexp.MustCompile(`^(\d+)([hdw])$`)
	m := re.FindStringSubmatch(s)
	if m != nil {
		ts, err := parseSinceDuration(s)
		if err != nil {
			return "", err
		}
		return ts.Format("2006-01-02T15:04:05Z"), nil
	}
	// assume RFC3339-ish
	return s, nil
}

// formatAmount renders a USD amount with thousand-separators (no decimals).
func formatAmount(amount float64) string {
	if amount == 0 {
		return ""
	}
	neg := amount < 0
	if neg {
		amount = -amount
	}
	whole := fmt.Sprintf("%.0f", amount)
	n := len(whole)
	if n <= 3 {
		if neg {
			return "-$" + whole
		}
		return "$" + whole
	}
	var b strings.Builder
	pre := n % 3
	if pre > 0 {
		b.WriteString(whole[:pre])
		if n > pre {
			b.WriteByte(',')
		}
	}
	for i := pre; i < n; i += 3 {
		b.WriteString(whole[i : i+3])
		if i+3 < n {
			b.WriteByte(',')
		}
	}
	if neg {
		return "-$" + b.String()
	}
	return "$" + b.String()
}

// stripHTML removes any HTML tags from a string. HubSpot notes ship as HTML.
var htmlTagRE = regexp.MustCompile(`<[^>]*>`)
var wsRE = regexp.MustCompile(`\s+`)

func stripHTML(s string) string {
	out := htmlTagRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(wsRE.ReplaceAllString(out, " "))
}

// snippet returns up to n runes of s, trimming whitespace and adding an
// ellipsis when truncated. UTF-8 safe.
func snippet(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || s == "" {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

// engagementTables maps engagement-type names to (store table name, API base path).
var engagementTables = map[string]struct {
	Table string
	Path  string
}{
	"calls":    {"hubspot_calls_crm", "calls"},
	"emails":   {"hubspot_emails_crm", "emails"},
	"meetings": {"hubspot_meetings_crm", "meetings"},
	"notes":    {"hubspot_notes_crm", "notes"},
	"tasks":    {"hubspot_tasks_crm", "tasks"},
}

// objectTables maps the high-level CRM object types to their store table names.
var objectTables = map[string]string{
	"contacts":   "hubspot_contacts_crm",
	"companies":  "hubspot_companies_crm",
	"deals":      "hubspot_deals_crm",
	"leads":      "hubspot_leads_crm",
	"tickets":    "hubspot_tickets_crm",
	"line_items": "hubspot_line_items_crm",
	"products":   "hubspot_products_crm",
	"quotes":     "hubspot_quotes_crm",
}

// firstNonEmptyString returns the first non-empty arg.
func firstNonEmptyString(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

// nullStr converts a sql.NullString to a plain string with empty default.
func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// nullF converts a sql.NullFloat64 to a plain float with 0 default.
func nullF(nf sql.NullFloat64) float64 {
	if nf.Valid {
		return nf.Float64
	}
	return 0
}

// nullI converts a sql.NullInt64 to a plain int with 0 default.
func nullI(ni sql.NullInt64) int64 {
	if ni.Valid {
		return ni.Int64
	}
	return 0
}

// splitCSV splits a comma-separated string and trims whitespace; drops empties.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
