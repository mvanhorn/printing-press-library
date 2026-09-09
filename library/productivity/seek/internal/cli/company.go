// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: company hiring scan.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/seek/internal/cliutil"
)

type companyOpening struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Location       string `json:"location"`
	SalaryLabel    string `json:"salary_label,omitempty"`
	ListingDate    string `json:"listing_date"`
	Classification string `json:"classification,omitempty"`
}

type companyView struct {
	Query            string           `json:"query"`
	AdvertiserID     string           `json:"advertiser_id,omitempty"`
	AdvertiserName   string           `json:"advertiser_name,omitempty"`
	Site             string           `json:"site"`
	OpeningsCount    int              `json:"openings_count"`
	ScannedListings  int              `json:"scanned_listings"`
	MaxScanPages     int              `json:"max_scan_pages"`
	Openings         []companyOpening `json:"openings"`
	Profile          json.RawMessage  `json:"company_profile,omitempty"`
	CompanySearchURL string           `json:"company_search_url,omitempty"`
	OtherMatches     []string         `json:"other_advertisers_matched,omitempty"`
	Note             string           `json:"note,omitempty"`
}

func newNovelCompanyCmd(flags *rootFlags) *cobra.Command {
	var flagSite string
	var flagActive bool
	var flagMaxScanPages, flagLimit int

	cmd := &cobra.Command{
		Use:   "company <advertiser>",
		Short: "Profile one advertiser and list all their current SEEK openings",
		Long: "Use 'company' to profile an advertiser and enumerate all their current openings\n" +
			"with a local count. Pass a company name or a numeric advertiser ID.\n\n" +
			"For the company block attached to a single posting, use 'listings get <id>'.",
		Example: strings.Trim(`
  seek-pp-cli company "Atlassian" --agent
  seek-pp-cli company 63228293 --site AU-Main --select openings.title,openings.location`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "advertiser=Woolworths", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "company")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an advertiser name or ID is required"))
			}
			query := strings.Join(args, " ")
			wantID := isAllDigits(query)

			maxPages := flagMaxScanPages
			if cliutil.IsDogfoodEnv() && maxPages > 2 {
				maxPages = 2
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			opts := seekSearchOpts{SiteKey: flagSite}
			if !wantID {
				opts.Keywords = query
			}
			if strings.Contains(flagSite, "NZ") {
				opts.Locale = "en-NZ"
			}
			jobs, _, _, err := scanSearch(ctx, c, opts, maxPages)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}

			ql := strings.ToLower(query)

			// Group every advertiser the query could refer to, keyed by their
			// SEEK advertiser ID. A substring query like "acme" must not blend
			// "Acme Corp" and "Acme Logistics" into one result set under one
			// company's profile — resolve to a single advertiser first.
			type advGroup struct {
				id, name string
				jobs     []seekJob
				exact    bool
			}
			groups := map[string]*advGroup{}
			for _, j := range jobs {
				id := j.Advertiser.ID
				if id == "" {
					continue
				}
				name := strings.ToLower(j.Advertiser.Description)
				var hit, exact bool
				if wantID {
					hit = id == query
					exact = hit
				} else {
					exact = name == ql
					hit = exact || strings.Contains(name, ql)
				}
				if !hit {
					continue
				}
				g := groups[id]
				if g == nil {
					g = &advGroup{id: id, name: j.Advertiser.Description}
					groups[id] = g
				}
				g.exact = g.exact || exact
				g.jobs = append(g.jobs, j)
			}

			// Pick one advertiser: an exact name/ID match wins; otherwise the
			// one with the most matched openings on the scanned pages, with the
			// lower ID breaking a tie for a deterministic result.
			var target *advGroup
			for _, g := range groups {
				switch {
				case target == nil:
					target = g
				case g.exact != target.exact:
					if g.exact {
						target = g
					}
				case len(g.jobs) != len(target.jobs):
					if len(g.jobs) > len(target.jobs) {
						target = g
					}
				case g.id < target.id:
					target = g
				}
			}

			var otherMatches []string
			for _, g := range groups {
				if target != nil && g.id != target.id {
					otherMatches = append(otherMatches, fmt.Sprintf("%s (%s, %d opening(s))", g.name, g.id, len(g.jobs)))
				}
			}
			sort.Strings(otherMatches)

			advID, advName := "", ""
			var openings []companyOpening
			if target != nil {
				advID, advName = target.id, target.name
				for _, j := range target.jobs {
					if flagActive {
						if listed, ok := parseSeekDate(j.ListingDate); ok && time.Since(listed) > 28*24*time.Hour {
							continue
						}
					}
					_, cn := j.topClassification()
					openings = append(openings, companyOpening{
						ID: j.ID, Title: j.Title, Location: locationLabel(j),
						SalaryLabel: j.SalaryLabel, ListingDate: j.ListingDate, Classification: cn,
					})
				}
			}
			sort.Slice(openings, func(i, k int) bool { return openings[i].ListingDate > openings[k].ListingDate })

			view := companyView{
				Query: query, AdvertiserID: advID, AdvertiserName: advName,
				Site:          firstNonEmpty(flagSite, "AU-Main"),
				OpeningsCount: len(openings), ScannedListings: len(jobs), MaxScanPages: maxPages,
				OtherMatches:  otherMatches,
			}
			if flagLimit > 0 && len(openings) > flagLimit {
				view.Openings = openings[:flagLimit]
			} else {
				view.Openings = openings
			}
			if view.Openings == nil {
				view.Openings = []companyOpening{}
			}

			if len(openings) > 0 {
				if jd, jErr := fetchJobDetails(ctx, c, openings[0].ID, flagSite); jErr == nil {
					if len(jd.CompanyProfile) > 0 && string(jd.CompanyProfile) != "null" {
						view.Profile = jd.CompanyProfile
					}
					view.CompanySearchURL = jd.CompanySearch
				}
			} else {
				view.Note = fmt.Sprintf("no openings from %q matched advertiser %q on the scanned pages; try the numeric advertiser ID or raise --max-scan-pages.", firstNonEmpty(opts.Keywords, "(broad scan)"), query)
			}
			if len(otherMatches) > 0 {
				more := fmt.Sprintf("%q also matched %d other advertiser(s); showing only %q (%s). Pass the numeric advertiser ID to target a different one.", query, len(otherMatches), advName, advID)
				if view.Note == "" {
					view.Note = more
				} else {
					view.Note += " " + more
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s — %d current opening(s) on %s\n", firstNonEmpty(advName, query), len(openings), view.Site)
			if view.CompanySearchURL != "" {
				fmt.Fprintf(w, "  %s\n", view.CompanySearchURL)
			}
			for _, o := range view.Openings {
				fmt.Fprintf(w, "  %-10s  %s — %s\n", o.ID, o.Title, o.Location)
			}
			for _, om := range view.OtherMatches {
				fmt.Fprintf(w, "  other match (not shown): %s\n", om)
			}
			if view.Note != "" {
				fmt.Fprintf(w, "  note: %s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSite, "site", "AU-Main", "Marketplace: AU-Main or NZ-Main")
	cmd.Flags().BoolVar(&flagActive, "active", false, "Only openings listed in the last 28 days (drops near-expiry ads; rarely changes the result set)")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 10, "Maximum search pages to scan for this advertiser's openings")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum openings to return (0 = all)")
	return cmd
}

func isAllDigits(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func locationLabel(j seekJob) string {
	if len(j.Locations) == 0 {
		return ""
	}
	return j.Locations[0].Label
}
