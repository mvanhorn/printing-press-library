# Task 04: Remaining Insight Engines

**Files:**
- Create: `internal/research/keyword.go`
- Create: `internal/research/keyword_test.go`
- Create: `internal/research/listing.go`
- Create: `internal/research/listing_test.go`
- Create: `internal/research/shop.go`
- Create: `internal/research/shop_test.go`
- Create: `internal/research/trend.go`
- Create: `internal/research/trend_test.go`
- Modify: `internal/cli/everbee_insights.go`

## Goal

Move the seven remaining flagship commands onto the same evidence pipeline as `opportunity shortlist`.

## Steps

- [ ] **Step 1: Add keyword and listing engine tests**

Tests must cover:

- `BuildNicheScore`
- `BuildKeywordClusters`
- `BuildTagGap`
- `BuildListingAudit`

Each test must include one matching evidence record and one unrelated evidence record. The unrelated record must not appear in ranked output.

- [ ] **Step 2: Implement keyword and listing engines**

Create these exported functions:

```go
func BuildNicheScore(scope ResearchScope, evidence []EvidenceRecord, plan ResearchPlan) ResponseEnvelope
func BuildKeywordClusters(scope ResearchScope, evidence []EvidenceRecord, limit int, plan ResearchPlan) ResponseEnvelope
func BuildTagGap(scope ResearchScope, evidence []EvidenceRecord, limit int, plan ResearchPlan) ResponseEnvelope
func BuildListingAudit(scope ResearchScope, evidence []EvidenceRecord, limit int, plan ResearchPlan) ResponseEnvelope
```

Required output:

- stable envelope fields
- non-empty `summary`
- non-empty `next_actions`
- warnings copied from the plan
- confidence based on coverage

Run:

```bash
go test ./internal/research -run 'TestNiche|TestKeyword|TestTag|TestListing' -count=1
```

Expected: pass.

- [ ] **Step 3: Wire keyword and listing commands**

Modify:

- `newNicheScoreCmd`
- `newKeywordsClusterCmd`
- `newTagsGapCmd`
- `newListingAuditCmd`

Add research flags and use resources:

- niche and keywords: `product_analytics`, `keyword_research`
- tags: `product_analytics`, `keyword_research`, `shops`
- listing: `product_analytics`, `keyword_research`

Run:

```bash
go test ./internal/cli -run 'TestNiche|TestKeywords|TestTags|TestListing|TestResearchRuntime' -count=1
```

Expected: pass.

- [ ] **Step 4: Add shop and trend tests**

Tests must cover:

- `BuildShopGaps`
- `BuildCompetitorWatch`
- `BuildTrendsDiff`

Trend tests must include two snapshots with the same scope and different `FetchedAt` values.

- [ ] **Step 5: Implement shop and trend engines**

Create these exported functions:

```go
func BuildShopGaps(scope ResearchScope, evidence []EvidenceRecord, plan ResearchPlan) ResponseEnvelope
func BuildCompetitorWatch(scope ResearchScope, snapshots []Snapshot, plan ResearchPlan) ResponseEnvelope
func BuildTrendsDiff(scope ResearchScope, snapshots []Snapshot, days int, plan ResearchPlan) ResponseEnvelope
```

Trend functions return low confidence and a warning when only one snapshot exists.

Run:

```bash
go test ./internal/research -run 'TestShop|TestCompetitor|TestTrend' -count=1
```

Expected: pass.

- [ ] **Step 6: Wire shop and trend commands**

Modify:

- `newShopGapsCmd`
- `newCompetitorsWatchCmd`
- `newTrendsDiffCmd`

Use resources:

- shop gaps: `shops`, `product_analytics`, `keyword_research`
- competitors watch: `shops`, `product_analytics`
- trends diff: `product_analytics`, `keyword_research`, `shops`

Run:

```bash
go test ./internal/research ./internal/cli -run 'TestShop|TestCompetitor|TestTrend|TestResearchRuntime' -count=1
```

Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add internal/research/keyword.go internal/research/keyword_test.go internal/research/listing.go internal/research/listing_test.go internal/research/shop.go internal/research/shop_test.go internal/research/trend.go internal/research/trend_test.go internal/cli/everbee_insights.go
git commit -m "feat: add evidence-backed everbee insight engines"
```
