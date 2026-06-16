package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/traffic-intel/internal/store"
	"github.com/spf13/cobra"
)

type moversResult struct {
	Profile           string          `json:"profile"`
	CurrentSnapshot   string          `json:"current_snapshot"`
	PreviousSnapshot  string          `json:"previous_snapshot,omitempty"`
	CurrentDateRange  store.DateRange `json:"current_date_range"`
	PreviousDateRange store.DateRange `json:"previous_date_range"`
	Callouts          []string        `json:"callouts"`
	Climbers          []moverRow      `json:"climbers"`
	Droppers          []moverRow      `json:"droppers"`
	NewStrikeZone     []moverRow      `json:"new_strike_zone_entrants"`
	NewRevenueAtRisk  []moverRow      `json:"new_revenue_at_risk"`
	LearningsPath     string          `json:"learnings_path"`
	RecommendedNext   string          `json:"recommended_next_command"`
}

type moverRow struct {
	URL              string  `json:"url"`
	Title            string  `json:"title,omitempty"`
	Query            string  `json:"query,omitempty"`
	PreviousPosition float64 `json:"previous_position,omitempty"`
	CurrentPosition  float64 `json:"current_position,omitempty"`
	PositionDelta    float64 `json:"position_delta"`
	PreviousClicks   int     `json:"previous_clicks"`
	CurrentClicks    int     `json:"current_clicks"`
	ClickDelta       int     `json:"click_delta"`
	PreviousRevenue  float64 `json:"previous_revenue"`
	CurrentRevenue   float64 `json:"current_revenue"`
	RevenueDelta     float64 `json:"revenue_delta"`
	Score            float64 `json:"score"`
	NextAction       string  `json:"next_action"`
}

func moversCmd(f *rootFlags) *cobra.Command {
	var limit int
	c := &cobra.Command{Use: "movers", Short: "Diff the latest snapshot against the prior snapshot", RunE: func(cmd *cobra.Command, args []string) error {
		s := st(f)
		snaps, err := s.LatestSnapshots(f.profile, 2)
		if err != nil {
			return err
		}
		if len(snaps) == 0 {
			return fmt.Errorf("no snapshots for profile %q: run sync first", f.profile)
		}
		result := buildMovers(snaps[0], store.Snapshot{}, limit)
		if len(snaps) > 1 {
			result = buildMovers(snaps[0], snaps[1], limit)
			if err := s.AppendLearning(f.profile, moversLearning(result)); err != nil {
				return err
			}
		} else {
			result.Callouts = []string{"No prior snapshot yet; run sync again after the next data refresh to see movers."}
		}
		result.LearningsPath = s.LearningsPath(f.profile)
		result.RecommendedNext = "traffic-intel-pp-cli refresh-queue --profile " + result.Profile
		lines := moversHuman(result)
		return out(cmd, f, result, strings.Join(lines, "\n")+"\n")
	}}
	c.Flags().IntVar(&limit, "limit", 10, "Rows per mover section")
	return c
}

func buildMovers(current, previous store.Snapshot, limit int) moversResult {
	result := moversResult{
		Profile:           current.Profile,
		CurrentSnapshot:   current.CapturedAt.Format("2006-01-02T15:04:05Z"),
		PreviousSnapshot:  previous.CapturedAt.Format("2006-01-02T15:04:05Z"),
		CurrentDateRange:  current.DateRange,
		PreviousDateRange: previous.DateRange,
		Callouts:          []string{},
		Climbers:          []moverRow{},
		Droppers:          []moverRow{},
		NewStrikeZone:     []moverRow{},
		NewRevenueAtRisk:  []moverRow{},
	}
	if previous.Profile == "" {
		result.PreviousSnapshot = ""
		return result
	}
	prevByURL := map[string]store.PageMetrics{}
	for _, p := range previous.Data.Pages {
		prevByURL[pageKey(p.URL)] = p
	}
	for _, cur := range current.Data.Pages {
		prev, hadPrev := prevByURL[pageKey(cur.URL)]
		row := moverRowFor(cur, prev)
		if isClimber(cur, prev, hadPrev) {
			row.Score = maxf(0, prev.Position-cur.Position)*10 + float64(max(0, cur.Clicks-prev.Clicks))
			row.NextAction = "double down with internal links and refreshed SERP copy while momentum is visible"
			result.Climbers = append(result.Climbers, row)
		}
		if isDropper(cur, prev, hadPrev) {
			row.Score = maxf(0, cur.Position-prev.Position)*10 + float64(max(0, prev.Clicks-cur.Clicks)) + maxf(0, prev.Revenue-cur.Revenue)/100
			row.NextAction = "inspect query mix, SERP changes, and recent page edits before the decline compounds"
			result.Droppers = append(result.Droppers, row)
		}
		if inStrikeZone(cur.Position) && (!hadPrev || !inStrikeZone(prev.Position)) {
			row.Score = opportunityScore(cur)
			row.NextAction = "treat as Strike Zone: improve title/meta, answer intent faster, and add related internal links"
			result.NewStrikeZone = append(result.NewStrikeZone, row)
		}
		if revenueRiskScore(cur) > 0 && (!hadPrev || revenueRiskScore(prev) <= 0) {
			row.Score = revenueRiskScore(cur)
			row.NextAction = riskAction(cur)
			result.NewRevenueAtRisk = append(result.NewRevenueAtRisk, row)
		}
	}
	sort.Slice(result.Climbers, func(i, j int) bool { return result.Climbers[i].Score > result.Climbers[j].Score })
	sort.Slice(result.Droppers, func(i, j int) bool { return result.Droppers[i].Score > result.Droppers[j].Score })
	sort.Slice(result.NewStrikeZone, func(i, j int) bool { return result.NewStrikeZone[i].Score > result.NewStrikeZone[j].Score })
	sort.Slice(result.NewRevenueAtRisk, func(i, j int) bool { return result.NewRevenueAtRisk[i].Score > result.NewRevenueAtRisk[j].Score })
	if limit > 0 {
		result.Climbers = trimMoverRows(result.Climbers, limit)
		result.Droppers = trimMoverRows(result.Droppers, limit)
		result.NewStrikeZone = trimMoverRows(result.NewStrikeZone, limit)
		result.NewRevenueAtRisk = trimMoverRows(result.NewRevenueAtRisk, limit)
	}
	result.Callouts = moverCallouts(result)
	return result
}

