package lancet

import "sort"

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func sortByAbsDelta(rows []TopicShift) {
	sort.Slice(rows, func(i, j int) bool {
		return abs(rows[i].DeltaShare) > abs(rows[j].DeltaShare)
	})
}

func sortByAbsGap(rows []VisibilityRow) {
	sort.Slice(rows, func(i, j int) bool {
		return abs(rows[i].Gap) > abs(rows[j].Gap)
	})
}
