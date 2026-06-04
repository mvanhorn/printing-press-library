# Task 03: Opportunity Shortlist Vertical Slice

**Files:**
- Create: `internal/research/opportunity.go`
- Create: `internal/research/opportunity_test.go`
- Modify: `internal/cli/everbee_insights.go`

## Goal

Convert `opportunity shortlist` to the new cache-aware evidence pipeline before touching the other flagship commands.

## Steps

- [ ] **Step 1: Write opportunity engine test**

Create `internal/research/opportunity_test.go`.

```go
func TestOpportunityShortlistRanksMatchingEvidence(t *testing.T) {
	scope := ResearchScope{Kind: ScopeQuery, Value: "teacher gift"}
	plan := ResearchPlan{Scope: scope, DataSource: DataSourceLocal}
	evidence := []EvidenceRecord{
		{ID: "p1", Title: "Teacher Gift Mug", Text: "teacher gift mug", Tags: []string{"teacher", "gift"}},
		{ID: "p2", Title: "Wedding Sign", Text: "wedding sign"},
	}

	out := BuildOpportunityShortlist(scope, evidence, 10, plan)

	if len(out.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(out.Records))
	}
	if out.Records[0].ID != "p1" {
		t.Fatalf("top id = %s, want p1", out.Records[0].ID)
	}
	if out.Confidence <= 0 {
		t.Fatalf("confidence = %f, want positive", out.Confidence)
	}
}
```

Run: `go test ./internal/research -run TestOpportunity -count=1`

Expected: fails because `BuildOpportunityShortlist` is not created yet.

- [ ] **Step 2: Add opportunity engine**

Create `internal/research/opportunity.go` with:

- `func BuildOpportunityShortlist(scope ResearchScope, evidence []EvidenceRecord, limit int, plan ResearchPlan) ResponseEnvelope`
- token matching against title, tags, keywords, and text
- scoring from text match, tag match, estimated sales, estimated revenue, and rank
- stable `next_actions`

The envelope must include `scope`, `data_source`, `freshness`, `summary`, `records`, `evidence`, `confidence`, `coverage`, `warnings`, and `next_actions`.

Run: `go test ./internal/research -run TestOpportunity -count=1`

Expected: pass.

- [ ] **Step 3: Wire opportunity command**

Modify `newOpportunityShortlistCmd` in `internal/cli/everbee_insights.go`.

Required behavior:

- add `addResearchFlags(cmd, &researchOpts)`
- build `research.ResearchScope{Kind: research.ScopeQuery, Value: query}`
- use resources `product_analytics` and `keyword_research`
- call the runtime resolver
- normalize snapshot raw records when needed
- print `research.BuildOpportunityShortlist`

Keep the existing command name, example, and `mcp:read-only` annotation.

- [ ] **Step 4: Add command test**

Create or extend `internal/cli/everbee_insights_commands_test.go`. Test `--no-refresh --agent` returns JSON envelope fields for `opportunity shortlist`.

Run:

```bash
go test ./internal/research ./internal/cli -run 'TestOpportunity|TestResearchRuntime|TestOpportunityShortlistCommand' -count=1
```

Expected: pass.

- [ ] **Step 5: Smoke test CLI command**

Run:

```bash
go run ./cmd/everbee-pp-cli opportunity shortlist --query "teacher gift" --no-refresh --agent
```

Expected JSON fields:

- `scope`
- `data_source`
- `freshness`
- `summary`
- `confidence`
- `coverage`
- `next_actions`

- [ ] **Step 6: Commit**

```bash
git add internal/research/opportunity.go internal/research/opportunity_test.go internal/cli/everbee_insights.go internal/cli/everbee_insights_commands_test.go
git commit -m "feat: add evidence-backed opportunity shortlist"
```