func moverRowFor(cur, prev store.PageMetrics) moverRow {
	return moverRow{
		URL:              cur.URL,
		Title:            cur.Title,
		Query:            primaryTopic(cur),
		PreviousPosition: prev.Position,
		CurrentPosition:  cur.Position,
		PositionDelta:    cur.Position - prev.Position,
		PreviousClicks:   prev.Clicks,
		CurrentClicks:    cur.Clicks,
		ClickDelta:       cur.Clicks - prev.Clicks,
		PreviousRevenue:  prev.Revenue,
		CurrentRevenue:   cur.Revenue,
		RevenueDelta:     cur.Revenue - prev.Revenue,
	}
}

func isClimber(cur, prev store.PageMetrics, hadPrev bool) bool {
	if !hadPrev {
		return cur.Clicks > 0 || cur.Revenue > 0
	}
	positionGain := prev.Position > 0 && cur.Position > 0 && prev.Position-cur.Position >= 0.5
	clickGain := cur.Clicks-prev.Clicks >= 10
	revenueGain := cur.Revenue-prev.Revenue >= 500
	return positionGain || clickGain || revenueGain
}

func isDropper(cur, prev store.PageMetrics, hadPrev bool) bool {
	if !hadPrev {
		return false
	}
	positionLoss := prev.Position > 0 && cur.Position > 0 && cur.Position-prev.Position >= 0.5
	clickLoss := prev.Clicks-cur.Clicks >= 10
	revenueLoss := prev.Revenue-cur.Revenue >= 500
	return positionLoss || clickLoss || revenueLoss
}

func inStrikeZone(position float64) bool {
	return position >= 5 && position <= 20
}

func trimMoverRows(rows []moverRow, limit int) []moverRow {
	if len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func moverCallouts(result moversResult) []string {
	callouts := []string{
		fmt.Sprintf("%d %s climbed since the last snapshot.", len(result.Climbers), plural("page", len(result.Climbers))),
		fmt.Sprintf("%d %s dropped since the last snapshot.", len(result.Droppers), plural("page", len(result.Droppers))),
		fmt.Sprintf("%d %s entered the Strike Zone.", len(result.NewStrikeZone), plural("page", len(result.NewStrikeZone))),
		fmt.Sprintf("%d %s became newly revenue-at-risk.", len(result.NewRevenueAtRisk), plural("page", len(result.NewRevenueAtRisk))),
	}
	return callouts
}

func plural(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func moversHuman(result moversResult) []string {
	lines := []string{
		"Act on what's already moving.",
		fmt.Sprintf("profile: %s", result.Profile),
	}
	for _, callout := range result.Callouts {
		lines = append(lines, "- "+callout)
	}
	addSection := func(title string, rows []moverRow) {
		lines = append(lines, "", title)
		if len(rows) == 0 {
			lines = append(lines, "- none")
			return
		}
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("- %s pos %.1f -> %.1f, clicks %+d, revenue %+0.2f", row.URL, row.PreviousPosition, row.CurrentPosition, row.ClickDelta, row.RevenueDelta))
		}
	}
	addSection("Climbers", result.Climbers)
	addSection("Droppers", result.Droppers)
	addSection("New Strike Zone entrants", result.NewStrikeZone)
	addSection("New revenue-at-risk", result.NewRevenueAtRisk)
	lines = append(lines, "", "learnings: "+result.LearningsPath)
	return lines
}

func moversLearning(result moversResult) string {
	lines := []string{"Movers snapshot diff:"}
	for _, callout := range result.Callouts {
		lines = append(lines, "- "+callout)
	}
	if len(result.Climbers) > 0 {
		lines = append(lines, "- Top climber: "+result.Climbers[0].URL)
	}
	if len(result.Droppers) > 0 {
		lines = append(lines, "- Top dropper: "+result.Droppers[0].URL)
	}
	return strings.Join(lines, "\n")
}
