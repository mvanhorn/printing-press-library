// Hand-authored novel feature. Body is hand-written; survives regen via regen-merge.
// pp:data-source live
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type wbCompareRow struct {
	Country     string   `json:"country"`
	CountryCode string   `json:"country_code"`
	Date        string   `json:"date"`
	Value       *float64 `json:"value"`
	DeltaVsBase *float64 `json:"delta_vs_base,omitempty"`
	PctVsBase   *float64 `json:"pct_vs_base,omitempty"`
}

type wbCompareView struct {
	Indicator string         `json:"indicator"`
	Baseline  string         `json:"baseline"`
	Date      string         `json:"date,omitempty"`
	Rows      []wbCompareRow `json:"rows"`
}

func buildCompareView(indicator, countries string, obs []wbObservation) (wbCompareView, error) {
	byCountry := make(map[string]map[string]wbObservation)
	firstSeen := make([]string, 0)
	for _, observation := range obs {
		if observation.Value == nil || observation.CountryISO3Code == "" || observation.Date == "" {
			continue
		}
		code := observation.CountryISO3Code
		if byCountry[code] == nil {
			byCountry[code] = make(map[string]wbObservation)
			firstSeen = append(firstSeen, code)
		}
		byCountry[code][observation.Date] = observation
	}

	orderedCodes := make([]string, 0, len(byCountry))
	seen := make(map[string]bool)
	for _, requestedCode := range strings.Split(countries, ";") {
		requestedCode = strings.ToUpper(strings.TrimSpace(requestedCode))
		if requestedCode == "" {
			return wbCompareView{}, fmt.Errorf("country list contains an empty code")
		}
		matched := false
		for _, code := range firstSeen {
			matchesRequested := strings.EqualFold(code, requestedCode)
			for _, candidate := range byCountry[code] {
				if strings.EqualFold(candidate.Country.ID, requestedCode) {
					matchesRequested = true
				}
			}
			if matchesRequested {
				if !seen[code] {
					orderedCodes = append(orderedCodes, code)
					seen[code] = true
				}
				matched = true
				break
			}
		}
		if !matched {
			return wbCompareView{}, fmt.Errorf("no non-null observations found for country %s", requestedCode)
		}
	}
	if len(orderedCodes) == 0 {
		return wbCompareView{}, fmt.Errorf("no non-null observations found for the requested countries")
	}

	// Compute the newest date present for every country. Comparing independent
	// "latest" values can silently subtract different years, which is not a
	// meaningful cross-country delta.
	commonDates := make(map[string]bool)
	for date := range byCountry[orderedCodes[0]] {
		commonDates[date] = true
	}
	for _, code := range orderedCodes[1:] {
		for date := range commonDates {
			if _, ok := byCountry[code][date]; !ok {
				delete(commonDates, date)
			}
		}
	}
	alignedDate := ""
	for date := range commonDates {
		if date > alignedDate {
			alignedDate = date
		}
	}
	if alignedDate == "" {
		return wbCompareView{}, fmt.Errorf("no common observation year exists for all requested countries")
	}

	view := wbCompareView{Indicator: indicator, Date: alignedDate}
	var baseVal *float64
	for _, code := range orderedCodes {
		o := byCountry[code][alignedDate]
		row := wbCompareRow{Country: o.Country.Value, CountryCode: o.CountryISO3Code, Date: alignedDate, Value: o.Value}
		if baseVal == nil {
			baseVal = o.Value
			view.Baseline = o.Country.Value
		} else {
			delta := *o.Value - *baseVal
			row.DeltaVsBase = &delta
			if *baseVal != 0 {
				percent := delta / *baseVal * 100
				row.PctVsBase = &percent
			}
		}
		view.Rows = append(view.Rows, row)
	}
	return view, nil
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var flagDate string

	cmd := &cobra.Command{
		Use:         "compare <indicator> <country;country;...>",
		Short:       "Line up one indicator across countries with deltas vs a baseline.",
		Long:        "Line up one indicator across countries in a single aligned table with deltas vs the first (baseline) country.\nUse this to compare economies on one indicator. Do NOT use it for one country's history; use 'trend'.",
		Example:     "  world-bank-pp-cli compare NY.GDP.MKTP.CD USA;CHN;IND --date 2024",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("indicator and country list are required"))
			}
			indicator := args[0]
			// Accept comma- or semicolon-separated country lists; the API wants ';'.
			countries := strings.ReplaceAll(args[1], ",", ";")

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			extra := map[string]string{}
			if flagDate != "" {
				extra["date"] = flagDate
			} else {
				// Fetch enough history to find the newest year shared by every
				// country instead of comparing mismatched independent latest years.
				extra["mrv"] = "25"
			}
			obs, err := wbFetchObservations(ctx, c, countries, indicator, extra, 10)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view, err := buildCompareView(indicator, countries, obs)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Year or range (e.g. 2024). Defaults to most recent value.")
	return cmd
}
