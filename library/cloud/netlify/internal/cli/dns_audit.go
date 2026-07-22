// Copyright 2026 Charles Denzel Segovia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: account-wide DNS audit.

package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type danglingRecord struct {
	Zone     string `json:"zone"`
	Hostname string `json:"hostname"`
	Type     string `json:"type"`
	Value    string `json:"value"`
	Reason   string `json:"reason"`
}

type expiringCert struct {
	Domain    string `json:"domain"`
	ExpiresAt string `json:"expires_at"`
	DaysLeft  int    `json:"days_left"`
}

type dnsAuditView struct {
	Zones        int              `json:"zones"`
	Records      int              `json:"records"`
	Dangling     []danglingRecord `json:"dangling"`
	ExpiringCert []expiringCert   `json:"expiring_certs"`
}

// pp:data-source local
func newNovelDnsAuditCmd(flags *rootFlags) *cobra.Command {
	var certDays int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "dns-audit",
		Short: "Audit DNS across all zones: flag records pointing at missing sites and expiring SNI certs.",
		Long: `Join DNS zones, DNS records, sites, and SNI certificates from the local mirror
to find NETLIFY-type records whose target site no longer exists and certificates
expiring within --cert-days. Run 'sync' first to populate DNS locally.`,
		Example:     "  netlify-pp-cli dns-audit --json --cert-days 30",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = novelDBPath()
			}
			if mirrorMissing(dbPath) {
				return noMirror(cmd, flags, dbPath, "dns-zones,sites", dnsAuditView{
					Dangling:     []danglingRecord{},
					ExpiringCert: []expiringCert{},
				})
			}
			st, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			zones := loadTyped(st, "dns-zones")
			records := loadTyped(st, "dns-records", "dns-zones-dns-records", "dns-zones-records")
			sites := loadTyped(st, "sites")
			certs := loadTyped(st, "sni-certificates", "sites-ssl", "ssl")

			// Build the set of known site names/domains.
			known := map[string]bool{}
			for _, s := range sites {
				for _, v := range []string{
					firstStr(s, "name"), firstStr(s, "custom_domain"),
				} {
					if v != "" {
						known[strings.ToLower(v)] = true
					}
				}
				if aliases, ok := s["domain_aliases"].([]any); ok {
					for _, a := range aliases {
						if as, ok := a.(string); ok && as != "" {
							known[strings.ToLower(as)] = true
						}
					}
				}
			}

			zoneName := map[string]string{}
			for _, z := range zones {
				zoneName[firstStr(z, "id")] = firstStr(z, "name")
			}

			dangling := make([]danglingRecord, 0)
			// Only flag when we actually know some sites; otherwise every record
			// would look dangling and produce noise.
			checkDangling := len(known) > 0
			for _, r := range records {
				rtype := strings.ToUpper(firstStr(r, "type"))
				val := firstStr(r, "value", "hostname")
				if checkDangling && rtype == "NETLIFY" && val != "" && !known[strings.ToLower(val)] {
					zn := zoneName[firstStr(r, "dns_zone_id", "zone_id")]
					if zn == "" {
						zn = firstStr(r, "dns_zone_id", "zone_id")
					}
					dangling = append(dangling, danglingRecord{
						Zone:     zn,
						Hostname: firstStr(r, "hostname"),
						Type:     rtype,
						Value:    val,
						Reason:   "NETLIFY record targets a site not present in the local mirror",
					})
				}
			}
			sort.Slice(dangling, func(i, j int) bool { return dangling[i].Hostname < dangling[j].Hostname })

			now := time.Now()
			expiring := make([]expiringCert, 0)
			for _, c := range certs {
				exp := firstStr(c, "expires_at", "expiry", "not_after")
				if exp == "" {
					continue
				}
				t, ok := parseTimeLoose(exp)
				if !ok {
					continue
				}
				days := int(t.Sub(now).Hours() / 24)
				if !t.Before(now) && days <= certDays {
					domain := firstStr(c, "domain")
					if domain == "" {
						if ds, ok := c["domains"].([]any); ok && len(ds) > 0 {
							if s, ok := ds[0].(string); ok {
								domain = s
							}
						}
					}
					expiring = append(expiring, expiringCert{Domain: domain, ExpiresAt: exp, DaysLeft: days})
				}
			}
			sort.Slice(expiring, func(i, j int) bool { return expiring[i].DaysLeft < expiring[j].DaysLeft })

			view := dnsAuditView{
				Zones:        len(zones),
				Records:      len(records),
				Dangling:     dangling,
				ExpiringCert: expiring,
			}
			if len(zones) == 0 && len(records) == 0 {
				cmd.PrintErrln("no DNS data in local mirror; run: netlify-pp-cli sync --resources dns-zones")
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&certDays, "cert-days", 30, "flag SNI certificates expiring within this many days")
	cmd.Flags().StringVar(&dbPath, "db", "", "local mirror path (default: standard data dir)")
	return cmd
}

// parseTimeLoose accepts RFC3339 and a couple of common date shapes.
func parseTimeLoose(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05Z07:00", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
