package salestech

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/store"
)

// ----- rep leaderboard -------------------------------------------------

// RepLeaderboardRow is one row of the per-rep sales leaderboard.
type RepLeaderboardRow struct {
	SoldByID         int64   `json:"sold_by_id"`
	EstimatesCreated int     `json:"estimates_created"`
	Sold             int     `json:"sold"`
	Dismissed        int     `json:"dismissed"`
	Open             int     `json:"open"`
	CloseRate        float64 `json:"close_rate"` // sold / (sold + dismissed)
	AvgDaysToSell    float64 `json:"avg_days_to_sell"`
	TotalSold        float64 `json:"total_sold_dollars"`
}

// RepLeaderboard groups every estimate created since `since` by soldById
// (estimates without a soldBy are bucketed under id 0 "unassigned") and
// reports close rate + avg days-to-sell + total sold $. Days-to-sell is
// computed from the status_changes feed: min(changedAt where toStatus=Sold)
// minus createdOn; estimates without a Sold transition contribute to the
// "sold" count only when their current status is Sold (typical for older
// estimates whose status_changes feed predates retention).
func RepLeaderboard(db *store.Store, since time.Time) ([]RepLeaderboardRow, error) {
	estimates, err := LoadEstimates(db)
	if err != nil {
		return nil, err
	}
	changes, err := LoadStatusChanges(db)
	if err != nil {
		changes = nil
	}
	soldAt := earliestSoldAt(changes)

	type acc struct {
		created, sold, dismissed, openN int
		totalSold                       float64
		soldDaysSum                     float64
		soldDaysCount                   int
	}
	by := map[int64]*acc{}
	for _, e := range estimates {
		if !since.IsZero() {
			t, ok := parseTimestamp(e.CreatedOn)
			if !ok || t.Before(since) {
				continue
			}
		}
		key := e.SoldByID()
		a, ok := by[key]
		if !ok {
			a = &acc{}
			by[key] = a
		}
		a.created++
		switch strings.ToLower(e.Status.String()) {
		case "sold":
			a.sold++
			a.totalSold += e.Total()
			if sold, has := soldAt[e.ID]; has {
				if created, ok := parseTimestamp(e.CreatedOn); ok {
					a.soldDaysSum += sold.Sub(created).Hours() / 24.0
					a.soldDaysCount++
				}
			}
		case "dismissed":
			a.dismissed++
		case "open":
			a.openN++
		}
	}
	out := make([]RepLeaderboardRow, 0, len(by))
	for id, a := range by {
		row := RepLeaderboardRow{
			SoldByID:         id,
			EstimatesCreated: a.created,
			Sold:             a.sold,
			Dismissed:        a.dismissed,
			Open:             a.openN,
			TotalSold:        a.totalSold,
		}
		if a.sold+a.dismissed > 0 {
			row.CloseRate = float64(a.sold) / float64(a.sold+a.dismissed)
		}
		if a.soldDaysCount > 0 {
			row.AvgDaysToSell = a.soldDaysSum / float64(a.soldDaysCount)
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TotalSold != out[j].TotalSold {
			return out[i].TotalSold > out[j].TotalSold
		}
		return out[i].Sold > out[j].Sold
	})
	return out, nil
}

// earliestSoldAt returns the earliest Sold-transition timestamp per
// estimate id derived from a status_changes feed.
func earliestSoldAt(changes []StatusChange) map[int64]time.Time {
	out := map[int64]time.Time{}
	for _, c := range changes {
		if !strings.EqualFold(c.To, "Sold") {
			continue
		}
		t, ok := parseTimestamp(c.ChangedAt)
		if !ok {
			continue
		}
		if existing, has := out[c.EstimateID]; !has || t.Before(existing) {
			out[c.EstimateID] = t
		}
	}
	return out
}

// ----- close rate ------------------------------------------------------

