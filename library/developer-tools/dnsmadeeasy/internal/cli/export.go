// Copyright 2026 Derick Ng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: serialize a zone's records from the local mirror to a BIND
// zone file or JSON, with no API call.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelExportCmd(flags *rootFlags) *cobra.Command {
	var flagFormat string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "export <zone>",
		Short: "Serialize a zone's records from the local mirror to a BIND zone file or JSON",
		Long: strings.Trim(`
Read a zone's records from the local mirror and serialize them to a
standards-compliant BIND zone file (default) or JSON. No API call is made, so
export is instant and never spends rate-limit budget.

Run 'dnsmadeeasy-pp-cli sync-records' first to populate the mirror.`, "\n"),
		Example:     "  dnsmadeeasy-pp-cli export example.com --format bind",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would export a zone from the local mirror")
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a zone name is required, e.g. example.com"))
			}
			zone := strings.TrimSuffix(args[0], ".")
			switch flagFormat {
			case "", "bind", "json":
			default:
				return usageErr(fmt.Errorf("--format must be 'bind' or 'json', got %q", flagFormat))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("dnsmadeeasy-pp-cli")
			}
			s, ok, err := openZoneMirror(ctx, cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer s.Close()

			all, err := loadZoneRecords(ctx, s)
			if err != nil {
				return fmt.Errorf("reading zone mirror: %w", err)
			}
			var zoneRecs []dmeRecord
			for _, r := range all {
				if strings.EqualFold(r.DomainName, zone) {
					zoneRecs = append(zoneRecs, r)
				}
			}
			if len(zoneRecs) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no records for zone %q in the local mirror; check the zone name or re-run 'dnsmadeeasy-pp-cli sync-records'\n", zone)
				return nil
			}

			if flagFormat == "json" || flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), zoneRecs, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "$ORIGIN %s.\n", zone)
			fmt.Fprintf(out, "; exported from the local DNS Made Easy mirror\n")
			sort.SliceStable(zoneRecs, func(i, j int) bool {
				if zoneRecs[i].Name != zoneRecs[j].Name {
					return zoneRecs[i].Name < zoneRecs[j].Name
				}
				return zoneRecs[i].Type < zoneRecs[j].Type
			})
			for _, r := range zoneRecs {
				fmt.Fprintln(out, bindRecordLine(r))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "bind", "Output format: bind or json")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local mirror database")
	return cmd
}

// bindRecordLine renders one record as a BIND master-file line. DNS Made Easy
// stores MX preference in mxLevel and SRV priority/weight/port in dedicated
// fields with the target in value.
func bindRecordLine(r dmeRecord) string {
	name := r.Name
	if name == "" {
		name = "@"
	}
	ttl := r.TTL
	if ttl <= 0 {
		ttl = 1800
	}
	typ := strings.ToUpper(r.Type)
	prefix := fmt.Sprintf("%s\t%d\tIN\t%s\t", name, ttl, typ)
	switch typ {
	case "MX":
		return prefix + fmt.Sprintf("%d %s", r.MxLevel, ensureTrailingDot(r.Value))
	case "SRV":
		return prefix + fmt.Sprintf("%d %d %d %s", r.Priority, r.Weight, r.Port, ensureTrailingDot(r.Value))
	case "TXT", "SPF":
		return prefix + strconv.Quote(r.Value)
	case "CNAME", "ANAME", "NS", "PTR":
		return prefix + ensureTrailingDot(r.Value)
	default:
		return prefix + r.Value
	}
}

func ensureTrailingDot(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return host
	}
	// Leave IPs and already-qualified names alone; only append a dot to
	// hostname targets that look like FQDNs without one.
	if strings.ContainsAny(host, ":") || isNumericIP(host) {
		return host
	}
	if strings.HasSuffix(host, ".") {
		return host
	}
	if strings.Contains(host, ".") {
		return host + "."
	}
	return host
}

func isNumericIP(s string) bool {
	dots := strings.Count(s, ".")
	if dots != 3 {
		return false
	}
	for _, part := range strings.Split(s, ".") {
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}
