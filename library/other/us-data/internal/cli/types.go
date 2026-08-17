// Copyright 2026 Dhilip Subramanian. Licensed under Apache-2.0. See LICENSE.

package cli

type Observation struct {
	Year       string `json:"year"`
	Period     string `json:"period"`
	PeriodName string `json:"period_name"`
	Value      string `json:"value"`
	Latest     bool   `json:"latest,omitempty"`
}

type SeriesResult struct {
	Kind          string        `json:"kind"`
	Source        string        `json:"source"`
	SeriesID      string        `json:"series_id"`
	Title         string        `json:"title"`
	Latest        Observation   `json:"latest"`
	Prior         *Observation  `json:"prior,omitempty"`
	PercentChange *float64      `json:"percent_change,omitempty"`
	Observations  []Observation `json:"observations,omitempty"`
	FreshnessNote string        `json:"freshness_note"`
	Caveats       []string      `json:"caveats,omitempty"`
}

type GuidanceResult struct {
	Kind     string   `json:"kind"`
	Status   string   `json:"status"`
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
	EnvVars  []string `json:"env_vars,omitempty"`
	Sources  []string `json:"sources,omitempty"`
}

type PopulationResult struct {
	Kind       string   `json:"kind"`
	Source     string   `json:"source"`
	Place      string   `json:"place"`
	Dataset    string   `json:"dataset"`
	Population string   `json:"population"`
	Year       string   `json:"year"`
	Caveats    []string `json:"caveats,omitempty"`
}

type CompareSide struct {
	Region     string            `json:"region"`
	Population *PopulationResult `json:"population,omitempty"`
}

type CompareResult struct {
	Kind    string      `json:"kind"`
	Left    CompareSide `json:"left"`
	Right   CompareSide `json:"right"`
	Notices []string    `json:"notices,omitempty"`
	Sources []string    `json:"sources,omitempty"`
}
