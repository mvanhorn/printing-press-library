package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/ecommerce-intel/internal/store"
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
	EntityType       string  `json:"entity_type"`
	Target           string  `json:"target"`
	Title            string  `json:"title,omitempty"`
	PreviousPosition float64 `json:"previous_position,omitempty"`
	CurrentPosition  float64 `json:"current_position,omitempty"`
	PositionDelta    float64 `json:"position_delta"`
	PreviousClicks   int     `json:"previous_clicks"`
	CurrentClicks    int     `json:"current_clicks"`
	ClickDelta       int     `json:"click_delta"`
	PreviousSessions int     `json:"previous_sessions"`
	CurrentSessions  int     `json:"current_sessions"`
	SessionDelta     int     `json:"session_delta"`
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
		result.RecommendedNext = "ecommerce-intel-pp-cli opportunities --profile " + result.Profile
		return out(cmd, f, result, strings.Join(moversHuman(result), "\n")+"\n")
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
	add := func(row moverRow, hadPrev bool, wasStrikeZone bool, prevRisk float64) {
		if isClimber(row, hadPrev) {
			row.Score = maxf(0, -row.PositionDelta)*10 + float64(max(0, row.ClickDelta)) + float64(max(0, row.SessionDelta))/2 + maxf(0, row.RevenueDelta)/100
			row.NextAction = "double down on merchandising, internal links, and offer evidence while momentum is visible"
			result.Climbers = append(result.Climbers, row)
		}
		if isDropper(row, hadPrev) {
			row.Score = maxf(0, row.PositionDelta)*10 + float64(max(0, -row.ClickDelta)) + float64(max(0, -row.SessionDelta))/2 + maxf(0, -row.RevenueDelta)/100
			row.NextAction = "inspect search demand, inventory, PDP evidence, and recent merchandising changes before the decline compounds"
			result.Droppers = append(result.Droppers, row)
		}
		if inStrikeZone(row.CurrentPosition) && !wasStrikeZone {
			row.Score = maxf(0, 21-row.CurrentPosition)*10 + float64(row.CurrentClicks) + row.CurrentRevenue/100
			row.NextAction = "treat as Strike Zone: improve answer-first copy, schema, reviews, and internal links"
			result.NewStrikeZone = append(result.NewStrikeZone, row)
		}
		risk := revenueRiskScore(row)
		if risk > 0 && prevRisk <= 0 {
			row.Score = risk
			row.NextAction = "protect revenue first: fix tracking gaps, inventory constraints, conversion friction, or lost search demand"
			result.NewRevenueAtRisk = append(result.NewRevenueAtRisk, row)
		}
	}

	prevProducts := map[string]store.Product{}
	for _, p := range previous.Data.Products {
		prevProducts[moverProductKey(p)] = p
	}
	for _, cur := range current.Data.Products {
		prev, hadPrev := prevProducts[moverProductKey(cur)]
		add(productMoverRow(cur, prev), hadPrev, hadPrev && inStrikeZone(prev.SearchPosition), productRisk(prev))
	}

	prevPages := map[string]store.Page{}
	for _, p := range previous.Data.Pages {
		prevPages[pageKey(p.URL)] = p
	}
	for _, cur := range current.Data.Pages {
		prev, hadPrev := prevPages[pageKey(cur.URL)]
		add(pageMoverRow(cur, prev), hadPrev, hadPrev && inStrikeZone(prev.SearchPosition), pageRisk(prev))
	}

	prevCategories := map[string]store.Category{}
	for _, c := range previous.Data.Categories {
		prevCategories[strings.ToLower(c.Handle)] = c
	}
	for _, cur := range current.Data.Categories {
		prev, hadPrev := prevCategories[strings.ToLower(cur.Handle)]
		add(categoryMoverRow(cur, prev), hadPrev, false, categoryRisk(prev))
	}

	sortMoverRows(result.Climbers)
	sortMoverRows(result.Droppers)
	sortMoverRows(result.NewStrikeZone)
	sortMoverRows(result.NewRevenueAtRisk)
	if limit > 0 {
		result.Climbers = trimMoverRows(result.Climbers, limit)
		result.Droppers = trimMoverRows(result.Droppers, limit)
		result.NewStrikeZone = trimMoverRows(result.NewStrikeZone, limit)
		result.NewRevenueAtRisk = trimMoverRows(result.NewRevenueAtRisk, limit)
	}
	result.Callouts = moverCallouts(result)
	return result
}

func moverProductKey(p store.Product) string {
	return strings.ToLower(first(p.Handle, p.URL, p.ID, p.Title))
}

func productMoverRow(cur, prev store.Product) moverRow {
	return moverRow{EntityType: "product", Target: first(cur.Handle, cur.URL, cur.ID), Title: cur.Title, PreviousPosition: prev.SearchPosition, CurrentPosition: cur.SearchPosition, PositionDelta: cur.SearchPosition - prev.SearchPosition, PreviousClicks: prev.SearchClicks, CurrentClicks: cur.SearchClicks, ClickDelta: cur.SearchClicks - prev.SearchClicks, PreviousSessions: prev.Sessions, CurrentSessions: cur.Sessions, SessionDelta: cur.Sessions - prev.Sessions, PreviousRevenue: prev.Revenue, CurrentRevenue: cur.Revenue, RevenueDelta: cur.Revenue - prev.Revenue}
}

