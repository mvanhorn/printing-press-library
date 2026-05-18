# Printing Press Retro: YouTube

## Session Stats
- API: youtube
- Spec source: vendor OpenAPI 3.0 (trimmed in-repo subset of YouTube Data API v3)
- Scorecard: 77/100 (B) after polish
- Verify pass rate: 100%
- Fix loops: 3 (publish→Greptile→re-publish)
- Manual code edits: ~12 (rename pass + new endpoint + 2 novel commands + flag + ctx wiring + html unescape in 3 files)
- Features built from scratch: 2 novel (`videos-comments`, `channel-uploads`); 1 typed endpoint hand-authored to avoid full regen (`comment-threads-list`)

## Findings

### 1. Publish skill regenerates and commits the cli-skills mirror; the public library now rejects this for fork PRs (skill instruction gap)

- **What happened:** `/printing-press-publish` Step 6 says *"Regenerate the flat cli-skills mirror from the library tree so library PR CI passes mirror parity"* and runs `go run ./tools/generate-skills/main.go` then commits the output. The public library's current `verify-library-conventions.yml` workflow has two checks that conflict for fork PRs: (a) **`Guard against hand-edits to cli-skills mirror`** rejects any non-`github-actions[bot]` commit touching `cli-skills/pp-*/SKILL.md`; (b) **`Fail on cli-skills drift from fork`** fails when the generator would produce a `cli-skills/pp-<slug>/SKILL.md` that isn't committed, AND the auto-fix step that bridges them is gated on `head.repo.full_name == github.repository`. Following the publish skill triggered (a); reversing course (dropping the mirror) triggered (b). No state satisfies both for a fork PR adding a new CLI. Cost: 3 push cycles, 1 pre-merge maintainer-blocker comment.
- **Scorer correct?** N/A — instruction conflict, not a scorer finding.
- **Root cause:** `skills/printing-press-publish/SKILL.md` Step 6's instruction to commit the mirror was correct against an older library workflow; the library has since added the Guard check that treats committed mirrors as hand-edits. The skill was not updated to follow.
- **Cross-API check:** Affects every fork-PR contributor publishing a new CLI (anyone who is not `mvanhorn`). Same-repo PRs are fine because the auto-fix bot can push to the PR head; fork PRs cannot delegate to the bot.
- **Frequency:** every fork PR for a brand-new CLI. Recurs at every external publish.
- **Fallback if the Printing Press doesn't fix it:** every external contributor hits the same 3-push fix cycle and posts the same maintainer-facing explanation comment.
- **Worth a Printing Press fix?** Yes. The skill is the contract bridge — it's the right place to fix the contributor-side behavior. The maintainer-side workflow fix (loosen Guard or extend auto-fix to fork PRs) is separate and lives in the public library.
- **Inherent or fixable:** Fixable. Drop the mirror-regen-and-commit from publish Step 6 entirely. Fork PRs ship without the mirror; same-repo PRs already had it auto-handled by the bot. If the maintainer hasn't yet patched the workflow, the fork PR's `Verify`/`Validate SKILL.md` jobs will flag and the maintainer can decide; that's a strictly better state than the current "publish skill produces guaranteed-rejected PRs."
- **Durable fix:** Edit `~/.claude/skills/printing-press-publish/SKILL.md` Step 6 to remove the mirror-regen lines (`if [ -f "$PUBLISH_REPO_DIR/tools/generate-skills/main.go" ]; then ... fi`) and add a note explaining why: *"The cli-skills/pp-<slug>/SKILL.md mirror is auto-regenerated post-merge by the library's `generate-skills.yml` workflow. Do not regenerate or commit it from the publish flow — the library's `Guard against hand-edits to cli-skills mirror` check rejects fork PRs that touch the mirror."* If `tools/generate-skills` is later moved to be re-runnable from fork PRs (e.g., via PAT-based push), revisit.
- **Test:** positive: a fork PR for a new CLI that does NOT include `cli-skills/pp-<slug>/SKILL.md` passes the Guard check. negative: regenerate-and-commit path is removed (grep `tools/generate-skills` in publish SKILL.md returns 0 hits).
- **Evidence:** PR #585. After the publish skill committed the mirror: Greptile P1 *"`cli-skills/pp-*/SKILL.md` must not be committed in a PR"* (`gh api repos/mvanhorn/printing-press-library/pulls/585/comments` comment id discussion_r3249672558). After dropping the mirror to fix that: `Validate SKILL.md against shipped CLI source` failed with *"cli-skills/ must be generated from library/<category>/<slug>/SKILL.md"*. After re-adding: `Verify` failed with *"Hand-edit to generated mirror in commit 216ea7301d85"* (run 25931789915).
- **Related prior retros:** None matched in `~/printing-press/manuscripts/*/proofs/` (zero prior retros locally). Adjacent open issue: **#971** *"[P3] Update publish SKILL Step 6 staged_dir guidance to api-slug keying"* — same Step 6 area but different concern (path keying); reference for context.

