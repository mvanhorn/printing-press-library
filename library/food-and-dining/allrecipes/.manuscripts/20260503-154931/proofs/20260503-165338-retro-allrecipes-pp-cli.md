# Printing Press Retro: allrecipes (run 20260503-154931)

## Session Stats
- API: allrecipes (re-validated reuse of run 20260426-230519)
- Spec source: synthetic (reused prior brief; transport upgraded from `browser-chrome` → `browser-chrome-h3` + `auth.type: cookie` per re-probe)
- Binary version: v3.7.0 → v3.8.0 (lefthook bumped local build mid-session)
- Prior binary version: v2.3.9 (10 minor releases earlier)
- Scorecard: 87/100 Grade B (after polish)
- Verify pass rate: 100% (post-polish), 97% (initial — single critical was browser-session-proof, expected for cookie-auth without interactive `auth login --chrome`)
- Fix loops: 1 shipcheck iteration + polish skill's diagnostic-fix-rediagnose loop
- Manual code edits: ~10 (port-and-adapt of cmd_browse / cmd_helpers / search.go to v3.7.0 patterns; pantry-aware grocery-list `--pantry-file` implementation; SKILL.md flag-name corrections)
- Features built from scratch: 0 net-new (8 transcendence features ported with reprint reconciliation: 6 prior-keep, 2 prior-reframe, 1 new — pantry-aware grocery-list)

## Findings

### F1. `mcp_token_efficiency` reads only the static `tools-manifest.json` and ignores cobratree-mirrored runtime tools (Scorer bug)

- **What happened:** Allrecipes ships 2 typed MCP tools in `tools-manifest.json` (recipes search + recipes get) plus ~37 user-facing Cobra commands that the runtime cobratree walker registers as additional MCP tools when the server starts. The scorecard reads only the static manifest, computes `avg tokens per tool = TotalTokens / 2 > 320` for two verbose typed-endpoint descriptions, and returns 0/10. The dimension is supposed to evaluate the agent-facing token cost; the agent actually sees ~37+ tools at runtime, with most cobratree tools carrying short one-line descriptions, so the true average is well within the "partial (4)" or "partial (7)" bands.
- **Scorer correct?** No. The scorer's denominator (`est.ToolCount`) measures the static manifest count, not the runtime cobratree mirror. For any CLI whose typed-endpoint count is small relative to its hand-built command tree (which is now most v3.x CLIs that opt into cobratree), this dimension reports a misleading 0/10.
- **Root cause:** `internal/pipeline/mcp_size.go:193` — `scoreMCPTokenEfficiency(dir)` calls `estimateMCPTokens(dir)`, which only walks `tools-manifest.json`. The function has no awareness of `internal/mcp/cobratree/` or the runtime walker that augments the surface from the Cobra tree.
- **Cross-API check:** Yes. Three concrete APIs with evidence:
  - **allrecipes** (this run): `tools-manifest.json` = 2 tools, runtime cobratree = ~37 commands → score 0/10.
  - **food52** (`~/printing-press/library/food52/`): synthetic spec, similar shape — small typed-endpoint count, large hand-built command surface (cmd_browse / cmd_recipe / cmd_open). Likely scores 0/10 on the same dimension.
  - **recipe-goat** (`~/printing-press/library/recipe-goat/`): multi-source synthetic CLI with even more hand-built fanout commands — same pattern. Predictable 0/10.
