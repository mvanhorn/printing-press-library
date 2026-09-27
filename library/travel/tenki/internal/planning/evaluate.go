package planning

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/tenki"
)

var jst = time.FixedZone("JST", 9*60*60)

type criterionSpec struct {
	key, unit, operator string
	threshold           *float64
}

func criteriaSpecs(criteria Criteria, hourly bool) []criterionSpec {
	rain := "mm/source daily period"
	if hourly {
		rain = "mm/h per preceding source hour"
	}
	return []criterionSpec{{"max-pop", "% per source interval", "<=", criteria.MaxPOP}, {"max-rain", rain, "<=", criteria.MaxRain}, {"min-temp", "°C morning low (daily) or instant (hourly)", ">=", criteria.MinTemp}, {"max-temp", "°C daytime high (daily) or instant (hourly)", "<=", criteria.MaxTemp}, {"max-wind", "m/s at source instants", "<=", criteria.MaxWind}}
}

// Validate checks computation bounds even for callers that do not use Cobra.
func Validate(options Options, places int) error {
	if places < 1 || places > 5 {
		return fmt.Errorf("--place must be supplied 1–5 times")
	}
	date, err := time.ParseInLocation("2006-01-02", options.From, jst)
	if err != nil || date.Format("2006-01-02") != options.From {
		return fmt.Errorf("--from must use YYYY-MM-DD")
	}
	if options.Days < 1 || options.Days > 14 {
		return fmt.Errorf("--days must be between 1 and 14")
	}
	if options.Limit < 1 || options.Limit > 70 {
		return fmt.Errorf("--limit must be between 1 and 70")
	}
	if options.Hours != nil && (options.Hours.Start < 0 || options.Hours.End > 24 || options.Hours.Start >= options.Hours.End) {
		return fmt.Errorf("--hours must have increasing whole-hour endpoints within 00:00-24:00")
	}
	count := 0
	chosen := false
	for _, spec := range criteriaSpecs(options.Criteria, options.Hours != nil) {
		if spec.threshold == nil {
			continue
		}
		count++
		if math.IsNaN(*spec.threshold) || math.IsInf(*spec.threshold, 0) {
			return fmt.Errorf("--%s must be finite", spec.key)
		}
		if spec.key == "max-pop" && (*spec.threshold < 0 || *spec.threshold > 100) {
			return fmt.Errorf("--max-pop must be between 0 and 100 percent")
		}
		if (spec.key == "max-rain" || spec.key == "max-wind") && *spec.threshold < 0 {
			return fmt.Errorf("--%s must be nonnegative", spec.key)
		}
		if spec.key == options.Sort {
			chosen = true
		}
	}
	if count == 0 {
		return fmt.Errorf("supply at least one criterion: --max-pop, --max-rain, --min-temp, --max-temp or --max-wind")
	}
	if options.Criteria.MinTemp != nil && options.Criteria.MaxTemp != nil && *options.Criteria.MinTemp > *options.Criteria.MaxTemp {
		return fmt.Errorf("--min-temp cannot exceed --max-temp")
	}
	if options.Sort != "" && !chosen {
		return fmt.Errorf("--sort must name a supplied criterion: max-pop, max-rain, min-temp, max-temp or max-wind")
	}
	if options.Season != "" && options.Season != "sakura" && options.Season != "kouyou" {
		return fmt.Errorf("--season must be sakura or kouyou")
	}
	if options.SeasonCondition != "" && options.Season == "" {
		return fmt.Errorf("--season-condition requires --season")
	}
	if options.Season != "" {
		if err := tenki.ValidateSeasonYear(options.Year); err != nil {
			return fmt.Errorf("--year: %w", err)
		}
	}
	return nil
}