### 2. Generator's HTTP client discards `cmd.Context()` (generator template)

- **What happened:** `internal/client/client.go`'s `do()` calls `http.NewRequest(method, ...)`, dropping the calling command's context. `--timeout`, `Ctrl+C` (SIGINT), and any `context.WithTimeout` set by the calling command are not honored by in-flight Data API calls — only `c.HTTPClient.Timeout` fires. Hand-authored novel commands in this CLI that thread `cmd.Context()` through the call stack lose it at the HTTP boundary.
- **Scorer correct?** Yes. Greptile P2 caught it at PR #585 review (correct; the bug is real and affects every printed CLI's generated client).
- **Root cause:** `internal/generator/templates/client.go.tmpl` (or wherever the client template lives) emits `http.NewRequest` instead of `http.NewRequestWithContext`, and the `do()` signature does not accept `context.Context`. The pipeline helper `resolvePaginatedRead` already takes a `ctx` parameter from typed-endpoint handlers, but it's discarded once it reaches the client.
- **Cross-API check:** Every printed CLI uses the same client.go template. Verified for `youtube-pp-cli` (search-bulk pages sequentially); applies identically to any CLI with multi-page sync (notion-pp-cli, spotify-pp-cli, linear-pp-cli — all have multi-page list endpoints in the catalog).
- **Frequency:** every printed CLI.
- **Fallback if the Printing Press doesn't fix it:** Claude does not catch this without a reviewer like Greptile pointing at it. The hand-authored opt-in workaround I added (`Client.WithContext(ctx)`) only fixes commands that explicitly call it; generated typed endpoints stay broken.
- **Worth a Printing Press fix?** Yes. Per-CLI patches don't compound; a generator template fix is one-and-done across every existing and future printed CLI.
- **Inherent or fixable:** Fixable. Two options:
  - **Minimal:** swap `http.NewRequest` → `http.NewRequestWithContext(ctx, ...)` in the client template and add `ctx context.Context` as the first parameter to `do()`. Update `Get`/`Post`/etc. callers to take `context.Context` first. Update `resolvePaginatedRead` and any other helper that calls into the client to pass the ctx through. Affects ~12 callers per CLI but mechanically.
  - **Less-invasive:** add `ctx` as a `Client` struct field with a `WithContext(ctx)` shallow-copy method, default to `context.Background()`. Each command does `c = c.WithContext(cmd.Context())` once after `flags.newClient()`. Smaller patch surface but requires every command to opt in. (This is what I patched in for youtube-pp-cli; recorded in `.printing-press-patches.json` machine_followups.)
- **Durable fix:** Prefer the **minimal** option (proper signature change) — it makes correct behavior the default. The opt-in `WithContext` pattern is the right per-CLI patch shape but should not be the generator's default. Implement in the client template + update `resolvePaginatedRead` to thread `ctx` through.
- **Test:** positive: regenerate a CLI, run `<cli> sync &` then `kill -SIGINT` — sync should exit within ~100ms, not after the in-flight HTTP call finishes. negative: `--timeout 1s` against a long-running endpoint should fail with `context deadline exceeded`, not hang for 30s on the default `HTTPClient.Timeout`.
- **Evidence:** PR #585 Greptile P2 (`gh api repos/mvanhorn/printing-press-library/pulls/585/comments` id discussion_r3249672648). The reviewer specifically noted *"`videos_transcript.go` correctly uses `http.NewRequestWithContext` for its own client; the inconsistency means long-running Data API calls (e.g., a multi-page `search-bulk`) cannot be cancelled cleanly."* — the `videos_transcript.go` was hand-authored novel code that did the right thing; the generated client did not.
- **Related prior retros:** **#436 CLOSED** *"Generator: propagate caller context through store.Open / migrate"* — `extends`. Same context-propagation theme but for the SQLite store layer. The fix landed for `store`; the HTTP client wasn't covered. This finding extends the same pattern to the client.

