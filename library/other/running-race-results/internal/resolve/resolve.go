// internal/resolve/resolve.go
package resolve

import (
	"errors"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/catalog"
	"github.com/mvanhorn/printing-press-library/library/other/running-race-results/internal/domain"
)

// ErrNoMatch means no catalog entry scored above the match threshold.
var ErrNoMatch = errors.New("no known race matches query")

const matchThreshold = 0.34

// Candidate is a scored resolution of a query to an event.
type Candidate struct {
	Event domain.Event
	Score float64
}

// Resolve scores the query against every entry's race name + aliases, keeps
// those above threshold, optionally filters by year, and sorts best-first.
func Resolve(entries []catalog.Entry, query string, year int) ([]Candidate, error) {
	var cands []Candidate
	for _, e := range entries {
		if year != 0 && e.Year != year {
			continue
		}
		best := Score(query, e.Race)
		for _, a := range e.Aliases {
			if s := Score(query, a); s > best {
				best = s
			}
		}
		if best >= matchThreshold {
			cands = append(cands, Candidate{
				Event: domain.Event{
					Provider: e.Provider, Name: e.Race, Year: e.Year,
					ID: e.EventID, BaseURL: e.BaseURL,
				},
				Score: best,
			})
		}
	}
	if len(cands) == 0 {
		return nil, ErrNoMatch
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Score > cands[j].Score })
	return cands, nil
}
