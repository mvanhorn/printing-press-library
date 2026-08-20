// Commands: tlds, pricing
package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/source/iana"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/source/porkbun"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/store"
)

func newTLDsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tlds",
		Short: "Manage the local TLD table (IANA RDAP bootstrap + metadata).",
	}
	cmd.AddCommand(newTLDsSyncCmd(flags))
	cmd.AddCommand(newTLDsListCmd(flags))
	cmd.AddCommand(newTLDsInfoCmd(flags))
	return cmd
}

func newTLDsSyncCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sync",
		Short:       "Pull the IANA RDAP bootstrap and refresh the local TLD table.",
		Example:     `  domain-goat-pp-cli tlds sync`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "source": iana.BootstrapURL})
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()
			bs, err := iana.Fetch(ctx)
			if err != nil {
				return apiErr(err)
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			tldMap := bs.TLDMap()
			count := 0
			for tld, rdapBase := range tldMap {
				kind := "gTLD"
				if len(tld) == 2 {
					kind = "ccTLD"
				}
				err := s.UpsertTLD(cmd.Context(), store.TLDRow{
					TLD: tld, Kind: kind, RDAPBase: rdapBase, HasRDAP: true,
					Prestige: prestige(tld),
				})
				if err == nil {
					count++
				}
			}
			out := map[string]any{
				"synced":      count,
				"source":      iana.BootstrapURL,
				"publication": bs.Publication,
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d TLDs from IANA bootstrap (publication %s)\n", count, bs.Publication)
			return nil
		},
	}
	return cmd
}

func newTLDsListCmd(flags *rootFlags) *cobra.Command {
	var kind string
	var hasRDAPOnly bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all TLDs in the local table.",
		Example: `  domain-goat-pp-cli tlds list
  domain-goat-pp-cli tlds list --kind gTLD
  domain-goat-pp-cli tlds list --has-rdap --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			tlds, err := s.ListTLDs(cmd.Context())
			if err != nil {
				return apiErr(err)
			}
			filtered := make([]store.TLDRow, 0, len(tlds))
			for _, t := range tlds {
				if kind != "" && !strings.EqualFold(t.Kind, kind) {
					continue
				}
				if hasRDAPOnly && !t.HasRDAP {
					continue
				}
				filtered = append(filtered, t)
			}
			if len(filtered) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no TLDs in local table — run `domain-goat-pp-cli tlds sync`")
				return emitJSON(cmd, flags, []store.TLDRow{})
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, filtered)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "TLD\tKIND\tHAS_RDAP\tPRESTIGE\tRDAP_BASE")
			for _, t := range filtered {
				fmt.Fprintf(tw, ".%s\t%s\t%v\t%d\t%s\n", t.TLD, t.Kind, t.HasRDAP, t.Prestige, t.RDAPBase)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Filter by kind: gTLD|ccTLD")
	cmd.Flags().BoolVar(&hasRDAPOnly, "has-rdap", false, "Show only TLDs with RDAP support")
	return cmd
}

func newTLDsInfoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <tld>",
		Short: "Show metadata for one TLD.",
		Example: `  domain-goat-pp-cli tlds get io
  domain-goat-pp-cli tlds get .com --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "tld": args[0]})
			}
			tld := strings.ToLower(strings.TrimPrefix(args[0], "."))
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			t, err := s.GetTLD(cmd.Context(), tld)
			if err != nil {
				return apiErr(err)
			}
			if t == nil {
				return notFoundErr(fmt.Errorf("tld %s not in local table — run `tlds sync`", tld))
			}
			p, _ := s.GetPricing(cmd.Context(), tld, "porkbun")
			out := map[string]any{
				"tld":          t.TLD,
				"kind":         t.Kind,
				"rdap_base":    t.RDAPBase,
				"whois_server": t.WHOISServer,
				"has_rdap":     t.HasRDAP,
				"prestige":     t.Prestige,
			}
			if p != nil {
				out["price"] = map[string]any{
					"registrar":    p.Registrar,
					"registration": p.Registration,
					"renewal":      p.Renewal,
					"transfer":     p.Transfer,
				}
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, out)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "TLD\t.%s\n", t.TLD)
			fmt.Fprintf(tw, "KIND\t%s\n", t.Kind)
			fmt.Fprintf(tw, "RDAP_BASE\t%s\n", t.RDAPBase)
			fmt.Fprintf(tw, "HAS_RDAP\t%v\n", t.HasRDAP)
			fmt.Fprintf(tw, "PRESTIGE\t%d\n", t.Prestige)
			if p != nil {
				fmt.Fprintf(tw, "REGISTRATION\t$%.2f (porkbun)\n", p.Registration)
				fmt.Fprintf(tw, "RENEWAL\t$%.2f (porkbun)\n", p.Renewal)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func newPricingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pricing",
		Short: "Manage Porkbun TLD pricing snapshots (no auth required).",
	}
	cmd.AddCommand(newPricingSyncCmd(flags))
	cmd.AddCommand(newPricingShowCmd(flags))
	cmd.AddCommand(newPricingCompareCmd(flags))
	return cmd
}

func newPricingSyncCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "sync",
		Short:       "Pull the full Porkbun TLD pricing table into local SQLite.",
		Example:     `  domain-goat-pp-cli pricing sync`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "source": porkbun.PricingEndpoint})
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 45*time.Second)
			defer cancel()
			entries, err := porkbun.FetchPricing(ctx)
			if err != nil {
				return apiErr(err)
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			count := 0
			for _, e := range entries {
				err := s.UpsertPricing(cmd.Context(), store.PricingRow{
					TLD: e.TLD, Registrar: "porkbun",
					Registration: e.Registration, Renewal: e.Renewal, Transfer: e.Transfer,
				})
				if err == nil {
					count++
				}
			}
			out := map[string]any{"synced": count, "source": porkbun.PricingEndpoint}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d TLD prices from Porkbun\n", count)
			return nil
		},
	}
	return cmd
}

func newPricingShowCmd(flags *rootFlags) *cobra.Command {
	var registrar string
	var limit int
	cmd := &cobra.Command{
		Use:   "show [tld]",
		Short: "Show pricing for one TLD or list all from local snapshot.",
		Example: `  domain-goat-pp-cli pricing show io
  domain-goat-pp-cli pricing show --limit 20 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if len(args) > 0 {
				tld := strings.ToLower(strings.TrimPrefix(args[0], "."))
				p, err := s.GetPricing(cmd.Context(), tld, registrar)
				if err != nil {
					return apiErr(err)
				}
				if p == nil {
					return notFoundErr(fmt.Errorf("no pricing for .%s on %s — run `pricing sync`", tld, registrar))
				}
				return emitJSON(cmd, flags, p)
			}
			rows, err := s.ListPricing(cmd.Context(), registrar, limit)
			if err != nil {
				return apiErr(err)
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, rows)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "TLD\tREGISTRAR\tREGISTRATION\tRENEWAL\tTRANSFER")
			for _, p := range rows {
				fmt.Fprintf(tw, ".%s\t%s\t$%.2f\t$%.2f\t$%.2f\n", p.TLD, p.Registrar, p.Registration, p.Renewal, p.Transfer)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&registrar, "registrar", "porkbun", "Registrar (currently only porkbun)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows (0 = all)")
	return cmd
}

func newPricingCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <tld...>",
		Short: "Compare pricing across multiple TLDs from the local snapshot.",
		Example: `  domain-goat-pp-cli pricing compare com io ai studio
  domain-goat-pp-cli pricing compare com io --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "tlds": args})
			}
			s, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			rows := []store.PricingRow{}
			for _, a := range args {
				tld := strings.ToLower(strings.TrimPrefix(a, "."))
				p, _ := s.GetPricing(cmd.Context(), tld, "porkbun")
				if p != nil {
					rows = append(rows, *p)
				} else {
					rows = append(rows, store.PricingRow{TLD: tld, Registrar: "porkbun"})
				}
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].Registration < rows[j].Registration })
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, rows)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "TLD\tREGISTRATION\tRENEWAL\tTRANSFER")
			for _, p := range rows {
				fmt.Fprintf(tw, ".%s\t$%.2f\t$%.2f\t$%.2f\n", p.TLD, p.Registration, p.Renewal, p.Transfer)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func prestige(tld string) int {
	tab := map[string]int{
		"com": 100, "net": 60, "org": 60,
		"io": 80, "ai": 90, "app": 70, "dev": 75, "co": 65,
		"studio": 55, "design": 55, "agency": 40, "tech": 50,
		"xyz": 20, "online": 15, "site": 15, "info": 25,
		"biz": 15, "us": 30, "me": 50, "tv": 45, "cc": 25,
	}
	return tab[tld]
}
