# Printing Press Retro: redfin

## Session Stats
- API: redfin
- Spec source: synthetic / browser-sniffed (Stingray endpoints documented from public scraper landscape)
- Scorecard: 84/100 (Grade A — after polish)
- Verify pass rate: 92%
- Fix loops: 2 shipcheck iterations + 2 polish rounds
- Manual code edits: ~6 substantive (rank `--limit`, trends parser, market `Use:`, summary `Use:`, default `--metric`, malformed research.json fix)
- Features built from scratch: 16 commands (3 promoted-rebuilds + 13 transcendence/absorbed)

## Findings

### F1: `validate-narrative` is a manual SKILL step that agents skip in practice (skill instruction gap → automation candidate)

- **What happened:** verify-skill failed on iteration 1 with 7 errors — `--region austin`, `--region austin-tx` referenced in research.json's `narrative.recipes` and `narrative.quickstart` for drops/rank/export/summary, but the actual CLI flags are `--region-id N --region-type N` (or `--region-slug` for export). Plus `summary <region-slug-or-id>` required positional broke 0-arg verify probes. The SKILL.md (Phase 2) documents `printing-press validate-narrative --strict --full-examples` as REQUIRED before saving examples, but I (agent) skipped that step in this session.
- **Scorer correct?** N/A — no scorer involved; verify-skill is doing its job.
- **Root cause:** SKILL relies on agent discipline to invoke `validate-narrative` after writing research.json. The same SKILL section also says it's REQUIRED before publishing. In practice, the agent jumps from research.json → generate → shipcheck and discovers the narrative defects post-shipcheck. The recurring pattern is that anything *not invoked by the binary itself* gets skipped.
- **Cross-API check:** Same root cause hit apartments-pp-cli in this same session — verify-skill caught `apartments-pp-cli search`, `apartments-pp-cli get`, and `--shortlist` as references to non-existent commands/flags in the SKILL. That's 2 of 2 generations in this session shipping narrative defects to shipcheck. Both required a fix-loop.
- **Frequency:** Recurring across synthetic-spec CLIs where the LLM authoring research.json doesn't have a built-in compiler check against actual CLI flags.
- **Fallback if not fixed:** verify-skill catches it at shipcheck; the agent then loops back and fixes research.json + sed-replaces SKILL/README. ~5-15 minutes per CLI of fix-loop work that would be eliminated if validate-narrative ran automatically.
- **Worth a fix?** Yes — borderline P2/P3. The fix is small (one extra binary step in `generate` or a new shipcheck leg), helps every synthetic-spec generation, and removes a recurring fix-loop.
- **Inherent or fixable:** Fully fixable. The binary already has the validator; it's a wiring problem, not a missing capability.
- **Durable fix:** Two reasonable options:
  1. **Add `validate-narrative` as a step in `printing-press generate`** after research.json is committed. Fail (or warn) on missing/wrong commands.
  2. **Add `validate-narrative` as a 6th shipcheck leg** so it runs automatically with every `printing-press shipcheck` invocation alongside dogfood/verify/workflow-verify/verify-skill/scorecard.
  Option 2 is preferred — it groups with the other "everything must hold" checks and gives the verdict more reach. Strip API-specific details: the validator already takes `--research` and `--binary`; shipcheck would discover both from `--dir`.
- **Test:**
  - Positive: A synthetic CLI whose research.json `narrative.recipes` reference a command with a wrong flag should fail shipcheck.
  - Negative: A CLI whose research.json is correct should pass shipcheck unchanged (no false positives on `narrative` shape).
- **Evidence:** apartments-pp-cli verify-skill iteration 1 failure (7 errors); redfin-pp-cli verify-skill iteration 1 failure (7 errors). Both required identical fix-loops (sed-replace research.json + SKILL/README + rebuild + re-shipcheck).
- **Step G case-against:** "validate-narrative is documented as REQUIRED in the SKILL. The fix is for the agent to follow the SKILL, not to make the binary stricter. Adding a 6th shipcheck leg adds runtime cost on every CLI even when narrative is correct." Counter to that: in practice, agent discipline alone doesn't catch this — 2/2 CLIs in this session shipped defects to shipcheck. The recurring fix-loop cost compounds across the catalog. Case-for is stronger because the failure mode is mechanical (wrong flag string in JSON) and the validator already exists.

## Prioritized Improvements

### P3 — Low priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Wire `validate-narrative` into shipcheck | Binary (`internal/pipeline/`) + SKILL | every synthetic-spec CLI | Catches it at verify-skill anyway, but only after a fix loop | small | None — validator already exists; just wire it as a leg |

### Skip

| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| Auto-refresh stderr noise on synthetic specs | `internal/cli/auto_refresh.go` calls Stingray-style endpoints without spec-default params (e.g., `al=1`), gets HTTP 400, prints `sync_error` warnings on every read | **Step B:** only 2 named with evidence (apartments-pp-cli, redfin-pp-cli); other catalog CLIs have specs that don't require unconditional defaults. The pattern is narrow to spec shapes that mandate query params for every call. |
| `<required-positional>` Use: in optional-arg commands | summary/market in redfin had `Use: "summary <region-slug-or-id>"` (required) breaking verify probes; SKILL principle 8 already documents this pattern | **Step G:** SKILL principle 8 explicitly documents `Use: <cmd> [x]` (square brackets) for optional positionals. The agent (me) violated documented SKILL guidance — this is a discipline issue, not a generator gap. The case-against is stronger than case-for. |
| LLM-authored response parsers fabricate fields not in actual response | redfin's first ParseTrendsResponse expected `payload.months[]` per-month array; Redfin's actual response is a flat `{medianListPrice, avgDaysOnMarket, ...}` snapshot | **Step G:** caught by Phase 5 dogfood within minutes; the generator can't realistically pre-fetch every API's response shape during Phase 3 (network access varies). Recurring cost is bounded — agents fix on first live test. The case-against ("contained by existing dogfood gate") is stronger. |
| `category` from spec doesn't propagate to `.printing-press.json` manifest | Both apartments and redfin had `category: <missing>` in manifest after promote despite spec setting `category: other` | **Step G:** manifest's category field is informational; publish skill re-asks via the user before opening the PR. Low impact; the publish flow has a fallback. |

### Dropped at triage

| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Sync-search `path` JSON output | redfin's sync-search emitted local DB path as `path` field instead of the URL search path | printed-CLI |
| `rank --limit` flag missing on first build | Phase 3 subagent oversight; added in iteration | iteration-noise (covered by acceptance test, easy fix) |
| `trends` default `--metric` non-empty filter | Default `--metric "median-sale"` filtered all rows when Redfin's response lacked that metric for the period | printed-CLI (specific to redfin's Phase 3 prompt) |
| Dogfood SKIP on malformed research.json | Polish round 2 fixed unescaped quotes in research.json that I'd typed; dogfood had silently SKIP-ed novel features | iteration-noise (typo recovery; polish caught it) |

## Work Units

### WU-1: Wire `validate-narrative` into shipcheck (from F1)
- **Goal:** `printing-press shipcheck` runs `validate-narrative --strict --full-examples` automatically as a 6th leg, failing the umbrella when narrative.recipes/quickstart reference commands or flags that don't exist on the built binary.
- **Target:** `internal/pipeline/` (the shipcheck umbrella that orchestrates dogfood + verify + workflow-verify + verify-skill + scorecard) — add `narrative-validate` to the canonical leg list. Possibly also `internal/cli/shipcheck.go` for the per-leg flag plumbing.
- **Acceptance criteria:**
  - Positive: a CLI whose research.json has `"command": "<cli> nonexistent --bogus"` in `narrative.recipes` fails shipcheck with a leg labelled `validate-narrative` reporting the bad command. Re-running `validate-narrative` standalone returns the same finding.
  - Negative: a CLI whose research.json `narrative.recipes` are all correct passes shipcheck unchanged. No new false positives. The leg costs <5 seconds.
  - Idempotency: running shipcheck twice on the same CLI gives the same verdict.
- **Scope boundary:** Does NOT change `validate-narrative`'s behavior or flags. Does NOT alter how research.json is authored. Just wires the existing binary command as an automatic shipcheck leg with a verdict that contributes to the umbrella's pass/fail.
- **Dependencies:** None — `validate-narrative` already exists in the binary.
- **Complexity:** small (one new leg in the umbrella, plumbing analogous to verify-skill).

## Anti-patterns
- **Treating "validate-narrative is documented as REQUIRED in the SKILL" as sufficient.** Two consecutive synthetic-CLI sessions shipped narrative defects through to shipcheck because the agent skipped the manual step. SKILL discipline doesn't compose — what's not enforced by the binary gets skipped at scale.
- **LLM-authored parsers without a real response sample.** When the subagent in Phase 3 invents a parser for an API's response shape from prose alone (no fixture, no live probe), it consistently picks reasonable-looking but wrong field names. This was caught by Phase 5 dogfood here, but it's a recurring cost that better Phase 3 prompts (or generator-supplied fixtures) could reduce.

## What the Printing Press Got Right
- **Surf with Chrome TLS fingerprint as the synthetic-CLI runtime default.** `probe-reachability` correctly classified Redfin as `browser_clearance_http` low-confidence; the printed CLI's Surf transport cleared `/stingray/api/gis` and `/stingray/api/region/.../aggregate-trends` first try with no clearance cookie capture. AWS-WAF marker was ambient, not a block.
- **Synthetic spec + traffic-analysis hint pipeline.** `printing-press generate` consumed both `redfin-spec.yaml` and `discovery/traffic-analysis.json`, emitted the `homes`/`listing`/`market` resource scaffolding, and the auto-generator's quality gates (go mod tidy / vet / build / --help / version / doctor) all passed first try.
- **Dogfood resync of README/SKILL after polish.** When polish round 2 fixed the malformed research.json, dogfood automatically synced the README's "Unique Features" section and `.printing-press.json` `novel_features` from `novel_features_built` — no manual rewrite needed.
- **Polish skill caught a typo.** Round 2 found unescaped quotes I'd typed in research.json's export-feature `example` field. Without the polish pass, dogfood would have continued silently SKIP-ing novel features and the publish-validate transcendence gate might have failed downstream.
