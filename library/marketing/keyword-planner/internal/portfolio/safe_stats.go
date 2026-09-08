// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// SafeStatsOptions selects offline portfolio rows. An empty SnapshotID means
// all snapshots, but every output group still contains its snapshot ID so
// repeated collections are never aggregated together.
type SafeStatsOptions struct {
	SnapshotID string   `json:"snapshot_id"`
	Keyword    string   `json:"keyword"`
	StartMonth string   `json:"start_month"`
	EndMonth   string   `json:"end_month"`
	Language   string   `json:"language"`
	GeoSetID   string   `json:"geo_set_id"`
	GeoTargets []string `json:"geo_targets"`
	Endpoint   string   `json:"endpoint"`
}

// SafeStatsGroup is a descriptive aggregate for one normalized metric
// observation and its exact target context. MetricID is retained as the local
// observation identity; no close-variant or cross-batch rows are collapsed.
type SafeStatsGroup struct {
	SnapshotID         string   `json:"snapshot_id"`
	MetricID           string   `json:"metric_id"`
	Keyword            string   `json:"keyword"`
	SubmittedKeyword   string   `json:"submitted_keyword"`
	SubmittedIndex     *int     `json:"submitted_index,omitempty"`
	VariantGroup       string   `json:"variant_group"`
	LinkageStatus      string   `json:"linkage_status"`
	Endpoint           string   `json:"endpoint"`
	GeoSetID           string   `json:"geo_set_id"`
	GeoTargets         []string `json:"geo_targets"`
	Language           string   `json:"language"`
	Network            string   `json:"network"`
	Currency           string   `json:"currency"`
	CurrencySource     string   `json:"currency_source"`
	RequestedStart     string   `json:"requested_start"`
	RequestedEnd       string   `json:"requested_end"`
	RequestedMonths    int      `json:"requested_months"`
	ReturnedMonths     int      `json:"returned_months"`
	ObservedMonths     int      `json:"observed_months"`
	NullRows           int      `json:"null_rows"`
	ExcludedRows       int      `json:"excluded_rows"`
	ShortWindow        bool     `json:"short_window"`
	NoEligibleValues   bool     `json:"no_eligible_values"`
	CombinedGeo        bool     `json:"combined_geo"`
	SumMonthlySearches string   `json:"sum_monthly_searches"`
	ObservedMean       string   `json:"observed_mean"`
	MinMonthlySearches *int64   `json:"min_monthly_searches,omitempty"`
	MaxMonthlySearches *int64   `json:"max_monthly_searches,omitempty"`
	Flags              []string `json:"flags"`
}

// SafeStatsResult contains only mechanical local summaries and explicit
// exclusion accounting. Empty Groups is meaningful when all rows were
// excluded; ExcludedRows and Warnings explain why.
type SafeStatsResult struct {
	Groups                []SafeStatsGroup `json:"groups"`
	RowsExamined          int              `json:"rows_examined"`
	RowsIncluded          int              `json:"rows_included"`
	ExcludedRows          int              `json:"excluded_rows"`
	ExcludedIncomplete    int              `json:"excluded_incomplete"`
	ExcludedNull          int              `json:"excluded_null"`
	ExcludedInvalid       int              `json:"excluded_invalid"`
	ExcludedNegative      int              `json:"excluded_negative"`
	ExcludedAmbiguousZero int              `json:"excluded_ambiguous_zero_unspecified"`
	ExcludedRawOnly       int              `json:"excluded_raw_only"`
	Warnings              []string         `json:"warnings"`
	NoEligibleValues      bool             `json:"no_eligible_values"`
}

// StatsResult is a short alias for SafeStatsResult.
type StatsResult = SafeStatsResult

