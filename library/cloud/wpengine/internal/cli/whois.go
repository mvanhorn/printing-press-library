// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: reverse domain lookup across the synced fleet.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// whoisMiss is the structured not-found envelope for JSON/agent mode. It
// echoes the query on stdout so JSON consumers can tell WHAT was not found
// without parsing stderr, on both the missing-mirror and zero-hits paths.
type whoisMiss struct {
	Query   string      `json:"query"`
	Found   bool        `json:"found"`
	Matches []whoisView `json:"matches"`
	Hint    string      `json:"hint"`
}

type whoisView struct {
	Domain          string   `json:"domain"`
	DomainID        string   `json:"domain_id"`
	Primary         bool     `json:"primary"`
	OwnershipStatus string   `json:"ownership_status,omitempty"`
	TxtVerified     bool     `json:"txt_verified"`
	RedirectsTo     []string `json:"redirects_to,omitempty"`
	Install         string   `json:"install"`
	InstallID       string   `json:"install_id"`
	Environment     string   `json:"environment,omitempty"`
	PrimaryDomain   string   `json:"install_primary_domain,omitempty"`
	PHPVersion      string   `json:"php_version,omitempty"`
	WPVersion       string   `json:"wp_version,omitempty"`
	Site            string   `json:"site,omitempty"`
	SiteID          string   `json:"site_id,omitempty"`
	Account         string   `json:"account,omitempty"`
	AccountID       string   `json:"account_id,omitempty"`
	CertStatus      string   `json:"cert_status"`
	CertCommonName  string   `json:"cert_common_name,omitempty"`
	CertExpires     string   `json:"cert_expires,omitempty"`
}

func newNovelWhoisCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "whois <domain>",
		Short: "Resolve any domain name to the install, site, and account that serve it, plus cert and redirect status",
		Long: strings.Trim(`
Use this command to resolve one domain name to the install, site, and account
that serve it, plus its cert coverage and redirect status — the first command
to run when a support ticket names a domain.

Do NOT use it for fleet-wide domain health scans; use 'audit domains' instead.

Requires a synced mirror: run 'wpengine-pp-cli sync' first.
`, "\n"),
		Example: strings.Trim(`
  # Which install, site, and account serve this domain?
  wpengine-pp-cli whois clientsite.com

  # Agent-friendly JSON with narrowed fields
  wpengine-pp-cli whois clientsite.com --agent --select install,account,environment,cert_status
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve the domain against the local mirror")
				return nil
			}
			if err := rejectLiveDataSource(flags); err != nil {
				return err
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a domain name is required"))
			}
			query := strings.ToLower(strings.TrimSpace(args[0]))
			query = strings.TrimPrefix(query, "https://")
			query = strings.TrimPrefix(query, "http://")
			query = strings.TrimSuffix(strings.TrimSuffix(query, "/"), ".")
			if query == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a domain name is required"))
			}

			missPayload, err := json.Marshal(whoisMiss{
				Query: query, Found: false, Matches: []whoisView{},
				Hint: "run 'wpengine-pp-cli sync' if the mirror is stale",
			})
			if err != nil {
				return fmt.Errorf("encoding not-found envelope: %w", err)
			}

			db, handled, err := openAuditStore(cmd, flags, dbPath, "domains", string(missPayload))
			if err != nil || handled {
				return err
			}
			defer db.Close()

			ctx := cmd.Context()

			// Drain matching domains joined to install identity first.
			type rawHit struct {
				domainID, installID, installName, environment, primaryDomain, php, wp string
				domainData, installData                                               []byte
			}
			rows, err := db.DB().QueryContext(ctx, `
				SELECT d.id, d.installs_id, d.data,
				       COALESCE(i.name, ''), COALESCE(i.environment, ''),
				       COALESCE(i.primary_domain, ''), COALESCE(i.php_version, ''),
				       COALESCE(i.wp_version, ''), COALESCE(i.data, '{}')
				FROM domains d
				LEFT JOIN installs i ON i.id = d.installs_id
				WHERE LOWER(json_extract(d.data, '$.name')) = ?`, query)
			if err != nil {
				return fmt.Errorf("querying domains: %w", err)
			}
			hits := make([]rawHit, 0)
			if err := drainRows(rows, "domain", func(rs *sql.Rows) error {
				var h rawHit
				if err := rs.Scan(&h.domainID, &h.installID, &h.domainData, &h.installName, &h.environment, &h.primaryDomain, &h.php, &h.wp, &h.installData); err != nil {
					return err
				}
				hits = append(hits, h)
				return nil
			}); err != nil {
				return err
			}

			if len(hits) == 0 {
				if flags.asJSON || flags.agent {
					// Exit 0 is deliberate — an empty lookup result is data,
					// not an error, in agent mode.
					fmt.Fprintln(cmd.OutOrStdout(), string(missPayload))
					fmt.Fprintf(cmd.ErrOrStderr(), "domain %q not found in the local mirror; run 'wpengine-pp-cli sync' if the mirror is stale\n", query)
					return nil
				}
				return notFoundErr(fmt.Errorf("domain %q not found in the local mirror (run 'wpengine-pp-cli sync' if stale)", query))
			}

			// Rows are drained; follow-up lookups are safe now.
			results := make([]whoisView, 0, len(hits))
			for _, h := range hits {
				var d struct {
					Primary           bool            `json:"primary"`
					OwnershipStatus   string          `json:"ownership_status"`
					TxtVerifiedDomain string          `json:"txt_verified_domain"`
					RedirectsTo       json.RawMessage `json:"redirects_to"`
				}
				_ = json.Unmarshal(h.domainData, &d)
				var inst struct {
					Site struct {
						ID string `json:"id"`
					} `json:"site"`
					Account struct {
						ID string `json:"id"`
					} `json:"account"`
				}
				_ = json.Unmarshal(h.installData, &inst)

				v := whoisView{
					Domain: query, DomainID: h.domainID, Primary: d.Primary,
					OwnershipStatus: d.OwnershipStatus,
					TxtVerified:     d.TxtVerifiedDomain != "",
					RedirectsTo:     decodeRedirectTargets(d.RedirectsTo),
					Install:         h.installName, InstallID: h.installID,
					Environment: h.environment, PrimaryDomain: h.primaryDomain,
					PHPVersion: h.php, WPVersion: h.wp,
					SiteID: inst.Site.ID, AccountID: inst.Account.ID,
					CertStatus: "uncovered",
				}

				if inst.Site.ID != "" {
					var siteName string
					_ = db.DB().QueryRowContext(ctx, `SELECT COALESCE(name, '') FROM sites WHERE id = ?`, inst.Site.ID).Scan(&siteName)
					v.Site = siteName
				}
				if inst.Account.ID != "" {
					var acctName string
					_ = db.DB().QueryRowContext(ctx, `SELECT COALESCE(name, '') FROM accounts WHERE id = ?`, inst.Account.ID).Scan(&acctName)
					v.Account = acctName
				}

				// Cert coverage: drain the install's certs, then match.
				// Best-effort enrichment: a failed cert lookup degrades to
				// cert_status "uncovered" rather than failing the lookup.
				crows, err := db.DB().QueryContext(ctx, `SELECT data FROM ssl_certificates WHERE installs_id = ?`, h.installID)
				if err == nil {
					type certInfo struct {
						CommonName  string   `json:"common_name"`
						Domains     []string `json:"domains"`
						ExpiresTime string   `json:"expires_time"`
						Status      string   `json:"status"`
					}
					certs := make([]certInfo, 0)
					_ = drainRows(crows, "certificate", func(rs *sql.Rows) error {
						var data []byte
						if err := rs.Scan(&data); err != nil {
							return err
						}
						var c certInfo
						if json.Unmarshal(data, &c) == nil {
							certs = append(certs, c)
						}
						return nil
					})
					for _, c := range certs {
						names := append([]string{c.CommonName}, c.Domains...)
						for _, n := range names {
							if certNameCovers(n, query) {
								v.CertStatus = "covered"
								if c.Status != "" {
									v.CertStatus = c.Status
								}
								v.CertCommonName = c.CommonName
								if exp, ok := parseAPITime(c.ExpiresTime); ok {
									v.CertExpires = exp.UTC().Format("2006-01-02")
								}
								break
							}
						}
						if v.CertStatus != "uncovered" {
							break
						}
					}
				}
				results = append(results, v)
			}

			if len(results) == 1 && !flags.csv {
				return printJSONFiltered(cmd.OutOrStdout(), results[0], flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local data dir)")
	return cmd
}
