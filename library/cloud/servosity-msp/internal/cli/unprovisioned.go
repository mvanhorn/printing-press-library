// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/store"
)

// unprovisionedItem is one row in the unprovisioned report. Fields are sized
// to match the JSON shape the spec calls for: every unprovisioned agent is a
// client device that's installed but not yet pulling backups.
type unprovisionedItem struct {
	CompanyID   int64   `json:"company_id"`
	CompanyName string  `json:"company_name"`
	AgentID     string  `json:"agent_id"`
	Hostname    string  `json:"hostname"`
	InstalledAt string  `json:"installed_at"`
	AgeHours    float64 `json:"age_hours"`
}

// newUnprovisionedCmd builds the unprovisioned-agents lost-revenue report.
//
// Flow:
//  1. Resolve reseller ID inline from /current-user/ (no global config knob;
//     the reseller is whoever the API token authenticates as).
//  2. GET /resellers/{reseller_id}/agents/unprovisioned/ — the list.
//  3. Apply --age and --company filters in-process.
//  4. LEFT JOIN against the local `companies` store table for friendly names;
//     "(unknown)" when no row matches (sync may not be fresh).
//
// Read-only, verify-friendly, --dry-run short-circuits before any IO.
func newUnprovisionedCmd(flags *rootFlags) *cobra.Command {
	var ageStr string
	var companyFilter int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "unprovisioned",
		Short: "List agents installed but not yet pulling backups (lost-revenue surface)",
		Long: `Lists agents that have been installed on client machines but are NOT yet
pulling backups. Every unprovisioned agent is a client device that is not
contributing to the backup subscription — follow up to finish onboarding.

Reseller ID is resolved automatically from /current-user/. Company names are
joined against the local sync store; run 'servosity-msp-cli sync' first
for friendly names (otherwise "(unknown)" is shown).`,
		Example: `  # Default: agents unprovisioned > 24h
  servosity-msp-cli unprovisioned

  # Show only agents stuck for more than a week
  servosity-msp-cli unprovisioned --age 168h

  # Narrow to one company
  servosity-msp-cli unprovisioned --company 4421

  # JSON for downstream tooling
  servosity-msp-cli unprovisioned --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			age, err := time.ParseDuration(ageStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --age value %q: %w", ageStr, err))
			}
			if age < 0 {
				return usageErr(fmt.Errorf("--age must be >= 0, got %s", ageStr))
			}

			// --dry-run short-circuits BEFORE any IO so verify probes don't
			// touch the API or the store.
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: resolve reseller ID. /current-user/ doesn't expose
			// it on partner-scoped tokens, so cliutil.ResolveResellerID
			// derives it from the first company's reseller URL field.
			// SERVOSITY_MSP_RESELLER_ID overrides the probe for CI/scripting.
			resellerInt64, err := cliutil.ResolveResellerID(cmd.Context(), c)
			if err != nil {
				return fmt.Errorf("resolving reseller ID: %w", err)
			}
			resellerID := resellerInt64

			// Step 2: fetch the unprovisioned list.
			path := fmt.Sprintf("/resellers/%d/agents/unprovisioned/", resellerID)
			data, err := c.Get(path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Step 3: parse list items + apply --age and --company filters.
			rawItems := unwrapList(data)
			cutoff := time.Now().Add(-age)
			items := make([]unprovisionedItem, 0, len(rawItems))
			companyIDs := make(map[int64]bool, len(rawItems))
			for _, raw := range rawItems {
				var row map[string]any
				if err := json.Unmarshal(raw, &row); err != nil {
					continue
				}
				cid := extractCompanyID(row)
				if companyFilter != 0 && cid != int64(companyFilter) {
					continue
				}
				installedRaw := firstString(row, "installed_at", "installedAt", "created_at", "createdAt")
				installedT, parsed := parseTimestamp(installedRaw)
				// --age default 24h: skip installs younger than the cutoff.
				// Rows with unparseable timestamps are kept (we can't tell —
				// surface them rather than silently drop).
				if parsed && installedT.After(cutoff) {
					continue
				}
				ageHours := 0.0
				if parsed {
					ageHours = time.Since(installedT).Hours()
				}
				item := unprovisionedItem{
					CompanyID:   cid,
					AgentID:     firstString(row, "agent_id", "agentId", "id"),
					Hostname:    firstString(row, "hostname", "host_name", "host"),
					InstalledAt: installedRaw,
					AgeHours:    ageHours,
				}
				items = append(items, item)
				if cid != 0 {
					companyIDs[cid] = true
				}
			}

			// Step 4: LEFT JOIN against local companies for friendly names.
			// Best-effort: a missing DB just leaves names as "(unknown)" —
			// the report still ships, sync isn't a hard dependency.
			names := map[int64]string{}
			if dbPath == "" {
				dbPath = defaultDBPath("servosity-msp-cli")
			}
			if db, err := store.OpenWithContext(cmd.Context(), dbPath); err == nil {
				for id := range companyIDs {
					var name string
					row := db.DB().QueryRowContext(cmd.Context(),
						`SELECT COALESCE(name, '') FROM companies WHERE id = ?`, id)
					if scanErr := row.Scan(&name); scanErr == nil && name != "" {
						names[id] = name
					}
				}
				db.Close()
			}
			for i := range items {
				if n, ok := names[items[i].CompanyID]; ok {
					items[i].CompanyName = n
				} else {
					items[i].CompanyName = "(unknown)"
				}
			}

			// Deterministic order: oldest install first (biggest lost-revenue
			// hit), then by company id for stability.
			sort.Slice(items, func(i, j int) bool {
				if items[i].AgeHours != items[j].AgeHours {
					return items[i].AgeHours > items[j].AgeHours
				}
				return items[i].CompanyID < items[j].CompanyID
			})

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), items, flags)
			}

			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No unprovisioned agents found.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-12s %-20s %-22s %s\n",
				"COMPANY", "AGENT_ID", "HOSTNAME", "INSTALLED", "AGE")
			for _, it := range items {
				company := fmt.Sprintf("%s (%d)", it.CompanyName, it.CompanyID)
				if len(company) > 25 {
					company = company[:24] + "…"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-12s %-20s %-22s %s\n",
					company, it.AgentID, it.Hostname, it.InstalledAt, formatAge(it.AgeHours))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&ageStr, "age", "24h",
		"Only show agents unprovisioned for longer than this duration (Go duration syntax: 24h, 168h, etc.). Filters out brand-new installs that haven't had time to phone home.")
	cmd.Flags().IntVar(&companyFilter, "company", 0, "Filter to one company (by company id)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servosity-msp-cli/data.db)")

	return cmd
}

// extractResellerID pulls the reseller id from a /current-user/ payload.
// The endpoint isn't strictly typed across deployments — some return
// "reseller_id" / "resellerId" as a number, others nest it under
// "reseller": {"id": N} or "reseller": N. Try each shape in turn.
func extractResellerID(u map[string]any) int64 {
	if id := coerceID(u["reseller_id"]); id != 0 {
		return id
	}
	if id := coerceID(u["resellerId"]); id != 0 {
		return id
	}
	if r, ok := u["reseller"]; ok {
		switch t := r.(type) {
		case map[string]any:
			if id := coerceID(t["id"]); id != 0 {
				return id
			}
		default:
			if id := coerceID(t); id != 0 {
				return id
			}
		}
	}
	return 0
}

// extractCompanyID pulls a company id from an unprovisioned-agent row.
// Same multi-shape tolerance as extractResellerID — the field may be
// company_id (numeric), company (numeric), or company: {id: N}.
func extractCompanyID(row map[string]any) int64 {
	if id := coerceID(row["company_id"]); id != 0 {
		return id
	}
	if id := coerceID(row["companyId"]); id != 0 {
		return id
	}
	if c, ok := row["company"]; ok {
		switch t := c.(type) {
		case map[string]any:
			if id := coerceID(t["id"]); id != 0 {
				return id
			}
		default:
			if id := coerceID(t); id != 0 {
				return id
			}
		}
	}
	return 0
}

// firstString returns the first non-empty string-typed value found at any of
// the candidate keys. JSON numbers fall through (they're not strings) which
// is correct here: every target field — hostname, agent_id, installed_at —
// is a string in every shape this endpoint emits.
func firstString(row map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := row[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// parseTimestamp returns the parsed time + ok=true when the string is a
// recognised installed_at format. RFC3339 covers every shape this API has
// emitted in practice; the fallback to RFC3339Nano catches sub-second
// timestamps without changing the happy path.
func parseTimestamp(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// formatAge renders hours as "Xd Yh" for the human table.
func formatAge(hours float64) string {
	if hours <= 0 {
		return "—"
	}
	days := int(hours) / 24
	rem := int(hours) % 24
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, rem)
	}
	return fmt.Sprintf("%dh", rem)
}
