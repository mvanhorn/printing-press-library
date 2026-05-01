---
title: "refactor: Reframe ebay CLI as discovery tool, gate broken bid flows"
type: refactor
status: active
date: 2026-04-30
---

# refactor: Reframe ebay CLI as discovery tool, gate broken bid flows

**Target repo:** `mvanhorn/printing-press-library`
**Target CLI path:** `library/commerce/ebay/`

## Overview

The ebay CLI currently advertises a "search, comp, watch, and snipe" capability set in its tagline, README, SKILL.md, and MCP manifests. Live dogfooding on 2026-04-30 confirmed two scope-breaking gaps:

1. **`bid` and `snipe` cannot complete** end-to-end. eBay's `/bfl/placebid/<id>` endpoint redirects browser-cookie sessions to sign-in (step-up auth), even with cookies that pass the `/deals` validation handshake. The three-step bid flow (`bid module` -> `bid trisk` -> `bid confirm`) cannot extract the `srt` token because eBay never serves the bid module HTML to non-browser sessions. This is an architectural mismatch between the CLI's HTTP-with-Chrome-cookies approach and eBay's anti-bot tier on bid traffic, not a fixable parser bug.
2. **`listings` returns null on every query.** The generated handler treats eBay's `/sch/i.html` HTML response as a JSON envelope and fails marshalling. A local prototype patch swaps the broken `resolveRead` path for the existing `srcebay.FetchActive` HTML scraper that `auctions` already uses.

Discovery surfaces (`auctions`, `sold`, `comp`, `watch`, `feed`, `saved-search`) work correctly and are genuine value over eBay's web UI - especially `auctions` with bid filtering, which the post-Finding-API web UI cannot replicate.

This plan reframes the CLI as a discovery and intelligence tool. It hides the broken bid flows behind an `--experimental` flag (preserving the code for a future browser-CDP rewrite), lands the listings parser fix, rewrites user-facing copy to set accurate expectations, and ships an evidence GIF showing the discovery flow that does work.

## Problem Frame

A user (Matt, the CLI author) tried to bid on two PSA-graded vintage Mariners cards via the CLI's `snipe` command. Auth flowed cleanly (`pycookiecheat` -> `auth login --chrome` -> 51 cookies, `/deals` validated). The bid attempt failed at the first HTTP hop: eBay returned a sign-in redirect for `/bfl/placebid/<id>`. Subsequent debugging confirmed this is not a stale-cookie or fingerprint issue - eBay step-ups bid traffic regardless of session age when traffic doesn't look like a real browser. The CLI's `surf`-impersonated requests aren't browser enough for the bid endpoint, even though they're enough for the deal/search endpoints.

In parallel, the user found `listings` returns `{"results": null}` for every query, regardless of keyword. Root cause: the generated handler calls `resolveRead` (a JSON API helper) against `/sch/i.html`, which is HTML. Marshal fails silently, output is null.

The current copy makes promises the CLI can't keep. A future user (or future-Matt) hitting the same wall would reasonably file a bug or abandon the tool. The fix is twofold: (a) be honest about what works, (b) land the listings fix so discovery is fully functional.

## Requirements Trace

- R1. The CLI's tagline, `--help` output, README, SKILL.md, MCP `tools-manifest.json`, and `agent-context` output must accurately describe current capabilities.
- R2. `bid` and `snipe` commands must not appear in default `--help` output. They must remain runnable for users who explicitly opt in (preserving the code path for future revival).
- R3. When a user runs an experimental command, the CLI must print a one-line warning explaining the limitation before attempting the operation.
- R4. `listings` must return parsed item data for valid queries, matching the shape `auctions` produces.
- R5. The README must include a "Known Limitations" section documenting the bid auth wall and rate-limit fragility.
- R6. The README must include visual evidence (GIF or screenshot) of the discovery flow.
- R7. Changes ship as a PR to `mvanhorn/printing-press-library`, not direct push to master.

---

## Scope Boundaries

- Not fixing bid placement. Routing bid traffic through a real browser (via CDP / `browser-use` / `agent-browser`) is a separate, larger project tracked as future work.
- Not adding new discovery features. This is scope correction and a single bug fix, not capability expansion.
- Not regenerating the CLI from scratch via `printing-press`. The fixes are surgical edits to the published source.
- Not changing the auth flow. `auth login --chrome` and `auth refresh` continue to work as-is.

