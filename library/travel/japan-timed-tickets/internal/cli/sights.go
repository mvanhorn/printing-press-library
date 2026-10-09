// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/tickets"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSightsCmd(flags))
		// The generated 'source' command dumps the raw Ghibli page. The
		// curated commands read it with provenance, so keep it out of the
		// help and MCP surfaces without breaking scripts that call it.
		for _, c := range root.Commands() {
			if c.Name() == "source" {
				c.Hidden = true
				if c.Annotations == nil {
					c.Annotations = map[string]string{}
				}
				c.Annotations["mcp:hidden"] = "true"
			}
		}
	})
}

type sightsMeta struct {
	Source        string `json:"source"`
	RuleCheckedOn string `json:"rule_checked_on"`
	Note          string `json:"note"`
}

type sightsView struct {
	Meta    sightsMeta      `json:"meta"`
	Results []tickets.Sight `json:"results"`
}

func newSightsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sights [sight...]",
		Short: "List supported sights with bilingual names, sale rules, channels, prices and coverage limits",
		Long: strings.Trim(`
Use this command to find sight IDs and to read each sight's sale rule, official
channels (with account and phone requirements), adult prices, slot length and
what 'availability' can and cannot see for it.

Data is stored from the official sources (rule_checked_on). 'onsale' and
'availability' re-check the live pages and warn when a rule changed. This
command makes no network requests.`, "\n"),
		Example: strings.Trim(`
  japan-timed-tickets-pp-cli sights --agent
  japan-timed-tickets-pp-cli sights shibuya-sky --agent
  japan-timed-tickets-pp-cli sights teamlab --select id,name_ja,sale_rule`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sights (stored registry; no network)")
			}
			sights, err := tickets.Select(strings.Join(args, ","))
			if err != nil {
				return notFoundErr(err)
			}
			view := sightsView{
				Meta:    sightsMeta{Source: "computed", RuleCheckedOn: tickets.RuleCheckedOn, Note: "stored from official sources; onsale and availability re-check them live"},
				Results: sights,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFilteredKeep(cmd.OutOrStdout(), view, flags, "id", "name_en", "name_ja", "city", "sale_rule", "handoff_url", "availability_coverage")
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "ID\tNAME\tNAME (JA)\tCITY\tSALE RULE")
			for _, s := range sights {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.NameEN, s.NameJA, s.City, s.SaleRule)
			}
			return w.Flush()
		},
	}
	return cmd
}
