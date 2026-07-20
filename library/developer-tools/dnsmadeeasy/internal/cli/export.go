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
			s, ok, err := openZoneMirror(ctx, cmd, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				if flagFormat == "json" || flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), []dmeRecord{}, flags)
				}
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
	case "ANAME":
		return fmt.Sprintf("; UNSUPPORTED: %s ANAME %s (DNS Made Easy apex flattening is not representable in standard BIND syntax)", name, ensureTrailingDot(r.Value))
	case "MX":
		return prefix + fmt.Sprintf("%d %s", r.MxLevel, ensureTrailingDot(r.Value))
	case "SRV":
		return prefix + fmt.Sprintf("%d %d %d %s", r.Priority, r.Weight, r.Port, ensureTrailingDot(r.Value))
	case "TXT", "SPF":
		return prefix + bindQuoteTXT(r.Value)
	case "CNAME", "NS", "PTR":
		return prefix + ensureTrailingDot(r.Value)
	default:
		return prefix + r.Value
	}
}

// bindTXTMaxChunk is the RFC 1035 §3.3.14 limit: each TXT character-string
// carries at most 255 octets on the wire.
const bindTXTMaxChunk = 255

// bindQuoteTXT renders a TXT/SPF value as BIND-syntax character-string(s).
//
// Two things the naive form gets wrong:
//   - strconv.Quote emits Go-specific escapes (\x, \u, \a, …) BIND rejects.
//   - A single value over 255 octets is invalid: RFC 1035 caps each
//     character-string at 255, and BIND refuses to load a longer one. DKIM
//     p= keys (300–500 bytes) hit this on virtually every mail domain.
//
// So recover the raw payload (DNS Made Easy may return it bare, as one quoted
// string, or as several adjacent quoted strings — its own >255 split), then
// re-chunk into ≤255-octet segments, escape each, and emit them space-separated
// as adjacent quoted strings, which BIND concatenates back into one RDATA value.
func bindQuoteTXT(v string) string {
	raw := txtRawContent(v)
	if raw == "" {
		return `""`
	}
	var parts []string
	for i := 0; i < len(raw); i += bindTXTMaxChunk {
		end := i + bindTXTMaxChunk
		if end > len(raw) {
			end = len(raw)
		}
		parts = append(parts, `"`+txtEscape(raw[i:end])+`"`)
	}
	return strings.Join(parts, " ")
}

// txtRawContent returns the unquoted payload of a TXT value. When the value is
// in quoted form it concatenates every quoted character-string (honoring \" and
// \\ escapes); otherwise it returns the value as-is. The chunk limit applies to
// these raw octets, not the escaped zone-file text.
func txtRawContent(v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, `"`) {
		return v
	}
	var b strings.Builder
	inQuote, esc := false, false
	for i := 0; i < len(v); i++ {
		c := v[i]
		if esc {
			b.WriteByte(c)
			esc = false
			continue
		}
		if inQuote {
			switch c {
			case '\\':
				esc = true
			case '"':
				inQuote = false
			default:
				b.WriteByte(c)
			}
			continue
		}
		if c == '"' {
			inQuote = true
		}
		// bytes outside quotes (whitespace between adjacent strings) are skipped
	}
	return b.String()
}

// txtEscape escapes a raw character-string segment for BIND zone-file syntax.
func txtEscape(s string) string {
	var escaped strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\':
			escaped.WriteString(`\\`)
		case c == '"':
			escaped.WriteString(`\"`)
		case c < 0x20 || c == 0x7f:
			fmt.Fprintf(&escaped, `\%03d`, c)
		default:
			escaped.WriteByte(c)
		}
	}
	return escaped.String()
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