### 3. MCPBinary name derived from spec.info.title instead of api_name slug, breaking installation when they diverge (subclass:multi-word-spec-title)

- **What happened:** YouTube's spec has `info.title: "YouTube Data API v3"`. The OpenAPI parser's `cleanSpecName(doc.Info.Title)` derives `apiSpec.Name = "youtube-data"`. `populateMCPMetadata` then sets `m.MCPBinary = naming.MCP(parsed.Name)` → `"youtube-data-pp-mcp"`. But the canonical api_name (catalog slug) is `"youtube"`, the cmd directory is `cmd/youtube-pp-mcp/`, and the goreleaser binary is `youtube-pp-mcp`. The manifest declares a binary name that doesn't exist in the build artifacts. The published-library `Validate MCPB manifest contract` workflow looks for `cmd/<mcp_binary>/` and fails with *"cmd/youtube-data-pp-mcp directory is missing"*. The npm registry installer would have the same problem at install time: `npx @mvanhorn/printing-press install youtube` resolves the MCP binary name from the manifest and looks for `youtube-data-pp-mcp` in release artifacts that don't exist.
- **Scorer correct?** Yes. The library CI check correctly flags the manifest/cmd-dir mismatch.
- **Root cause:** `internal/pipeline/climanifest.go:407` — `m.MCPBinary = naming.MCP(parsed.Name)`. Should use `m.APIName` (the canonical slug used by every other naming derivation: `cli_name`, cmd directories, the goreleaser binary names, the catalog slug). `parsed.Name` is whatever `cleanSpecName(doc.Info.Title)` produces and only happens to match `m.APIName` when the spec title is a single word.
- **Cross-API check:** I verified Spotify (`info.title: Spotify`), Notion, Linear, and Stripe — all have single-word titles aligning with their slugs, all have correctly named MCP binaries. YouTube is the only catalog entry today that surfaces the bug. Future APIs at risk: any vendor whose `info.title` is multi-word (`"Reddit Public API"`, `"Discord Gateway"`, `"Twitter API v2"`, etc.) where the catalog slug deliberately picks a shorter brand name.
- **Frequency:** subclass — APIs whose spec.info.title diverges from api_name slug. Only 1 concrete catalog example today, but the bug is silent for the agent (the manifest validates locally; only the public-library MCPB workflow catches it post-publish), so it would recur.
- **Fallback if the Printing Press doesn't fix it:** the bug surfaces only at publish-time when the public-library MCPB validator runs. Local `printing-press publish validate` passed all 11 checks for this CLI; the bug was invisible until the PR opened. Claude has no way to know this without explicit guidance.
- **Worth a Printing Press fix?** Yes. The fix is one line and the bug is silent locally — both factors push toward fixing in the machine rather than relying on per-CLI catch.
- **Inherent or fixable:** Fixable. One-line change: `m.MCPBinary = naming.MCP(m.APIName)` instead of `naming.MCP(parsed.Name)` in `populateMCPMetadata`. Any other naming derivation in that function that uses `parsed.Name` instead of `m.APIName` should be audited at the same time.
- **Durable fix:** `internal/pipeline/climanifest.go` — change `populateMCPMetadata` to source from `m.APIName` for binary-name derivation. Add a regression test using a fixture spec with multi-word `info.title` (e.g., `"Test API v3 Beta"`) and assert `m.MCPBinary == naming.MCP(m.APIName)`. While there, audit the other fields populated from `parsed.Name` in this function for the same bug shape.
- **Test:** positive: generate a CLI with spec.info.title `"YouTube Data API v3"` and api_name `youtube`; assert `manifest.json.name == "youtube-pp-mcp"` and `cmd/youtube-pp-mcp/` exists. negative: generate a CLI where spec.info.title equals api_name (`Spotify`); assert no regression on the working case.
- **Evidence:** PR #585 — the library's `Validate MCPB manifest contract` job at run 25929845288 emitted *"cmd/youtube-data-pp-mcp directory is missing"* against a tree that had `cmd/youtube-pp-mcp/` present. Greptile P1 at the same line (manifest.json:14) confirmed the registry generator would propagate the wrong name to install instructions and break `npx install`.
- **Related prior retros:** **#1396 OPEN** *"skill(publish): binary cleanup misses <api-slug>-pp-mcp; root-level executables not stripped from staged dir"* — `extends`. Same MCP-binary-naming surface, different slice (cleanup vs derivation). My fix is upstream of #1396's: if MCPBinary derivation is correct, the cleanup step in #1396 has the right name to look for.

