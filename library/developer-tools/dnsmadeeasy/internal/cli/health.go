// Copyright 2026 Derick Ng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: DNS-hygiene audit across all zones from the local mirror.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type healthFinding struct {
	Zone     string `json:"zone"`
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	Detail   string `json:"detail"`
	RecordID string `json:"record_id,omitempty"`
}

type healthView struct {
	ScannedZones int             `json:"scanned_zones"`
	ScannedRecs  int             `json:"scanned_records"`
	Findings     []healthFinding `json:"findings"`
	Note         string          `json:"note,omitempty"`
}

func newNovelHealthCmd(flags *rootFlags) *cobra.Command {
	var flagZone string
	var minTTL, maxTTL int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Audit all zones for dangling CNAMEs, bad TTLs, missing SPF/DMARC/CAA, and duplicates",
		Long: strings.Trim(`
Run mechanical DNS-hygiene checks across every zone in the local mirror:
  - CNAME records pointing into a managed zone with no matching target (dangling)
  - TTLs outside the sane band (--min-ttl / --max-ttl)
  - mail-enabled zones (with MX) missing an SPF or DMARC record
  - zones missing a CAA record
  - duplicate records (same name/type/value)

Run 'dnsmadeeasy-pp-cli sync-records' first to populate the mirror.`, "\n"),
		Example:     "  dnsmadeeasy-pp-cli health --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Reject an inverted TTL band before doing any work: with
			// minTTL > maxTTL every positive-TTL record satisfies
			// (ttl < minTTL || ttl > maxTTL), so the audit would flag the whole
			// corpus as out-of-band with no hint the flags are transposed.
			if minTTL > maxTTL {
				return usageErr(fmt.Errorf("--min-ttl (%d) cannot exceed --max-ttl (%d)", minTTL, maxTTL))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would audit all zones in the local mirror")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("dnsmadeeasy-pp-cli")
			}
			s, ok, err := openZoneMirror(ctx, cmd, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), healthView{
						Findings: []healthFinding{},
						Note:     "no local record mirror yet; run 'dnsmadeeasy-pp-cli sync-records' first",
					}, flags)
				}
				return nil
			}
			defer s.Close()

			recs, err := loadZoneRecords(ctx, s)
			if err != nil {
				return fmt.Errorf("reading zone mirror: %w", err)
			}

			// Group by zone; build an FQDN set for dangling-CNAME detection.
			byZone := map[string][]dmeRecord{}
			zoneNames := map[string]bool{}
			fqdnSet := map[string]bool{}
			scannedRecords := 0
			for _, r := range recs {
				if flagZone != "" && !strings.EqualFold(r.DomainName, strings.TrimSuffix(flagZone, ".")) {
					continue
				}
				scannedRecords++
				byZone[r.DomainName] = append(byZone[r.DomainName], r)
				zoneNames[strings.ToLower(r.DomainName)] = true
				fqdnSet[strings.ToLower(recordFQDN(r))] = true
			}

			findings := []healthFinding{}
			for zone, zrecs := range byZone {
				hasMX := false
				hasSPF := false
				hasDMARC := false
				hasCAA := false
				seen := map[string]int{}
				for _, r := range zrecs {
					typ := strings.ToUpper(r.Type)
					switch typ {
					case "MX":
						hasMX = true
					case "TXT", "SPF":
						if strings.Contains(strings.ToLower(r.Value), "v=spf1") {
							hasSPF = true
						}
						if hasDMARCPolicy(r) {
							hasDMARC = true
						}
					case "CAA":
						hasCAA = true
					}
					// TTL band
					if r.TTL > 0 && (r.TTL < minTTL || r.TTL > maxTTL) {
						findings = append(findings, healthFinding{Zone: zone, Severity: "warning", Rule: "ttl-out-of-band",
							Detail: fmt.Sprintf("%s %s TTL=%d outside [%d,%d]", nameOrApex(r.Name), typ, r.TTL, minTTL, maxTTL), RecordID: r.ID.String()})
					}
					// Dangling CNAME into a managed zone
					if typ == "CNAME" {
						target := strings.ToLower(strings.TrimSuffix(r.Value, "."))
						if pointsIntoManagedZone(target, zoneNames) && !fqdnSet[target] {
							findings = append(findings, healthFinding{Zone: zone, Severity: "error", Rule: "dangling-cname",
								Detail: fmt.Sprintf("%s -> %s (target not found in any managed zone)", nameOrApex(r.Name), r.Value), RecordID: r.ID.String()})
						}
					}
					key := strings.ToLower(r.Name + "|" + typ + "|" + r.Value)
					seen[key]++
				}
				for key, count := range seen {
					if count > 1 {
						parts := strings.SplitN(key, "|", 3)
						findings = append(findings, healthFinding{Zone: zone, Severity: "warning", Rule: "duplicate-record",
							Detail: fmt.Sprintf("%s %s %s appears %d times", nameOrApex(parts[0]), strings.ToUpper(parts[1]), parts[2], count)})
					}
				}
				if hasMX && !hasSPF {
					findings = append(findings, healthFinding{Zone: zone, Severity: "warning", Rule: "missing-spf", Detail: "zone has MX but no SPF (v=spf1) record"})
				}
				if hasMX && !hasDMARC {
					findings = append(findings, healthFinding{Zone: zone, Severity: "warning", Rule: "missing-dmarc", Detail: "zone has MX but no DMARC (_dmarc) record"})
				}
				if !hasCAA {
					findings = append(findings, healthFinding{Zone: zone, Severity: "info", Rule: "missing-caa", Detail: "zone has no CAA record"})
				}
			}

			sort.SliceStable(findings, func(i, j int) bool {
				if findings[i].Zone != findings[j].Zone {
					return findings[i].Zone < findings[j].Zone
				}
				return findings[i].Rule < findings[j].Rule
			})

			view := healthView{ScannedZones: len(byZone), ScannedRecs: scannedRecords, Findings: findings}
			if len(findings) == 0 {
				view.Note = "no hygiene issues found across the scanned zones"
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			for _, f := range findings {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s: %s — %s\n", strings.ToUpper(f.Severity), f.Zone, f.Rule, f.Detail)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d finding(s) across %d zone(s).\n", len(findings), len(byZone))
			return nil
		},
	}
	cmd.Flags().StringVar(&flagZone, "zone", "", "Audit only this zone")
	cmd.Flags().IntVar(&minTTL, "min-ttl", 30, "Minimum acceptable TTL (seconds)")
	cmd.Flags().IntVar(&maxTTL, "max-ttl", 604800, "Maximum acceptable TTL (seconds)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local mirror database")
	return cmd
}

func hasDMARCPolicy(r dmeRecord) bool {
	if !strings.EqualFold(r.Type, "TXT") || !strings.EqualFold(r.Name, "_dmarc") {
		return false
	}
	value := strings.ToLower(strings.TrimSpace(txtRawContent(r.Value)))
	if !strings.HasPrefix(value, "v=dmarc1") {
		return false
	}
	remainder := strings.TrimSpace(strings.TrimPrefix(value, "v=dmarc1"))
	return remainder == "" || strings.HasPrefix(remainder, ";")
}

func recordFQDN(r dmeRecord) string {
	zone := strings.TrimSuffix(r.DomainName, ".")
	if r.Name == "" || r.Name == "@" {
		return zone
	}
	return r.Name + "." + zone
}

func nameOrApex(name string) string {
	if name == "" {
		return "@"
	}
	return name
}

func pointsIntoManagedZone(target string, zones map[string]bool) bool {
	for z := range zones {
		if target == z || strings.HasSuffix(target, "."+z) {
			return true
		}
	}
	return false
}