// SafeStats calculates descriptive statistics from local normalized rows. It
// always excludes incomplete snapshots, non-eligible rows, NULL/invalid/
// negative values, and ambiguous zero/UNSPECIFIED observations. Sums use
// arbitrary precision and are emitted as decimal text, so int64 overflow
// cannot corrupt the report.
func (s *Store) SafeStats(ctx context.Context, opts SafeStatsOptions) (SafeStatsResult, error) {
	queryOpts := QueryOptions{SnapshotID: opts.SnapshotID, Keyword: opts.Keyword, StartMonth: opts.StartMonth, EndMonth: opts.EndMonth, Language: opts.Language, GeoSetID: opts.GeoSetID, GeoTargets: opts.GeoTargets, Endpoint: opts.Endpoint}
	if err := validateQueryOptions(queryOpts); err != nil {
		return SafeStatsResult{}, err
	}
	resolvedSnapshot := ""
	var err error
	if strings.TrimSpace(opts.SnapshotID) != "" {
		resolvedSnapshot, err = s.ResolveSnapshot(ctx, opts.SnapshotID)
		if err != nil {
			return SafeStatsResult{}, err
		}
	}
	geoID, err := queryGeoID(queryOpts)
	if err != nil {
		return SafeStatsResult{}, err
	}
	endpoint := ""
	if strings.TrimSpace(opts.Endpoint) != "" {
		endpoint, err = NormalizeEndpoint(opts.Endpoint)
		if err != nil {
			return SafeStatsResult{}, err
		}
	}

	rows, err := s.safeStatsRows(ctx, resolvedSnapshot, opts, geoID, endpoint)
	if err != nil {
		return SafeStatsResult{}, err
	}
	result := SafeStatsResult{Groups: []SafeStatsGroup{}, Warnings: []string{}}
	groups := make(map[string]*safeStatsAccumulator)
	for _, row := range rows {
		result.RowsExamined++
		key := safeStatsKey(row)
		acc := groups[key]
		if acc == nil {
			acc = newSafeStatsAccumulator(row)
			groups[key] = acc
		}
		acc.observeRow(row, &result)
	}
	result.Groups = make([]SafeStatsGroup, 0, len(groups))
	for _, acc := range groups {
		result.Groups = append(result.Groups, acc.finish())
	}
	sort.SliceStable(result.Groups, func(i, j int) bool { return safeStatsGroupKey(result.Groups[i]) < safeStatsGroupKey(result.Groups[j]) })
	result.Warnings = sortedUnique(result.Warnings)
	result.NoEligibleValues = result.RowsIncluded == 0
	if result.NoEligibleValues && result.RowsExamined > 0 {
		result.Warnings = append(result.Warnings, "no_eligible_values")
		result.Warnings = sortedUnique(result.Warnings)
	}
	return result, nil
}

// ComputeSafeStats is a descriptive alias for SafeStats.
func (s *Store) ComputeSafeStats(ctx context.Context, opts SafeStatsOptions) (SafeStatsResult, error) {
	return s.SafeStats(ctx, opts)
}