### Deferred to Follow-Up Work

- Browser-CDP-based bid placement (separate plan, larger scope)
- Rate-limit hardening (separate plan; current behavior is "eBay 403s after sustained scraping" - no easy fix without IP rotation or a proper Browse API)

---

## Context & Research

### Relevant Code and Patterns

- `library/commerce/ebay/internal/cli/root.go` - root command registration; this is where commands get added to the cobra tree
- `library/commerce/ebay/internal/cli/bid.go`, `bid_module.go`, `bid_trisk.go`, `bid_confirm.go` - the broken bid command tree
- `library/commerce/ebay/internal/cli/snipe.go` - snipe command; depends on bid placer
- `library/commerce/ebay/internal/cli/auctions.go` - working pattern: calls `srcebay.FetchActive` directly, passes through HTML scraper
- `library/commerce/ebay/internal/cli/promoted_listings.go` - broken handler; calls `resolveRead` on HTML endpoint. Local prototype swapping to `FetchActive` proven to compile clean.
- `library/commerce/ebay/internal/source/ebay/scrape.go` - `FetchActive` accepts both `Auction` and `BIN` flags; reuse for listings.
- `library/commerce/ebay/internal/source/ebay/bid.go` - `Place` does the 3-step flow; `Plan` calls `FetchItem`. Both fail at step 1 because eBay redirects `/bfl/placebid/*` to sign-in for non-browser sessions.
- `library/commerce/ebay/internal/cli/agent_context.go` - emits structured JSON describing CLI capabilities; agents read this to decide what the CLI is for.
- `library/commerce/ebay/mcp-descriptions.json`, `tools-manifest.json` - MCP server tool descriptions; consumed by Claude Desktop and other MCP clients.
- Cobra `Hidden: true` field on `cobra.Command` removes a command from default help while keeping it runnable - the standard pattern for experimental gating in Go CLIs.

### Institutional Learnings

- Memory `feedback_pr_then_merge.md`: every change goes through a PR with a description, then gets merged. Never push directly to master.
- Memory `feedback_no_process_in_pr_body.md`: PR bodies don't document AI tool usage or review rounds. One-sentence disclosure max.
- Memory `feedback_show_gifs_inline.md`: when including evidence, Read the .gif file inline rather than linking paths.
- Memory `feedback_evidence_every_pr.md`: never skip evidence on a PR.
- Memory `reference_pp_cli_binaries.md`: PP CLIs live at `~/printing-press/library/<name>/<name>` locally; published path is `library/<category>/<name>/`.
- The 2026-04-10 plan `2026-04-10-001-fix-cli-quality-bugs-plan.md` is precedent for surgical CLI quality fixes via PR.

### External References

- eBay step-up auth on placebid: confirmed empirically via `curl -L https://www.ebay.com/bfl/placebid/<id>` with valid `/deals`-validated cookies returns a sign-in redirect HTML page, not the bid module.
- Cobra `Hidden` field: https://pkg.go.dev/github.com/spf13/cobra#Command - standard Go CLI experimental-command pattern.

---

## Key Technical Decisions

- **Hide-via-`Hidden`-flag, not delete.** Keeping bid/snipe source on disk makes a future browser-CDP rewrite incremental rather than green-field. Cobra's `Hidden: true` removes the command from default help while leaving it runnable for users who know it exists.
- **No `--experimental` global flag.** Commands stay reachable by name (`ebay-pp-cli bid` still works, just not listed in `--help`). A global gate adds parsing complexity for a single use case. The warning-on-run pattern (decision below) carries the user-education load.
- **Print a one-line limitation warning when experimental commands run.** Prevents silent confusion. Format: `Warning: bid is experimental and currently fails at eBay's step-up auth wall. See README#known-limitations.`
- **Listings fix uses `FetchActive` directly, not a generated wrapper.** Matches the `auctions` pattern. The `resolveRead` path is structurally wrong for HTML endpoints and shouldn't be reused.
- **Tagline format follows the discovery framing:** "Discover, monitor, and analyze eBay listings, auctions, and sold comps from the terminal." Drops "snipe" and "bid" entirely.
- **Plan targets the published repo (`printing-press-library`), not the local working tree.** Local edits already exist (the listings fix is prototyped); this plan codifies them as a PR. The local tree can be re-synced from the published repo after merge.