func Evaluate(inputs []Input, options Options) (Result, error) {
	if err := Validate(options, len(inputs)); err != nil {
		return Result{}, err
	}
	if options.Sort == "" {
		for _, spec := range criteriaSpecs(options.Criteria, options.Hours != nil) {
			if spec.threshold != nil {
				options.Sort = spec.key
				break
			}
		}
	}
	result := Result{Cells: []Cell{}, Sort: options.Sort, SortDirection: "ascending", SortSemantics: "status: meets_criteria, fails_criteria, insufficient_data; then chosen observed criterion; then date and canonical place URL. Ties mean equal status and chosen observed value.", RequestedCells: len(inputs) * options.Days, Criteria: options.Criteria, From: options.From, Days: options.Days, RainUnit: "mm/source daily period", Assumptions: []string{"Thresholds describe user preferences; no universal score or safety certification.", "Only fresh, source-compatible forecast evidence can pass; missing values remain unknown.", "Daily low is the source morning low; daily high is the source daytime high.", "Maximum interval precipitation probability is not the probability of any rain during the outing."}}
	if options.Sort == "min-temp" {
		result.SortDirection = "descending"
	}
	if options.Hours != nil {
		result.Hours = fmt.Sprintf("%02d:00-%02d:00", options.Hours.Start, options.Hours.End)
		result.RainUnit = "mm/h per preceding source hour"
		result.Assumptions = append(result.Assumptions, "Rain uses complete hourly intervals inside the requested window; temperature/wind instants include both endpoints. No daily or six-hour substitution.")
	} else if options.Criteria.MaxWind != nil {
		result.Assumptions = append(result.Assumptions, "Daily wind reduction is max_of_five_source_instants (00/06/12/18/24), not a continuous daily maximum.")
	}
	start, _ := time.ParseInLocation("2006-01-02", options.From, jst)
	for _, input := range inputs {
		for offset := 0; offset < options.Days; offset++ {
			result.Cells = append(result.Cells, evaluateCell(input, start.AddDate(0, 0, offset), options))
		}
	}
	sortCells(result.Cells, options.Sort)
	markTies(result.Cells)
	result.Truncated = len(result.Cells) > options.Limit
	if result.Truncated {
		result.Cells = result.Cells[:options.Limit]
	}
	result.ReturnedCells = len(result.Cells)
	return result, nil
}

func evaluateCell(input Input, date time.Time, options Options) Cell {
	day := date.Format("2006-01-02")
	cell := Cell{Place: input.Place, Date: day, Status: "insufficient_data", WeatherScope: input.Weather.Place.Scope, WeatherSource: input.Weather.Source, SourceStatus: nonempty(input.Weather.Status, "unavailable"), CoverageStart: input.Weather.CoverageStart, CoverageEnd: input.Weather.CoverageEnd, Criteria: map[string]CriterionResult{}, Assumptions: []string{}, Reasons: []string{}}
	compatible, scopeReason := compatibleForecast(input)
	if !compatible {
		cell.Reasons = append(cell.Reasons, scopeReason)
	}
	fresh := sourceFresh(input.Weather.Source)
	if !fresh {
		cell.Reasons = append(cell.Reasons, "Weather source/cache is stale or freshness is unknown.")
	}
	if input.WeatherError != "" {
		cell.Reasons = append(cell.Reasons, "Weather fetch failed: "+input.WeatherError)
	}
	if input.Weather.Status != "ok" {
		cell.Reasons = append(cell.Reasons, "Source availability: "+nonempty(input.Weather.Status, "unavailable"))
	}
	if input.Place.Kind != "municipality" {
		cell.Assumptions = append(cell.Assumptions, "Weather criteria evaluate the linked municipality, not exact-site weather.")
	}
	anyFail, allPass := false, true
	criteriaCount, knownCount := 0, 0
	for _, spec := range criteriaSpecs(options.Criteria, options.Hours != nil) {
		if spec.threshold == nil {
			continue
		}
		criteriaCount++
		criterion := evaluateCriterion(input.Weather, date, options.Hours, spec, fresh && input.WeatherError == "" && input.Weather.Status == "ok" && compatible)
		cell.Criteria[spec.key] = criterion
		if criterion.Verdict != "unknown" {
			knownCount++
		}
		if criterion.Verdict == "fail" {
			anyFail = true
		}
		if criterion.Verdict != "pass" {
			allPass = false
		}
	}
	cell.CriteriaCoverage = coverage(criteriaCount, knownCount)
	if options.Season != "" {
		cell.Seasonal = evaluateSeason(input, day, options)
		if cell.Seasonal.Verdict == "fail" {
			anyFail = true
		}
		if cell.Seasonal.Verdict != "pass" {
			allPass = false
		}
	}
	// A mountain identity asks for destination-level evidence. A municipal
	// threshold failure/pass cannot establish a summit-level conclusion.
	if !compatible {
		cell.Status = "insufficient_data"
	} else if anyFail {
		cell.Status = "fails_criteria"
	} else if allPass {
		cell.Status = "meets_criteria"
	}
	if selected, ok := cell.Criteria[options.Sort]; ok {
		cell.SortValue = selected.Value
	}
	return cell
}

