// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cdcparse

import (
	"fmt"
	"regexp"
	"strings"

	xhtml "golang.org/x/net/html"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/cliutil"
)

// StatMetric is one row of the CDS aggregate statistics table.
//
// The table is OVERWRITTEN monthly, so a given month's figures cease to exist
// once the next month lands. Rows are therefore append-only snapshots, never a
// live query.
type StatMetric struct {
	// Label is CDC's own row text, preserved verbatim for audit.
	Label string `json:"label"`
	// Key is the normalised, drift-resistant identifier. Row labels DO change:
	// between Apr-2024 and Jul-2026 "Number of Shares" became "Number of
	// Shares/Debt Instruments", "Number of Securities" SPLIT into Listed and
	// Unlisted, and the "Sahulat Accounts" row was DELETED. Keying on the raw
	// label would silently start a new series on every rewording.
	Key string `json:"key"`
	// Raw is the cell text exactly as published, e.g.
	// "PKR 21.8 Trillion / USD 77.90 Billion".
	Raw string `json:"raw"`
	// Known is false when the label matched no rule. Unknown rows are KEPT with
	// Known=false rather than dropped -- a dropped row is an invisible loss.
	Known bool `json:"known"`
}

// StatSnapshot is one capture of the whole table.
type StatSnapshot struct {
	// AsOf is the table's own period text, e.g. "July-2026", taken from the
	// "Facts (As of ...)" header cell. It is NOT the fetch date.
	AsOf    string       `json:"as_of"`
	Metrics []StatMetric `json:"metrics"`
	// UnknownLabels lists labels that matched no rule, so drift surfaces as data
	// rather than as a silent gap.
	UnknownLabels []string `json:"unknown_labels,omitempty"`
}

var asOfRe = regexp.MustCompile(`(?i)as\s+of\s+([A-Za-z]+[-\s]?\d{4})`)

// labelRules maps normalised label fragments to a stable key.
//
// ORDER IS LOAD-BEARING and two orderings here are bug fixes, not style:
//   - "unlisted" MUST precede "listed", because "Securities-Unlisted" contains
//     the substring "listed" and otherwise collapses onto securities_listed --
//     which produced two rows with the same key and silently lost 59,217.
//   - "share registrar" MUST precede "number of securities", because
//     "Total Number of Securities under Share Registrar" contains the latter and
//     was being keyed as a securities count rather than a registrar count.
//
// `excludes` guards the remaining ambiguous cases explicitly.
var labelRules = []struct {
	contains []string
	excludes []string
	key      string
}{
	{[]string{"sub", "account", "individual"}, nil, "sub_accounts_individual"},
	{[]string{"sub", "account", "corporate"}, nil, "sub_accounts_corporate"},
	{[]string{"sahulat"}, nil, "sahulat_accounts"},
	{[]string{"investor account", "individual"}, nil, "investor_accounts_individual"},
	{[]string{"investor account", "corporate"}, nil, "investor_accounts_corporate"},
	{[]string{"shares in investor account"}, nil, "shares_in_investor_accounts"},
	{[]string{"number of rda"}, nil, "rda_accounts"},
	{[]string{"investment through rda"}, nil, "rda_investment"},
	// share-registrar BEFORE any generic securities-count rule
	{[]string{"share registrar"}, nil, "securities_under_share_registrar"},
	{[]string{"transfer agent"}, nil, "securities_under_share_registrar"},
	// unlisted BEFORE listed
	{[]string{"securities", "unlisted"}, nil, "securities_unlisted"},
	{[]string{"securities", "listed"}, []string{"unlisted"}, "securities_listed"},
	{[]string{"number of securities"}, []string{"registrar", "transfer agent"}, "securities_total"},
	{[]string{"percentage", "cds"}, nil, "pct_shares_in_cds_excl_gop"},
	{[]string{"trusteeship"}, nil, "funds_under_trusteeship"},
	{[]string{"net assets", "custody"}, nil, "net_assets_in_custody"},
	{[]string{"value of securities"}, nil, "value_of_securities"},
	{[]string{"value of shares"}, nil, "value_of_securities"},
	{[]string{"number of shares"}, nil, "number_of_shares"},
}

// NormaliseLabel maps a published row label onto a stable key. It returns
// known=false for an unrecognised label so the caller can keep it and flag it.
func NormaliseLabel(label string) (string, bool) {
	l := strings.ToLower(cliutil.CleanText(label))
	l = strings.ReplaceAll(l, "–", "-")
	for _, r := range labelRules {
		all := true
		for _, frag := range r.contains {
			if !strings.Contains(l, frag) {
				all = false
				break
			}
		}
		if !all {
			continue
		}
		blocked := false
		for _, ex := range r.excludes {
			if strings.Contains(l, ex) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		return r.key, true
	}
	return "", false
}

// ParseStatistics extracts the aggregate table from the statistics page.
func ParseStatistics(body string) (*StatSnapshot, error) {
	if IsChallenge(body) {
		return nil, ErrChallenge
	}
	root, err := xhtml.Parse(strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parsing statistics page: %w", err)
	}

	snap := &StatSnapshot{Metrics: []StatMetric{}}
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "tr" {
			var cells []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == xhtml.ElementNode && (c.Data == "td" || c.Data == "th") {
					cells = append(cells, cliutil.CleanText(textOf(c)))
				}
			}
			if len(cells) >= 2 && cells[0] != "" {
				if m := asOfRe.FindStringSubmatch(cells[1]); m != nil && snap.AsOf == "" {
					// This is the header row: "Information | Facts (As of July-2026)".
					snap.AsOf = strings.TrimSpace(m[1])
				} else {
					key, known := NormaliseLabel(cells[0])
					if !known {
						snap.UnknownLabels = append(snap.UnknownLabels, cells[0])
					}
					snap.Metrics = append(snap.Metrics, StatMetric{
						Label: cells[0], Key: key, Raw: cells[1], Known: known,
					})
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)

	if len(snap.Metrics) == 0 {
		return nil, fmt.Errorf("statistics page yielded no metric rows (layout may have changed)")
	}
	return snap, nil
}