---

## Open Questions

### Resolved During Planning

- Should we delete bid/snipe source entirely? Resolved: no. Hide via `Hidden: true`, keep code for future revival. (User confirmed.)
- Should the README rewrite be minimal or full? Resolved: full pass with Known Limitations section, accurate examples, evidence GIF. (User confirmed.)
- Should the listings patch live in this PR or split? Resolved: same PR. The fix is small, related, and shipping the honesty pass without it would mean shipping a CLI where `listings --help` is documented but the command still returns null.

### Deferred to Implementation

- Exact wording of the warning line printed when experimental commands run. Choose during implementation; the constraint is "one line, points to README, no panic".
- Whether to record the demo GIF via `pp-agent-capture` skill or a manual screen recording. Choose during implementation based on what produces a clean, sub-2MB file.
- Whether to register `bid` and `snipe` under a parent `experimental` command or leave them at the root with `Hidden: true`. Both work; pick whichever produces less diff churn during implementation.

---

## Implementation Units

- [ ] U1. **Fix listings parser to use HTML scraper**

**Goal:** Replace the broken `resolveRead`-on-HTML path in the `listings` command with a direct call to `srcebay.FetchActive`, the same scraper `auctions` uses.

**Requirements:** R4

**Dependencies:** None

**Files:**
- Modify: `library/commerce/ebay/internal/cli/promoted_listings.go`
- Test: `library/commerce/ebay/internal/cli/promoted_listings_test.go` (create if absent)

**Approach:**
- Drop the `resolveRead(c, flags, "listings", false, "/sch/i.html", params, nil)` call and the `extractResponseData`/JSON-marshal envelope below it.
- Construct a `srcebay.SearchOptions` struct from the flag values (`flagNkw`, `flagLHAuction == "1"`, `flagLHBIN == "1"`, `flagSacat`, parsed `flagUdlo`/`flagUdhi` floats, parsed `flagIpg` int).
- Call `srcebay.New(c).FetchActive(context.Background(), opts)` and marshal the returned `[]Listing` to JSON.
- Build a `DataProvenance{Source: "live", ResourceType: "listings"}` literal for the existing provenance-printing path.
- Preserve the existing `--csv`, `--json`, `--select`, `--compact`, and human-table output branches.

**Patterns to follow:**
- `library/commerce/ebay/internal/cli/auctions.go` calls `srcebay.New(c).FetchActive(...)` directly - mirror its shape.
- `library/commerce/ebay/internal/cli/helpers.go` defines `DataProvenance` - use the existing struct, do not invent a new one.

**Test scenarios:**
- Happy path: `listings --nkw "PSA Mariners" --udlo 10 --udhi 30 --lh-bin 1` returns at least one parsed item with non-empty `title`, `price`, and `item_id` fields when eBay is reachable.
- Happy path: `listings --nkw "Griffey" --lh-auction 1` returns auction items (filter passes through to `LH_Auction=1` query param).
- Edge case: empty result set returns an empty array `[]`, not `null`. The current bug shows up as `null`.
- Edge case: missing `--nkw` returns the existing required-flag error, unchanged.
- Error path: when the underlying scraper returns a `RateLimitError`, the command surfaces it through `classifyAPIError` rather than panicking or emitting null.
- Integration: `--csv` output includes header row and one row per item; `--json` output wraps results in the provenance envelope just like before the fix.

**Verification:**
- Running the listings command with a known-good query produces a non-empty results array.
- `go test ./internal/cli/...` passes.
- Manual smoke: `ebay-pp-cli listings --nkw "PSA Mariners" --udlo 10 --udhi 30 --lh-bin 1 --plain` shows real items in the terminal.

---

- [ ] U2. **Hide `bid` and `snipe` from default help; add limitation warning**

