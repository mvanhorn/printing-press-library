// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.

// Package cli — contacts_analytics.go
// contacts analytics subcommands: countries, domains, missing.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newContactsAnalyticsCmd(f *rootFlags) *cobra.Command {
	a := &cobra.Command{
		Use:   "analytics",
		Short: "Analytics on your contacts (countries, domains, coverage)",
	}
	a.AddCommand(newAnalyticsCountriesCmd(f))
	a.AddCommand(newAnalyticsDomainsCmd(f))
	a.AddCommand(newAnalyticsMissingCmd(f))
	return a
}

// ── countries ─────────────────────────────────────────────────────────────────

func newAnalyticsCountriesCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "countries",
		Short: "Contacts by country, resolved from phone number prefix",
		Long: `Groups your contacts by country using their phone number's dialing prefix.

A contact may appear in multiple countries if they have numbers from different countries.
The count column shows unique contacts; phones shows total phone numbers from that country.`,
		Example: `  icloud-pp-cli contacts analytics countries
  icloud-pp-cli contacts analytics countries --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			store, err := openContactStore()
			if err != nil {
				return err
			}
			defer store.Close()

			rows, err := store.AnalyticsCountries()
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(out, "No phone data found. Run: icloud-pp-cli contacts sync")
				return nil
			}

			if f.asJSON || !isTerminal(out) {
				return printJSON(out, rows)
			}

			// Count total known-country contacts for percentage
			total := 0
			for _, r := range rows {
				total += r.Count
			}

			tw := newTabWriter(out)
			fmt.Fprintln(out, bold(f, out, "Contacts by country (phone prefix)"))
			fmt.Fprintln(out)
			header := "  " + bold(f, out, "Flag") + "\t" +
				bold(f, out, "Country") + "\t" +
				bold(f, out, "Code") + "\t" +
				bold(f, out, "Contacts") + "\t" +
				bold(f, out, "Phones") + "\t" +
				bold(f, out, "Share")
			fmt.Fprintln(tw, header)
			fmt.Fprintln(tw, "  "+strings.Repeat("─", 4)+"\t"+
				strings.Repeat("─", 30)+"\t"+
				strings.Repeat("─", 5)+"\t"+
				strings.Repeat("─", 8)+"\t"+
				strings.Repeat("─", 6)+"\t"+
				strings.Repeat("─", 6))

			for _, r := range rows {
				pct := 0.0
				if total > 0 {
					pct = float64(r.Count) / float64(total) * 100
				}
				flag := countryFlag(r.ISO)
				fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%.1f%%\n",
					flag,
					r.Country,
					r.Code,
					formatInt(int64(r.Count)),
					formatInt(int64(r.Phones)),
					pct,
				)
			}
			tw.Flush()
			fmt.Fprintln(out)
			fmt.Fprintf(out, "  %d countries · %d contacts with known country\n", len(rows), total)

			// Show unresolved count
			var unresolved int
			if err := store.db.QueryRow(`
				SELECT COUNT(DISTINCT contact_id) FROM contact_phones
				WHERE country IS NULL OR country = ''
			`).Scan(&unresolved); err != nil {
				return fmt.Errorf("query unresolved-country count: %w", err)
			}
			if unresolved > 0 {
				fmt.Fprintf(out, "  %s %d contacts have phones with unresolved country (no E.164 prefix)\n",
					yellow(f, out, "⚠"), unresolved)
			}
			return nil
		},
	}
}

// ── domains ───────────────────────────────────────────────────────────────────

func newAnalyticsDomainsCmd(f *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "domains",
		Short: "Top email domains across your contacts",
		Example: `  icloud-pp-cli contacts analytics domains
  icloud-pp-cli contacts analytics domains --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			store, err := openContactStore()
			if err != nil {
				return err
			}
			defer store.Close()

			rows, err := store.AnalyticsDomains(limit)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(out, "No email data found. Run: icloud-pp-cli contacts sync")
				return nil
			}

			if f.asJSON || !isTerminal(out) {
				return printJSON(out, rows)
			}

			fmt.Fprintln(out, bold(f, out, "Top email domains"))
			fmt.Fprintln(out)
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "  "+bold(f, out, "Domain")+"\t"+bold(f, out, "Contacts"))
			fmt.Fprintln(tw, "  "+strings.Repeat("─", 30)+"\t"+strings.Repeat("─", 8))
			for _, r := range rows {
				fmt.Fprintf(tw, "  %s\t%s\n", r.Domain, formatInt(int64(r.Count)))
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Max domains to show")
	return cmd
}

// ── missing ───────────────────────────────────────────────────────────────────

func newAnalyticsMissingCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "missing",
		Short: "Coverage report: contacts missing phone, email, or name",
		Example: `  icloud-pp-cli contacts analytics missing
  icloud-pp-cli contacts analytics missing --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			store, err := openContactStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ms, err := store.AnalyticsMissing()
			if err != nil {
				return err
			}

			if f.asJSON || !isTerminal(out) {
				return printJSON(out, ms)
			}

			pct := func(n int) string {
				if ms.Total == 0 {
					return "0%"
				}
				return fmt.Sprintf("%.1f%%", float64(n)/float64(ms.Total)*100)
			}

			fmt.Fprintln(out, bold(f, out, "Contact coverage report"))
			fmt.Fprintln(out)
			tw := newTabWriter(out)
			fmt.Fprintf(tw, "  Total contacts\t%s\n", formatInt(int64(ms.Total)))
			fmt.Fprintln(tw, "  "+strings.Repeat("─", 24)+"\t"+strings.Repeat("─", 14))
			fmt.Fprintf(tw, "  No phone number\t%s  (%s)\n", formatInt(int64(ms.NoPhone)), pct(ms.NoPhone))
			fmt.Fprintf(tw, "  No email address\t%s  (%s)\n", formatInt(int64(ms.NoEmail)), pct(ms.NoEmail))
			fmt.Fprintf(tw, "  No organization\t%s  (%s)\n", formatInt(int64(ms.NoOrg)), pct(ms.NoOrg))
			fmt.Fprintf(tw, "  No name (first or last)\t%s  (%s)\n", formatInt(int64(ms.NoName)), pct(ms.NoName))
			tw.Flush()
			return nil
		},
	}
}
