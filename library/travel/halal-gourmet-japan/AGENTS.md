# Halal Gourmet Japan maintenance contract

Read current `--help` and `SKILL.md` before changing behavior. This source-specific adapter is maintained in `internal/hgj`, with preserved CLI hooks in `internal/cli/hgj_commands.go` and planning wiring in `hgj_plan.go` plus the five `plan_*.go` bodies.

## Evidence invariants

Canonical keys are source kind plus numeric alias ID; restaurant and prayer IDs never collide. Only successful full-detail reads can replace snapshot history. Keep source URLs, names/Japanese names when supplied and observation timestamps. Search cards never become matching or history baselines. Preserve the reported, missing, hidden-card, inapplicable and explicit-negative states; do not infer certification, ingredients, alcohol/pork policies, cross-contamination, hours or access from another label. HGJ verification month is separate from certification body/validity.

Plan commands are local and consume explicit selections. Their `// pp:data-source local` directives and MCP read-only annotations must match. Searches are live; inspection supports explicit local reads and treats auto as a fresh source read, with no silent fallback on failed requests. Preserve truthful provenance in agent output.

## Bounds and storage

Keep the AdaptiveLimiter and typed throttle errors in the sibling HTTP client. All network command calls need `boundCtx`. Retain the 8 MiB body cap, redirect boundary, input/result caps, maximum 20 IDs per kind and at most 400 pair candidates. Only two successful versions per entity are retained, with a 1,000-place cache cap. Perform history writes through one transaction; drain/close SQLite rows before follow-up queries. The stateless generated profile has no sync/staleness helpers, so domain hints use actual saved observation timestamps and direct get instructions rather than suggesting unsupported bulk sync.

## Verification

Run `go test -count=1 ./...` and `go vet ./...` after meaningful changes. Parsing/domain tests must check streamed SSR cards, identity isolation, hidden versus missing conditions, certification versus HGJ verification, explicit negatives, empty/error distinction, repeat query keys, throttle/timeout/body caps, kind-safe two-version persistence, requirement/applicability matching, bounded distances and absence of fake drift.

Before shipping, build CLI and MCP from the same source, exercise actual live food/prayer search and detail reads, assert each planning result against saved real details, and verify runtime MCP tool schemas/calls. Keep README/SKILL examples runnable. Preserve the generator-owned canonical installation block. Record downstream customizations under `.printing-press-patches` per the library contract.

## Release Ledger

`CHANGELOG.md` and `.printing-press-release.json` are assigned by public-library automation after merge; preserve them and do not hand-bump versions. Printer and creator handle are personal `zjsng`. Publication requires fresh source checks, the full Press matrix, independent review, current-head CI and the library readiness gate. Never merge the maintainer's PR from this task.