**Goal:** Mark the broken bid flows as experimental so they don't appear in `--help` and so users who reach them by name see a clear warning before the command attempts its (currently failing) work.

**Requirements:** R2, R3

**Dependencies:** None

**Files:**
- Modify: `library/commerce/ebay/internal/cli/root.go`
- Modify: `library/commerce/ebay/internal/cli/bid.go`
- Modify: `library/commerce/ebay/internal/cli/snipe.go`
- Modify: `library/commerce/ebay/internal/cli/bid_module.go`
- Modify: `library/commerce/ebay/internal/cli/bid_trisk.go` (and any other `bid_*.go` subcommands)
- Test: `library/commerce/ebay/internal/cli/bid_visibility_test.go` (create)
- Test: `library/commerce/ebay/internal/cli/snipe_test.go` (create or extend)

**Approach:**
- Set `Hidden: true` on the `bid` and `snipe` `cobra.Command` definitions so they're suppressed from the parent `--help` listing.
- For each command's `RunE`, prepend a single-line stderr warning before any other work: `Warning: <name> is experimental. eBay's anti-bot gate on /bfl/placebid blocks this command end-to-end. See README#known-limitations.` The exact wording can be tuned during implementation; the contract is one line, no panic, mentions README.
- Update each command's `Short` and `Long` fields to include `[experimental]` prefix so anyone reading the source or running `ebay-pp-cli bid --help` directly sees the status.

**Patterns to follow:**
- Cobra `Hidden` field: search for any existing `Hidden: true` use in the repo first; if none, this is a new pattern - keep it idiomatic (`Hidden: true,` on the command literal).
- The warning print should use `cmd.ErrOrStderr()` to match how other commands emit out-of-band messages.

**Test scenarios:**
- Happy path: `ebay-pp-cli --help` does not list `bid` or `snipe` in the Available Commands section.
- Happy path: `ebay-pp-cli bid --help` still works and prints the bid subcommand tree (Hidden hides from parent listing, not from direct help).
- Happy path: `ebay-pp-cli snipe <id> --max 1.00 --simulate` still runs and prints the simulate output, but emits the limitation warning to stderr first.
- Edge case: shell completion script (`ebay-pp-cli completion zsh`) still emits completions for hidden commands - this is cobra's standard behavior; do not break it.
- Integration: warning appears on stderr (not stdout), so `--json` consumers still get clean JSON on stdout.

**Verification:**
- `ebay-pp-cli --help | grep -E "^  (bid|snipe)"` returns nothing.
- `ebay-pp-cli snipe 1 --max 1 --simulate 2>&1 >/dev/null` prints the warning.
- `ebay-pp-cli snipe 1 --max 1 --simulate --json` produces valid JSON on stdout.

---

- [ ] U3. **Update CLI metadata: tagline, agent-context, MCP manifests**

**Goal:** Anywhere the CLI declares its own capabilities to a downstream consumer (humans reading `--help`, agents reading `agent-context`, MCP clients reading `tools-manifest.json` and `mcp-descriptions.json`), the description must match what the CLI actually does.

**Requirements:** R1

**Dependencies:** U2 (commands need to be hidden first so the metadata can describe them as experimental)

**Files:**
- Modify: `library/commerce/ebay/internal/cli/root.go` (root command `Long` field / tagline)
- Modify: `library/commerce/ebay/internal/cli/agent_context.go`
- Modify: `library/commerce/ebay/mcp-descriptions.json`
- Modify: `library/commerce/ebay/tools-manifest.json`
- Modify: `library/commerce/ebay/manifest.json` (`description` field)
- Test: `library/commerce/ebay/internal/cli/agent_context_test.go` (create or extend)

**Approach:**
- New tagline: "Discover, monitor, and analyze eBay listings, auctions, and sold comps from the terminal." Replaces the current "Search, comp, watch, and snipe..." phrasing.
- `agent_context.go` emits structured JSON; ensure the capabilities array no longer claims bid placement as a stable capability. If the file lists each command, mark `bid` and `snipe` with an `"experimental": true` flag and a `"limitation"` field pointing to README.
- `mcp-descriptions.json` and `tools-manifest.json` should drop bid-related tool descriptions, or mark them experimental. Goal: an MCP client reading these files should not advertise placebid as a working tool.
- `manifest.json` `description` field updated to discovery framing.