// CloseRateRow is one pivot slice's close-rate stats.
type CloseRateRow struct {
	Group     string  `json:"group"` // dimension value (rep id, BU id, month YYYY-MM, ...)
	Sold      int     `json:"sold"`
	Dismissed int     `json:"dismissed"`
	Open      int     `json:"open"`
	CloseRate float64 `json:"close_rate"`
	TotalSold float64 `json:"total_sold_dollars"`
}

// GroupByDim names the supported pivot dimensions.
type GroupByDim string

const (
	GroupByBusinessUnit GroupByDim = "businessUnit"
	GroupByRep          GroupByDim = "rep"
	GroupByMonth        GroupByDim = "month"
)

// CloseRate computes sold/(sold+dismissed) grouped by the chosen dimension
// across estimates created since `since`. Unknown dimensions are an error.
func CloseRate(db *store.Store, dim GroupByDim, since time.Time) ([]CloseRateRow, error) {
	estimates, err := LoadEstimates(db)
	if err != nil {
		return nil, err
	}
	key := func(e Estimate) string {
		switch dim {
		case GroupByBusinessUnit:
			if e.BusinessUnitID != nil {
				if e.BusinessUnitName != "" {
					return fmt.Sprintf("%d:%s", *e.BusinessUnitID, e.BusinessUnitName)
				}
				return fmt.Sprintf("%d", *e.BusinessUnitID)
			}
			return "unknown"
		case GroupByRep:
			return fmt.Sprintf("%d", e.SoldByID())
		case GroupByMonth:
			t, ok := parseTimestamp(e.CreatedOn)
			if !ok {
				return "unknown"
			}
			return t.Format("2006-01")
		}
		return ""
	}
	if key(Estimate{}) == "" {
		return nil, fmt.Errorf("unknown --group-by %q (use businessUnit, rep, or month)", string(dim))
	}
	type acc struct {
		sold, dismissed, openN int
		total                  float64
	}
	by := map[string]*acc{}
	for _, e := range estimates {
		if !since.IsZero() {
			t, ok := parseTimestamp(e.CreatedOn)
			if !ok || t.Before(since) {
				continue
			}
		}
		k := key(e)
		a, ok := by[k]
		if !ok {
			a = &acc{}
			by[k] = a
		}
		switch strings.ToLower(e.Status.String()) {
		case "sold":
			a.sold++
			a.total += e.Total()
		case "dismissed":
			a.dismissed++
		case "open":
			a.openN++
		}
	}
	out := make([]CloseRateRow, 0, len(by))
	for k, a := range by {
		row := CloseRateRow{Group: k, Sold: a.sold, Dismissed: a.dismissed, Open: a.openN, TotalSold: a.total}
		if a.sold+a.dismissed > 0 {
			row.CloseRate = float64(a.sold) / float64(a.sold+a.dismissed)
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if dim == GroupByMonth {
			return out[i].Group < out[j].Group
		}
		return out[i].CloseRate > out[j].CloseRate
	})
	return out, nil
}

// ----- days to sell ----------------------------------------------------

// DaysToSellRow is one rep's percentile distribution of close time in days.
type DaysToSellRow struct {
	GroupID int64   `json:"group_id"`
	Group   string  `json:"group"`
	N       int     `json:"n"`
	Min     float64 `json:"min"`
	P50     float64 `json:"p50"`
	P90     float64 `json:"p90"`
	Max     float64 `json:"max"`
	Avg     float64 `json:"avg"`
}

