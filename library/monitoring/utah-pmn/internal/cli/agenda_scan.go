// Copyright 2026 Paul Gradeff and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: agenda keyword scan.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

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
			matches := make([]agendaMatch, 0)
			for _, n := range notices {
				if m, ok := matchAgendaTerm(n, term); ok {
					matches = append(matches, m)
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

// matchAgendaTerm reports whether term appears in the notice agenda or title.
// Matching is case-insensitive; the snippet is taken from the original text.
func matchAgendaTerm(n pmnNotice, term string) (agendaMatch, bool) {
	needle := strings.ToLower(term)
	if needle == "" {
		return agendaMatch{}, false
	}
	hay := n.MeetingAgenda + "\n" + n.MeetingTitle
	lowered, origAt := foldWithOrigMap(hay)
	idx := strings.Index(lowered, needle)
	if idx < 0 {
		return agendaMatch{}, false
	}
	origIdx := origAt[idx]
	origEnd := origAt[idx+len(needle)]
	return agendaMatch{pmnNotice: n, Snippet: snippetAround(hay, origIdx, origEnd-origIdx)}, true
}

// foldWithOrigMap lowercases s with Unicode special casing and records, for
// each lowered byte, the original byte offset of the rune that produced it.
// origAt has length len(lowered)+1; the last entry is len(s).
func foldWithOrigMap(s string) (string, []int) {
	var b strings.Builder
	b.Grow(len(s))
	origAt := make([]int, 0, len(s)+1)
	for i := 0; i < len(s); {
		_, size := utf8.DecodeRuneInString(s[i:])
		folded := strings.ToLower(s[i : i+size])
		for j := 0; j < len(folded); j++ {
			origAt = append(origAt, i)
		}
		b.WriteString(folded)
		i += size
	}
	origAt = append(origAt, len(s))
	return b.String(), origAt
}

// snippetAround returns ~60 bytes of context on each side of a match.
// idx and matchLen are byte offsets into s (the same string that was searched).
func snippetAround(s string, idx, matchLen int) string {
	const pad = 60
	if idx < 0 {
		idx = 0
	}
	if idx > len(s) {
		idx = len(s)
	}
	matchEnd := idx + matchLen
	if matchEnd < idx || matchEnd > len(s) {
		matchEnd = len(s)
	}
	lo := idx - pad
	if lo < 0 {
		lo = 0
	}
	for lo > 0 && !utf8.RuneStart(s[lo]) {
		lo--
	}
	hi := matchEnd + pad
	if hi > len(s) {
		hi = len(s)
	}
	for hi < len(s) && !utf8.RuneStart(s[hi]) {
		hi++
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
