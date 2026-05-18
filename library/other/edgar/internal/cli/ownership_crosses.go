// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// `edgar-pp-cli ownership-crosses <TICKER>` — enumerate 13D/13G filings.

package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/edgar/internal/store"
	"github.com/spf13/cobra"
)

type ownershipCross struct {
	Accession    string  `json:"accession"`
	Filer        string  `json:"filer,omitempty"`
	Form         string  `json:"form"`
	PercentOwned float64 `json:"percent_owned,omitempty"`
	FiledAt      string  `json:"filed_at"`
	Error        string  `json:"error,omitempty"`
	Reason       string  `json:"reason,omitempty"`
}

var percentOwnedRE = regexp.MustCompile(`(?is)percent\s+of\s+class.{0,200}?(\d+\.?\d*)\s*%`)
var filerHeaderRE = regexp.MustCompile(`(?is)NAME\s+OF\s+REPORTING\s+PERSON.{0,200}?\n\s*([A-Z][A-Za-z0-9 ,.\&\-']{2,80})`)

func newOwnershipCrossesCmd(flags *rootFlags) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:         "ownership-crosses <ticker-or-cik>",
		Short:       "Enumerate 13D/13G/13D-A/13G-A filings against an issuer (someone else crosses 5%)",
		Example:     "  edgar-pp-cli ownership-crosses AAPL --since 1y",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := store.OpenWithContext(cmd.Context(), edgarDBPath())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := db.EnsureEdgarSchema(cmd.Context()); err != nil {
				return err
			}
			ec, err := resolveCIKOrTicker(cmd.Context(), c, db, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			sinceISO, perr := parseSinceDate(since)
			if perr != nil {
				return usageErr(perr)
			}
			if _, ferr := fetchSubmissions(cmd.Context(), c, db, ec.CIK); ferr != nil {
				return classifyAPIError(ferr, flags)
			}
			filings, err := db.ListEdgarFilings(cmd.Context(), ec.CIK,
				[]string{"SC 13D", "SC 13G", "SC 13D/A", "SC 13G/A"}, sinceISO, 100)
			if err != nil {
				return err
			}
			var out []ownershipCross
			for _, f := range filings {
				cached, _ := db.GetEdgarFiling(cmd.Context(), f.Accession)
				body := cached.BodyText
				if body == "" && cached.PrimaryDocURL != "" {
					body, _ = fetchFilingBody(cmd.Context(), c, db, &cached)
				}
				oc := ownershipCross{Accession: f.Accession, Form: f.FormType, FiledAt: f.FiledAt}
				if body == "" {
					oc.Error = "percent_unparseable"
					oc.Reason = "primary document body unavailable"
					out = append(out, oc)
					continue
				}
				// Parse percent
				m := percentOwnedRE.FindStringSubmatch(body)
				if len(m) < 2 {
					oc.Error = "percent_unparseable"
					oc.Reason = "no 'percent of class' field found"
				} else {
					v, perr := strconv.ParseFloat(m[1], 64)
					if perr != nil {
						oc.Error = "percent_unparseable"
						oc.Reason = "failed to parse percent value: " + perr.Error()
					} else {
						oc.PercentOwned = v
					}
				}
				// Parse filer name
				if fm := filerHeaderRE.FindStringSubmatch(body); len(fm) >= 2 {
					oc.Filer = strings.TrimSpace(fm[1])
				}
				out = append(out, oc)
			}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Earliest filing date (ISO or 90d/12mo/1y)")
	return cmd
}
