# Task 05: Workflow, Docs, And Verification

**Files:**
- Create: `workflow_verify.yaml`
- Modify: `README.md`
- Modify: `SKILL.md`

## Goal

Document the cache-aware research flow, add workflow verification, and run final quality gates.

## Steps

- [ ] **Step 1: Add workflow manifest**

Create `workflow_verify.yaml`.

```yaml
workflows:
  - name: "EverBee local research loop"
    primary: true
    steps:
      - command: "opportunity shortlist"
        args:
          query: "teacher gift"
          no-refresh: "true"
          agent: "true"
        mode: local
        expect_fields:
          - "scope"
          - "data_source"
          - "summary"
          - "confidence"
          - "next_actions"
      - command: "niche score"
        args:
          keyword: "teacher gift"
          no-refresh: "true"
          agent: "true"
        mode: local
        expect_fields:
          - "scope"
          - "summary"
          - "confidence"
      - command: "tags gap"
        args:
          query: "teacher gift"
          no-refresh: "true"
          agent: "true"
        mode: local
        expect_fields:
          - "scope"
          - "summary"
          - "next_actions"
      - command: "listing audit"
        args:
          listing-id: "123456789"
          no-refresh: "true"
          agent: "true"
        mode: local
        expect_fields:
          - "scope"
          - "summary"
          - "warnings"
```

- [ ] **Step 2: Update docs**

Add this text to `README.md` and `SKILL.md` near the insight command guidance:

```markdown
Insight commands read local research snapshots first. If matching data is missing or stale, they refresh only the EverBee data needed for that query and save the result locally for repeat analysis. Use `--no-refresh` for offline/local-only runs, `--refresh` to force a targeted pull, and `--max-age` to control freshness.
```

- [ ] **Step 3: Run workflow and skill verification**

Run:

```bash
cli-printing-press workflow-verify --dir /Users/smacdonald/homegit/everbee-cli --json
cli-printing-press verify-skill --dir /Users/smacdonald/homegit/everbee-cli --json
```

Expected:

- workflow verdict is not `workflow-fail`
- verify-skill findings list is empty

- [ ] **Step 4: Commit workflow and docs**

```bash
git add workflow_verify.yaml README.md SKILL.md
git commit -m "docs: document everbee research freshness workflow"
```

- [ ] **Step 5: Run core Go verification**

Run:

```bash
go test ./...
go vet ./...
go build ./...
```

Expected: all pass.

- [ ] **Step 6: Run Printing Press verification**

Run:

```bash
cli-printing-press dogfood --dir /Users/smacdonald/homegit/everbee-cli --spec /Users/smacdonald/homegit/everbee-cli/spec.yaml --research-dir /Users/smacdonald/printing-press/.runstate/everbee-cli-b328e021/runs/20260522-153448
cli-printing-press verify --dir /Users/smacdonald/homegit/everbee-cli --spec /Users/smacdonald/homegit/everbee-cli/spec.yaml --json
cli-printing-press workflow-verify --dir /Users/smacdonald/homegit/everbee-cli --json
cli-printing-press verify-skill --dir /Users/smacdonald/homegit/everbee-cli --json
cli-printing-press scorecard --dir /Users/smacdonald/homegit/everbee-cli --spec /Users/smacdonald/homegit/everbee-cli/spec.yaml --research-dir /Users/smacdonald/printing-press/.runstate/everbee-cli-b328e021/runs/20260522-153448 --live-check --json
cli-printing-press tools-audit /Users/smacdonald/homegit/everbee-cli --json
cli-printing-press pii-audit /Users/smacdonald/homegit/everbee-cli --manuscripts-dir /Users/smacdonald/printing-press/.runstate/everbee-cli-b328e021/runs/20260522-153448 --json
```

Expected:

- dogfood verdict is `PASS`
- verify pass rate is 100%
- workflow verdict is not `workflow-fail`
- verify-skill findings are empty
- tools-audit returns `null`
- pii-audit returns `null`
- scorecard remains grade `A`

- [ ] **Step 7: Run gosec**

Run:

```bash
go run github.com/securego/gosec/v2/cmd/gosec@latest -fmt=json -out=/tmp/everbee-scorecard-gosec.json ./...
```

Expected:

- zero unresolved findings in hand-authored files changed by this plan
- generated-file findings remain documented as generated-code findings

- [ ] **Step 8: Commit verification artifacts when present**

If verification changes tracked artifacts, inspect the diff and commit relevant files:

```bash
git add .printing-press.json .printing-press-patches.json .manuscripts
git commit -m "chore: refresh everbee verification artifacts"
```

Skip this commit when there are no tracked verification artifact changes.