// DaysToSell computes the percentile distribution of (Sold timestamp -
// createdOn) in days, grouped by either rep (default) or businessUnit.
// Estimates without a Sold status_change are skipped.
func DaysToSell(db *store.Store, dim GroupByDim, since time.Time) ([]DaysToSellRow, error) {
	estimates, err := LoadEstimates(db)
	if err != nil {
		return nil, err
	}
	changes, err := LoadStatusChanges(db)
	if err != nil {
		changes = nil
	}
	soldAt := earliestSoldAt(changes)

	groupKey := func(e Estimate) (int64, string) {
		switch dim {
		case GroupByBusinessUnit:
			if e.BusinessUnitID != nil {
				return *e.BusinessUnitID, e.BusinessUnitName
			}
			return 0, "unknown"
		default:
			id := e.SoldByID()
			return id, fmt.Sprintf("rep:%d", id)
		}
	}

	buckets := map[int64]*struct {
		label string
		days  []float64
	}{}
	for _, e := range estimates {
		if !since.IsZero() {
			t, ok := parseTimestamp(e.CreatedOn)
			if !ok || t.Before(since) {
				continue
			}
		}
		if !strings.EqualFold(e.Status.String(), "Sold") {
			continue
		}
		sold, ok := soldAt[e.ID]
		if !ok {
			continue
		}
		created, ok := parseTimestamp(e.CreatedOn)
		if !ok {
			continue
		}
		days := sold.Sub(created).Hours() / 24.0
		if days < 0 {
			days = 0
		}
		id, label := groupKey(e)
		b, has := buckets[id]
		if !has {
			b = &struct {
				label string
				days  []float64
			}{label: label}
			buckets[id] = b
		}
		b.days = append(b.days, days)
	}
	out := make([]DaysToSellRow, 0, len(buckets))
	for id, b := range buckets {
		sort.Float64s(b.days)
		n := len(b.days)
		var sum float64
		for _, d := range b.days {
			sum += d
		}
		row := DaysToSellRow{
			GroupID: id,
			Group:   b.label,
			N:       n,
			Min:     b.days[0],
			P50:     percentile(b.days, 0.50),
			P90:     percentile(b.days, 0.90),
			Max:     b.days[n-1],
			Avg:     sum / float64(n),
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].P50 < out[j].P50
	})
	return out, nil
}