- **Frequency:** subclass:cobratree-mirrored CLIs with low typed-endpoint count. This is most v3.x CLIs that opted into the runtime cobratree mirror with synthetic specs (food52, recipe-goat, allrecipes, pagliacci-pizza, postman-explore, plus future CLIs in the same shape).
- **Fallback if the Printing Press doesn't fix it:** Polish skill currently absorbs the friction by classifying it as a "structural skip — calibrated for 50+ endpoint surfaces" and rejecting it from the surface-correctness narrative (see polish output for allrecipes). Reliable across CLIs but **wastes polish budget on every cobratree CLI** and propagates a misleading 0/10 in the scorecard JSON consumed downstream by `osc-status`, audit pipelines, and the public-library catalog ingestor.
- **Worth a Printing Press fix?** Yes — small change, generalizable benefit. Either (a) augment `estimateMCPTokens` to also count cobratree-walked Cobra commands when `internal/mcp/cobratree/` is present, OR (b) mark the dimension N/A (excluded from denominator) when the typed-endpoint count is below a threshold AND cobratree mirroring is enabled. (b) is the simpler safer fix; (a) is the more accurate fix.
- **Inherent or fixable:** Fixable. Both options are mechanical scorer changes.
- **Durable fix:** Path (b) — extend `recordOptionalScore`'s scoring branch to mark `mcp_token_efficiency` unscored when `tools-manifest.json` lists fewer than 5 tools AND `internal/mcp/cobratree/cobratree.go` exists in the CLI directory. Path (a) — walk `internal/cli/` Cobra command literals (the same AST the runtime walker uses) and add their description token count to the average. Both fixes leave standalone-typed-endpoint CLIs (no cobratree) scored as before.
- **Test:** Positive: re-score allrecipes after the fix → dimension marked N/A or scores ≥4/10. Negative: re-score a CLI without cobratree (none in current catalog, but `kalshi`/`pokeapi` are typed-endpoint-heavy) → dimension still scored, value unchanged.
- **Evidence:** Polish skill output for allrecipes — "mcp_token_efficiency 0/10 — structural. The CLI exposes only 2 typed MCP tools; code-orchestration (mcp.orchestration: code) is calibrated for 50+ endpoint surfaces. Adding it for a 2-tool surface would be scaffolding, not improvement." The polish skill correctly recognized the scorer is misapplied; the fix should be in the scorer, not in the polish workaround.
- **Step G case-against (and why it fails):** Case-against — "the scorer is reading the canonical manifest; cobratree is ad-hoc and shouldn't be double-counted; polish has a workaround." Why it fails — the manifest count of 2 isn't truthful about agent-facing surface; agents see ~37 tools at runtime; the polish workaround triggers on every cobratree CLI which is now most v3.x CLIs and wastes polish-skill attention on a recurring scorer artifact rather than real improvements.

## Prioritized Improvements

### P3 — Low priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | `mcp_token_efficiency` ignores cobratree-mirrored runtime tools | Scorer (`internal/pipeline/mcp_size.go`) | subclass: cobratree CLIs with <5 typed endpoints | high (polish skill consistently classifies as structural skip) | small | gate the new behavior on cobratree presence + low typed-endpoint count to avoid changing scoring for typed-endpoint-heavy CLIs |

### Skip

| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| C1 | verify-skill's static AST scanner can't follow `cobra.Command{Use: spec.use}` indirection through a helper struct, producing 4 false-positive "unknown command" errors for cmd_browse.go's `category`/`cuisine`/`ingredient`/`occasion` constructors | Step G: case-against stronger. The fix in this CLI was a 5-minute refactor to literal `Use:` strings, and the agent learned the pattern immediately. Verify-skill is intentionally a static AST scanner; teaching it to follow arbitrary struct-field indirection adds complexity and false-positives in the other direction. The smaller cheaper fix is a one-line note in `skills/printing-press/SKILL.md` Phase 3 build checklist: "Use literal `Use:` strings on hand-written commands so verify-skill's AST scanner finds them." That's a SKILL tweak, not a generator change. |
| C5 | `printing-press lock promote --cli <api>-pp-cli --dir <work-dir>` writes `.printing-press.json` with `run_id: null` because it looks up `research.json` at a binary-derived scope (`allrecipes-pp-cli-745f4045`), not the scope used by `generate` (`cli-printing-press-8bdedb85`) | Step B: only one APIs with evidence (allrecipes). The standard `/printing-press` flow may not hit this path because state.RunID is set elsewhere. Need observation on at least 2 more CLIs before filing — could be a manual-invocation-only edge case. |

### Dropped at triage

| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| C2 | Synthetic spec `extra_commands:` declarations generate no code; agent must hand-build all 22 commands. Could the generator emit stubs? | `printed-CLI` — `extra_commands` is intentionally a marker, not a code generator. The skill already provides starter RunE skeletons in Phase 3. Auto-generated stubs would be deleted-and-rewritten boilerplate. |
| C3 | Phase 1.5c.5 subagent reprint reconciliation: prior 8 features kept/reframed/dropped, audit trail persisted | `iteration-noise` — this worked correctly and produced clean output. Not a finding. |
| C6 | Polish skill reported `verify_after: 100%` even though pre-polish verify reported a critical browser-session-proof failure | `unproven-one-off` — only one observation in this cookie-auth path. Could be polish ran shipcheck differently, or the browser-session-proof check was relaxed when no cookie was available. Insufficient evidence for a generalizable finding. |

