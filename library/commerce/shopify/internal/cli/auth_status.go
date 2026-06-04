package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// auth_status implements `shopify-pp-cli auth audit`, which audits the OAuth
// client_credentials grant setup. The optional SHOPIFY_REFRESHER_HINT env var
// lets operators surface a path to their own token-refresh script in the audit
// JSON output, so on-call automation knows where to rotate.
//
// The press-emitted doctor command only knows about SHOPIFY_ACCESS_TOKEN
// presence; it doesn't understand client_credentials rotation. This command
// fills the gap with three additional checks that survive press regens
// (this file is novel-only and preserved by the generator's merge).

// AuthStatus is the JSON payload `auth status` emits.
type AuthStatus struct {
	OK              bool        `json:"ok"`
	Checks          []AuthCheck `json:"checks"`
	TokenAgeHours   float64     `json:"token_age_hours,omitempty"`
	StaleThresholdH float64     `json:"stale_threshold_hours"`
	RefresherHint   string      `json:"refresher_hint,omitempty"`
}

// AuthCheck is one row in the status report.
type AuthCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok" | "warn" | "fail" | "info"
	Detail string `json:"detail"`
}

func newAuthAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit OAuth client_credentials setup and access-token freshness.",
		Long: `Checks the Shopify auth setup assuming the Hermes-style OAuth client_credentials
grant rotation pattern:

  1. SHOPIFY_CLIENT_ID    — required for token rotation
  2. SHOPIFY_CLIENT_SECRET — required for token rotation
  3. SHOPIFY_ACCESS_TOKEN  — the rotated bearer the CLI sends to Shopify

Token freshness is determined from one of (in order):

  - SHOPIFY_TOKEN_REFRESHED_AT_FILE: path to a file whose mtime is the
    last-rotation timestamp. Set this from your refresher script with
    something like: ` + "`" + `touch "$SHOPIFY_TOKEN_REFRESHED_AT_FILE"` + "`" + ` after
    a successful exchange.
  - SHOPIFY_TOKEN_REFRESHED_AT: RFC3339 timestamp string of the last rotation.

Tokens older than 20 hours warn (Shopify access tokens for the
admin API typically have a 24h validity window when issued via
client_credentials with the standard scope set).`,
		Example: "  shopify-pp-cli auth audit --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			status := AuthStatus{
				StaleThresholdH: 20,
				RefresherHint:   strings.TrimSpace(os.Getenv("SHOPIFY_REFRESHER_HINT")),
			}
			ok := true

			if v := strings.TrimSpace(os.Getenv("SHOPIFY_CLIENT_ID")); v == "" {
				status.Checks = append(status.Checks, AuthCheck{
					Name: "SHOPIFY_CLIENT_ID", Status: "warn",
					Detail: "not set; required for OAuth client_credentials rotation. If you rotate tokens externally and only set SHOPIFY_ACCESS_TOKEN, this is informational.",
				})
			} else {
				status.Checks = append(status.Checks, AuthCheck{
					Name: "SHOPIFY_CLIENT_ID", Status: "ok",
					Detail: "set",
				})
			}

			if v := strings.TrimSpace(os.Getenv("SHOPIFY_CLIENT_SECRET")); v == "" {
				status.Checks = append(status.Checks, AuthCheck{
					Name: "SHOPIFY_CLIENT_SECRET", Status: "warn",
					Detail: "not set; required for OAuth client_credentials rotation.",
				})
			} else {
				status.Checks = append(status.Checks, AuthCheck{
					Name: "SHOPIFY_CLIENT_SECRET", Status: "ok",
					Detail: "set (value not printed)",
				})
			}

			tok := strings.TrimSpace(os.Getenv("SHOPIFY_ACCESS_TOKEN"))
			if tok == "" {
				status.Checks = append(status.Checks, AuthCheck{
					Name: "SHOPIFY_ACCESS_TOKEN", Status: "fail",
					Detail: "not set; the CLI cannot reach the Shopify Admin API without this. Run your refresher script.",
				})
				ok = false
			} else {
				status.Checks = append(status.Checks, AuthCheck{
					Name: "SHOPIFY_ACCESS_TOKEN", Status: "ok",
					Detail: fmt.Sprintf("set (length %d, value not printed)", len(tok)),
				})
			}

			// Freshness: try file-mtime first, then env timestamp, then "unknown".
			var refreshedAt time.Time
			var ageSource string
			if path := strings.TrimSpace(os.Getenv("SHOPIFY_TOKEN_REFRESHED_AT_FILE")); path != "" {
				if fi, err := os.Stat(path); err == nil {
					refreshedAt = fi.ModTime()
					ageSource = "file mtime: " + path
				}
			}
			if refreshedAt.IsZero() {
				if ts := strings.TrimSpace(os.Getenv("SHOPIFY_TOKEN_REFRESHED_AT")); ts != "" {
					if t, err := time.Parse(time.RFC3339, ts); err == nil {
						refreshedAt = t
						ageSource = "env SHOPIFY_TOKEN_REFRESHED_AT"
					}
				}
			}

			if refreshedAt.IsZero() {
				status.Checks = append(status.Checks, AuthCheck{
					Name: "token_freshness", Status: "info",
					Detail: "no rotation timestamp available. Set SHOPIFY_TOKEN_REFRESHED_AT_FILE or SHOPIFY_TOKEN_REFRESHED_AT in your refresher script to enable staleness checks.",
				})
			} else {
				age := time.Since(refreshedAt)
				ageHours := age.Hours()
				status.TokenAgeHours = round2(ageHours)
				if ageHours > status.StaleThresholdH {
					status.Checks = append(status.Checks, AuthCheck{
						Name: "token_freshness", Status: "warn",
						Detail: fmt.Sprintf("token is %.1fh old (source: %s); threshold is %.0fh. Run your refresher script.", ageHours, ageSource, status.StaleThresholdH),
					})
				} else {
					status.Checks = append(status.Checks, AuthCheck{
						Name: "token_freshness", Status: "ok",
						Detail: fmt.Sprintf("token is %.1fh old (source: %s); under %.0fh threshold.", ageHours, ageSource, status.StaleThresholdH),
					})
				}
			}

			status.OK = ok
			b, _ := json.Marshal(status)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	return cmd
}
