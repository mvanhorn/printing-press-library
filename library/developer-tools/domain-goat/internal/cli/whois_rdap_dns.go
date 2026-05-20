// Commands: whois, rdap, dns, cert
package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/source/dnssrc"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/source/rdap"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/source/whoissrc"
)

func newWhoisCmd(flags *rootFlags) *cobra.Command {
	var rawOnly bool
	cmd := &cobra.Command{
		Use:   "whois <domain>",
		Short: "WHOIS lookup (RFC 3912, TCP/43) with parsed output.",
		Example: `  domain-goat-pp-cli whois example.com
  domain-goat-pp-cli whois example.io --raw
  domain-goat-pp-cli whois example.com --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Short-circuit before normalizeAll so verify's synthetic
			// positional ("mock-value") doesn't fail validation in dry-run.
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "fqdn": args[0]})
			}
			fqdns, err := normalizeAll(args)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()
			res, err := whoissrc.Lookup(ctx, fqdns[0])
			if err != nil && res == nil {
				return apiErr(err)
			}
			s, _ := openStore(cmd.Context())
			if s != nil {
				defer s.Close()
				_ = s.SaveWhoisRecord(cmd.Context(), fqdns[0], res.Raw, res.ParsedJSONString(), "port-43")
			}
			if rawOnly {
				fmt.Fprintln(cmd.OutOrStdout(), res.Raw)
				return nil
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, res)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "FQDN\t%s\n", res.FQDN)
			fmt.Fprintf(tw, "REGISTRAR\t%s\n", res.Registrar)
			fmt.Fprintf(tw, "STATUS\t%s\n", strings.Join(res.Status, ", "))
			fmt.Fprintf(tw, "CREATED\t%s\n", res.CreatedAt)
			fmt.Fprintf(tw, "EXPIRES\t%s\n", res.ExpiresAt)
			fmt.Fprintf(tw, "UPDATED\t%s\n", res.UpdatedAt)
			fmt.Fprintf(tw, "NAMESERVERS\t%s\n", strings.Join(res.NameServers, ", "))
			fmt.Fprintf(tw, "AVAILABLE\t%v\n", res.Available)
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&rawOnly, "raw", false, "Print raw WHOIS response only")
	return cmd
}

func newRdapCmd(flags *rootFlags) *cobra.Command {
	var rawOnly bool
	cmd := &cobra.Command{
		Use:   "rdap <domain>",
		Short: "RDAP lookup (RFC 7480-7484) via IANA bootstrap.",
		Example: `  domain-goat-pp-cli rdap example.com
  domain-goat-pp-cli rdap example.io --raw
  domain-goat-pp-cli rdap example.com --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// Short-circuit before normalizeAll so verify's synthetic
			// positional ("mock-value") doesn't fail validation in dry-run.
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "fqdn": args[0]})
			}
			fqdns, err := normalizeAll(args)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			res, err := rdap.Lookup(ctx, fqdns[0])
			if err != nil && res == nil {
				return apiErr(err)
			}
			s, _ := openStore(cmd.Context())
			if s != nil {
				defer s.Close()
				if res != nil {
					_ = s.SaveRDAPRecord(cmd.Context(), fqdns[0], string(res.Raw), res.StatusText, res.EventsJSON())
				}
			}
			if rawOnly && res != nil {
				fmt.Fprintln(cmd.OutOrStdout(), string(res.Raw))
				return nil
			}
			if wantJSON(cmd, flags) || res == nil {
				return emitJSON(cmd, flags, res)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "FQDN\t%s\n", res.FQDN)
			fmt.Fprintf(tw, "AVAILABLE\t%v\n", res.Available)
			fmt.Fprintf(tw, "STATUS\t%s\n", res.StatusText)
			fmt.Fprintf(tw, "CREATED\t%s\n", res.CreatedAt())
			fmt.Fprintf(tw, "EXPIRES\t%s\n", res.ExpiresAt())
			fmt.Fprintf(tw, "EVENTS\t%d\n", len(res.Events))
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&rawOnly, "raw", false, "Print raw RDAP JSON only")
	return cmd
}

