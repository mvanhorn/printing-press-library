// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ErrIncomparableSnapshots is returned when two snapshots have different
// collection identities. A range change is deliberately not an identity
// mismatch: it is reported in RequestChanges and in the per-row reason.
var ErrIncomparableSnapshots = errors.New("portfolio snapshots are not comparable")

// SnapshotDiff is a read-only comparison of two immutable collections. The
// rows and coverage are compared as observations; duplicate observations are
// retained with an ordinal instead of being silently deduplicated.
type SnapshotDiff struct {
	LeftID             string            `json:"left_id"`
	RightID            string            `json:"right_id"`
	Left               Snapshot          `json:"left"`
	Right              Snapshot          `json:"right"`
	Comparable         bool              `json:"comparable"`
	Incompatibilities  []string          `json:"incompatibilities"`
	RequestChanges     []DiffFieldChange `json:"request_changes"`
	Changes            []DiffChange      `json:"changes"`
	CoverageChanges    []CoverageDiff    `json:"coverage_changes"`
	Summary            DiffSummary       `json:"summary"`
	RangeChanged       bool              `json:"range_changed"`
	RangeChangeWarning string            `json:"range_change_warning,omitempty"`
}

// DiffResult is retained as a concise name for callers that prefer result
// terminology.
type DiffResult = SnapshotDiff

// DiffFieldChange records a non-target request-context difference. Values are
// rendered as JSON text where the source field is structured, preserving order
// for submitted inputs.
type DiffFieldChange struct {
	Field string `json:"field"`
	Left  string `json:"left"`
	Right string `json:"right"`
}

// DiffObservationKey identifies one monthly observation without using random
// database IDs. ObservationOrdinal keeps repeated same-month observations
// visible instead of inventing cross-batch deduplication.
type DiffObservationKey struct {
	Keyword            string `json:"keyword"`
	SubmittedKeyword   string `json:"submitted_keyword"`
	SubmittedIndex     *int   `json:"submitted_index,omitempty"`
	VariantGroup       string `json:"variant_group"`
	Month              string `json:"month"`
	ObservationOrdinal int    `json:"observation_ordinal"`
	DuplicateOrdinal   int    `json:"duplicate_ordinal"`
}

// DiffChange describes an added, removed, or changed monthly row. A newly
// included month caused solely by a wider requested range is labelled as such;
// callers must not interpret it as newly measured demand.
type DiffChange struct {
	Kind          string             `json:"kind"`
	Key           DiffObservationKey `json:"key"`
	Left          *Row               `json:"left,omitempty"`
	Right         *Row               `json:"right,omitempty"`
	ChangedFields []string           `json:"changed_fields"`
	Reason        string             `json:"reason,omitempty"`
	ScopeNote     string             `json:"scope_note,omitempty"`
}

// CoverageKey identifies a page, batch and attempt. It intentionally excludes
// receipt IDs because those are unique to each snapshot.
type CoverageKey struct {
	Endpoint    string `json:"endpoint"`
	PageNumber  int    `json:"page_number"`
	BatchNumber int    `json:"batch_number"`
	Attempt     int    `json:"attempt"`
}

// CoverageDiff compares persisted coverage accounting without collapsing
// separate pages, batches or retry attempts.
type CoverageDiff struct {
	Kind          string        `json:"kind"`
	Key           CoverageKey   `json:"key"`
	Left          *CoverageView `json:"left,omitempty"`
	Right         *CoverageView `json:"right,omitempty"`
	ChangedFields []string      `json:"changed_fields"`
}

// DiffSummary provides stable counts for agent output.
type DiffSummary struct {
	ChangedRows       int `json:"changed_rows"`
	AddedRows         int `json:"added_rows"`
	RemovedRows       int `json:"removed_rows"`
	UnchangedRows     int `json:"unchanged_rows"`
	ChangedCoverage   int `json:"changed_coverage"`
	AddedCoverage     int `json:"added_coverage"`
	RemovedCoverage   int `json:"removed_coverage"`
	UnchangedCoverage int `json:"unchanged_coverage"`
}

