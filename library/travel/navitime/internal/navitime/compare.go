package navitime

import "sort"

func summary(r Route) RouteSummary {
	return RouteSummary{ID: r.ID, IDProvenance: r.IDProvenance, SourceID: r.SourceID, SourceIndex: r.SourceIndex, SourceURL: r.SourceURL, From: r.From, To: r.To, DepartureAt: r.DepartureAt, ArrivalAt: r.ArrivalAt, DurationMinutes: r.DurationMinutes, DurationSeconds: r.DurationSeconds, DurationMinutesBasis: r.DurationMinutesBasis, TransportKinds: r.TransportKinds, TimingBasis: r.TimingBasis, WalkingMeters: r.WalkingMeters, Transfers: r.Transfers, Fare: r.Fare, Pass: r.Pass}
}
func Summaries(result RouteResult, limit int) any {
	if limit <= 0 {
		limit = 3
	}
	if limit > 10 {
		limit = 10
	}
	out := []RouteSummary{}
	for _, r := range result.Routes {
		if len(out) >= limit {
			break
		}
		out = append(out, summary(r))
	}
	notes := append([]string{}, result.Notes...)
	if len(out) < len(result.Routes) {
		notes = append(notes, "Display is bounded; returned_alternatives counts the source alternatives retained for detail/comparison.")
	}
	return SummaryResult{result.Meta, result.Query, out, len(result.Routes), notes}
}
func CompareRoutes(result RouteResult, options CompareOptions) (any, error) {
	switch options.Sort {
	case "":
		options.Sort = "duration"
	case "duration", "fare", "walk", "transfers", "source":
	default:
		return nil, &ArgumentError{"compare sort must be duration, fare, walk, transfers, or source"}
	}
	for _, cap := range []int{options.MaxDurationMinutes, options.MaxFareJPY, options.MaxWalkMeters, options.MaxTransfers} {
		if cap < -1 {
			return nil, &ArgumentError{"comparison limits must be nonnegative or -1 (unset)"}
		}
	}
	out := []RouteSummary{}
	unknownExcluded := false
	for _, r := range result.Routes {
		accepted := true
		if options.MaxDurationMinutes >= 0 && r.DurationSeconds != nil && float64(*r.DurationSeconds) > float64(options.MaxDurationMinutes)*60 {
			accepted = false
		}
		for _, pair := range []struct {
			value *int
			cap   int
		}{{r.DurationMinutes, options.MaxDurationMinutes}, {r.Fare.TotalJPY, options.MaxFareJPY}, {r.WalkingMeters, options.MaxWalkMeters}, {r.Transfers, options.MaxTransfers}} {
			if pair.cap >= 0 && (pair.value == nil || *pair.value > pair.cap) {
				accepted = false
				if pair.value == nil {
					unknownExcluded = true
				}
			}
		}
		if accepted {
			out = append(out, summary(r))
		}
	}
	metric := func(r RouteSummary) *int {
		switch options.Sort {
		case "duration":
			if r.DurationSeconds != nil {
				return r.DurationSeconds
			}
			if r.DurationMinutes != nil {
				return ptr(*r.DurationMinutes * 60)
			}
			return nil
		case "fare":
			return r.Fare.TotalJPY
		case "walk":
			return r.WalkingMeters
		case "transfers":
			return r.Transfers
		default:
			return &r.SourceIndex
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := metric(out[i]), metric(out[j])
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return *a < *b
	})
	notes := []string{"Only returned alternatives are compared; this is not a global optimum or exhaustive journey search.", "Unknown metrics remain null and sort after known values. Displayed estimates and published transit fares have different bases; inspect each row's fare.basis."}
	if unknownExcluded {
		notes = append(notes, "A capped unknown metric cannot establish eligibility and is excluded.")
	}
	if len(out) == 0 {
		notes = append(notes, "No returned alternatives match the local limits.")
	}
	return CompareResult{result.Meta, result.Query, "returned_alternatives_only", options.Sort, out, notes}, nil
}