func compatibleForecast(input Input) (bool, string) {
	if input.Place.Kind == "mountain" || input.Place.Scope == "foothill" || input.Place.Scope == "nearby_model" {
		return false, "Destination forecast level mismatch: mountain weather is foothill/nearby model evidence, not a summit forecast. Compare the municipality URL explicitly for foothill sightseeing."
	}
	expected := input.Place.ForecastReferenceURL
	if expected == "" && input.Place.Kind == "municipality" {
		expected = input.Place.URL
	}
	actual := input.Weather.Place.ForecastReferenceURL
	if actual == "" {
		actual = input.Weather.Place.URL
	}
	if expected == "" || actual == "" || expected != actual {
		return false, "Forecast reference municipality is absent or does not match the destination's source-linked municipality."
	}
	if input.Weather.Place.Scope != "municipal" {
		return false, "Forecast scope is not municipality-level weather."
	}
	return true, ""
}

func sourceFresh(source tenki.Source) bool {
	return source.URL != "" && source.Freshness == "fresh" && !source.CacheStale && !source.SourceStale
}

func evaluateCriterion(forecast tenki.ForecastResult, date time.Time, window *Window, spec criterionSpec, trusted bool) CriterionResult {
	result := CriterionResult{Threshold: *spec.threshold, Operator: spec.operator, Unit: spec.unit, Verdict: "unknown", Evidence: []EvidenceValue{}, Reasons: []string{}, Reduction: criterionReduction(spec.key, window != nil)}
	var samples []tenki.Period
	expected := 1
	complete := true
	ambiguous := false
	if window != nil {
		start := date.Add(time.Duration(window.Start) * time.Hour)
		end := date.Add(time.Duration(window.End) * time.Hour)
		rain := spec.key == "max-pop" || spec.key == "max-rain"
		expected = window.End - window.Start
		if !rain {
			expected++
		}
		byTime := map[string]tenki.Period{}
		duplicate := map[string]bool{}
		for _, period := range forecast.Periods {
			at, err := time.Parse(time.RFC3339, period.ValidAt)
			if rain {
				at, err = time.Parse(time.RFC3339, period.End)
			}
			if err != nil || at.Before(start) || at.After(end) {
				continue
			}
			if rain {
				beg, e := time.Parse(time.RFC3339, period.Start)
				if e != nil || !beg.Equal(at.Add(-time.Hour)) || beg.Before(start) {
					continue
				}
			}
			key := at.Format(time.RFC3339)
			if _, exists := byTime[key]; exists {
				duplicate[key] = true
			}
			byTime[key] = period
		}
		first := window.Start
		if rain {
			first++
		}
		for hour := first; hour <= window.End; hour++ {
			key := date.Add(time.Duration(hour) * time.Hour).Format(time.RFC3339)
			if period, ok := byTime[key]; ok && !duplicate[key] {
				samples = append(samples, period)
			} else {
				complete = false
			}
		}
	} else {
		day := date.Format("2006-01-02")
		if spec.key == "max-wind" {
			expected = 5
			byTime := map[string]tenki.Period{}
			duplicates := map[string]bool{}
			for _, period := range forecast.Instants {
				if period.Date == day {
					if _, ok := byTime[period.ValidAt]; ok {
						duplicates[period.ValidAt] = true
					}
					byTime[period.ValidAt] = period
				}
			}
			for _, hour := range []int{0, 6, 12, 18, 24} {
				key := date.Add(time.Duration(hour) * time.Hour).Format(time.RFC3339)
				if period, ok := byTime[key]; ok && !duplicates[key] {
					samples = append(samples, period)
				} else {
					complete = false
				}
			}
		} else {
			for _, period := range forecast.Periods {
				if period.Date == day {
					samples = append(samples, period)
				}
			}
			if len(samples) != 1 {
				complete = false
				ambiguous = len(samples) > 1
			}
		}
	}
	available := 0
	hasFailure := false
	for _, period := range samples {
		value := criterionValue(period, spec.key, window != nil)
		kind := period.Kind
		start := period.Start
		if spec.key == "max-pop" && period.WeatherProbabilityKind != "" {
			kind = period.WeatherProbabilityKind
			if period.WeatherProbabilityFrom != "" {
				start = period.WeatherProbabilityFrom
			}
		}
		if (spec.key == "min-temp" || spec.key == "max-temp") && period.TemperatureKind != "" {
			kind = period.TemperatureKind
		}
		result.Evidence = append(result.Evidence, EvidenceValue{Value: value, Date: period.Date, Start: start, End: period.End, ValidAt: period.ValidAt, Kind: kind, RecordKind: period.Kind, Partial: period.Partial, Confidence: period.Confidence})
		if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) || kind != "forecast" {
			complete = false
			continue
		}
		if result.Value == nil || (spec.operator == "<=" && *value > *result.Value) || (spec.operator == ">=" && *value < *result.Value) {
			v := *value
			result.Value = &v
		}
		failed := spec.operator == "<=" && *value > result.Threshold || spec.operator == ">=" && *value < result.Threshold
		if failed {
			hasFailure = true
		}
		if period.Partial {
			complete = false
		} else {
			available++
		}
	}
	if available > expected {
		available = expected
	}
	result.Coverage = coverage(expected, available)
	if ambiguous {
		result.Reasons = append(result.Reasons, "Conflicting duplicate daily records prevent a forecast decision.")
		return result
	}
	if !trusted {
		result.Reasons = append(result.Reasons, "Source freshness, availability or municipality scope does not support a current forecast decision.")
		return result
	}
	if hasFailure {
		result.Verdict = "fail"
		result.Reasons = append(result.Reasons, "At least one supplied forecast value violates the threshold.")
		if !complete || available != expected {
			result.Reasons = append(result.Reasons, "Other requested evidence is incomplete; failure uses the explicit violating source value.")
		}
		return result
	}
	if complete && available == expected {
		result.Verdict = "pass"
		return result
	}
	result.Reasons = append(result.Reasons, "Missing, duplicate, partial or past-estimated source samples prevent a positive result.")
	return result
}

