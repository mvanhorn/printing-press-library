// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package verdict

// Source records one manifest calendar's read attempt with its upstream
// freshness evidence. Field names are part of the verdict contract — do not
// rename.
type Source struct {
	Account  string `json:"account"`
	Calendar string `json:"calendar"`
	// FetchedAt is when this CLI performed the read (RFC3339 UTC).
	FetchedAt string `json:"fetched_at"`
	// UpstreamUpdatedMax is the newest `updated` stamp observed in the
	// upstream response (RFC3339 UTC); empty when the endpoint carries none
	// (freebusy) or the read failed.
	UpstreamUpdatedMax string `json:"upstream_updated_max"`
	// EtagPresent reports whether the upstream response carried an etag.
	EtagPresent bool `json:"etag_present"`
	// Error is the read failure, empty on success.
	Error string `json:"error,omitempty"`
}

// Coverage is the confidence block every verdict-bearing output carries. A
// verdict (conflicts found / all clear / open slot) is only CONFIDENT when
// Complete is true: every manifest calendar was read successfully with
// upstream freshness evidence. Otherwise the verdict downgrades explicitly.
// Field names are part of the verdict contract — do not rename.
type Coverage struct {
	Checked  int      `json:"checked"`
	Of       int      `json:"of"`
	Complete bool     `json:"complete"`
	Sources  []Source `json:"sources"`
}

// BuildCoverage derives the coverage block from per-source read results.
// Checked counts sources that read cleanly; Complete requires every source
// clean AND at least one source — a verdict over zero sources can never be
// confident.
func BuildCoverage(sources []Source) Coverage {
	if sources == nil {
		sources = []Source{}
	}
	checked := 0
	for _, s := range sources {
		if s.Error == "" {
			checked++
		}
	}
	return Coverage{
		Checked:  checked,
		Of:       len(sources),
		Complete: len(sources) > 0 && checked == len(sources),
		Sources:  sources,
	}
}
