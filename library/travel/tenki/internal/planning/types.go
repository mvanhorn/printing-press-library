// Package planning evaluates explicit preferences against compatible source
// periods. It performs no source acquisition and assigns no universal score.
package planning

import "github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/tenki"

type Criteria struct {
	MaxPOP  *float64 `json:"max_pop_pct,omitempty"`
	MaxRain *float64 `json:"max_rain,omitempty"`
	MinTemp *float64 `json:"min_temperature_c,omitempty"`
	MaxTemp *float64 `json:"max_temperature_c,omitempty"`
	MaxWind *float64 `json:"max_wind_speed_m_s,omitempty"`
}

type Window struct{ Start, End int }

type Options struct {
	From            string
	Days            int
	Hours           *Window
	Criteria        Criteria
	Sort            string
	Limit           int
	Season          string
	Year            int
	SeasonCondition string
}

type Input struct {
	Place        tenki.Place
	Weather      tenki.ForecastResult
	Season       *tenki.SeasonalResult
	WeatherError string
	SeasonError  string
}

type Coverage struct {
	Expected  int     `json:"expected"`
	Available int     `json:"available"`
	Ratio     float64 `json:"ratio"`
}

type EvidenceValue struct {
	Value      *float64 `json:"value"`
	Date       string   `json:"date,omitempty"`
	Start      string   `json:"start,omitempty"`
	End        string   `json:"end,omitempty"`
	ValidAt    string   `json:"valid_at,omitempty"`
	Kind       string   `json:"kind"`
	RecordKind string   `json:"record_kind"`
	Partial    bool     `json:"partial"`
	Confidence string   `json:"confidence,omitempty"`
}

type CriterionResult struct {
	Threshold float64         `json:"threshold"`
	Operator  string          `json:"operator"`
	Reduction string          `json:"reduction"`
	Unit      string          `json:"unit"`
	Value     *float64        `json:"value"`
	Verdict   string          `json:"verdict"`
	Coverage  Coverage        `json:"coverage"`
	Evidence  []EvidenceValue `json:"evidence"`
	Reasons   []string        `json:"reasons"`
}

type SeasonalEvidence struct {
	Verdict             string                `json:"verdict"`
	RequestedYear       int                   `json:"requested_year"`
	Evidence            *tenki.SeasonalResult `json:"evidence"`
	Reasons             []string              `json:"reasons"`
	ConditionPreference string                `json:"condition_preference,omitempty"`
	EvidenceKind        string                `json:"evidence_kind"`
}

type Cell struct {
	Place            tenki.Place                `json:"place"`
	Date             string                     `json:"date"`
	Status           string                     `json:"status"`
	WeatherScope     string                     `json:"weather_scope"`
	WeatherSource    tenki.Source               `json:"weather_source"`
	SourceStatus     string                     `json:"source_status"`
	CoverageStart    string                     `json:"source_coverage_start,omitempty"`
	CoverageEnd      string                     `json:"source_coverage_end,omitempty"`
	Criteria         map[string]CriterionResult `json:"criteria"`
	CriteriaCoverage Coverage                   `json:"criteria_coverage"`
	Seasonal         *SeasonalEvidence          `json:"seasonal,omitempty"`
	SortValue        *float64                   `json:"sort_value"`
	Tie              bool                       `json:"tie"`
	TieGroup         int                        `json:"tie_group,omitempty"`
	Assumptions      []string                   `json:"assumptions"`
	Reasons          []string                   `json:"reasons"`
}

type Result struct {
	Cells          []Cell   `json:"cells"`
	Sort           string   `json:"sort"`
	SortDirection  string   `json:"sort_direction"`
	SortSemantics  string   `json:"sort_semantics"`
	RequestedCells int      `json:"requested_cells"`
	ReturnedCells  int      `json:"returned_cells"`
	Truncated      bool     `json:"truncated"`
	Criteria       Criteria `json:"thresholds"`
	RainUnit       string   `json:"rain_unit"`
	From           string   `json:"requested_from"`
	Days           int      `json:"requested_days"`
	Hours          string   `json:"requested_hours,omitempty"`
	Assumptions    []string `json:"assumptions"`
}