func newDNSCmd(flags *rootFlags) *cobra.Command {
	var types string
	var reverse bool
	cmd := &cobra.Command{
		Use:   "dns <domain-or-ip>",
		Short: "Run DNS lookups (A/AAAA/NS/MX/SOA) — fast availability pre-filter.",
		Example: `  domain-goat-pp-cli dns example.com
  domain-goat-pp-cli dns example.com --types NS,MX
  domain-goat-pp-cli dns 8.8.8.8 --reverse`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			target := strings.ToLower(args[0])
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "target": target})
			}
			if reverse {
				names, err := net.DefaultResolver.LookupAddr(cmd.Context(), target)
				if err != nil {
					return apiErr(err)
				}
				return emitJSON(cmd, flags, map[string]any{"ip": target, "ptr": names})
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 8*time.Second)
			defer cancel()
			res, _ := dnssrc.Probe(ctx, target)
			if wantJSON(cmd, flags) {
				out := map[string]any{
					"fqdn":                res.FQDN,
					"ns":                  res.NS,
					"a":                   res.A,
					"aaaa":                res.AAAA,
					"mx":                  res.MX,
					"has_any":             res.HasAny,
					"available_heuristic": res.Available,
				}
				wantTypes := joinTLDs(types) // reuse the csv-split helper
				if len(wantTypes) > 0 {
					filtered := map[string]any{"fqdn": res.FQDN}
					for _, t := range wantTypes {
						switch t {
						case "ns":
							filtered["ns"] = res.NS
						case "a":
							filtered["a"] = res.A
						case "aaaa":
							filtered["aaaa"] = res.AAAA
						case "mx":
							filtered["mx"] = res.MX
						}
					}
					out = filtered
				}
				return emitJSON(cmd, flags, out)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "FQDN\t%s\n", res.FQDN)
			fmt.Fprintf(tw, "NS\t%s\n", strings.Join(res.NS, ", "))
			fmt.Fprintf(tw, "A\t%s\n", strings.Join(res.A, ", "))
			fmt.Fprintf(tw, "AAAA\t%s\n", strings.Join(res.AAAA, ", "))
			fmt.Fprintf(tw, "MX\t%s\n", strings.Join(res.MX, ", "))
			fmt.Fprintf(tw, "HAS_ANY\t%v\n", res.HasAny)
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&types, "types", "", "Comma-separated record types: a,aaaa,ns,mx")
	cmd.Flags().BoolVar(&reverse, "reverse", false, "Reverse PTR lookup on an IP")
	return cmd
}

func newCertCmd(flags *rootFlags) *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "cert <domain>",
		Short: "Inspect the TLS certificate for a domain.",
		Example: `  domain-goat-pp-cli cert example.com
  domain-goat-pp-cli cert example.com --port 8443 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			host := strings.ToLower(args[0])
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "host": host})
			}
			addr := fmt.Sprintf("%s:%d", host, port)
			d := &net.Dialer{Timeout: 8 * time.Second}
			conn, err := tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: host})
			if err != nil {
				return apiErr(err)
			}
			defer conn.Close()
			chain := conn.ConnectionState().PeerCertificates
			if len(chain) == 0 {
				return apiErr(fmt.Errorf("no certificates"))
			}
			cert := chain[0]
			info := certInfo{
				Subject:    cert.Subject.CommonName,
				Issuer:     cert.Issuer.CommonName,
				NotBefore:  cert.NotBefore,
				NotAfter:   cert.NotAfter,
				DNSNames:   cert.DNSNames,
				SerialHex:  cert.SerialNumber.Text(16),
				SignatureA: cert.SignatureAlgorithm.String(),
				IsCA:       cert.IsCA,
				DaysLeft:   int(time.Until(cert.NotAfter).Hours() / 24),
				Chain:      summarizeChain(chain),
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, info)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "SUBJECT\t%s\n", info.Subject)
			fmt.Fprintf(tw, "ISSUER\t%s\n", info.Issuer)
			fmt.Fprintf(tw, "VALID\t%s -> %s\n", info.NotBefore.Format("2006-01-02"), info.NotAfter.Format("2006-01-02"))
			fmt.Fprintf(tw, "DAYS LEFT\t%d\n", info.DaysLeft)
			fmt.Fprintf(tw, "SANs\t%s\n", strings.Join(info.DNSNames, ", "))
			fmt.Fprintf(tw, "SIG ALG\t%s\n", info.SignatureA)
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&port, "port", 443, "TLS port")
	return cmd
}

type certInfo struct {
	Subject    string    `json:"subject"`
	Issuer     string    `json:"issuer"`
	NotBefore  time.Time `json:"not_before"`
	NotAfter   time.Time `json:"not_after"`
	DNSNames   []string  `json:"dns_names"`
	SerialHex  string    `json:"serial_hex"`
	SignatureA string    `json:"signature_algorithm"`
	IsCA       bool      `json:"is_ca"`
	DaysLeft   int       `json:"days_left"`
	Chain      []string  `json:"chain"`
}

func summarizeChain(chain []*x509.Certificate) []string {
	out := make([]string, 0, len(chain))
	for _, c := range chain {
		out = append(out, c.Subject.CommonName)
	}
	return out
}

// declared in helpers (keeping a no-op so json import is referenced).
var _ = json.RawMessage(nil)
