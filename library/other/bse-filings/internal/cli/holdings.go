// Copyright 2026 rushyant-m. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/bseutil"
	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/bse-filings/internal/store"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// seedHolding is one entry in the default portfolio that auto-seeds an empty
// holdings table. Scrip codes and sectors are the human-curated starter set;
// names come from the BSE long names.
type seedHolding struct {
	Code   string
	Name   string
	Sector string
}

// pp:novel-static-reference — the starter portfolio. Curated reference data,
// not an API response: a fresh install seeds these 19 holdings so sync and the
// read commands have something to operate on before the user edits the list.
var defaultHoldings = []seedHolding{
	{"534309", "NBCC", "Construction"},
	{"500400", "TATAPOWER", "Power"},
	{"500180", "HDFCBANK", "Banking"},
	{"500325", "RELIANCE", "Energy"},
	{"532540", "TCS", "IT"},
	{"500209", "INFY", "IT"},
	{"500875", "ITC", "FMCG"},
	{"500696", "HINDUNILVR", "FMCG"},
	{"500510", "LT", "Construction"},
	{"500112", "SBIN", "Banking"},
	{"532215", "AXISBANK", "Banking"},
	{"532454", "BHARTIARTL", "Telecom"},
	{"532500", "MARUTI", "Auto"},
	{"500820", "ASIANPAINT", "FMCG"},
	{"500570", "TATAMOTORS", "Auto"},
	{"532898", "POWERGRID", "Power"},
	{"500312", "ONGC", "Energy"},
	{"532555", "NTPC", "Power"},
	{"533278", "COALINDIA", "Energy"},
}

// seedHoldingsIfEmpty populates the default portfolio when the holdings table
// has no rows yet. No-op once the user has any holdings of their own.
func seedHoldingsIfEmpty(s *store.Store) error {
	n, err := s.CountHoldings()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	for _, h := range defaultHoldings {
		if err := s.UpsertHolding(h.Code, h.Name, h.Sector); err != nil {
			return err
		}
	}
	return nil
}

func newHoldingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "holdings",
		Short: "Manage the local portfolio of BSE scrip codes that sync and the read commands operate on.",
		Long: strings.Trim(`
Manage the portfolio of BSE scrip codes that 'sync' and the analysis commands
(concall-grep, thesis-drift, cross, due-soon, stale, critical) scope to.

A fresh install auto-seeds a starter portfolio of 19 large-cap holdings on the
first 'holdings list' or 'sync'. Use 'holdings add' / 'holdings remove' to
curate your own.`, "\n"),
		Example: strings.Trim(`
  bse-filings-pp-cli holdings list
  bse-filings-pp-cli holdings add 500209 --name INFY --sector IT
  bse-filings-pp-cli holdings remove 500209`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newHoldingsListCmd(flags))
	cmd.AddCommand(newHoldingsAddCmd(flags))
	cmd.AddCommand(newHoldingsRemoveCmd(flags))
	return cmd
}

func newHoldingsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the holdings in the local portfolio.",
		Example:     "  bse-filings-pp-cli holdings list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openBSEStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := seedHoldingsIfEmpty(s); err != nil {
				return err
			}
			holdings, err := s.ListHoldings()
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, holdings)
		},
	}
	return cmd
}

func newHoldingsAddCmd(flags *rootFlags) *cobra.Command {
	var name, sector string
	cmd := &cobra.Command{
		Use:     "add [scrip]",
		Short:   "Add a scrip to the portfolio, resolving its name from BSE when --name is omitted.",
		Example: "  bse-filings-pp-cli holdings add 500209 --sector IT",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			scrip := strings.TrimSpace(args[0])

			resolvedName := name
			if resolvedName == "" {
				// Resolve the company name via PeerSmartSearch unless under
				// verify (no network); fall back to the bare code otherwise.
				if cliutil.IsVerifyEnv() {
					fmt.Fprintf(cmd.ErrOrStderr(), "verify: would resolve name for scrip %s via PeerSmartSearch\n", scrip)
				} else if n := resolveScripName(flags, scrip); n != "" {
					resolvedName = n
				}
			}

			s, err := openBSEStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.UpsertHolding(scrip, resolvedName, sector); err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]string{
				"scrip_code": scrip,
				"scrip_name": resolvedName,
				"sector":     sector,
				"status":     "added",
			})
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Display name (resolved from BSE when omitted).")
	cmd.Flags().StringVar(&sector, "sector", "", "Sector label used by cross/thesis-drift grouping.")
	return cmd
}

func newHoldingsRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove [scrip]",
		Short:   "Remove a scrip from the portfolio.",
		Example: "  bse-filings-pp-cli holdings remove 500209",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			scrip := strings.TrimSpace(args[0])
			s, err := openBSEStore(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			removed, err := s.RemoveHolding(scrip)
			if err != nil {
				return err
			}
			status := "removed"
			if !removed {
				status = "not_found"
			}
			return flags.printJSON(cmd, map[string]string{"scrip_code": scrip, "status": status})
		},
	}
	return cmd
}

// resolveScripName calls PeerSmartSearch and returns the best-match company
// name for a scrip code, or "" when the call fails or returns no match.
// pp:client-call — performs a real PeerSmartSearch HTTP request.
func resolveScripName(flags *rootFlags, scrip string) string {
	c, err := flags.newClient()
	if err != nil {
		return ""
	}
	data, err := c.Get("/PeerSmartSearch/w", map[string]string{"Type": "SS", "text": scrip})
	if err != nil {
		return ""
	}
	// PeerSmartSearch returns a JSON-encoded HTML string; unwrap the quotes.
	html := string(data)
	html = strings.TrimSpace(html)
	html = strings.Trim(html, `"`)
	html = strings.ReplaceAll(html, `\"`, `"`)
	for _, m := range bseutil.ParsePeerSearch(html) {
		if m.ScripCode == scrip {
			return m.Name
		}
	}
	// Fall back to the first match if no exact code match.
	if matches := bseutil.ParsePeerSearch(html); len(matches) > 0 {
		return matches[0].Name
	}
	return ""
}