func pageMoverRow(cur, prev store.Page) moverRow {
	return moverRow{EntityType: "page", Target: cur.URL, Title: cur.Title, PreviousPosition: prev.SearchPosition, CurrentPosition: cur.SearchPosition, PositionDelta: cur.SearchPosition - prev.SearchPosition, PreviousClicks: prev.SearchClicks, CurrentClicks: cur.SearchClicks, ClickDelta: cur.SearchClicks - prev.SearchClicks, PreviousSessions: prev.Sessions, CurrentSessions: cur.Sessions, SessionDelta: cur.Sessions - prev.Sessions, PreviousRevenue: prev.Revenue, CurrentRevenue: cur.Revenue, RevenueDelta: cur.Revenue - prev.Revenue}
}

func categoryMoverRow(cur, prev store.Category) moverRow {
	return moverRow{EntityType: "category", Target: cur.Handle, Title: cur.Title, PreviousClicks: prev.SearchClicks, CurrentClicks: cur.SearchClicks, ClickDelta: cur.SearchClicks - prev.SearchClicks, PreviousSessions: prev.Sessions, CurrentSessions: cur.Sessions, SessionDelta: cur.Sessions - prev.Sessions, PreviousRevenue: prev.Revenue, CurrentRevenue: cur.Revenue, RevenueDelta: cur.Revenue - prev.Revenue}
}

func isClimber(row moverRow, hadPrev bool) bool {
	if !hadPrev {
		return row.CurrentClicks > 0 || row.CurrentSessions > 0 || row.CurrentRevenue > 0
	}
	return row.PositionDelta <= -0.5 || row.ClickDelta >= 10 || row.SessionDelta >= 50 || row.RevenueDelta >= 500
}

func isDropper(row moverRow, hadPrev bool) bool {
	if !hadPrev {
		return false
	}
	return row.PositionDelta >= 0.5 || row.ClickDelta <= -10 || row.SessionDelta <= -50 || row.RevenueDelta <= -500
}

func inStrikeZone(position float64) bool {
	return position >= 5 && position <= 20
}

func revenueRiskScore(row moverRow) float64 {
	return maxf(0, -row.RevenueDelta)/100 + float64(max(0, -row.ClickDelta))*2 + float64(max(0, -row.SessionDelta))
}

func productRisk(p store.Product) float64 {
	return maxf(0, p.PreviousRevenue-p.Revenue)/100 + float64(max(0, p.PreviousClicks-p.SearchClicks))*2 + float64(max(0, p.PreviousSessions-p.Sessions))
}

func pageRisk(p store.Page) float64 {
	return maxf(0, p.PreviousRevenue-p.Revenue)/100 + float64(max(0, p.PreviousClicks-p.SearchClicks))*2 + float64(max(0, p.PreviousSessions-p.Sessions))
}

func categoryRisk(c store.Category) float64 {
	return maxf(0, c.PreviousRevenue-c.Revenue) / 100
}

func sortMoverRows(rows []moverRow) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Score > rows[j].Score })
}

func trimMoverRows(rows []moverRow, limit int) []moverRow {
	if len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func moverCallouts(result moversResult) []string {
	return []string{
		fmt.Sprintf("%d commerce %s climbed since the last snapshot.", len(result.Climbers), plural("surface", len(result.Climbers))),
		fmt.Sprintf("%d commerce %s dropped since the last snapshot.", len(result.Droppers), plural("surface", len(result.Droppers))),
		fmt.Sprintf("%d product/page %s entered the Strike Zone.", len(result.NewStrikeZone), plural("surface", len(result.NewStrikeZone))),
		fmt.Sprintf("%d commerce %s became newly revenue-at-risk.", len(result.NewRevenueAtRisk), plural("surface", len(result.NewRevenueAtRisk))),
	}
}

func plural(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func moversHuman(result moversResult) []string {
	lines := []string{"Act on what's already moving.", fmt.Sprintf("profile: %s", result.Profile)}
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
			lines = append(lines, fmt.Sprintf("- %s %s pos %.1f -> %.1f, clicks %+d, sessions %+d, revenue %+0.2f", row.EntityType, row.Target, row.PreviousPosition, row.CurrentPosition, row.ClickDelta, row.SessionDelta, row.RevenueDelta))
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
		lines = append(lines, "- Top climber: "+result.Climbers[0].EntityType+" "+result.Climbers[0].Target)
	}
	if len(result.Droppers) > 0 {
		lines = append(lines, "- Top dropper: "+result.Droppers[0].EntityType+" "+result.Droppers[0].Target)
	}
	return strings.Join(lines, "\n")
}
