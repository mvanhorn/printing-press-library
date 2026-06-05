// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.
// Hand-written transcendence command. Not generated.

package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/dataforseo/internal/store"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const backlinksSnapshotSchema = `CREATE TABLE IF NOT EXISTS backlinks_snapshot (
  domain TEXT,
  referring_domain TEXT,
  anchor TEXT,
  first_seen_at TIMESTAMP,
  PRIMARY KEY (domain, referring_domain)
)`

type backlinkNewRow struct {
	Domain          string `json:"domain"`
	ReferringDomain string `json:"referring_domain"`
	Anchor          string `json:"anchor,omitempty"`
	FirstSeenAt     string `json:"first_seen_at"`
}

// newBacklinksNewCmd is wired into the existing backlinks group from root.go.
func newBacklinksNewCmd(flags *rootFlags) *cobra.Command {
	var domain string

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Surface newly-acquired referring domains vs the prior snapshot",
		Long: `Calls backlinks/summary + backlinks/referring_domains for the target domain,
diffs the referring_domains list against the prior snapshot in local SQLite,
and outputs the new entries with their anchor text.

First run populates the snapshot and reports every referring domain as new;
subsequent runs report only the deltas.`,
		Example: strings.Trim(`
  dataforseo-pp-cli backlinks new --domain floridatreemen.com
  dataforseo-pp-cli backlinks new --domain fsmstumpgrinding.com --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would snapshot backlinks and diff against prior run")
				return nil
			}
			if domain == "" {
				return usageErr(fmt.Errorf("--domain is required"))
			}

			ctx := context.Background()
			s, err := store.OpenWithContext(ctx, defaultDBPath("dataforseo-pp-cli"))
			if err != nil {
				return err
			}
			defer s.Close()
			if _, err := s.DB().Exec(backlinksSnapshotSchema); err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body := []map[string]any{{"target": domain, "limit": 1000}}
			respRaw, _, err := c.Post("/v3/backlinks/referring_domains/live", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			refDomains := extractReferringDomains(respRaw)

			// Pull the existing set
			existing := map[string]bool{}
			rows, err := s.DB().Query(`SELECT referring_domain FROM backlinks_snapshot WHERE domain = ?`, domain)
			if err != nil {
				return err
			}
			for rows.Next() {
				var r string
				if err := rows.Scan(&r); err == nil {
					existing[r] = true
				}
			}
			rows.Close()

			now := time.Now().UTC().Format(time.RFC3339)
			out := make([]backlinkNewRow, 0)
			for _, rd := range refDomains {
				if existing[rd.domain] {
					continue
				}
				_, _ = s.DB().Exec(
					`INSERT OR IGNORE INTO backlinks_snapshot (domain, referring_domain, anchor, first_seen_at) VALUES (?, ?, ?, ?)`,
					domain, rd.domain, rd.anchor, now,
				)
				out = append(out, backlinkNewRow{
					Domain:          domain,
					ReferringDomain: rd.domain,
					Anchor:          rd.anchor,
					FirstSeenAt:     now,
				})
			}

			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&domain, "domain", "", "Target domain (e.g. floridatreemen.com)")
	return cmd
}

type refDomain struct {
	domain string
	anchor string
}

func extractReferringDomains(raw json.RawMessage) []refDomain {
	var resp struct {
		Tasks []struct {
			Result []struct {
				Items []struct {
					Domain string `json:"domain"`
					Anchor string `json:"anchor"`
				} `json:"items"`
			} `json:"result"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil
	}
	var out []refDomain
	for _, t := range resp.Tasks {
		for _, r := range t.Result {
			for _, item := range r.Items {
				if item.Domain != "" {
					out = append(out, refDomain{domain: item.Domain, anchor: item.Anchor})
				}
			}
		}
	}
	return out
}