## Prioritized Improvements

### P1 — High priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Publish skill regenerates cli-skills mirror, breaking fork PRs | skill | every fork PR for new CLI | None — skill is the contract; agent follows what it says | small | none needed |

### P2 — Medium priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | HTTP client doesn't propagate `cmd.Context()` | generator | every printed CLI | Low — only an external reviewer (Greptile) caught this; not a verify/dogfood/scorecard signal | medium | none — fix is correct for all APIs |

### P3 — Low priority

| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F3 | MCPBinary derived from spec title not api_name slug | spec-parser | subclass:multi-word-spec-title | None — silent locally, fails only at public-library publish | small | none — fix is unconditionally correct |

### Skip

| Finding | Title | Why it didn't make it (Step B / Step D / Step G) |
|---------|-------|--------------------------------------------------|
| F4 | Generator emits API responses without HTML-unescape at boundary; titles like `Don&#39;t Look Up` leak `&#39;` into JSON consumers | **Step B**: only 1 concrete API with evidence (YouTube, hit 3x). Hypothesized for Reddit/Hacker News but not verified. The polish/output-review step would likely catch this on the next CLI where it actually matters. Revisit if a second concrete API surfaces. |
| F5 | Pre-publish vendor-prefix secret scanner has no allowlist for documented-public client keys (e.g., InnerTube Android key used by yt-dlp) | **Step B**: only 1 concrete case (YouTube InnerTube). Other "documented public" keys I considered (Spotify client IDs, Reddit OAuth client IDs) are OAuth client identifiers, not API keys — different shape, scanner doesn't flag them. The string-concat workaround documented in the patch is cheap; an allowlist pragma is nice-to-have but Step B can't justify it yet. |

### Dropped at triage

| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Trimmed YouTube spec missed `commentThreads.list` endpoint | Catalog/spec trimmer left out a high-value endpoint for a media research CLI | printed-CLI — different APIs trim differently; not a generalizable Printing Press gap |
| `--for-handle` flag missing from generated `channels-list` even though `--for-username` (deprecated) was wired | Spec wiring picked deprecated flag set | API-quirk — the spec's parameter list determines what's wired; YouTube's spec under-declared modern params |

## Work Units

