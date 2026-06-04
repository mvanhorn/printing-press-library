# eRank Scorecard Improvement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Raise the eRank CLI scorecard through honest local improvements: split oversized insight code, replace generated command test stubs with real tests, remove confirmed dead code, add a narrow typed keyword-signal store slice, preserve publish readiness, and document upstream-only scoring gaps.

**Architecture:** Keep the root command surface unchanged. Move root-wired novel commands into focused files, move shared keyword-signal logic into `internal/cli/erank_signals.go`, add persistence through `internal/store/extras.go`, and record generated-tree customizations in `.printing-press-patches.json`.

**Tech Stack:** Go 1.26.3, Cobra 1.9.1, SQLite via `modernc.org/sqlite` 1.37.0, `erank-pp-cli`, and `cli-printing-press`.

---

## Baseline

Branch: `codex/import-erank-cli`.

Known scorecard: `80/100 - Grade A`.

Local target rows: `Workflows 6/10`, `Insight 4/10`, `Data Pipeline Integrity 7/10`, `Dead Code 4/5`.

Out-of-scope rows:

- `Auth Protocol 2/10`: eRank uses browser cookies plus `X-XSRF-TOKEN`; do not add fake bearer-token code.
- `MCP Token Efficiency 0/10`: Printing Press MCP catalog shape; do not hide useful local tools.

Current facts:

- `internal/cli/erank_insights.go` is 768 lines and owns shared helpers plus plain insight constructors.
- Root registration uses `newNovel*` constructors in focused files.
- Focused files mostly delegate into `erank_insights.go`.
- `internal/store/extras.go` is the migration hook for novel-feature auxiliary tables.
- `extractResponseData` appears unused; verify immediately before deletion.

---

## Task 1: Reconfirm Runtime Truth

- [ ] Run discovery:

```bash
erank-pp-cli doctor --json
erank-pp-cli agent-context --pretty
erank-pp-cli which "keyword opportunity scoring" --json
```

Expected: valid JSON diagnostics, novel commands visible, and `which` finds an opportunity-related command.

- [ ] Run baseline tests and scorecard:

```bash
go test ./...
cli-printing-press scorecard --dir /Users/smacdonald/printing-press/library/erank --json
```

Expected: tests pass; score remains near `80/100`; auth and MCP token rows remain known upstream gaps.

- [ ] Verify constructor wiring:

```bash
rg -n "newNovel(Opportunity|Listing|ListingGaps|Tags|TagsConsensus|Watch|WatchDrift|Lists|ListsOptimize|Saturation|Angles)Cmd|new(Opportunity|Listing|ListingGaps|Tags|TagsConsensus|Watch|WatchDrift|Lists|ListsOptimize|Saturation|Angles)Cmd" internal/cli
```

Expected: `newNovel*` constructors are root-wired, and plain `new*` insight constructors are in `internal/cli/erank_insights.go`.

No commit.

---

## Task 2: Add Characterization Tests

- [ ] Move helper tests from `internal/cli/erank_insights_test.go` to `internal/cli/erank_signals_test.go`.

Required helper tests:

```text
TestScoreKeywordRatesUsefulKeyword
TestRankConsensusTagsMergesSources
TestDriftSummaryFlagsThreshold
TestBuildAnglesUsesRelatedSearchesAndTags
TestNormalizeTokenSetDeduplicatesTerms
```

New helper test pattern:

```go
func TestBuildAnglesUsesRelatedSearchesAndTags(t *testing.T) {
	signals := keywordSignals{
		Keyword: "ceramic mug",
		Related: []map[string]any{{"keyword": "handmade ceramic mug"}},
		EtsyTags: []map[string]any{{"tag": "pottery gift"}},
	}
	got := buildAngles(signals, 2)
	if len(got) != 2 || got[0]["angle"] == "" {
		t.Fatalf("buildAngles() = %#v", got)
	}
}
```

- [ ] Replace generated command test stubs:

```text
internal/cli/opportunity_test.go
internal/cli/listing_gaps_test.go
internal/cli/tags_consensus_test.go
internal/cli/watch_drift_test.go
internal/cli/lists_optimize_test.go
internal/cli/saturation_test.go
internal/cli/angles_test.go
```

Constructor assertion pattern:

```go
func TestNovelOpportunityCommandIsExecutable(t *testing.T) {
	cmd := newNovelOpportunityCmd(&rootFlags{})
	if cmd.Use == "" || cmd.RunE == nil {
		t.Fatalf("command is not executable: %#v", cmd)
	}
}
```

Behavior assertions:

