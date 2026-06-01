// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type crossSellRow struct {
	Email     string   `json:"email"`
	PaidOn    []string `json:"paid_on"`
	FreeOn    []string `json:"free_on,omitempty"`
	MissingOn []string `json:"missing_on,omitempty"`
}

func newSubsCrossSellCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "cross-sell",
		Short: "Find emails paid on one publication but free or absent on the others.",
		Long: `Joins subscribers across every publication you own and surfaces emails that
already pay on at least one publication but are free subscribers — or not yet
subscribers at all — on your other publications.

The classic cross-sell list. Substack's web UI does not ship this.`,
		Example:     "  substack-pp-cli subs cross-sell --json --limit 100",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "subs cross-sell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("substack-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()
			db := s.DB()

			// Step 1: collect distinct publications and a name map
			pubs := map[string]string{} // id -> display name (subdomain || name)
			prows, err := db.QueryContext(cmd.Context(),
				`SELECT id, COALESCE(NULLIF(subdomain, ''), name) FROM publications`)
			if err != nil {
				return fmt.Errorf("listing publications: %w", err)
			}
			for prows.Next() {
				var id, name string
				if err := prows.Scan(&id, &name); err != nil {
					prows.Close()
					return err
				}
				pubs[id] = name
			}
			prows.Close()
			// Cross-sell is only meaningful across two or more owned
			// publications. A single-publication account (the common case) has
			// no cross-sell surface — that's an empty result, not an error.
			// Returning cleanly here keeps single-pub users from hitting a
			// non-zero exit on a legitimately-empty query.
			if len(pubs) < 2 {
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage("[]"), flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(),
					"No cross-sell candidates: cross-sell compares subscribers across publications you own, and only %d is synced. Sync a second owned publication to see candidates.\n", len(pubs))
				return nil
			}

			// Step 2: collect every (email, publication_id, tier) row
			srows, err := db.QueryContext(cmd.Context(),
				`SELECT email, publication_id, COALESCE(tier, '') FROM subscribers WHERE email IS NOT NULL AND email != ''`)
			if err != nil {
				return fmt.Errorf("scanning subscribers: %w", err)
			}
			defer srows.Close()

			byEmail := map[string]map[string]string{} // email -> pubID -> tier
			for srows.Next() {
				var email, pubID, tier string
				if err := srows.Scan(&email, &pubID, &tier); err != nil {
					return err
				}
				if _, ok := byEmail[email]; !ok {
					byEmail[email] = map[string]string{}
				}
				byEmail[email][pubID] = tier
			}

			// Step 3: classify
			var out []crossSellRow
			for email, tiers := range byEmail {
				row := crossSellRow{Email: email}
				paying := false
				for pubID, tier := range tiers {
					name := pubs[pubID]
					if name == "" {
						name = pubID
					}
					if tier == "paid" || tier == "founding" {
						row.PaidOn = append(row.PaidOn, name)
						paying = true
					} else if tier == "free" {
						row.FreeOn = append(row.FreeOn, name)
					}
				}
				if !paying {
					continue
				}
				// what publications is this email NOT subscribed to at all?
				for pubID, name := range pubs {
					if _, has := tiers[pubID]; !has {
						row.MissingOn = append(row.MissingOn, name)
					}
				}
				// Only surface rows where there IS a cross-sell opportunity
				if len(row.FreeOn) == 0 && len(row.MissingOn) == 0 {
					continue
				}
				out = append(out, row)
			}

			// Sort: most paid-on first
			sortCrossSellByPaidCount(out)
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				raw, _ := json.Marshal(out)
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "No cross-sell candidates.")
				return nil
			}
			fmt.Fprintf(w, "%d cross-sell candidate(s):\n", len(out))
			fmt.Fprintln(w, strings.Repeat("─", 78))
			for _, r := range out {
				fmt.Fprintf(w, "  %s\n", r.Email)
				fmt.Fprintf(w, "    paid on:    %s\n", strings.Join(r.PaidOn, ", "))
				if len(r.FreeOn) > 0 {
					fmt.Fprintf(w, "    free on:    %s\n", strings.Join(r.FreeOn, ", "))
				}
				if len(r.MissingOn) > 0 {
					fmt.Fprintf(w, "    missing on: %s\n", strings.Join(r.MissingOn, ", "))
				}
				fmt.Fprintln(w)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max candidates")
	return cmd
}

func sortCrossSellByPaidCount(rows []crossSellRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && len(rows[j].PaidOn) > len(rows[j-1].PaidOn); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}