func (s *Store) safeStatsRows(ctx context.Context, snapshotID string, opts SafeStatsOptions, geoID, endpoint string) ([]Row, error) {
	query := `SELECT
		mv.snapshot_id, mv.receipt_id, mv.metric_id, km.endpoint,
		km.returned_text, km.submitted_text, km.submitted_index,
		km.variant_group, km.linkage_status,
		s.geo_set_id, s.geo_target_constants, s.language, s.network,
		COALESCE((SELECT am.currency_code FROM account_metadata am
		          WHERE am.snapshot_id = s.id ORDER BY am.recorded_at DESC, am.id DESC LIMIT 1), s.currency_code),
		COALESCE((SELECT am.source FROM account_metadata am
		          WHERE am.snapshot_id = s.id ORDER BY am.recorded_at DESC, am.id DESC LIMIT 1), s.currency_source),
		mv.month, mv.raw_month, mv.raw_year, mv.monthly_searches, mv.value_state,
		km.low_bid_micros, km.high_bid_micros, km.average_cpc_micros,
		km.average_monthly_searches, km.competition, km.competition_index,
		mv.eligibility, mv.flags, s.fetched_at, s.requested_start, s.requested_end,
		s.complete
	FROM monthly_volumes mv
	JOIN keyword_metrics km ON km.id = mv.metric_id
	JOIN snapshots s ON s.id = mv.snapshot_id
	WHERE 1=1`
	args := make([]any, 0, 12)
	if snapshotID != "" {
		query += ` AND s.id = ?`
		args = append(args, snapshotID)
	}
	if opts.Keyword != "" {
		query += ` AND (instr(LOWER(km.returned_text), LOWER(?)) > 0
			OR instr(LOWER(km.submitted_text), LOWER(?)) > 0
			OR EXISTS (SELECT 1 FROM variant_links vl WHERE vl.metric_id = km.id
				AND vl.relationship IN ('EXACT', 'NORMALIZED', 'CLOSE_VARIANT')
				AND (instr(LOWER(vl.submitted_text), LOWER(?)) > 0
					 OR instr(LOWER(vl.returned_text), LOWER(?)) > 0)))`
		args = append(args, opts.Keyword, opts.Keyword, opts.Keyword, opts.Keyword)
	}
	if opts.Language != "" {
		query += ` AND s.language = ?`
		args = append(args, opts.Language)
	}
	if geoID != "" {
		query += ` AND s.geo_set_id = ?`
		args = append(args, geoID)
	}
	if endpoint != "" {
		query += ` AND km.endpoint = ?`
		args = append(args, endpoint)
	}
	if opts.StartMonth != "" {
		query += ` AND (mv.month IS NULL OR mv.month >= ?)`
		args = append(args, opts.StartMonth+"-01")
	}
	if opts.EndMonth != "" {
		query += ` AND (mv.month IS NULL OR mv.month <= ?)`
		args = append(args, opts.EndMonth+"-01")
	}
	query += ` ORDER BY s.fetched_at DESC, s.rowid DESC, km.id, mv.month, mv.id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query safe statistics rows: %w", err)
	}
	defer rows.Close()
	result := make([]Row, 0)
	for rows.Next() {
		item, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read safe statistics rows: %w", err)
	}
	return result, nil
}

type safeStatsAccumulator struct {
	group    SafeStatsGroup
	sum      big.Int
	observed int
	months   map[string]struct{}
	flags    map[string]struct{}
	hasMin   bool
	hasMax   bool
	min      int64
	max      int64
}

func newSafeStatsAccumulator(row Row) *safeStatsAccumulator {
	requested := 0
	if from, errFrom := ParseYearMonth(row.RequestedStart); errFrom == nil {
		if to, errTo := ParseYearMonth(row.RequestedEnd); errTo == nil {
			requested = len(monthsBetween(from, to))
		}
	}
	return &safeStatsAccumulator{
		group: SafeStatsGroup{
			SnapshotID: row.SnapshotID, MetricID: row.MetricID, Keyword: row.Keyword,
			SubmittedKeyword: row.SubmittedKeyword, SubmittedIndex: cloneInt(row.SubmittedIndex),
			VariantGroup: row.VariantGroup, LinkageStatus: row.LinkageStatus, Endpoint: row.Endpoint,
			GeoSetID: row.GeoSetID, GeoTargets: append([]string(nil), row.GeoTargets...),
			Language: row.Language, Network: row.Network, Currency: row.Currency,
			CurrencySource: row.CurrencySource, RequestedStart: row.RequestedStart,
			RequestedEnd: row.RequestedEnd, RequestedMonths: requested,
			Flags: []string{},
		},
		months: make(map[string]struct{}), flags: make(map[string]struct{}),
	}
}

func (a *safeStatsAccumulator) observeRow(row Row, result *SafeStatsResult) {
	for _, flag := range row.Flags {
		a.flags[flag] = struct{}{}
	}
	if len(row.GeoTargets) > 1 {
		a.group.CombinedGeo = true
		result.Warnings = append(result.Warnings, "combined_geo_scope")
	}
	if row.Month != "" {
		a.months[row.Month] = struct{}{}
	}
	if !row.Complete {
		a.group.ExcludedRows++
		result.ExcludedRows++
		result.ExcludedIncomplete++
		return
	}
	if row.Status != EligibilityEligible {
		a.group.ExcludedRows++
		result.ExcludedRows++
		result.ExcludedRawOnly++
		return
	}
	ambiguous := hasIntegrityFlag(row.Flags, "ambiguous_zero_unspecified")
	negative := row.MonthlySearches != nil && *row.MonthlySearches < 0
	invalid := row.ValueState == "invalid" || hasSafeStatsInvalidFlag(row.Flags) || hasIntegrityFlag(row.Flags, "open_or_future_month") || hasIntegrityFlag(row.Flags, "outside_requested_range")
	nullValue := row.MonthlySearches == nil || row.ValueState == "null" || row.ValueState == "absent"
	if ambiguous {
		result.ExcludedAmbiguousZero++
	}
	if negative {
		result.ExcludedNegative++
	}
	if invalid {
		result.ExcludedInvalid++
	}
	if nullValue {
		result.ExcludedNull++
		a.group.NullRows++
	}
	if ambiguous || negative || invalid || nullValue || row.Month == "" {
		if row.Month == "" && !invalid {
			result.ExcludedInvalid++
			invalid = true
		}
		a.group.ExcludedRows++
		result.ExcludedRows++
		return
	}
	if row.MonthlySearches == nil {
		return
	}
	result.RowsIncluded++
	a.observed++
	a.group.ObservedMonths++
	value := *row.MonthlySearches
	a.sum.Add(&a.sum, big.NewInt(value))
	if !a.hasMin || value < a.min {
		a.min, a.hasMin = value, true
	}
	if !a.hasMax || value > a.max {
		a.max, a.hasMax = value, true
	}
}

func (a *safeStatsAccumulator) finish() SafeStatsGroup {
	result := a.group
	result.ReturnedMonths = len(a.months)
	result.ObservedMonths = a.observed
	result.Flags = make([]string, 0, len(a.flags))
	for flag := range a.flags {
		result.Flags = append(result.Flags, flag)
	}
	sort.Strings(result.Flags)
	if result.RequestedMonths > 0 && result.ReturnedMonths < result.RequestedMonths {
		result.ShortWindow = true
		result.Flags = append(result.Flags, "PLANNER-SHORT-WINDOW")
	}
	if a.observed == 0 {
		result.NoEligibleValues = true
		return result
	}
	result.SumMonthlySearches = a.sum.String()
	result.ObservedMean = safeStatsMean(&a.sum, a.observed)
	if a.hasMin {
		value := a.min
		result.MinMonthlySearches = &value
	}
	if a.hasMax {
		value := a.max
		result.MaxMonthlySearches = &value
	}
	return result
}

func safeStatsMean(sum *big.Int, count int) string {
	if count <= 0 {
		return ""
	}
	rat := new(big.Rat).SetFrac(new(big.Int).Set(sum), big.NewInt(int64(count)))
	text := rat.FloatString(12)
	text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}

func safeStatsKey(row Row) string {
	return strings.Join([]string{row.SnapshotID, row.MetricID, row.Keyword, row.GeoSetID, row.Language, row.Network, row.Currency, row.VariantGroup}, "\x00")
}

func safeStatsGroupKey(group SafeStatsGroup) string {
	return strings.Join([]string{group.SnapshotID, group.MetricID, group.Keyword, group.GeoSetID, group.Language, group.Network, group.Currency, group.VariantGroup}, "\x00")
}

func hasSafeStatsInvalidFlag(flags []string) bool {
	for _, flag := range flags {
		if strings.HasPrefix(flag, "invalid_") || strings.HasSuffix(flag, "_invalid") {
			return true
		}
	}
	return false
}