- `opportunity_test.go`: lower competition gives higher `scoreKeyword`.
- `listing_gaps_test.go`: missing title and tag terms are reported.
- `tags_consensus_test.go`: duplicate tags merge in `rankConsensusTags`.
- `watch_drift_test.go`: `driftSummary` reports threshold breach.
- `lists_optimize_test.go`: optimizer helper sorts scores descending.
- `saturation_test.go`: `saturationLabel` separates low, medium, high competition.
- `angles_test.go`: `buildAngles` respects limit.

- [ ] Validate and commit:

```bash
go test ./internal/cli -run 'Test(ScoreKeyword|RankConsensusTags|DriftSummary|BuildAngles|NormalizeTokenSet|Novel|Saturation)'
git add internal/cli/*_test.go
git commit -m "test: characterize erank insight commands"
```

Expected: focused tests pass before refactor and stub test symbols are gone.

---

## Task 3: Split `erank_insights.go`

- [ ] Create `internal/cli/erank_signals.go` and move shared logic:

```text
keywordSignals, scoredKeyword, consensusTag
fetchKeywordSignals, isPrintingPressInvalidValue, fetchKeywordSignalsWithClient
withDefaultMatch, scoreKeyword, rating, saturationLabel
rankConsensusTags, buildAngles, updateDriftHistory, scoredKeywordSnapshot, driftSummary
decodeRows, decodeObject, rowsFromAny, collectStrings, extractKeywordTerms
bestNumber, keyMatches, splitCSV, normalizeTokenSet, normalizeToken
```

- [ ] Move command bodies into root-wired focused files:

```text
internal/cli/opportunity.go       -> newNovelOpportunityCmd
internal/cli/listing.go           -> newNovelListingCmd
internal/cli/listing_gaps.go      -> newNovelListingGapsCmd
internal/cli/tags.go              -> newNovelTagsCmd
internal/cli/tags_consensus.go    -> newNovelTagsConsensusCmd
internal/cli/watch.go             -> newNovelWatchCmd
internal/cli/watch_drift.go       -> newNovelWatchDriftCmd
internal/cli/lists.go             -> newNovelListsCmd
internal/cli/lists_optimize.go    -> newNovelListsOptimizeCmd
internal/cli/saturation.go        -> newNovelSaturationCmd
internal/cli/angles.go            -> newNovelAnglesCmd
```

Rules:

- Preserve root-wired `newNovel*` names.
- Delete wrapper-only delegation functions and plain insight constructors.
- Delete `internal/cli/erank_insights.go` once empty.
- Keep edited hand-authored files under 500 lines.
- Preserve flags, examples, dry-run behavior, output fields, and agent-mode JSON.

- [ ] Verify, test, and commit:

```bash
rg -n "func new(Opportunity|Listing|ListingGaps|Tags|TagsConsensus|Watch|WatchDrift|Lists|ListsOptimize|Saturation|Angles)Cmd" internal/cli
gofmt -w internal/cli/erank_signals.go internal/cli/opportunity.go internal/cli/listing.go internal/cli/listing_gaps.go internal/cli/tags.go internal/cli/tags_consensus.go internal/cli/watch.go internal/cli/watch_drift.go internal/cli/lists.go internal/cli/lists_optimize.go internal/cli/saturation.go internal/cli/angles.go internal/cli/*_test.go
go test ./internal/cli
go test ./...
git add internal/cli
git commit -m "refactor: split erank insight commands"
```

Expected: first command has no matches, tests pass, and `internal/cli/erank_insights.go` is gone.

---

## Task 4: Remove Confirmed Dead Code

- [ ] Verify references:

```bash
rg -n "extractResponseData" .
```

Expected: only the definition and adjacent comment in `internal/cli/helpers.go`. If a real caller appears, stop this task and leave the helper in place.

- [ ] Remove `extractResponseData` and its adjacent comment from `internal/cli/helpers.go`.

- [ ] Validate and commit:

```bash
gofmt -w internal/cli/helpers.go
go test ./internal/cli
go test ./...
cli-printing-press scorecard --dir /Users/smacdonald/printing-press/library/erank --json
git add internal/cli/helpers.go
git commit -m "refactor: remove dead response helper"
```

Expected: tests pass; Dead Code does not regress and should improve if this was the remaining local dead helper.

---

## Task 5: Add Keyword Signal Snapshot Storage

- [ ] Update `internal/store/extras.go`:

```go
migrations := []string{
	`CREATE TABLE IF NOT EXISTS keyword_signal_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		keyword TEXT NOT NULL,
		source TEXT NOT NULL,
		country TEXT NOT NULL,
		score REAL NOT NULL,
		rating TEXT NOT NULL,
		search_signal REAL NOT NULL DEFAULT 0,
		competition_signal REAL NOT NULL DEFAULT 0,
		difficulty_signal REAL NOT NULL DEFAULT 0,
		tag_count INTEGER NOT NULL DEFAULT 0,
		top_listing_count INTEGER NOT NULL DEFAULT 0,
		captured_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_keyword_signal_snapshots_lookup
		ON keyword_signal_snapshots(keyword, country, source, captured_at DESC)`,
}
```

- [ ] Add `internal/store/keyword_signal_snapshots.go` with:

```go
type KeywordSignalSnapshot struct {
	Keyword string
	Source string
	Country string
	Score float64
	Rating string
	SearchSignal float64
	CompetitionSignal float64
	DifficultySignal float64
	TagCount int
	TopListingCount int
	CapturedAt time.Time
}

type KeywordSignalSnapshotFilter struct {
	Keyword string
	Source string
	Country string
	Since time.Time
	Limit int
}

func (s *Store) InsertKeywordSignalSnapshot(ctx context.Context, snapshot KeywordSignalSnapshot) error
func (s *Store) ListKeywordSignalSnapshots(ctx context.Context, filter KeywordSignalSnapshotFilter) ([]KeywordSignalSnapshot, error)
```

Implementation requirements:

- Insert under `s.writeMu`.
- Store timestamps with `time.RFC3339Nano`.
- Default zero `CapturedAt` to `time.Now().UTC()`.
- Default nonpositive `Limit` to 50.
- Query newest snapshots first.
- Wrap errors with operation names.

- [ ] Add `internal/store/keyword_signal_snapshots_test.go`:

```go
func TestKeywordSignalSnapshotsRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := OpenWithContext(ctx, filepath.Join(t.TempDir(), "store.sqlite"))
	if err != nil { t.Fatalf("OpenWithContext() error = %v", err) }
	defer db.Close()

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	err = db.InsertKeywordSignalSnapshot(ctx, KeywordSignalSnapshot{
		Keyword: "ceramic mug", Source: "live", Country: "us",
		Score: 72.5, Rating: "strong", CapturedAt: now,
	})
	if err != nil { t.Fatalf("InsertKeywordSignalSnapshot() error = %v", err) }

	got, err := db.ListKeywordSignalSnapshots(ctx, KeywordSignalSnapshotFilter{
		Keyword: "ceramic mug", Country: "us", Limit: 10,
	})
	if err != nil { t.Fatalf("ListKeywordSignalSnapshots() error = %v", err) }
	if len(got) != 1 || got[0].Score != 72.5 || !got[0].CapturedAt.Equal(now) {
		t.Fatalf("snapshot mismatch: %#v", got)
	}
}
```

- [ ] Add best-effort CLI recording in `internal/cli/erank_signals.go`:

```go
func recordKeywordSignalSnapshot(ctx context.Context, signals keywordSignals, score scoredKeyword) {
	db, err := store.OpenWithContext(ctx, defaultDBPath("erank-pp-cli"))
	if err != nil { return }
	defer db.Close()

	source, country := signals.Source, signals.Country
	if source == "" { source = "live" }
	if country == "" { country = "us" }

	_ = db.InsertKeywordSignalSnapshot(ctx, store.KeywordSignalSnapshot{
		Keyword: score.Keyword, Source: source, Country: country,
		Score: score.Score, Rating: score.Rating,
		SearchSignal: score.SearchSignal, CompetitionSignal: score.CompetitionSignal,
		DifficultySignal: score.DifficultySignal, TagCount: score.TagCount,
		TopListingCount: score.TopListingCount, CapturedAt: time.Now().UTC(),
	})
}
```

Call after `scoreKeyword(signals)` in `newNovelOpportunityCmd`, `newNovelSaturationCmd`, `newNovelListsOptimizeCmd`, and `newNovelWatchDriftCmd`. Recording must not change output and must not fail live analysis if SQLite is unavailable.

- [ ] Validate and commit:

```bash
gofmt -w internal/store/extras.go internal/store/keyword_signal_snapshots.go internal/store/keyword_signal_snapshots_test.go internal/cli/erank_signals.go internal/cli/opportunity.go internal/cli/watch_drift.go internal/cli/lists_optimize.go internal/cli/saturation.go
go test ./internal/store -run TestKeywordSignalSnapshotsRoundTrip
go test ./internal/store
go test ./internal/cli
go test ./...
git add internal/store/extras.go internal/store/keyword_signal_snapshots.go internal/store/keyword_signal_snapshots_test.go internal/cli/erank_signals.go internal/cli/opportunity.go internal/cli/watch_drift.go internal/cli/lists_optimize.go internal/cli/saturation.go
git commit -m "feat: persist erank keyword signal snapshots"
```