**Patterns to follow:**
- The existing JSON structure of `mcp-descriptions.json` and `tools-manifest.json` - add new fields rather than restructure.
- Other CLIs in the library (e.g. `library/commerce/instacart/`) for reference on how their agent-context and manifests describe capabilities.

**Test scenarios:**
- Happy path: `ebay-pp-cli --help` shows the new tagline.
- Happy path: `ebay-pp-cli agent-context` JSON output lists discovery commands as primary capabilities and marks bid/snipe with the experimental flag.
- Happy path: `tools-manifest.json` and `mcp-descriptions.json` parse as valid JSON after edit (smoke test via `jq empty < file.json`).
- Integration: an MCP client reading `tools-manifest.json` does not surface `placebid` as a tool, or surfaces it with an `experimental` annotation.

**Verification:**
- All four files lint clean (no JSON syntax errors).
- agent-context output matches the new framing.
- `--help` reflects the new tagline.

---

- [ ] U4. **Rewrite README with Known Limitations section and accurate examples**

**Goal:** The README is the first thing future users see. It needs to describe what the CLI actually does, surface the bid limitation upfront, and replace any examples that demonstrate broken functionality.

**Requirements:** R1, R5, R6

**Dependencies:** U1 (so listings examples in README actually work), U2 (so README can reference experimental flag accurately), U3 (so README and metadata agree)

**Files:**
- Modify: `library/commerce/ebay/README.md`
- Create: `library/commerce/ebay/docs/discovery-demo.gif` (or similar path, see U6)

**Approach:**
- Rewrite the top-of-file tagline and intro paragraph to match the new framing.
- Restructure the README sections in this order:
  1. Tagline + 2-3 sentence intro
  2. Quick install
  3. Quickstart - show `auctions`, `sold`, `comp`, `watch` with real examples that produce real output
  4. Embedded GIF (from U6)
  5. Full command reference - keep alphabetical, omit `bid`/`snipe`
  6. Known Limitations - new section
  7. Auth setup
  8. Contributing / license
- The Known Limitations section must call out, in plain language: (a) bid placement does not work end-to-end because eBay step-ups auth on bid traffic; the commands exist but are hidden and experimental, (b) aggressive use can trigger eBay rate limits / 403s, with `auth refresh` as the recovery, (c) `listings` was rewritten to use the HTML scraper after the initial release; if you cloned an early version and listings returned null, that's why.
- Remove any quickstart example that references `bid` or `snipe`.

**Patterns to follow:**
- `library/commerce/ebay/SKILL.md` already has prose patterns and example formatting that can be mirrored.
- Other CLIs in `library/commerce/` for README structure conventions.

**Test scenarios:**
- Test expectation: none -- README is documentation, validated by manual review and the doc-review pass below.
- Manual review: every code example in the new README runs cleanly against a fresh CLI build with valid auth.
- Manual review: Known Limitations section is written so a first-time user understands the bid gap before they try to use it.
- Manual review: no broken internal links (e.g. `#known-limitations` anchor resolves).

**Verification:**
- A new user reading the README in 60 seconds can answer: "what does this do" and "what doesn't it do".
- All README examples produce non-error output when run.
- Embedded GIF renders inline on GitHub.

---

- [ ] U5. **Update SKILL.md trigger phrases and capabilities**

**Goal:** SKILL.md is what causes Claude (in Claude Code, Desktop, etc.) to invoke the CLI. Its trigger phrases and capability description should match what the CLI actually does, so the skill fires for discovery requests and not for "place a bid" requests.

**Requirements:** R1

**Dependencies:** U3 (so SKILL.md and other metadata describe the same scope)

**Files:**
- Modify: `library/commerce/ebay/SKILL.md`

**Approach:**
- Drop trigger phrases like "snipe an ebay auction" or "bid on this listing".
- Add or strengthen trigger phrases for discovery: "find ebay auctions ending soon", "what's the sold-comp price on this card", "watch this ebay listing", "find listings under $X for [keyword]".
- Update the capability description block to match the new tagline framing.
- Keep auth setup, install, and command examples in lockstep with README.