// percentile returns the linearly-interpolated p-th percentile (0..1) of a
// sorted slice. Empty input is 0.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[n-1]
	}
	idx := p * float64(n-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

// ----- dismissed reasons -----------------------------------------------

// DismissedReasonRow is one frequency bucket of dismissal reasons.
type DismissedReasonRow struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// DismissedReasons returns top-N reason strings from status_changes whose
// to-status was Dismissed and that landed since `since`. Empty/missing
// reason values bucket under "<no reason recorded>" so the user sees that
// state rather than silently losing rows.
func DismissedReasons(db *store.Store, since time.Time, top int) ([]DismissedReasonRow, error) {
	if top <= 0 {
		top = 20
	}
	changes, err := LoadStatusChanges(db)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, c := range changes {
		if !strings.EqualFold(c.To, "Dismissed") {
			continue
		}
		if !since.IsZero() {
			t, ok := parseTimestamp(c.ChangedAt)
			if !ok || t.Before(since) {
				continue
			}
		}
		reason := strings.TrimSpace(c.Reason)
		if reason == "" {
			reason = "<no reason recorded>"
		}
		counts[reason]++
	}
	out := make([]DismissedReasonRow, 0, len(counts))
	for k, n := range counts {
		out = append(out, DismissedReasonRow{Reason: k, Count: n})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	if len(out) > top {
		out = out[:top]
	}
	return out, nil
}

// ----- pipeline snapshot -----------------------------------------------

// PipelineRow is the $ total in each status as-of the requested date.
type PipelineRow struct {
	Status string  `json:"status"`
	Count  int     `json:"count"`
	Total  float64 `json:"total_dollars"`
}

// PipelineSnapshot reconstructs the status of every estimate as-of the
// given date by replaying status_changes (chronological apply) up to that
// date. Estimates created after the date are excluded. Returns counts +
// $ totals bucketed by computed status. The `warning` return is non-empty
// when the requested as-of date is older than the oldest status_change in
// the store — meaning the reconstruction may rely on the estimate's
// current status as a baseline and silently miss earlier transitions.
func PipelineSnapshot(db *store.Store, asOf time.Time) ([]PipelineRow, string, error) {
	estimates, err := LoadEstimates(db)
	if err != nil {
		return nil, "", err
	}
	changes, err := LoadStatusChanges(db)
	if err != nil {
		changes = nil
	}
	// Find the oldest status_change to compute the warning.
	var oldestChange time.Time
	for _, c := range changes {
		if t, ok := parseTimestamp(c.ChangedAt); ok {
			if oldestChange.IsZero() || t.Before(oldestChange) {
				oldestChange = t
			}
		}
	}
	warning := ""
	if !oldestChange.IsZero() && asOf.Before(oldestChange) {
		warning = fmt.Sprintf("requested as-of %s is older than the oldest status_change in the store (%s); reconstruction may miss earlier transitions",
			asOf.Format("2006-01-02"), oldestChange.Format("2006-01-02"))
	}
	// Group status_changes by estimate id and sort chronologically.
	byEst := map[int64][]StatusChange{}
	for _, c := range changes {
		byEst[c.EstimateID] = append(byEst[c.EstimateID], c)
	}
	for k := range byEst {
		sort.SliceStable(byEst[k], func(i, j int) bool {
			return byEst[k][i].ChangedAt < byEst[k][j].ChangedAt
		})
	}

	type bucket struct {
		n     int
		total float64
	}
	by := map[string]*bucket{
		"Open":      {},
		"Sold":      {},
		"Dismissed": {},
	}
	for _, e := range estimates {
		created, ok := parseTimestamp(e.CreatedOn)
		if !ok || created.After(asOf) {
			continue
		}
		// Start at the earliest known state: "Open" (the API status enum
		// has only Open/Sold/Dismissed; no canonical "draft"). Replay
		// changes up to and including asOf.
		statusAsOf := "Open"
		for _, c := range byEst[e.ID] {
			t, ok := parseTimestamp(c.ChangedAt)
			if !ok || t.After(asOf) {
				break
			}
			if c.To != "" {
				statusAsOf = c.To
			}
		}
		// If we have no changes at all for this estimate, fall back to its
		// current status — historical info is lost but it's better than
		// silently dropping the estimate.
		if len(byEst[e.ID]) == 0 {
			if cur := e.Status.String(); cur != "" {
				statusAsOf = cur
			}
		}
		b, has := by[statusAsOf]
		if !has {
			b = &bucket{}
			by[statusAsOf] = b
		}
		b.n++
		if strings.EqualFold(statusAsOf, "Sold") {
			b.total += e.Total()
		}
	}
	out := make([]PipelineRow, 0, len(by))
	for k, b := range by {
		out = append(out, PipelineRow{Status: k, Count: b.n, Total: b.total})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Status < out[j].Status })
	return out, warning, nil
}

// ----- sku frequency ---------------------------------------------------

// SkuFreqRow is one SKU's appearance count on estimates with the chosen
// status, since the chosen window.
type SkuFreqRow struct {
	SkuID         int64   `json:"sku_id"`
	SkuName       string  `json:"sku_name"`
	SkuDisplay    string  `json:"sku_display"`
	SkuType       string  `json:"sku_type"`
	Appearances   int     `json:"appearances"`
	EstimateCount int     `json:"estimate_count"`
	TotalQty      float64 `json:"total_qty"`
	TotalDollars  float64 `json:"total_dollars"`
}

// SkuFrequency joins line items × estimates filtered by status and
// (optionally) sold-on / created-on window, then groups by sku id.
// onStatus is case-insensitive ("Sold" / "Dismissed" / "Open"); empty
// means any status.
func SkuFrequency(db *store.Store, onStatus string, since time.Time, top int) ([]SkuFreqRow, error) {
	if top <= 0 {
		top = 50
	}
	estimates, err := LoadEstimates(db)
	if err != nil {
		return nil, err
	}
	itemsBy, err := LoadItemsByEstimate(db)
	if err != nil {
		return nil, err
	}
	want := strings.TrimSpace(onStatus)
	type acc struct {
		name, display, kind string
		appear              int
		estIDs              map[int64]struct{}
		qty, dollars        float64
	}
	by := map[int64]*acc{}
	for _, e := range estimates {
		if want != "" && !strings.EqualFold(e.Status.String(), want) {
			continue
		}
		if !since.IsZero() {
			t, ok := parseTimestamp(e.CreatedOn)
			if !ok || t.Before(since) {
				continue
			}
		}
		for _, it := range itemsBy[e.ID] {
			a, ok := by[it.Sku.ID]
			if !ok {
				a = &acc{
					name: it.Sku.Name, display: it.Sku.DisplayName, kind: it.Sku.Type,
					estIDs: map[int64]struct{}{},
				}
				by[it.Sku.ID] = a
			}
			a.appear++
			a.estIDs[e.ID] = struct{}{}
			a.qty += it.Qty
			a.dollars += it.Total
		}
	}
	out := make([]SkuFreqRow, 0, len(by))
	for id, a := range by {
		out = append(out, SkuFreqRow{
			SkuID: id, SkuName: a.name, SkuDisplay: a.display, SkuType: a.kind,
			Appearances: a.appear, EstimateCount: len(a.estIDs),
			TotalQty: a.qty, TotalDollars: a.dollars,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Appearances != out[j].Appearances {
			return out[i].Appearances > out[j].Appearances
		}
		return out[i].TotalDollars > out[j].TotalDollars
	})
	if len(out) > top {
		out = out[:top]
	}
	return out, nil
}

// ----- follow-ups (rep call list) --------------------------------------

// FollowUpCallRow is one estimate from the last N hours, with the fields a
// rep needs to call the customer back.
type FollowUpCallRow struct {
	SoldByID   int64   `json:"sold_by_id"`
	EstimateID int64   `json:"estimate_id"`
	JobNumber  string  `json:"job_number"`
	Name       string  `json:"name"`
	CustomerID int64   `json:"customer_id,omitempty"`
	JobID      int64   `json:"job_id,omitempty"`
	ProjectID  int64   `json:"project_id,omitempty"`
	Status     string  `json:"status"`
	Total      float64 `json:"total"`
	CreatedOn  string  `json:"created_on"`
	AgeHours   int     `json:"age_hours"`
	Deeplink   string  `json:"deeplink,omitempty"`
}

// FollowUps returns Open estimates created within the requested window,
// optionally filtered to a single rep (soldById). repID == 0 returns
// every rep (or "all"). The result is grouped/sorted by rep id then by
// age descending — oldest stuck quote per rep comes first. tenantID, when
// non-empty, populates a ServiceTitan web deeplink for each row.
func FollowUps(db *store.Store, repID int64, since time.Duration, tenantID string) ([]FollowUpCallRow, error) {
	if since <= 0 {
		since = 48 * time.Hour
	}
	cutoff := time.Now().UTC().Add(-since)
	estimates, err := LoadEstimates(db)
	if err != nil {
		return nil, err
	}
	var out []FollowUpCallRow
	for _, e := range estimates {
		if !strings.EqualFold(e.Status.String(), "Open") {
			continue
		}
		if repID > 0 {
			if e.SoldBy == nil || *e.SoldBy != repID {
				continue
			}
		}
		created, ok := parseTimestamp(e.CreatedOn)
		if !ok || created.Before(cutoff) {
			continue
		}
		age := int(time.Since(created).Hours())
		row := FollowUpCallRow{
			EstimateID: e.ID,
			JobNumber:  e.JobNumber,
			Name:       e.Name,
			Status:     e.Status.String(),
			Total:      e.Total(),
			CreatedOn:  e.CreatedOn,
			AgeHours:   age,
		}
		if e.SoldBy != nil {
			row.SoldByID = *e.SoldBy
		}
		if e.CustomerID != nil {
			row.CustomerID = *e.CustomerID
		}
		if e.JobID != nil {
			row.JobID = *e.JobID
		}
		if e.ProjectID != nil {
			row.ProjectID = *e.ProjectID
		}
		if tenantID != "" {
			row.Deeplink = fmt.Sprintf("https://go.servicetitan.com/#/Estimate/View/%d", e.ID)
		}
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SoldByID != out[j].SoldByID {
			return out[i].SoldByID < out[j].SoldByID
		}
		return out[i].AgeHours > out[j].AgeHours
	})
	return out, nil
}