func criterionValue(period tenki.Period, key string, hourly bool) *float64 {
	switch key {
	case "max-pop":
		return period.PrecipProbabilityPct
	case "max-rain":
		if hourly {
			return period.PrecipRateMMH
		}
		return period.PrecipAmountMM
	case "min-temp":
		if hourly {
			return period.TemperatureC
		}
		return period.MinTemperatureC
	case "max-temp":
		if hourly {
			return period.TemperatureC
		}
		return period.MaxTemperatureC
	case "max-wind":
		return period.WindSpeedMS
	}
	return nil
}

func criterionReduction(key string, hourly bool) string {
	switch key {
	case "max-pop":
		if hourly {
			return "max_of_source_interval_probabilities"
		}
		return "source_daily_probability"
	case "max-rain":
		if hourly {
			return "max_of_source_hourly_rates"
		}
		return "source_daily_amount"
	case "min-temp":
		if hourly {
			return "min_of_source_temperature_instants"
		}
		return "source_morning_low"
	case "max-temp":
		if hourly {
			return "max_of_source_temperature_instants"
		}
		return "source_daytime_high"
	case "max-wind":
		if hourly {
			return "max_of_source_wind_instants"
		}
		return "max_of_five_source_instants"
	}
	return "unknown"
}

func evaluateSeason(input Input, date string, options Options) *SeasonalEvidence {
	result := &SeasonalEvidence{Verdict: "unknown", RequestedYear: options.Year, Evidence: input.Season, Reasons: []string{}, ConditionPreference: options.SeasonCondition, EvidenceKind: "unavailable"}
	if options.SeasonCondition == "" {
		result.Reasons = append(result.Reasons, "No seasonal-condition preference was supplied; this pairs applicable seasonal evidence with weather criteria.")
	}
	if input.SeasonError != "" {
		result.Reasons = append(result.Reasons, "Seasonal fetch failed: "+input.SeasonError)
		return result
	}
	if input.Season == nil {
		result.Reasons = append(result.Reasons, "No seasonal evidence for the selected destination.")
		return result
	}
	season := input.Season
	if season.Kind != options.Season || season.Year != options.Year || season.Spot.Year != options.Year || !strings.HasPrefix(date, fmt.Sprintf("%04d-", options.Year)) {
		result.Reasons = append(result.Reasons, "Seasonal product/requested year/date year mismatch.")
		return result
	}
	if season.Status != "ok" || season.UpdateState != "active" || !sourceFresh(season.Source) {
		result.Reasons = append(result.Reasons, "Seasonal source is unavailable, ended, inactive, stale or unknown freshness.")
		return result
	}
	expected := input.Place.ForecastReferenceURL
	actual := season.Place.ForecastReferenceURL
	if actual == "" {
		actual = season.Spot.Place.ForecastReferenceURL
	}
	if expected == "" || actual != expected {
		result.Reasons = append(result.Reasons, "Seasonal spot's source-linked forecast municipality does not match this weather pairing.")
		return result
	}
	if season.Place.URL != input.Place.URL && season.Spot.Place.URL != input.Place.URL {
		result.Reasons = append(result.Reasons, "Seasonal evidence belongs to a different spot.")
		return result
	}
	condition := season.Spot.Condition
	if season.Spot.ReportDate == date && condition != "" {
		result.EvidenceKind = "seasonal_report"
	} else {
		// Only exact source-backed predicted dates have day precision. A
		// normal/fuzzy best-period string cannot certify a selected date.
		if season.Spot.PredictedFullBloomDate == date {
			condition = "満開"
			result.EvidenceKind = "seasonal_prediction"
		} else if season.Spot.PredictedFloweringDate == date {
			condition = "開花"
			result.EvidenceKind = "seasonal_prediction"
		} else {
			result.Reasons = append(result.Reasons, "Current report is not a prediction for this date; no applicable exact source prediction is available.")
			return result
		}
	}
	result.Verdict = "pass"
	if options.SeasonCondition != "" && condition != options.SeasonCondition {
		result.Verdict = "fail"
		result.Reasons = append(result.Reasons, "Source condition differs from the explicitly requested exact seasonal state.")
	}
	return result
}

