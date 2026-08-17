# Printing Press Retro: Placeit

## Session Stats
- API: placeit (website-itself build of placeit.net)
- Spec source: hand-authored internal YAML (placeit.net cookie-auth endpoints) + hand-built Algolia catalog client; authenticated browser-sniff
- Scorecard: 80/100 (Grade A)
- Verify pass rate: 100%
- Fix loops: shipcheck 2 (verify-skill + live-probe), publish/Greptile 2 rounds
- Manual code edits: substantial (catalog core + 7 novel features all hand-built — expected for a no-spec website CLI)
- Features built from scratch: Algolia client + 6 catalog commands + 7 novel features + hand-built bookmarks

## Findings

### 1. Generated HTTP client strips only a hardcoded `api_key` cookie on cross-host redirects (Bug / Default gap)
- **What happened:** The generated `internal/client/client.go` `CheckRedirect` handler drops only a cookie literally named `api_key` on cross-host redirects, re-adding all others. Greptile flagged it P1 (security) on the publish PR.
- **Scorer correct?** N/A (PR-review finding, not a scorecard penalty).
- **Root cause:** generator client template hardcodes the strip-by-name to `api_key`. For cookie/composed/session_handshake auth modes, the auth material is a different cookie (e.g. `logged_session`), so a by-name strip of `api_key` is mode-specific rather than mode-agnostic.
- **Cross-API check:** Real-world exposure is low — the generated client names *its own* auth cookie `api_key` (so the strip covers it), and browser-imported cookies live in a host-scoped cookie jar that Go never emits cross-host. So the concrete leak Greptile described does not actually occur for the jar path. The improvement is that "strip the entire Cookie header on any cross-host redirect" is strictly safer and mode-agnostic than "strip a hardcoded name."
- **Frequency:** every cookie/composed/session_handshake-auth CLI (20+ in the library: openart, ebay, substack, doordash, amazon-orders, myfitnesspal, …).
- **Fallback if the Printing Press doesn't fix it:** Independent reviewers (Greptile) will keep flagging it P1 on every cookie-auth publish, costing a fix+reply round per PR.
- **Worth a Printing Press fix?** Low priority but yes — a 1-line template change removes a recurring P1 review flag across all cookie-auth CLIs.
- **Inherent or fixable:** Fixable. `req.Header.Del("Cookie")` on cross-host, no re-add.
- **Durable fix:** In the client template, replace the by-name `api_key` strip with an unconditional `Cookie` header delete on cross-host redirects (same-host keeps cookies). Mode-agnostic; benefits api_key, cookie, composed, and session auth identically.
- **Test:** positive — a cookie-auth CLI following a cross-host 302 sends no Cookie header to the new host; negative — a same-host 302 still carries cookies.
- **Evidence:** Greptile P1 on PR #1234 client.go:143; fixed in placeit as patch `placeit-redirect-cookie-strip`.
- **Related prior retros:** None found.
- **Step G (case against):** The generated auth cookie is already named `api_key` and stripped; jar cookies are host-scoped — so no real leak, and a maintainer could close as "works as designed." Case-against is real, which is why this is **P3, not P1** — it survives only as a mode-agnostic hardening that removes a recurring independent-reviewer flag, not as a live vulnerability.