**Patterns to follow:**
- Other `SKILL.md` files in the printing-press library (e.g. `library/commerce/instacart/SKILL.md`) for trigger-phrase conventions.

**Test scenarios:**
- Test expectation: none -- SKILL.md is documentation and configuration; correctness is validated by manual review and skill firing in a real Claude session.
- Manual review: trigger phrases cover the discovery use cases without claiming bid capability.
- Manual review: capability description matches the new tagline.

**Verification:**
- Skill fires when Claude is asked "find me ending-soon ebay auctions for X".
- Skill does not fire when Claude is asked "bid on this listing for me".

---

- [ ] U6. **Capture and embed discovery-flow demo GIF**

**Goal:** README evidence per Matt's "show GIFs inline" memory and "evidence on every PR" memory. Visual proof that the discovery flow works.

**Requirements:** R6

**Dependencies:** U1 (so listings demo works), U2 (so the demo doesn't accidentally show experimental commands), U4 (so the GIF can be referenced from the README)

**Files:**
- Create: `library/commerce/ebay/docs/discovery-demo.gif` (or `media/discovery-demo.gif`, exact path TBD during implementation)

**Approach:**
- Record a short (15-30 second) terminal session running 2-3 representative discovery commands. Suggested script:
  1. `ebay-pp-cli auctions "PSA Mariners" --has-bids --ending-within 24h --max-price 30` -> shows the killer feature working
  2. `ebay-pp-cli comp "Ken Griffey Jr 1989 PSA 9" --days 90` -> shows sold-comp pricing
  3. `ebay-pp-cli listings --nkw "PSA Mariners 1980s" --lh-bin 1 --udhi 30` -> shows the just-fixed listings command
- Use the `pp-agent-capture` skill or a manual recorder. Target file size: under 2MB. Resolution: legible at GitHub's default README rendering width.
- Embed in README at the position chosen in U4.

**Patterns to follow:**
- Memory `feedback_show_gifs_inline.md`: Read the .gif file inline, do not link via HTML.
- Memory `feedback_evidence_every_pr.md`: every PR has evidence, no skipping.
- Other CLIs in the library that ship with demo GIFs - mirror their dimensions and embed style.

**Test scenarios:**
- Test expectation: none -- this is media capture, validated by manual review of the produced file.
- Manual review: GIF plays cleanly on GitHub README.
- Manual review: every command shown in the GIF produces real, non-error output.
- Manual review: file size is under 2MB so it doesn't bloat the repo.

**Verification:**
- GIF renders inline in the README on the PR preview.
- All commands shown in the GIF are stable enough that re-running them today reproduces qualitatively similar output.

---

- [ ] U7. **Open PR to printing-press-library**

**Goal:** Land everything above as a single, reviewable PR per Matt's "always PR before main" memory.

**Requirements:** R7

**Dependencies:** U1, U2, U3, U4, U5, U6 (all changes need to be in the working tree before opening the PR)

**Files:**
- No new files. This unit is the PR-creation wrapper.

**Approach:**
- Branch off `master` in `~/printing-press/.publish-repo` with a name like `fix/ebay-honest-capabilities`.
- Stage all U1-U6 changes (which target `library/commerce/ebay/...`) plus this plan file.
- Commit with a message that summarizes the scope correction in 1-2 sentences. Per memory `feedback_no_process_in_pr_body.md`, no AI-tool disclosure unless required by repo convention.
- Open PR via `gh pr create` with a body that has Summary (1-3 bullets) and Test plan (manual smoke checklist). Embed the U6 GIF inline.
- Per memory `feedback_always_track_submitted_prs.md`: record the PR URL in `~/.osc/projects.json` `submitted_prs`.

**Patterns to follow:**
- Memory `feedback_pr_then_merge.md`: PR with description, then merge.
- Memory `feedback_no_process_in_pr_body.md`: no process narrative, no review-rounds documentation.
- Memory `feedback_evidence_every_pr.md`: GIF in the PR body.
- Memory `feedback_always_track_submitted_prs.md`: log to `submitted_prs` after creation.
- Existing PR descriptions in `mvanhorn/printing-press-library` for format conventions.

**Test scenarios:**
- Test expectation: none -- this is workflow.
- Verification: PR opens, CI passes, GIF renders, body is one-Summary-and-one-Test-plan format.

**Verification:**
- PR exists on `mvanhorn/printing-press-library`.
- CI is green.
- PR body has Summary, Test plan, and inline GIF.
- PR URL is logged to `submitted_prs`.

---

## System-Wide Impact

- **Interaction graph:** The MCP server (`bin/ebay-pp-mcp`) reads `mcp-descriptions.json` and `tools-manifest.json` at startup. Updating these in U3 changes which tools the MCP server advertises to Claude Desktop and other clients. After the PR merges, MCP users will see the new tool list on their next reconnect.
- **Error propagation:** The new warning print in U2 goes to stderr; downstream pipelines that consume stdout JSON should be unaffected. Verify the `--json --agent` paths still produce stdout-only JSON.
- **API surface parity:** The `agent-context` JSON structure is consumed by agents that scope their tool use based on the `experimental` flag. Adding the field is additive; clients that don't know about it will ignore it. No breaking change.
- **Integration coverage:** The listings fix (U1) silently changes the on-the-wire request shape for `listings` (different scraper path, same eBay endpoint). Verify `--csv` and `--json` outputs still match the structure consumers expect.
- **Unchanged invariants:** Auth flow (`auth login --chrome`, `auth refresh`, `auth status`), discovery commands (`auctions`, `sold`, `comp`, `watch`, `feed`, `saved-search`), and MCP server transport remain unchanged. The CLI binary names (`ebay-pp-cli`, `ebay-pp-mcp`) and module path are unchanged.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Hiding bid/snipe via `Hidden: true` is reversed by a future printing-press regeneration that doesn't preserve the flag. | Add a comment above each `Hidden: true` line explaining why it's set, so a regeneration diff surfaces it for review. Track the regeneration risk in the printing-press generator separately. |
| Listings fix (U1) introduces a regression by replacing a generated handler with a hand-edited one. | Test scenarios in U1 cover the happy path, empty result, and error paths. Run `go test ./...` and a manual smoke before merging. |
| Demo GIF (U6) goes stale as eBay UI / inventory changes. | Pick query terms (PSA Mariners, Griffey) that have continuous listing volume. Re-record if the GIF demo breaks during PR review. |
| The bid commands still exist on disk; a curious user runs them, hits the warning, and files a "this is broken" issue anyway. | The warning text points to README#known-limitations, and the README explicitly explains the architectural mismatch. Repeated reports are signal to invest in the browser-CDP rewrite, not a bug in this PR. |
| eBay's anti-bot escalates further during PR review and breaks the discovery commands too. | If this happens, document it in the PR thread; the honesty-pass framing is robust to it (the README's Known Limitations already mentions rate-limit fragility). |