### WU-1: Publish skill drops mirror regen for fork-safe behavior (from F1)
- **Priority:** P1
- **Component:** skill
- **Goal:** `/printing-press-publish` no longer regenerates or commits `cli-skills/pp-<slug>/SKILL.md`, so fork PRs are not pre-rejected by the public library's `Guard against hand-edits to cli-skills mirror` check.
- **Target:** `~/.claude/skills/printing-press-publish/SKILL.md` Step 6 (the *"Regenerate the flat cli-skills mirror"* block — `if [ -f "$PUBLISH_REPO_DIR/tools/generate-skills/main.go" ]; then ... fi`).
- **Acceptance criteria:**
  - positive test: a fresh `/printing-press-publish <api>` run from a fork-only contributor produces a PR whose `cli-skills/` diff is empty; `Verify`'s Guard step passes.
  - negative test: `grep "tools/generate-skills" ~/.claude/skills/printing-press-publish/SKILL.md` returns 0 matches; the SKILL has a comment explaining why (so a future agent doesn't re-introduce the regen).
- **Scope boundary:** This WU does not modify the public library's workflows. The library-side fix (loosen Guard or extend auto-fix to fork PRs) is a separate issue against `mvanhorn/printing-press-library` and is outside the Printing Press's surface area. If that fix lands and fork PRs become required to commit the mirror again, this WU should be reverted with an updated SKILL note.
- **Dependencies:** none.
- **Complexity:** small.

### WU-2: Generator client template threads ctx through HTTP layer (from F2)
- **Priority:** P2
- **Component:** generator
- **Goal:** Generated CLIs honor `cmd.Context()` cancellation and `--timeout` end-to-end on outbound HTTP calls.
- **Target:** the client template emitted to `internal/client/client.go` in every printed CLI (located somewhere under `internal/generator/templates/` — resolve via Glob/Grep on `func.*do.*method`), plus the `resolvePaginatedRead` helper that calls into the client (probably emitted to `internal/cli/helpers.go` or a `pipelineutil` package).
- **Acceptance criteria:**
  - positive test: regenerate a sample CLI; `Client.do(ctx, ...)` and `http.NewRequestWithContext(ctx, ...)` are present; `Get`/`Post`/`GetWithHeaders`/etc. take `ctx context.Context` as the first parameter; `resolvePaginatedRead` threads its `ctx` arg through to the client. Run a sample `<cli> sync` against a slow-ish endpoint, send SIGINT, observe exit within ~100ms.
  - negative test: with `--timeout 1s` set against an endpoint that takes >5s, the call fails with `context deadline exceeded` rather than hanging on the default `HTTPClient.Timeout`.
- **Scope boundary:** This is the client-side ctx threading only. Subscriber side (commands that call the client) are mechanically updated as part of the signature change. Does not include adding ctx to every helper function in the generated CLI — only the path from cmd handler → `resolvePaginatedRead` (or equivalent) → `Client.do`.
- **Dependencies:** none.
- **Complexity:** medium (~12 callers per CLI affected; mechanical signature change; needs golden refresh).

### WU-3: MCPBinary name derives from canonical api_name slug (from F3)
- **Priority:** P3
- **Component:** spec-parser
- **Goal:** `manifest.json.name` and `cmd/<binary>/` always agree, regardless of how the spec's `info.title` is shaped.
- **Target:** `internal/pipeline/climanifest.go:407` — `populateMCPMetadata`'s assignment `m.MCPBinary = naming.MCP(parsed.Name)` should be `naming.MCP(m.APIName)`. While editing, audit the other field assignments in the function for the same `parsed.Name`-vs-`m.APIName` confusion.
- **Acceptance criteria:**
  - positive test: a generator test fixture with `info.title = "Test API v3 Beta"` and api_name `test` produces `manifest.json.name = "test-pp-mcp"` and asserts the cmd directory matches.
  - negative test: existing fixtures where `info.title == api_name` (single-word case like Spotify) produce identical output to before — no regression.
- **Scope boundary:** Just the MCPBinary derivation in `populateMCPMetadata`. If the audit finds other fields with the same bug shape, scope can expand; otherwise stop at the one-line fix + test.
- **Dependencies:** none.
- **Complexity:** small.

## Anti-patterns
- **Skill instructions out of sync with the contract they bridge.** `/printing-press-publish` Step 6's mirror-regen instruction was correct against an older version of the public library's workflow; when the library added the Guard check, the skill wasn't updated. The skill is the only thing an agent reads end-to-end during publish — when its contract diverges from the destination, the agent has no signal until CI fails. Skills that bridge contracts (publish, install, MCP add) should carry an explicit "if this fails, the skill is wrong, not the agent" note.
- **Hand-waving cross-API evidence in retros.** Two of my candidates (HTML-unescape, secret-scanner allowlist) were tempting to file as P3 systemic findings, but Step B couldn't name a second API with evidence. The retro skill's own anti-pattern callout names this exact failure mode ("Pagliacci retro: every finding warrants action"). I dropped them to Skip; if a future CLI hits the same issue, that retro's evidence will combine with this one's and the case will become real.

## What the Printing Press Got Right
- **The `// PATCH:` marker convention + `.printing-press-patches.json` schema** is genuinely useful. When I needed to record 5 distinct hand-authored customizations across 9 files, the existing schema (id/summary/reason/files/validated_outcome) absorbed them cleanly. The `machine_followups` field — for upstream generator gaps that the per-CLI patch only worked around — was a clean extension and the right place to hand off to this retro.
- **Polish skill output-review caught the HTML-entities bug in `videos-related` before publish**, which got me to spot the same bug in `videos-embed` and `search-bulk` proactively after Greptile flagged it. The polish→fix→re-polish loop on a single CLI worked exactly as intended.
- **The mandatory vendor-prefix secret scan** — even though it false-positived on the InnerTube key — is the right default. Failing closed on a documented-public key is the correct trade-off vs. failing open on a real leak. The fix (allowlist pragma) is a refinement, not a rebuke.
- **`printing-press publish validate`'s 11-check rollup** caught everything it was supposed to catch locally; the bugs that escaped to PR CI (MCPBinary name, missing patches.json) were ones the validator legitimately doesn't cover yet. No false negatives in the per-check signal.
