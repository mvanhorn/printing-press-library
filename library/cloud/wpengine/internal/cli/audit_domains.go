// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: fleet-wide domain health audit.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type domainIssueRow struct {
	Domain      string   `json:"domain"`
	DomainID    string   `json:"domain_id"`
	Install     string   `json:"install"`
	InstallID   string   `json:"install_id"`
	Environment string   `json:"environment,omitempty"`
	Primary     bool     `json:"primary,omitempty"`
	Detail      string   `json:"detail,omitempty"`
	Targets     []string `json:"targets,omitempty"`
}

type auditDomainsView struct {
	Unverified        []domainIssueRow `json:"unverified"`
	MissingCert       []domainIssueRow `json:"missing_cert"`
	DanglingRedirects []domainIssueRow `json:"dangling_redirects"`
	Duplicates        []domainIssueRow `json:"duplicates"`
	ScannedDomains    int              `json:"scanned_domains"`
}

// platformDomainSuffixes are WP Engine-managed hostnames that get platform
// TLS automatically; reporting them as "missing cert" is noise.
var platformDomainSuffixes = []string{".wpengine.com", ".wpenginepowered.com"}

func isPlatformDomain(name string) bool {
	n := strings.ToLower(name)
	for _, sfx := range platformDomainSuffixes {
		if strings.HasSuffix(n, sfx) {
			return true
		}
	}
	return false
}

func newNovelAuditDomainsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var flagIncludePlatform bool

	cmd := &cobra.Command{
		Use:   "domains",
		Short: "One list of everything blocked across launches: unverified domains, missing cert coverage, dangling redirects",
		Long: strings.Trim(`
Use this command for fleet-wide domain health. It joins synced domains,
SSL certificates, and installs in the local mirror and reports:

  - unverified: domains with a pending TXT verification (key issued, not verified)
  - missing_cert: domains no certificate on their install covers (CN/SAN/wildcard match)
  - dangling_redirects: redirects pointing at domain names that no longer exist in the fleet
  - duplicates: domains the API flags as duplicated across installs

Do NOT use it to look up which install serves one domain; use 'whois' instead.
Do NOT use it for cert expiry windows; use 'audit certs' instead.

Requires a synced mirror: run 'wpengine-pp-cli sync' first.
`, "\n"),
		Example: strings.Trim(`
  # Full domain health report
  wpengine-pp-cli audit domains

  # Agent-friendly JSON, only the unverified section
  wpengine-pp-cli audit domains --agent --select unverified
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan local mirror for unverified domains, missing cert coverage, and dangling redirects")
				return nil
			}
			if err := rejectLiveDataSource(flags); err != nil {
				return err
			}
			db, handled, err := openAuditStore(cmd, flags, dbPath, "domains", "{}")
			if err != nil || handled {
				return err
			}
			defer db.Close()

			ctx := cmd.Context()

			// Drain domains joined to install identity.
			type rawDomain struct {
				id, installID, installName, environment string
				data                                    []byte
			}
			drows, err := db.DB().QueryContext(ctx, `
				SELECT d.id, d.installs_id, d.data,
				       COALESCE(i.name, ''), COALESCE(i.environment, '')
				FROM domains d
				LEFT JOIN installs i ON i.id = d.installs_id`)
			if err != nil {
				return fmt.Errorf("querying domains: %w", err)
			}
			doms := make([]rawDomain, 0)
			if err := drainRows(drows, "domain", func(rs *sql.Rows) error {
				var r rawDomain
				if err := rs.Scan(&r.id, &r.installID, &r.data, &r.installName, &r.environment); err != nil {
					return err
				}
				doms = append(doms, r)
				return nil
			}); err != nil {
				return err
			}

			// Drain certificates per install.
			crows, err := db.DB().QueryContext(ctx, `SELECT installs_id, data FROM ssl_certificates`)
			if err != nil {
				return fmt.Errorf("querying certificates: %w", err)
			}
			certNames := map[string][]string{} // install_id -> CN + SAN entries
			if err := drainRows(crows, "certificate", func(rs *sql.Rows) error {
				var installID string
				var data []byte
				if err := rs.Scan(&installID, &data); err != nil {
					return err
				}
				var c struct {
					CommonName string   `json:"common_name"`
					Domains    []string `json:"domains"`
				}
				if err := json.Unmarshal(data, &c); err != nil {
					return nil // malformed row: skip, don't fail the audit
				}
				if c.CommonName != "" {
					certNames[installID] = append(certNames[installID], c.CommonName)
				}
				certNames[installID] = append(certNames[installID], c.Domains...)
				return nil
			}); err != nil {
				return err
			}

			// Fleet-wide set of known domain names for dangling-redirect checks.
			fleet := map[string]bool{}
			type parsedDomain struct {
				raw        rawDomain
				name       string
				primary    bool
				duplicate  bool
				pendingTXT bool
				redirects  []string
			}
			parsed := make([]parsedDomain, 0, len(doms))
			for _, r := range doms {
				var d struct {
					Name               string          `json:"name"`
					Primary            bool            `json:"primary"`
					Duplicate          bool            `json:"duplicate"`
					RedirectsTo        json.RawMessage `json:"redirects_to"`
					TxtVerificationKey string          `json:"txt_verification_key"`
					TxtVerifiedDomain  string          `json:"txt_verified_domain"`
				}
				if err := json.Unmarshal(r.data, &d); err != nil || d.Name == "" {
					continue
				}
				p := parsedDomain{
					raw: r, name: d.Name, primary: d.Primary, duplicate: d.Duplicate,
					pendingTXT: d.TxtVerificationKey != "" && d.TxtVerifiedDomain == "",
					redirects:  decodeRedirectTargets(d.RedirectsTo),
				}
				fleet[strings.ToLower(d.Name)] = true
				parsed = append(parsed, p)
			}

			view := auditDomainsView{
				Unverified:        make([]domainIssueRow, 0),
				MissingCert:       make([]domainIssueRow, 0),
				DanglingRedirects: make([]domainIssueRow, 0),
				Duplicates:        make([]domainIssueRow, 0),
				ScannedDomains:    len(parsed),
			}
			for _, p := range parsed {
				base := domainIssueRow{
					Domain: p.name, DomainID: p.raw.id,
					Install: p.raw.installName, InstallID: p.raw.installID,
					Environment: p.raw.environment, Primary: p.primary,
				}
				if p.pendingTXT {
					row := base
					row.Detail = "TXT verification key issued but domain not verified yet"
					view.Unverified = append(view.Unverified, row)
				}
				covered := false
				for _, cn := range certNames[p.raw.installID] {
					if certNameCovers(cn, p.name) {
						covered = true
						break
					}
				}
				if !covered && (flagIncludePlatform || !isPlatformDomain(p.name)) {
					row := base
					row.Detail = "no certificate on this install covers this domain (CN/SAN/wildcard checked)"
					view.MissingCert = append(view.MissingCert, row)
				}
				var dangling []string
				for _, target := range p.redirects {
					if target != "" && !fleet[strings.ToLower(target)] {
						dangling = append(dangling, target)
					}
				}
				if len(dangling) > 0 {
					row := base
					row.Detail = "redirect targets not found anywhere in the fleet"
					row.Targets = dangling
					view.DanglingRedirects = append(view.DanglingRedirects, row)
				}
				if p.duplicate {
					row := base
					row.Detail = "API flags this domain as duplicated across installs"
					view.Duplicates = append(view.Duplicates, row)
				}
			}
			for _, s := range [][]domainIssueRow{view.Unverified, view.MissingCert, view.DanglingRedirects, view.Duplicates} {
				sort.Slice(s, func(i, j int) bool { return s[i].Domain < s[j].Domain })
			}

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local data dir)")
	cmd.Flags().BoolVar(&flagIncludePlatform, "include-platform", false, "Also report *.wpengine.com / *.wpenginepowered.com hostnames in missing_cert (platform TLS covers them; excluded by default)")
	return cmd
}
