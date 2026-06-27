// Copyright 2026 Paul Gradeff and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: agenda keyword scan.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// agendaMatch is a notice whose agenda/title contained the search term.
type agendaMatch struct {
	pmnNotice
	Snippet string `json:"snippet"`
}

// pp:data-source live
func newNovelAgendaScanCmd(flags *rootFlags) *cobra.Command {
	var flagLocation string
	var flagDays int
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "scan <term>",
		Short: "Search inline agenda text for a project, parcel, or applicant",
		Long: "Search the inline agenda and title text of upcoming notices for a term (e.g. a project " +
			"name, parcel, or applicant) and show the surrounding context. With --location, scans one " +
			"ZIP/city; without it, sweeps all Millard County towns.",
		Example:     "  utah-pmn-pp-cli agenda scan \"subdivision\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan agendas for the given term")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search term is required"))
			}
			term := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			start, end := dateWindow(flagDays)
			var notices []pmnNotice
			if flagLocation != "" {
				notices, err = fetchNotices(ctx, c, flagLocation, start, end, flagLimit)
				sortNoticesByDate(notices)
			} else {
				notices, err = sweepLocations(ctx, c, millardCityNames(), start, end, flagLimit)
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			needle := strings.ToLower(term)
			matches := make([]agendaMatch, 0)
			for _, n := range notices {
				hay := n.MeetingAgenda + "\n" + n.MeetingTitle
				if idx := strings.Index(strings.ToLower(hay), needle); idx >= 0 {
					matches = append(matches, agendaMatch{pmnNotice: n, Snippet: snippetAround(hay, idx, len(term))})
				}
			}
			b, err := json.Marshal(matches)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagLocation, "location", "", "ZIP or city to scan (default: all Millard County towns)")
	cmd.Flags().IntVar(&flagDays, "days", 90, "Days ahead to scan from today")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max notices per location before scanning")
	return cmd
}

// snippetAround returns ~60 chars of context on each side of a match.
func snippetAround(s string, idx, matchLen int) string {
	const pad = 60
	lo := idx - pad
	if lo < 0 {
		lo = 0
	}
	hi := idx + matchLen + pad
	if hi > len(s) {
		hi = len(s)
	}
	snip := strings.TrimSpace(s[lo:hi])
	snip = strings.ReplaceAll(snip, "\r", " ")
	snip = strings.ReplaceAll(snip, "\n", " ")
	if lo > 0 {
		snip = "…" + snip
	}
	if hi < len(s) {
		snip = snip + "…"
	}
	return snip
}
