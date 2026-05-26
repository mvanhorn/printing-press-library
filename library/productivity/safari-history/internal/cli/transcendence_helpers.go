package cli

import (
	"database/sql"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/categorize"
	"github.com/mvanhorn/printing-press-library/library/productivity/safari-history/internal/source"
)

// domainProductivitySplit classifies every domain in the window — not just the
// top-N shown to the user — so the productive/neutral/distracting split reflects
// all visits. Computing it from a truncated top-domain list badly skews the
// result for users whose activity is spread across many domains.
func domainProductivitySplit(src source.Source, db *sql.DB, start time.Time, device string) (map[string]int64, error) {
	rows, err := src.DomainStats(db, source.VisitFilter{Since: start, Limit: 1_000_000, Device: device})
	if err != nil {
		return nil, err
	}
	split := map[string]int64{"productive": 0, "neutral": 0, "distracting": 0}
	for _, r := range rows {
		_, level := categorize.Classify(source.DomainFromURL(r.Domain))
		split[level] += r.VisitSum
	}
	return split, nil
}

type session struct {
	Start time.Time
	End   time.Time
	Items []source.VisitRow
}

func splitSessions(events []source.VisitRow, gap time.Duration) []session {
	if len(events) == 0 {
		return nil
	}
	sort.Slice(events, func(i, j int) bool { return events[i].VisitTime.Before(events[j].VisitTime) })
	out := []session{}
	cur := session{Start: events[0].VisitTime, End: events[0].VisitTime, Items: []source.VisitRow{events[0]}}
	for i := 1; i < len(events); i++ {
		e := events[i]
		if e.VisitTime.Sub(cur.End) > gap {
			out = append(out, cur)
			cur = session{Start: e.VisitTime, End: e.VisitTime, Items: []source.VisitRow{e}}
			continue
		}
		cur.Items = append(cur.Items, e)
		cur.End = e.VisitTime
	}
	out = append(out, cur)
	return out
}

func estimateDwellMicros(s session, gap time.Duration) []int64 {
	out := make([]int64, 0, len(s.Items))
	for i, e := range s.Items {
		d := e.VisitDuration
		if d <= 0 {
			if i < len(s.Items)-1 {
				d = s.Items[i+1].VisitTime.Sub(e.VisitTime)
			} else {
				d = 0
			}
		}
		if d > gap {
			d = gap
		}
		out = append(out, d.Microseconds())
	}
	return out
}