// Diff compares two exact IDs or the latest/previous aliases. It is entirely
// offline and does not mutate either snapshot.
func (s *Store) Diff(ctx context.Context, leftAlias, rightAlias string) (SnapshotDiff, error) {
	leftID, err := s.ResolveSnapshot(ctx, leftAlias)
	if err != nil {
		return SnapshotDiff{}, err
	}
	rightID, err := s.ResolveSnapshot(ctx, rightAlias)
	if err != nil {
		return SnapshotDiff{}, err
	}
	left, err := s.snapshotForDiff(ctx, leftID)
	if err != nil {
		return SnapshotDiff{}, err
	}
	right, err := s.snapshotForDiff(ctx, rightID)
	if err != nil {
		return SnapshotDiff{}, err
	}

	result := SnapshotDiff{
		LeftID:            leftID,
		RightID:           rightID,
		Left:              left,
		Right:             right,
		Comparable:        true,
		Incompatibilities: []string{},
		RequestChanges:    []DiffFieldChange{},
		Changes:           []DiffChange{},
		CoverageChanges:   []CoverageDiff{},
	}
	result.Incompatibilities = snapshotIdentityMismatches(left, right)
	if len(result.Incompatibilities) > 0 {
		result.Comparable = false
		return result, fmt.Errorf("%w: %s", ErrIncomparableSnapshots, strings.Join(result.Incompatibilities, ", "))
	}

	result.RequestChanges = snapshotRequestChanges(left, right)
	if left.RequestedStart != right.RequestedStart || left.RequestedEnd != right.RequestedEnd {
		result.RangeChanged = true
		result.RangeChangeWarning = "requested range changed; added or removed months are scope changes, not demand changes"
	}

	leftRows, err := s.diffRowsAll(ctx, leftID)
	if err != nil {
		return SnapshotDiff{}, err
	}
	rightRows, err := s.diffRowsAll(ctx, rightID)
	if err != nil {
		return SnapshotDiff{}, err
	}
	leftCoverage, err := s.listCoverage(ctx, leftID)
	if err != nil {
		return SnapshotDiff{}, err
	}
	rightCoverage, err := s.listCoverage(ctx, rightID)
	if err != nil {
		return SnapshotDiff{}, err
	}

	result.Changes = diffRows(leftRows, rightRows, left, right, &result.Summary)
	result.CoverageChanges = diffCoverage(leftCoverage, rightCoverage, &result.Summary)
	return result, nil
}

// diffRowsAll reads every persisted monthly row, including NULL, invalid and
// raw-only eligibility states. Rows intentionally filters to eligible values,
// which is correct for ordinary analytics but would hide state/flag changes
// from a provenance diff.
func (s *Store) diffRowsAll(ctx context.Context, snapshotID string) ([]Row, error) {
	return s.safeStatsRows(ctx, snapshotID, SafeStatsOptions{}, "", "")
}

// CompareSnapshots is a descriptive alias for Diff.
func (s *Store) CompareSnapshots(ctx context.Context, leftAlias, rightAlias string) (SnapshotDiff, error) {
	return s.Diff(ctx, leftAlias, rightAlias)
}

func (s *Store) snapshotForDiff(ctx context.Context, id string) (Snapshot, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+snapshotColumns+` FROM snapshots WHERE id = ?`, id)
	return scanSnapshot(row)
}

func snapshotIdentityMismatches(left, right Snapshot) []string {
	mismatches := []string{}
	if left.Endpoint != right.Endpoint {
		mismatches = append(mismatches, "endpoint")
	}
	if left.CustomerID != right.CustomerID {
		mismatches = append(mismatches, "customer_id")
	}
	if left.Language != right.Language {
		mismatches = append(mismatches, "language")
	}
	if left.Network != right.Network {
		mismatches = append(mismatches, "network")
	}
	if left.CurrencyCode != right.CurrencyCode {
		mismatches = append(mismatches, "currency")
	}
	if left.GeoSetID != right.GeoSetID || strings.Join(left.GeoTargetConstants, "\x00") != strings.Join(right.GeoTargetConstants, "\x00") {
		mismatches = append(mismatches, "geo_targets")
	}
	return mismatches
}