## Work Units

### WU-1: `mcp_token_efficiency` excludes cobratree-mirrored CLIs from the scored denominator (from F1)
- **Goal:** When a CLI uses runtime cobratree mirroring AND has fewer than 5 typed MCP endpoints, mark `mcp_token_efficiency` as unscored (N/A) so it's excluded from the tier1 denominator and doesn't propagate misleading 0/10 to the scorecard JSON.
- **Target:** `internal/pipeline/mcp_size.go` (`scoreMCPTokenEfficiency`) and `internal/pipeline/scorecard.go` (`recordOptionalScore` / `scorecardTierMax`).
- **Acceptance criteria:**
  - **Positive test:** Re-score allrecipes after the change → dimension reported as N/A in human output, excluded from tier1 denominator, omitted from `Scorecard.UnscoredDimensions` callouts in the same way `path_validity` and `live_api_verification` are.
  - **Negative test:** Re-score a typed-endpoint-heavy CLI without cobratree (e.g., `kalshi`, `pokeapi`) → `mcp_token_efficiency` is still scored, value unchanged from prior runs.
  - **Detection:** The change should detect cobratree presence by checking for `internal/mcp/cobratree/cobratree.go` in the CLI directory (the canonical generator-emitted file). The "low typed-endpoint count" threshold can be calibrated against the existing tools-manifest.json tool count.
- **Scope boundary:** Does NOT add cobratree-walked tool counts to the per-tool token average. That's the more accurate option (path (a) in the finding) but adds complexity and risks double-counting where typed and cobratree tools overlap. The N/A path is the smaller safer fix.
- **Dependencies:** None.
- **Complexity:** small.

## Anti-patterns

(none observed in this session worth flagging — the workflow ran cleanly through preflight, briefing, version-delta re-validation, brief reuse, subagent reprint, generate, port-and-adapt, shipcheck, polish, promote, archive)

## What the Printing Press Got Right

- **Major-version re-validation prompt fired correctly.** v2.3.9 → v3.7.0 triggered the mandatory prompt with the five-bucket "what changed" list. The prompt converted into actionable re-probing (probe-reachability surfaced the new Cloudflare clearance situation that wasn't in the prior brief), preventing a stale-assumption regression.
- **`probe-reachability` distinguished `standard_http` (browse/search) from `browser_clearance_http` (recipe detail) on the same domain.** This nuanced result drove the user to opt into Chrome clearance cookie capture, which then routed cleanly through the cookie-auth template (auto-emitted `auth login --chrome`, `auth status`, `auth refresh`, `auth logout` plus the `auth_hint` in `doctor` output).
- **Phase 1.5c.5 novel-features subagent's reprint reconciliation worked.** Prior 8 transcendence features → 6 prior-keep + 2 prior-reframe + 1 new + 1 reclassified-to-absorbed, with the dropped `grocery-list` (as novel) properly captured in the reprint surface so the user could override at the gate. Customer model (4 personas) and Killed candidates (8 with sibling-kill rationale) persisted to the brainstorm artifact for retro/dogfood reference.
- **Cookie-auth template's doctor integration.** The generator-emitted `doctor` command already prints `auth_hint: "allrecipes-pp-cli auth login --chrome"` when no cookie is present — exactly the UX the prior retro F1 fix was designed to enable. The hint emerges from the cookie-auth path without per-CLI customization.
- **Polish skill's "structural skip" classification.** Correctly identified `mcp_token_efficiency 0/10`, `type_fidelity 2/5`, `breadth 6/10`, `insight 6/10` as scorer-artifact-not-defect and skipped them with reasoning rather than chasing scoring noise. The pattern is mature and prevents agents from churning on calibration artifacts.
- **`printing-press validate-narrative` flagged 9 of 10 missing commands at first build.** Caught the pre-port state cleanly; after the port + literal-Use refactor + grocery-list `--pantry-file` implementation, validate-narrative reported `OK: 10 narrative commands resolved against the CLI tree`. The validator made the gap visible early instead of letting it ship.
- **Lock heartbeat pattern.** `lock acquire`, `lock update --phase ...` between expensive steps, and `lock promote` at the end. The heartbeat ran cleanly across the long port-then-shipcheck-then-polish sequence and prevented the staleness threshold from firing.