Expected: store and CLI tests pass; Data Pipeline Integrity has typed local-data evidence.

---

## Task 6: Record Generated-Tree Customizations

- [ ] Update `.printing-press-patches.json` with this entry:

```json
{
  "id": "erank-scorecard-insight-workflow-data",
  "summary": "Split eRank insight commands, added tests, removed a dead helper, and added keyword signal snapshot storage.",
  "reason": "These are eRank-specific local quality improvements that improve maintainability and data-pipeline evidence without changing upstream Printing Press.",
  "files": [
    "internal/cli/erank_signals.go",
    "internal/cli/opportunity.go",
    "internal/cli/listing.go",
    "internal/cli/listing_gaps.go",
    "internal/cli/tags.go",
    "internal/cli/tags_consensus.go",
    "internal/cli/watch.go",
    "internal/cli/watch_drift.go",
    "internal/cli/lists.go",
    "internal/cli/lists_optimize.go",
    "internal/cli/saturation.go",
    "internal/cli/angles.go",
    "internal/cli/helpers.go",
    "internal/cli/erank_drift_history.go",
    "internal/cli/erank_signals_test.go",
    "internal/cli/listing_gaps_test.go",
    "internal/cli/lists_optimize_test.go",
    "internal/cli/watch_drift_test.go",
    "internal/store/extras.go",
    "internal/store/keyword_signal_snapshots.go",
    "internal/store/keyword_signal_snapshots_test.go"
  ],
  "validated_outcome": "go test ./... passes; cli-printing-press publish validate passes; local scorecard rows improve where local code quality is measurable."
}
```

- [ ] Record validation notes outside code:

```text
Auth Protocol: unchanged because eRank browser-session auth is real and should be scored upstream.
MCP Token Efficiency: unchanged because the Printing Press MCP catalog shape should be improved upstream.
```

- [ ] Commit:

```bash
git add .printing-press-patches.json
git commit -m "docs: record erank scorecard patches"
```

---

## Task 7: Mirror Into Publishing Library

- [ ] Preview sync:

```bash
rsync -a --dry-run --itemize-changes \
  --exclude ".git/" \
  /Users/smacdonald/homegit/erank-cli/ \
  /Users/smacdonald/printing-press/library/erank/
```

Expected: output is limited to plan-touched files and safe metadata. If unrelated destructive changes appear, copy only touched files manually.

- [ ] Sync and verify:

```bash
rsync -a \
  --exclude ".git/" \
  /Users/smacdonald/homegit/erank-cli/ \
  /Users/smacdonald/printing-press/library/erank/
test -f /Users/smacdonald/printing-press/library/erank/internal/cli/erank_signals.go
test -f /Users/smacdonald/printing-press/library/erank/internal/store/keyword_signal_snapshots.go
test ! -f /Users/smacdonald/printing-press/library/erank/internal/cli/erank_insights.go
```

Expected: all checks exit successfully. No commit for this task.

---

## Task 8: Final Validation

- [ ] Run tests and diagnostics:

```bash
go test ./...
erank-pp-cli doctor --json
erank-pp-cli agent-context --pretty
cli-printing-press publish validate --dir /Users/smacdonald/printing-press/library/erank --json
```

Expected: tests pass, diagnostics are valid, publish validation passes.

- [ ] Run final scorecard:

```bash
cli-printing-press scorecard --dir /Users/smacdonald/printing-press/library/erank --json
cli-printing-press scorecard --dir /Users/smacdonald/printing-press/library/erank
```

Expected: total does not regress below `80/100`; local target rows improve where recognized; Auth Protocol and MCP Token Efficiency may remain unchanged.

- [ ] Run structural checks:

```bash
rg -n "func new(Opportunity|Listing|ListingGaps|Tags|TagsConsensus|Watch|WatchDrift|Lists|ListsOptimize|Saturation|Angles)Cmd" internal/cli
rg -n "extractResponseData" internal
rg -n 'CommandT''ODO|T''ODO' internal/cli/*_test.go
git status --short
git diff --stat HEAD
```

Expected: no matches for the first three commands and a clean working tree.

---

## Execution Recommendation

Use `superpowers:subagent-driven-development` for implementation. The tasks split cleanly into test, refactor, store, manifest, and validation slices. Inline execution with `superpowers:executing-plans` is also valid; keep the same task order and commit points.