### 2. Publish skill's `git add library/` silently skips a fresh print's gitignored MCP source (Skill instruction gap)
- **What happened:** On a fresh publish, the MCPB manifest-contract CI check failed with `cmd/placeit-pp-mcp directory is missing`. The committed tree had `cmd/placeit-pp-cli/main.go` but not `cmd/placeit-pp-mcp/main.go`.
- **Scorer correct?** N/A (CI contract check, correctly failing — the source genuinely wasn't committed).
- **Root cause:** The public library `.gitignore` has `*-pp-mcp` (intended to ignore the compiled MCP binary), which also matches the `cmd/<slug>-pp-mcp/` *source directory*. The publish skill (Step 8) instructs `git add library/`, which honors `.gitignore` and silently skips the untracked MCP source. Existing CLIs are immune because their MCP source is already tracked; a *fresh* print's source is untracked and gets skipped.
- **Cross-API check:** Recurs on **every future fresh print that ships an MCP server** (the default for most CLIs). 245 of 258 library CLIs ship MCP; all new ones are exposed.
- **Frequency:** every fresh MCP print.
- **Fallback if the Printing Press doesn't fix it:** Each fresh publish fails the MCPB CI check, requiring a manual `git add -f cmd/<slug>-pp-mcp/` + amend + force-push round (exactly what happened here).
- **Worth a Printing Press fix?** Yes — concrete, reproducible publish failure that recurs for every new MCP CLI.
- **Inherent or fixable:** Fixable.
- **Durable fix (primary, in cli-printing-press):** In `skills/printing-press-publish/SKILL.md` Step 6/8, after copying the staged CLI, explicitly `git add -f "$DEST_CLI_DIR/cmd/"*-pp-mcp/ ` (force, since the dir is gitignored) before the commit. Alternative (library-repo side): anchor the ignore rule to root binaries only (`/*-pp-mcp` or `library/*/*/*-pp-mcp` as a file, not the `cmd/` source dir) — but the publish-skill force-add is robust regardless of the library .gitignore.
- **Test:** positive — a fresh MCP print's `cmd/<slug>-pp-mcp/main.go` is staged and committed; the MCPB contract check passes on the first push. negative — root-level compiled `<slug>-pp-mcp` binaries are still excluded.
- **Evidence:** PR #1234 MCPB check `Validate MCPB manifest contract` failed on first push (`cmd/placeit-pp-mcp directory is missing`); fixed by `git add -f` + amend.
- **Related prior retros:** None found (search ran over `$PRESS_MANUSCRIPTS/*/proofs/*-retro-*.md`).
- **Step G (case against):** "The agent should know to force-add." But the publish SKILL explicitly says `git add library/`, and 245 tracked siblings mask the issue — so a careful agent following the documented step still fails. The case-against is weak: this is a documented-step gap, not agent error.

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 2 | Publish force-add gitignored MCP source | skill | every fresh MCP print | Low (silent skip; CI fails) | small | none |

### P3 — Low priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | Mode-agnostic cross-host cookie strip | generator | every cookie/composed/session-auth CLI | Medium (reviewers re-flag) | small | same-host keeps cookies |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| 3 | scorecard `auth_protocol` 2/10 for cookie-auth CLIs | Step 2f: cannot trace scorer source externally (not IN_REPO). Polish flagged it as a scorer-pattern mismatch (rubric appears calibrated for API-key auth; cookie/composed/session_handshake are legitimate modes with no API-key option). Worth an in-repo retro to confirm whether `auth_protocol` should recognize these modes rather than penalizing to 2/10 — but unverifiable from outside the repo, so not filed. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| Hand-written traffic-analysis.json missing `version` caused `generate` to error | I authored a documentation-shaped traffic-analysis by hand; the generator expects its own schema | iteration-noise (my artifact was wrong; dropped the flag and regenerated) |
| Generated promoted command used a UUID example for an int-typed required param + hard-failed without a session | placeit bookmarks needs the user's own int id + a cookie session | printed-CLI (resolved by normal hand-building of an auth-aware command; the exact shape — auth-gated required-param user-scoped GET — is narrow) |

## Work Units

### WU-1: Publish skill force-adds fresh-print MCP source (from F2)
- **Priority:** P2
- **Component:** skill
- **Goal:** A fresh MCP print's `cmd/<slug>-pp-mcp/` source is committed so the public library's MCPB manifest-contract CI check passes on first push.
- **Target:** `skills/printing-press-publish/SKILL.md` Step 6 (after staged-CLI copy) / Step 8 (before commit).
- **Acceptance criteria:**
  - positive: after publish package + copy, `git add -f "$DEST_CLI_DIR"/cmd/*-pp-mcp/` runs so `cmd/<slug>-pp-mcp/main.go` is staged; the MCPB contract check passes without a manual amend.
  - negative: root-level compiled `<slug>-pp-mcp` / `<slug>-pp-cli` binaries remain excluded from the commit.
- **Scope boundary:** Does not change the library repo's `.gitignore` (that's an alternative fix in a different repo); the publish-skill force-add is robust regardless.
- **Dependencies:** none.
- **Complexity:** small.

### WU-2: Mode-agnostic cross-host cookie strip in client template (from F1)
- **Priority:** P3
- **Component:** generator
- **Goal:** The generated HTTP client drops the entire Cookie header on any cross-host redirect, independent of auth mode/cookie name.
- **Target:** generator client template (`internal/generator/`), the `CheckRedirect` block in `internal/client/client.go`.
- **Acceptance criteria:**
  - positive: a cookie/composed/session-auth CLI following a cross-host redirect sends no Cookie header to the new host.
  - negative: a same-host redirect still carries cookies; api_key-header auth behavior unchanged.
- **Scope boundary:** Cross-host redirects only; same-host path untouched.
- **Dependencies:** none.
- **Complexity:** small.

## Anti-patterns
- Over-trusting an independent reviewer's severity label: Greptile rated the cookie strip P1, but tracing the actual auth wiring (auth cookie named `api_key`; jar host-scoping) showed minimal real exposure. The retro correctly downgrades it to a P3 hardening rather than filing a P1 "vulnerability."

## What the Printing Press Got Right
- The cookie/composed-auth + browser-chrome (Surf) transport cleared Cloudflare on placeit.net first try — `campaigns`/`account` returned live data with no clearance-cookie dance.
- The generator pre-scaffolded all 7 novel-command files from `research.json`, so Phase 3 was filling RunE bodies on a wired Cobra tree, not creating files.
- `dogfood --live` + `--write-acceptance`, `verify-skill`, and `validate-narrative --full-examples` caught every real issue (param-shape, prose-embedded commands, auth-gated happy-path) deterministically before publish.
- The patches index (`.printing-press-patches/`) gave a clean, durable home for the hand-authored catalog/novel/bookmarks layers and the cross-host cookie fix.
