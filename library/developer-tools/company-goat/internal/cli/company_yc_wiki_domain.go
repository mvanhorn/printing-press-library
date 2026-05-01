// Hand-written: yc, wiki, and domain commands.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/rdap"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/wikidata"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/yc"
	"github.com/spf13/cobra"
)

func newYCCmd(flags *rootFlags) *cobra.Command {
	var t targetFlags

	cmd := &cobra.Command{
		Use:   "yc [co]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Y Combinator directory entry if backed: batch, status, location, description.",
		Long: `yc looks up the resolved company in the Y Combinator directory snapshot. Returns batch, status (Active/Acquired/Public/Inactive), location, team size, and the YC one-liner description.

Source: yc-oss/api daily snapshot of YC's Algolia index.`,
		Example: strings.Trim(`
  company-goat-pp-cli yc stripe
  company-goat-pp-cli yc cruise --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if t.Domain == "" && len(args) == 0 {
				return cmd.Help()
			}
			domain, err := runResolveOrExit(cmd, flags, args, t)
			if err != nil {
				return err
			}
			c := yc.NewClient()
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			// First try domain match (canonical).
			entry, err := c.FindByDomain(ctx, domain)
			if err != nil {
				return fmt.Errorf("yc: %w", err)
			}
			// Fallback: name match.
			if entry == nil && len(args) > 0 {
				matches, mErr := c.FindByName(ctx, strings.Join(args, " "), 1)
				if mErr == nil && len(matches) > 0 {
					entry = &matches[0]
				}
			}
			w := cmd.OutOrStdout()
			asJSON := flags.asJSON || !isTerminal(w)
			out := map[string]any{
				"domain":   domain,
				"yc_entry": entry,
			}
			if entry == nil {
				out["note"] = "no YC entry found for " + domain
			}
			if asJSON {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(w, "Domain: %s\n", domain)
			if entry == nil {
				fmt.Fprintf(w, "YC: no entry found\n")
				return nil
			}
			fmt.Fprintf(w, "YC: %s  (batch %s, status %s)\n", entry.Name, entry.Batch, entry.Status)
			if entry.OneLiner != "" {
				fmt.Fprintf(w, "  %s\n", entry.OneLiner)
			}
			if entry.Industry != "" || entry.Subindustry != "" {
				fmt.Fprintf(w, "  Industry: %s / %s\n", entry.Industry, entry.Subindustry)
			}
			if entry.LocationCity != "" || entry.Country != "" {
				fmt.Fprintf(w, "  Location: %s, %s\n", entry.LocationCity, entry.Country)
			}
			if entry.TeamSize > 0 {
				fmt.Fprintf(w, "  Team size: %d\n", entry.TeamSize)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&t.Domain, "domain", "", "Skip name resolution and use this domain (e.g. stripe.com)")
	cmd.Flags().IntVar(&t.Pick, "pick", 0, "Pick candidate N (1-indexed) from a previous ambiguous resolve")
	return cmd
}

func newWikiCmd(flags *rootFlags) *cobra.Command {
	var t targetFlags

	cmd := &cobra.Command{
		Use:   "wiki [co]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Wikidata company facts: founded date, founders, HQ, industry, key people.",
		Long: `wiki looks up the resolved company on Wikidata via its official-website (P856) property. Returns structured facts: founded date, headquarters location, country, industry, founders.

Wikidata coverage of early-stage startups is sparse; established companies are well-represented.`,
		Example: strings.Trim(`
  company-goat-pp-cli wiki stripe
  company-goat-pp-cli wiki anthropic --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if t.Domain == "" && len(args) == 0 {
				return cmd.Help()
			}
			domain, err := runResolveOrExit(cmd, flags, args, t)
			if err != nil {
				return err
			}
			c := wikidata.NewClient()
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()
			entry, err := c.LookupByDomain(ctx, domain)
			if err != nil {
				return fmt.Errorf("wikidata: %w", err)
			}
			w := cmd.OutOrStdout()
			asJSON := flags.asJSON || !isTerminal(w)
			out := map[string]any{
				"domain":         domain,
				"wikidata_entry": entry,
			}
			if entry == nil {
				out["note"] = "no Wikidata page found for " + domain
			}
			if asJSON {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(w, "Domain: %s\n", domain)
			if entry == nil {
				fmt.Fprintf(w, "Wikidata: no entry found\n")
				return nil
			}
			fmt.Fprintf(w, "Wikidata: %s (%s)\n", entry.Label, entry.QID)
			if entry.Description != "" {
				fmt.Fprintf(w, "  %s\n", entry.Description)
			}
			if entry.Founded != "" {
				fmt.Fprintf(w, "  Founded:   %s\n", entry.Founded)
			}
			if entry.HQLocation != "" {
				fmt.Fprintf(w, "  HQ:        %s\n", entry.HQLocation)
			}
			if entry.Country != "" {
				fmt.Fprintf(w, "  Country:   %s\n", entry.Country)
			}
			if entry.Industry != "" {
				fmt.Fprintf(w, "  Industry:  %s\n", entry.Industry)
			}
			if len(entry.Founders) > 0 {
				fmt.Fprintf(w, "  Founders:  %s\n", strings.Join(entry.Founders, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&t.Domain, "domain", "", "Skip name resolution and use this domain (e.g. stripe.com)")
	cmd.Flags().IntVar(&t.Pick, "pick", 0, "Pick candidate N (1-indexed) from a previous ambiguous resolve")
	return cmd
}

func newDomainCmd(flags *rootFlags) *cobra.Command {
	var t targetFlags

	cmd := &cobra.Command{
		Use:   "domain [co]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Domain age via RDAP/WHOIS, DNS records, and CNAME-based hosting hint.",
		Long: `domain returns RDAP registration data (age, registrar, status) plus DNS records (CNAME, A, NS) and a hosting hint derived from the CNAME (Vercel/Netlify/Heroku/Cloudflare Pages/AWS/GCP/etc.).

Useful for "is this a modern startup stack" signals — a Vercel/Cloudflare CNAME on a small startup is a strong "modern, JS-heavy team" hint.`,
		Example: strings.Trim(`
  company-goat-pp-cli domain stripe
  company-goat-pp-cli domain --domain anthropic.com --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if t.Domain == "" && len(args) == 0 {
				return cmd.Help()
			}
			domain, err := runResolveOrExit(cmd, flags, args, t)
			if err != nil {
				return err
			}
			c := rdap.NewClient()
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()
			info, err := c.Lookup(ctx, domain)
			if err != nil {
				return fmt.Errorf("rdap/dns: %w", err)
			}
			w := cmd.OutOrStdout()
			asJSON := flags.asJSON || !isTerminal(w)
			if asJSON {
				return flags.printJSON(cmd, info)
			}
			fmt.Fprintf(w, "Domain: %s\n", info.Domain)
			if info.Registered != "" {
				fmt.Fprintf(w, "  Registered:    %s\n", info.Registered)
			}
			if info.LastChanged != "" {
				fmt.Fprintf(w, "  Last changed:  %s\n", info.LastChanged)
			}
			if info.ExpiresAt != "" {
				fmt.Fprintf(w, "  Expires:       %s\n", info.ExpiresAt)
			}
			if info.Registrar != "" {
				fmt.Fprintf(w, "  Registrar:     %s\n", info.Registrar)
			}
			if info.HostingHint != "" {
				fmt.Fprintf(w, "  Hosting hint:  %s\n", info.HostingHint)
				if info.HostingCNAME != "" {
					fmt.Fprintf(w, "    via CNAME:   %s\n", info.HostingCNAME)
				}
			} else if info.HostingCNAME != "" {
				fmt.Fprintf(w, "  CNAME:         %s (no hosting hint matched)\n", info.HostingCNAME)
			}
			if len(info.Nameservers) > 0 {
				fmt.Fprintf(w, "  Nameservers:   %s\n", strings.Join(info.Nameservers, ", "))
			}
			if len(info.IPv4Addresses) > 0 {
				fmt.Fprintf(w, "  IPv4:          %s\n", strings.Join(info.IPv4Addresses, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&t.Domain, "domain", "", "Skip name resolution and use this domain (e.g. stripe.com)")
	cmd.Flags().IntVar(&t.Pick, "pick", 0, "Pick candidate N (1-indexed) from a previous ambiguous resolve")
	return cmd
}