func snapshotRequestChanges(left, right Snapshot) []DiffFieldChange {
	changes := []DiffFieldChange{}
	add := func(field, leftValue, rightValue string) {
		if leftValue != rightValue {
			changes = append(changes, DiffFieldChange{Field: field, Left: leftValue, Right: rightValue})
		}
	}
	add("requested_start", left.RequestedStart, right.RequestedStart)
	add("requested_end", left.RequestedEnd, right.RequestedEnd)
	add("submitted_seeds", jsonText(left.SubmittedSeeds), jsonText(right.SubmittedSeeds))
	add("submitted_keywords", jsonText(left.SubmittedKeywords), jsonText(right.SubmittedKeywords))
	add("request_body_sha256", left.RequestBodyHash, right.RequestBodyHash)
	add("source_variant", left.SourceVariant, right.SourceVariant)
	add("currency_source", left.CurrencySource, right.CurrencySource)
	return changes
}

func jsonText(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "<unencodable>"
	}
	return string(encoded)
}

type diffRowRef struct {
	row    Row
	key    DiffObservationKey
	mapKey string
}

func diffRows(leftRows, rightRows []Row, left, right Snapshot, summary *DiffSummary) []DiffChange {
	leftRefs, rightRefs := buildDiffRowRefs(leftRows, rightRows)
	leftByKey := make(map[string]diffRowRef, len(leftRefs))
	rightByKey := make(map[string]diffRowRef, len(rightRefs))
	keys := make(map[string]struct{}, len(leftRefs)+len(rightRefs))
	for _, ref := range leftRefs {
		leftByKey[ref.mapKey] = ref
		keys[ref.mapKey] = struct{}{}
	}
	for _, ref := range rightRefs {
		rightByKey[ref.mapKey] = ref
		keys[ref.mapKey] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	changes := make([]DiffChange, 0, len(ordered))
	for _, mapKey := range ordered {
		leftRef, hasLeft := leftByKey[mapKey]
		rightRef, hasRight := rightByKey[mapKey]
		switch {
		case !hasLeft:
			change := DiffChange{Kind: "added", Key: rightRef.key, Right: rowPointer(rightRef.row), ChangedFields: []string{}, Reason: rowScopeReason(rightRef.row, left, right, true)}
			change.ScopeNote = diffScopeNote(change.Reason, true)
			changes = append(changes, change)
			summary.AddedRows++
		case !hasRight:
			change := DiffChange{Kind: "removed", Key: leftRef.key, Left: rowPointer(leftRef.row), ChangedFields: []string{}, Reason: rowScopeReason(leftRef.row, left, right, false)}
			change.ScopeNote = diffScopeNote(change.Reason, false)
			changes = append(changes, change)
			summary.RemovedRows++
		default:
			changed := changedRowFields(leftRef.row, rightRef.row)
			if len(changed) == 0 {
				summary.UnchangedRows++
				continue
			}
			changes = append(changes, DiffChange{
				Kind:          "changed",
				Key:           rightRef.key,
				Left:          rowPointer(leftRef.row),
				Right:         rowPointer(rightRef.row),
				ChangedFields: changed,
				Reason:        "observation_changed",
			})
			summary.ChangedRows++
		}
	}
	return changes
}

type diffMetricSeries struct {
	rows       []Row
	contentKey string
}

type diffMetricPair struct {
	left     diffMetricSeries
	right    diffMetricSeries
	hasLeft  bool
	hasRight bool
}

type diffRowPair struct {
	left     Row
	right    Row
	hasLeft  bool
	hasRight bool
}

type diffSeriesBuilder struct {
	metrics    []diffMetricSeries
	byMetricID map[string]int
}

// buildDiffRowRefs pairs duplicate metric series by their persisted content.
// IDs are generated per response and therefore cannot establish identity
// across snapshots. Exact series contents are paired first; residual series
// are paired in content order so real changes and count deltas remain visible.
func buildDiffRowRefs(leftRows, rightRows []Row) ([]diffRowRef, []diffRowRef) {
	leftSeries := groupDiffMetricSeries(leftRows)
	rightSeries := groupDiffMetricSeries(rightRows)
	seriesKeys := make(map[string]struct{}, len(leftSeries)+len(rightSeries))
	for key := range leftSeries {
		seriesKeys[key] = struct{}{}
	}
	for key := range rightSeries {
		seriesKeys[key] = struct{}{}
	}
	orderedSeries := make([]string, 0, len(seriesKeys))
	for key := range seriesKeys {
		orderedSeries = append(orderedSeries, key)
	}
	sort.Strings(orderedSeries)

	leftRefs := make([]diffRowRef, 0, len(leftRows))
	rightRefs := make([]diffRowRef, 0, len(rightRows))
	for _, seriesKey := range orderedSeries {
		pairs := pairDiffMetricSeries(leftSeries[seriesKey], rightSeries[seriesKey])
		for ordinal, pair := range pairs {
			appendDiffMetricPairRefs(&leftRefs, &rightRefs, seriesKey, ordinal, pair)
		}
	}
	return leftRefs, rightRefs
}

func groupDiffMetricSeries(rows []Row) map[string][]diffMetricSeries {
	builders := make(map[string]*diffSeriesBuilder)
	for _, row := range rows {
		seriesKey := diffSeriesKey(row)
		builder := builders[seriesKey]
		if builder == nil {
			builder = &diffSeriesBuilder{byMetricID: make(map[string]int)}
			builders[seriesKey] = builder
		}
		metricIndex, ok := builder.byMetricID[row.MetricID]
		if !ok {
			metricIndex = len(builder.metrics)
			builder.byMetricID[row.MetricID] = metricIndex
			builder.metrics = append(builder.metrics, diffMetricSeries{})
		}
		builder.metrics[metricIndex].rows = append(builder.metrics[metricIndex].rows, row)
	}

	grouped := make(map[string][]diffMetricSeries, len(builders))
	for seriesKey, builder := range builders {
		for index := range builder.metrics {
			builder.metrics[index].contentKey = diffMetricContentKey(builder.metrics[index].rows)
		}
		sort.SliceStable(builder.metrics, func(i, j int) bool {
			return builder.metrics[i].contentKey < builder.metrics[j].contentKey
		})
		grouped[seriesKey] = builder.metrics
	}
	return grouped
}

func pairDiffMetricSeries(left, right []diffMetricSeries) []diffMetricPair {
	leftByContent := make(map[string][]diffMetricSeries)
	rightByContent := make(map[string][]diffMetricSeries)
	contentKeys := make(map[string]struct{}, len(left)+len(right))
	for _, series := range left {
		leftByContent[series.contentKey] = append(leftByContent[series.contentKey], series)
		contentKeys[series.contentKey] = struct{}{}
	}
	for _, series := range right {
		rightByContent[series.contentKey] = append(rightByContent[series.contentKey], series)
		contentKeys[series.contentKey] = struct{}{}
	}
	orderedContent := make([]string, 0, len(contentKeys))
	for key := range contentKeys {
		orderedContent = append(orderedContent, key)
	}
	sort.Strings(orderedContent)

	pairs := make([]diffMetricPair, 0, maxInt(len(left), len(right)))
	remainingLeft := make([]diffMetricSeries, 0, len(left))
	remainingRight := make([]diffMetricSeries, 0, len(right))
	for _, contentKey := range orderedContent {
		leftMatches := leftByContent[contentKey]
		rightMatches := rightByContent[contentKey]
		paired := minInt(len(leftMatches), len(rightMatches))
		for index := 0; index < paired; index++ {
			pairs = append(pairs, diffMetricPair{left: leftMatches[index], right: rightMatches[index], hasLeft: true, hasRight: true})
		}
		remainingLeft = append(remainingLeft, leftMatches[paired:]...)
		remainingRight = append(remainingRight, rightMatches[paired:]...)
	}

	sort.SliceStable(remainingLeft, func(i, j int) bool {
		return remainingLeft[i].contentKey < remainingLeft[j].contentKey
	})
	sort.SliceStable(remainingRight, func(i, j int) bool {
		return remainingRight[i].contentKey < remainingRight[j].contentKey
	})
	paired := minInt(len(remainingLeft), len(remainingRight))
	for index := 0; index < paired; index++ {
		pairs = append(pairs, diffMetricPair{left: remainingLeft[index], right: remainingRight[index], hasLeft: true, hasRight: true})
	}
	for _, series := range remainingLeft[paired:] {
		pairs = append(pairs, diffMetricPair{left: series, hasLeft: true})
	}
	for _, series := range remainingRight[paired:] {
		pairs = append(pairs, diffMetricPair{right: series, hasRight: true})
	}
	return pairs
}

func appendDiffMetricPairRefs(leftRefs, rightRefs *[]diffRowRef, seriesKey string, ordinal int, pair diffMetricPair) {
	leftByMonth := make(map[string][]Row)
	rightByMonth := make(map[string][]Row)
	if pair.hasLeft {
		leftByMonth = groupDiffRowsByMonth(pair.left.rows)
	}
	if pair.hasRight {
		rightByMonth = groupDiffRowsByMonth(pair.right.rows)
	}
	months := make(map[string]struct{}, len(leftByMonth)+len(rightByMonth))
	for month := range leftByMonth {
		months[month] = struct{}{}
	}
	for month := range rightByMonth {
		months[month] = struct{}{}
	}
	orderedMonths := make([]string, 0, len(months))
	for month := range months {
		orderedMonths = append(orderedMonths, month)
	}
	sort.Strings(orderedMonths)

	for _, month := range orderedMonths {
		rowPairs := pairDiffRowsByContent(leftByMonth[month], rightByMonth[month])
		for duplicateOrdinal, rowPair := range rowPairs {
			keyRow := rowPair.right
			if !rowPair.hasRight {
				keyRow = rowPair.left
			}
			key := DiffObservationKey{
				Keyword:            keyRow.Keyword,
				SubmittedKeyword:   keyRow.SubmittedKeyword,
				SubmittedIndex:     cloneInt(keyRow.SubmittedIndex),
				VariantGroup:       keyRow.VariantGroup,
				Month:              month,
				ObservationOrdinal: ordinal,
				DuplicateOrdinal:   duplicateOrdinal,
			}
			mapKey := diffObservationMapKey(seriesKey, key)
			if rowPair.hasLeft {
				*leftRefs = append(*leftRefs, diffRowRef{row: rowPair.left, key: key, mapKey: mapKey})
			}
			if rowPair.hasRight {
				*rightRefs = append(*rightRefs, diffRowRef{row: rowPair.right, key: key, mapKey: mapKey})
			}
		}
	}
}

func groupDiffRowsByMonth(rows []Row) map[string][]Row {
	grouped := make(map[string][]Row)
	for _, row := range rows {
		grouped[row.Month] = append(grouped[row.Month], row)
	}
	return grouped
}

func pairDiffRowsByContent(left, right []Row) []diffRowPair {
	leftByContent := make(map[string][]Row)
	rightByContent := make(map[string][]Row)
	contentKeys := make(map[string]struct{}, len(left)+len(right))
	for _, row := range left {
		contentKey := diffRowContentKey(row)
		leftByContent[contentKey] = append(leftByContent[contentKey], row)
		contentKeys[contentKey] = struct{}{}
	}
	for _, row := range right {
		contentKey := diffRowContentKey(row)
		rightByContent[contentKey] = append(rightByContent[contentKey], row)
		contentKeys[contentKey] = struct{}{}
	}
	orderedContent := make([]string, 0, len(contentKeys))
	for key := range contentKeys {
		orderedContent = append(orderedContent, key)
	}
	sort.Strings(orderedContent)

	pairs := make([]diffRowPair, 0, maxInt(len(left), len(right)))
	remainingLeft := make([]Row, 0, len(left))
	remainingRight := make([]Row, 0, len(right))
	for _, contentKey := range orderedContent {
		leftMatches := leftByContent[contentKey]
		rightMatches := rightByContent[contentKey]
		paired := minInt(len(leftMatches), len(rightMatches))
		for index := 0; index < paired; index++ {
			pairs = append(pairs, diffRowPair{left: leftMatches[index], right: rightMatches[index], hasLeft: true, hasRight: true})
		}
		remainingLeft = append(remainingLeft, leftMatches[paired:]...)
		remainingRight = append(remainingRight, rightMatches[paired:]...)
	}

	sort.SliceStable(remainingLeft, func(i, j int) bool {
		return diffRowContentKey(remainingLeft[i]) < diffRowContentKey(remainingLeft[j])
	})
	sort.SliceStable(remainingRight, func(i, j int) bool {
		return diffRowContentKey(remainingRight[i]) < diffRowContentKey(remainingRight[j])
	})
	paired := minInt(len(remainingLeft), len(remainingRight))
	for index := 0; index < paired; index++ {
		pairs = append(pairs, diffRowPair{left: remainingLeft[index], right: remainingRight[index], hasLeft: true, hasRight: true})
	}
	for _, row := range remainingLeft[paired:] {
		pairs = append(pairs, diffRowPair{left: row, hasLeft: true})
	}
	for _, row := range remainingRight[paired:] {
		pairs = append(pairs, diffRowPair{right: row, hasRight: true})
	}
	return pairs
}

func diffMetricContentKey(rows []Row) string {
	contentKeys := make([]string, 0, len(rows))
	for _, row := range rows {
		contentKeys = append(contentKeys, diffRowContentKey(row))
	}
	sort.Strings(contentKeys)
	return jsonText(contentKeys)
}

type diffRowContent struct {
	Month                  string   `json:"month"`
	RawMonth               string   `json:"raw_month"`
	RawYear                string   `json:"raw_year"`
	MonthlySearches        *int64   `json:"monthly_searches"`
	ValueState             string   `json:"value_state"`
	Flags                  []string `json:"flags"`
	LowBidMicros           *int64   `json:"low_bid_micros"`
	HighBidMicros          *int64   `json:"high_bid_micros"`
	AverageCPCMicros       *int64   `json:"average_cpc_micros"`
	AverageMonthlySearches *int64   `json:"average_monthly_searches"`
	Competition            string   `json:"competition"`
	CompetitionIndex       *int64   `json:"competition_index"`
	Status                 string   `json:"status"`
}

func diffRowContentKey(row Row) string {
	return jsonText(diffRowContent{
		Month:                  row.Month,
		RawMonth:               row.RawMonth,
		RawYear:                row.RawYear,
		MonthlySearches:        row.MonthlySearches,
		ValueState:             row.ValueState,
		Flags:                  append([]string(nil), row.Flags...),
		LowBidMicros:           row.LowBidMicros,
		HighBidMicros:          row.HighBidMicros,
		AverageCPCMicros:       row.AverageCPCMicros,
		AverageMonthlySearches: row.AverageMonthlySearches,
		Competition:            row.Competition,
		CompetitionIndex:       row.CompetitionIndex,
		Status:                 row.Status,
	})
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func diffSeriesKey(row Row) string {
	return strings.Join([]string{row.Keyword, row.SubmittedKeyword, nullableIndexText(row.SubmittedIndex), row.VariantGroup}, "\x00")
}

func diffObservationMapKey(series string, key DiffObservationKey) string {
	return strings.Join([]string{series, key.Month, strconv.Itoa(key.ObservationOrdinal), strconv.Itoa(key.DuplicateOrdinal)}, "\x00")
}

func nullableIndexText(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func rowPointer(value Row) *Row {
	copyValue := value
	copyValue.GeoTargets = append([]string(nil), value.GeoTargets...)
	copyValue.Flags = append([]string(nil), value.Flags...)
	return &copyValue
}

func changedRowFields(left, right Row) []string {
	changed := []string{}
	if !equalInt64Ptr(left.MonthlySearches, right.MonthlySearches) {
		changed = append(changed, "monthly_searches")
	}
	if left.ValueState != right.ValueState {
		changed = append(changed, "value_state")
	}
	if !equalStrings(left.Flags, right.Flags) {
		changed = append(changed, "flags")
	}
	if left.RawMonth != right.RawMonth {
		changed = append(changed, "raw_month")
	}
	if left.RawYear != right.RawYear {
		changed = append(changed, "raw_year")
	}
	if !equalInt64Ptr(left.LowBidMicros, right.LowBidMicros) {
		changed = append(changed, "low_bid_micros")
	}
	if !equalInt64Ptr(left.HighBidMicros, right.HighBidMicros) {
		changed = append(changed, "high_bid_micros")
	}
	if !equalInt64Ptr(left.AverageCPCMicros, right.AverageCPCMicros) {
		changed = append(changed, "average_cpc_micros")
	}
	if !equalInt64Ptr(left.AverageMonthlySearches, right.AverageMonthlySearches) {
		changed = append(changed, "average_monthly_searches")
	}
	if left.Competition != right.Competition {
		changed = append(changed, "competition")
	}
	if !equalInt64Ptr(left.CompetitionIndex, right.CompetitionIndex) {
		changed = append(changed, "competition_index")
	}
	if left.Status != right.Status {
		changed = append(changed, "status")
	}
	return changed
}

func equalInt64Ptr(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func rowScopeReason(row Row, left, right Snapshot, addedToRight bool) string {
	leftWithin := monthInRange(row.Month, left.RequestedStart, left.RequestedEnd)
	rightWithin := monthInRange(row.Month, right.RequestedStart, right.RequestedEnd)
	if addedToRight {
		if rightWithin && !leftWithin {
			return "newly_included_month"
		}
		if leftWithin && !rightWithin {
			return "outside_left_requested_range"
		}
	} else {
		if leftWithin && !rightWithin {
			return "outside_right_requested_range"
		}
		if rightWithin && !leftWithin {
			return "outside_left_requested_range"
		}
	}
	if addedToRight {
		return "observation_added"
	}
	return "observation_removed"
}

func diffScopeNote(reason string, addedToRight bool) string {
	switch reason {
	case "newly_included_month":
		return "requested range changed; this row is newly in scope and is not evidence of newly created demand"
	case "outside_right_requested_range":
		return "requested range changed; this row is outside the right scope and is not evidence of lost demand"
	case "outside_left_requested_range":
		if addedToRight {
			return "requested range changed; this row is outside the left scope and is not evidence of newly created demand"
		}
		return "requested range changed; this row is outside the left scope and is not evidence of lost demand"
	default:
		return ""
	}
}

func monthInRange(value, start, end string) bool {
	if value == "" || start == "" || end == "" {
		return false
	}
	monthText := strings.TrimSuffix(value, "-01")
	month, err := ParseYearMonth(monthText)
	if err != nil {
		return false
	}
	from, err := ParseYearMonth(start)
	if err != nil {
		return false
	}
	to, err := ParseYearMonth(end)
	if err != nil {
		return false
	}
	return compareMonth(month, from) >= 0 && compareMonth(month, to) <= 0
}

type coverageRef struct {
	value  CoverageView
	mapKey string
}

func diffCoverage(left, right []CoverageView, summary *DiffSummary) []CoverageDiff {
	leftByKey := make(map[string]coverageRef, len(left))
	rightByKey := make(map[string]coverageRef, len(right))
	keys := make(map[string]struct{}, len(left)+len(right))
	for _, value := range left {
		key := coverageMapKey(value)
		leftByKey[key] = coverageRef{value: value, mapKey: key}
		keys[key] = struct{}{}
	}
	for _, value := range right {
		key := coverageMapKey(value)
		rightByKey[key] = coverageRef{value: value, mapKey: key}
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	result := make([]CoverageDiff, 0, len(ordered))
	for _, key := range ordered {
		leftRef, hasLeft := leftByKey[key]
		rightRef, hasRight := rightByKey[key]
		coverageKey := coverageKeyFor(leftRef, rightRef, hasLeft, hasRight)
		switch {
		case !hasLeft:
			result = append(result, CoverageDiff{Kind: "added", Key: coverageKey, Right: coveragePointer(rightRef.value), ChangedFields: []string{}})
			summary.AddedCoverage++
		case !hasRight:
			result = append(result, CoverageDiff{Kind: "removed", Key: coverageKey, Left: coveragePointer(leftRef.value), ChangedFields: []string{}})
			summary.RemovedCoverage++
		default:
			changed := changedCoverageFields(leftRef.value, rightRef.value)
			if len(changed) == 0 {
				summary.UnchangedCoverage++
				continue
			}
			result = append(result, CoverageDiff{Kind: "changed", Key: coverageKey, Left: coveragePointer(leftRef.value), Right: coveragePointer(rightRef.value), ChangedFields: changed})
			summary.ChangedCoverage++
		}
	}
	return result
}

func coverageMapKey(value CoverageView) string {
	return strings.Join([]string{value.Endpoint, strconv.Itoa(value.PageNumber), strconv.Itoa(value.BatchNumber), strconv.Itoa(value.Attempt)}, "\x00")
}

func coverageKeyFor(left, right coverageRef, hasLeft, hasRight bool) CoverageKey {
	value := right.value
	if !hasRight && hasLeft {
		value = left.value
	}
	return CoverageKey{Endpoint: value.Endpoint, PageNumber: value.PageNumber, BatchNumber: value.BatchNumber, Attempt: value.Attempt}
}

func coveragePointer(value CoverageView) *CoverageView {
	copyValue := value
	copyValue.RequestedTerms = append([]string(nil), value.RequestedTerms...)
	copyValue.ReturnedTerms = append([]string(nil), value.ReturnedTerms...)
	copyValue.ReturnedMonths = append([]string(nil), value.ReturnedMonths...)
	copyValue.MissingMonths = append([]string(nil), value.MissingMonths...)
	copyValue.UnavailableMonths = append([]string(nil), value.UnavailableMonths...)
	copyValue.RawOnlyMonths = append([]string(nil), value.RawOnlyMonths...)
	if value.VendorTotalSize != nil {
		v := *value.VendorTotalSize
		copyValue.VendorTotalSize = &v
	}
	return &copyValue
}

func changedCoverageFields(left, right CoverageView) []string {
	changed := []string{}
	if left.RequestedStart != right.RequestedStart || left.RequestedEnd != right.RequestedEnd {
		changed = append(changed, "requested_range")
	}
	if !equalStrings(left.RequestedTerms, right.RequestedTerms) {
		changed = append(changed, "requested_terms")
	}
	if !equalStrings(left.ReturnedTerms, right.ReturnedTerms) {
		changed = append(changed, "returned_terms")
	}
	if !equalStrings(left.ReturnedMonths, right.ReturnedMonths) {
		changed = append(changed, "returned_months")
	}
	if !equalStrings(left.MissingMonths, right.MissingMonths) {
		changed = append(changed, "missing_months")
	}
	if !equalStrings(left.UnavailableMonths, right.UnavailableMonths) {
		changed = append(changed, "unavailable_months")
	}
	if !equalStrings(left.RawOnlyMonths, right.RawOnlyMonths) {
		changed = append(changed, "raw_only_months")
	}
	if left.RequestedCount != right.RequestedCount {
		changed = append(changed, "requested_count")
	}
	if left.ReturnedCount != right.ReturnedCount {
		changed = append(changed, "returned_count")
	}
	if left.ReturnedMonthCount != right.ReturnedMonthCount {
		changed = append(changed, "returned_month_count")
	}
	if left.NoResult != right.NoResult {
		changed = append(changed, "no_result")
	}
	if left.CollectionStatus != right.CollectionStatus {
		changed = append(changed, "collection_status")
	}
	if left.Complete != right.Complete {
		changed = append(changed, "complete")
	}
	if left.NextPageToken != right.NextPageToken {
		changed = append(changed, "next_page_token")
	}
	if !equalInt64Ptr(left.VendorTotalSize, right.VendorTotalSize) {
		changed = append(changed, "vendor_total_size")
	}
	return changed
}