func coverage(expected, available int) Coverage {
	ratio := 0.0
	if expected > 0 {
		ratio = float64(available) / float64(expected)
	}
	return Coverage{Expected: expected, Available: available, Ratio: ratio}
}
func nonempty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
func statusOrder(status string) int {
	switch status {
	case "meets_criteria":
		return 0
	case "fails_criteria":
		return 1
	default:
		return 2
	}
}

func sortCells(cells []Cell, key string) {
	sort.SliceStable(cells, func(i, j int) bool {
		a, b := cells[i], cells[j]
		if statusOrder(a.Status) != statusOrder(b.Status) {
			return statusOrder(a.Status) < statusOrder(b.Status)
		}
		if a.SortValue == nil || b.SortValue == nil {
			if a.SortValue != nil {
				return true
			}
			if b.SortValue != nil {
				return false
			}
		} else if *a.SortValue != *b.SortValue {
			if key == "min-temp" {
				return *a.SortValue > *b.SortValue
			}
			return *a.SortValue < *b.SortValue
		}
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		return a.Place.URL < b.Place.URL
	})
}

func markTies(cells []Cell) {
	group := 0
	for start := 0; start < len(cells); {
		end := start + 1
		for end < len(cells) && sameSort(cells[start], cells[end]) {
			end++
		}
		if end-start > 1 {
			group++
			for i := start; i < end; i++ {
				cells[i].Tie = true
				cells[i].TieGroup = group
			}
		}
		start = end
	}
}

func sameSort(a, b Cell) bool {
	if a.Status != b.Status || a.SortValue == nil || b.SortValue == nil {
		return false
	}
	return *a.SortValue == *b.SortValue
}