---

## Documentation / Operational Notes

- After merge, the user should re-sync their local working tree at `~/printing-press/library/ebay/` from the published source so further development starts from the corrected state.
- No deploy step. The CLI is installed via `go install` from the published repo, so users get the changes by re-running their install command.
- No monitoring impact. There are no metrics emitted by this CLI.

---

## Sources & References

- Live dogfooding session 2026-04-30 (this Claude Code conversation) confirmed:
  - `bid` and `snipe` fail with `could not extract srt token from bid module` because eBay redirects `/bfl/placebid/<id>` to sign-in for cookie-only sessions.
  - `listings` returns `{"results": null}` for every query due to JSON-on-HTML marshal failure.
  - `auctions` returns clean parsed results (13 PSA Mariners auctions found in initial query).
- Local prototype patches in `~/printing-press/library/ebay/internal/cli/promoted_listings.go` and `snipe.go` (the snipe `--now` patch is functionally a workaround for the FetchItem path; not carrying it forward in this plan since U2 hides snipe entirely).
- Cobra docs: https://pkg.go.dev/github.com/spf13/cobra
- eBay step-up auth on bid endpoints: empirically confirmed via `curl https://www.ebay.com/bfl/placebid/<id>` returning a sign-in redirect with valid `/deals`-validated cookies.
- Memory file references throughout (no inline duplication).
