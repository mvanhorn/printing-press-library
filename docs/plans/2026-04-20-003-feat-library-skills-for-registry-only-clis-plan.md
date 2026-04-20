---
title: "feat: Ship library SKILL.md for the 8 registry-only CLIs (slack, linear, dominos, trigger-dev, steam-web, postman-explore, pagliacci-pizza, agent-capture)"
type: feat
status: active
date: 2026-04-20
---

# feat: Ship library SKILL.md for the 8 registry-only CLIs

**Target repo:** printing-press-library

## Overview

Eight CLIs in the library currently lack a `SKILL.md` file and ship only as registry-only plugin mirrors auto-generated from `registry.json`. The generated mirrors list raw command names but have no "when to use" guidance, no trigger-phrase coverage for natural-language matching, no CLI-specific auth steps, and no curated command table. As a result these skills:

1. Don't fire reliably from natural language (description is too generic).
2. Won't gain a `/pp-<slug>` slash command from the sibling plan (2026-04-20-002) because that plan skips registry-only entries.
3. Feel like dead ends compared to real skills like `pp-contact-goat` or `pp-kalshi`.

The 8 affected CLIs: `dominos-pp-cli`, `agent-capture` (dev-tools), `postman-explore`, `trigger-dev`, `pagliacci-pizza`, `steam-web`, `slack`, `linear`. This plan ships a production-quality `SKILL.md` for each, modeled on the `pp-contact-goat` template, so the plugin mirror regeneration picks them up, slash commands become available, and natural-language triggers actually match.

## Problem Frame

Agents reading a registry-only SKILL.md can invoke the CLI but get no guidance on:
- When to prefer this CLI vs. a sibling (e.g. dominos vs. pagliacci for pizza ordering).
- How to authenticate (SLACK_BOT_TOKEN, LINEAR_API_KEY, STEAM_WEB_API_KEY, etc.) and where to get a key.
- What natural-language phrases should trigger the skill (the auto-generated "Trigger phrases" list is thin: "install slack, use slack, run slack").
- Which of the 20+ listed commands are high-value for a fresh user vs. advanced flags.
- Exit-code semantics for error handling.

Users hitting `/pp` in Claude Code (see screenshot 2026-04-20) also don't see these skills because, as the companion slash-commands plan documents, command generation skips entries that have no library SKILL.md. So writing the SKILL.md files is the unblocker for the slash-command surface too.

## Requirements Trace

- R1. Each of the 8 CLIs MUST have a `library/<category>/<slug>/SKILL.md` file that mirrors the `pp-contact-goat` shape: YAML frontmatter, intro, "When to Use", argument parsing, auth setup, direct-use workflow, notable commands table, exit codes.
- R2. Each SKILL.md's `description` frontmatter MUST include domain-specific trigger phrases so the skill fires on natural-language queries (e.g. "who's on my team", "show my open Linear issues", "order a pizza").
- R3. Each SKILL.md's auth section MUST list the required env var(s) with the correct name (e.g. `SLACK_BOT_TOKEN`, `LINEAR_API_KEY`) derived from the CLI's actual source or `--help` output, not guessed.
- R4. Each SKILL.md's "Notable Commands" table MUST be curated (5-10 high-signal commands), not the full raw command dump.
- R5. After regeneration (`go run ./tools/generate-skills/main.go`), each `plugin/skills/pp-<slug>/SKILL.md` MUST match its library source and `.github/scripts/verify-skill/verify_skill.py` MUST pass for each CLI.
- R6. Plugin version MUST bump (current 1.1.21 or the value after the sibling slash-commands plan lands). AGENTS.md's "Keeping plugin/skills in sync" rule applies.
- R7. The sibling plan (`docs/plans/2026-04-20-002-feat-pp-slash-commands-plan.md`) MUST be able to generate `/pp-dominos`, `/pp-linear`, etc. after these SKILL.md files land.
- R8. No CLI source code changes. No behavioral changes. This plan is documentation-only.

## Scope Boundaries

- Not fixing bugs discovered during CLI exploration. If `--help` reveals a broken command or misleading help text, note it and file a separate issue / follow-up plan.
- Not regenerating the CLIs via `printing-press`. Hand-writing SKILL.md only.
- Not adding MCP server coverage for CLIs that don't ship one (e.g. agent-capture, pagliacci-pizza). Only document MCP installation when `cmd/<slug>-pp-mcp` exists on disk.
- Not rewriting `pp-contact-goat` or other existing skills. Use them as the reference template only.
- Not updating README.md or AGENTS.md's "SKILL.md coverage is not universal" language until the regeneration lands and all 8 skills verify green.

### Deferred to Separate Tasks

- Sibling slash-commands plan: `docs/plans/2026-04-20-002-feat-pp-slash-commands-plan.md` (adds `/pp-<slug>` commands; benefits from this plan landing first).
- Future follow-up to add MCP servers for the CLIs that lack them.
- Any SKILL.md for brand-new CLIs added after 2026-04-20 (this plan's scope is fixed at the 8 currently missing).

## Context & Research

### Relevant Code and Patterns

- `library/sales-and-crm/contact-goat/SKILL.md` (canonical template): full workflow, "When to Use", auth surfaces, notable commands.
- `library/payments/kalshi/SKILL.md`: a second strong reference. Includes sport/event/market shape.
- `library/media-and-entertainment/espn/SKILL.md`: live-sports reference with trigger-phrase coverage.
- `library/productivity/cal-com/SKILL.md`: personal-productivity reference with OAuth + env-var auth pattern.
- `library/marketing/dub/SKILL.md`: API-key auth pattern with verifier-friendly flag references.
- `library/media-and-entertainment/movie-goat/SKILL.md`: streaming / region / comparison pattern that may fit steam-web.
- `plugin/skills/pp-slack/SKILL.md` (registry-only mirror, current thin state): shows the starting point; the new library SKILL.md will replace this.
- `tools/generate-skills/main.go`: regeneration engine. Already knows how to promote a registry-only mirror to a library-backed one when a SKILL.md appears.
- `.github/scripts/verify-skill/verify_skill.py`: CI verifier. Each new SKILL.md will need its flag references to match actual cobra declarations.
- `AGENTS.md` sections "SKILL.md coverage is not universal" and "Keeping plugin/skills in sync": the rules this plan operates under.

### Institutional Learnings

- PR #99 demonstrated the regeneration + version-bump flow for a single-CLI SKILL.md edit. This plan repeats that flow once across all 8 CLIs at the end.
- `verify_skill.py` catches flag references in SKILL.md that don't match the cobra source. Derive command/flag lists from `<cli> --help` output or directly from `internal/cli/*.go` to avoid drift.
- Prior plan `docs/plans/2026-04-20-002-feat-pp-slash-commands-plan.md` defines the slash-command generator that benefits from this plan's output.
- User feedback from 2026-04-20 session: clear preference for "when to use" framing, explicit trigger phrases, and auth setup upfront (per the "Enrichment Preflight" pattern added in PR #99).

### External References

- Each upstream API (Slack, Linear, Trigger.dev, Steam, etc.) ships its own API-key issuance page. Link those in the auth section per SKILL.md.

## Key Technical Decisions

- Decision: Each SKILL.md is written by reading the CLI's `--help` (recursively) and skimming `internal/cli/` as needed. Rationale: grounding in actual source prevents the verifier from blocking CI on phantom flags.
- Decision: Install any CLI that isn't on PATH before writing its SKILL.md (`go install ...@latest`). Rationale: `--help` is the fastest accurate surface; reading Go source alone is slower and more error-prone for curation decisions.
- Decision: Trigger-phrase list per skill is curated (5-15 phrases) not exhaustive. Rationale: over-broad triggers cause false positives; too-narrow triggers miss real queries. Model the phrase list on `pp-contact-goat`'s description, which has worked in practice.
- Decision: "Notable Commands" table is 5-10 rows, not the full command catalog. Rationale: agents reading the skill need to know which 5-10 commands cover 80% of real use; the raw --help remains available for advanced work.
- Decision: Each SKILL.md carries an `openclaw` metadata block like existing skills, with `requires.bins` and any primary env var. Rationale: keeps parity with the contact-goat / kalshi / dub shape the OpenClaw variant depends on.
- Decision: Order of implementation is install-friction-first: start with the 3 already-installed CLIs (slack, linear, dominos) to build the template discipline before tackling the 5 that need fresh installs. Rationale: faster feedback loop.
- Decision: Regeneration and plugin.json version bump are the LAST unit, not interleaved. Rationale: avoid N partial version bumps; one clean 1.1.21 -> 1.1.22 (or whatever comes after the sibling plan) bump after all 8 SKILL.md files land.

## Open Questions

### Resolved During Planning

- Q: Should pagliacci-pizza's SKILL.md cross-reference dominos (the two compete for "order a pizza" triggers)? A: Yes, each should carry a short "vs. other CLIs" note so agents know which to pick (e.g. pagliacci is Seattle-local; dominos is national).
- Q: Should the slack SKILL.md document Slack workspace cookie auth as a fallback to SLACK_BOT_TOKEN? A: Only if the CLI actually supports cookie auth; check `slack-pp-cli doctor` / `slack-pp-cli auth` subcommand. Resolve during Unit 8 implementation.
- Q: Does agent-capture need auth at all? A: It's a macOS screen-capture CLI, most likely uses native Screen Recording permission rather than an API key. Confirm during Unit 9.
- Q: Should registry-only plugin mirrors be deleted before regeneration? A: No. The generator overwrites them; stale files don't accumulate. Only the plugin mirrors are touched.

### Deferred to Implementation

- Q: Exact per-CLI trigger phrases. Resolve during each unit by inspecting the CLI's README.md and --help and thinking about what a user would actually type.
- Q: Which 5-10 commands go in each "Notable Commands" table. Resolve during each unit by reading --help and picking the ones that cover the top use cases.
- Q: Exact env var names (SLACK_BOT_TOKEN vs. SLACK_API_TOKEN vs. SLACK_USER_TOKEN; LINEAR_API_KEY vs. LINEAR_TOKEN). Resolve by grepping `internal/config/` or `internal/cli/doctor.go` in each CLI's source.
- Q: Exact cost model for any paid API (Linear plan tiers, Slack rate limits). Document what the CLI actually exposes; leave upstream pricing to their own docs.

## Implementation Units

- [ ] **Unit 1: Establish per-CLI SKILL.md template with shared structure**

**Goal:** Produce a reusable "template crib sheet" (internal to this plan, can live in `docs/plans/`) that every subsequent unit fills in, so all 8 SKILL.md files come out structurally consistent.

**Requirements:** R1

**Dependencies:** None

**Files:**
- Create: `docs/plans/2026-04-20-003-template.md` (working artifact; can be deleted after all 8 SKILL.md files land)

**Approach:**
- Read `library/sales-and-crm/contact-goat/SKILL.md` end-to-end.
- Extract the section order: frontmatter (name, description, argument-hint, allowed-tools, metadata), intro, "When to Use This CLI", "Argument Parsing", "CLI Installation" (including GOPRIVATE + @main fallback), "MCP Server Installation" (if applicable), "Direct Use", "Notable Commands", "Exit Codes".
- Capture which pieces are CLI-agnostic (argument parsing, exit codes, install blocks with just the slug swapped) vs. CLI-specific (intro, "when to use", auth setup, notable commands).
- Document decision criteria for skippable sections (e.g. skip MCP install block if no `cmd/<slug>-pp-mcp` exists on disk).

**Patterns to follow:**
- `library/sales-and-crm/contact-goat/SKILL.md` as the canonical reference.

**Test scenarios:**
- Test expectation: none -- documentation artifact for subsequent units.

**Verification:**
- Template document exists and enumerates each section with a "constant / per-CLI" marker so the next 8 units can fill it in confidently.

- [ ] **Unit 2: Write library/productivity/slack/SKILL.md**

**Goal:** Ship a real SKILL.md for `slack-pp-cli` replacing the registry-only mirror.

**Requirements:** R1, R2, R3, R4

**Dependencies:** Unit 1

**Files:**
- Create: `library/productivity/slack/SKILL.md`

**Approach:**
- Binary is already on PATH (`/Users/mvanhorn/go/bin/slack-pp-cli`). Run `slack-pp-cli --help` and `slack-pp-cli <top-level-cmd> --help` for each top-level command.
- Read `library/productivity/slack/internal/cli/root.go` and `internal/cli/doctor.go` for auth surface (env var names, token types - bot vs user, scopes expected).
- Write frontmatter:
  - `name: pp-slack`
  - `description`: include trigger phrases like "send a Slack message", "find my Slack conversation", "check Slack for", "summarize #channel", "who's online on Slack", "my Slack DMs".
  - `argument-hint: "<command> [args] | install cli|mcp"`
  - `allowed-tools: "Read Bash"`
  - `metadata`: openclaw block with `bins: [slack-pp-cli]`, `env: [SLACK_BOT_TOKEN]` (or the actual env name found during research), standard install command.
- Write "When to Use This CLI" section listing 5-8 concrete user asks.
- Write "Auth Setup" section with `SLACK_BOT_TOKEN` + minimum scopes (derive from source), link to `https://api.slack.com/apps`.
- "Notable Commands" table: 5-8 rows curating send-message, search, channel digest, user lookup, history/tail, emoji/reactions (pick based on --help).
- Standard "Argument Parsing", "Direct Use", "Exit Codes" sections from Unit 1 template.

**Patterns to follow:**
- `library/productivity/cal-com/SKILL.md` for env-var auth + productivity phrasing.
- `library/sales-and-crm/contact-goat/SKILL.md` for frontmatter shape.

**Test scenarios:**
- Integration: `python3 .github/scripts/verify-skill/verify_skill.py --dir library/productivity/slack` passes (all --flag references in SKILL.md match cobra declarations).
- Integration: `slack-pp-cli <each-notable-command> --help` actually produces the flags that SKILL.md references.

**Verification:**
- File exists with the target shape.
- Verifier green.

- [ ] **Unit 3: Write library/project-management/linear/SKILL.md**

**Goal:** Ship real SKILL.md for `linear-pp-cli`.

**Requirements:** R1, R2, R3, R4

**Dependencies:** Unit 1

**Files:**
- Create: `library/project-management/linear/SKILL.md`

**Approach:**
- Binary is on PATH. Run `linear-pp-cli --help` and recurse one level for each subcommand.
- Read `library/project-management/linear/internal/config/` or root.go for auth (LINEAR_API_KEY most likely per plugin mirror; verify).
- Note the offline/SQLite sync feature documented in the existing plugin description -- lean into it in the "When to Use" section ("search my Linear issues offline", "resolve Linear tickets without an API round-trip").
- Frontmatter description: include trigger phrases like "my Linear issues", "create a Linear ticket", "what's open on my Linear", "Linear sprint status", "assign Linear issue to X", "close Linear issue", "search Linear".
- "When to Use": cover issues (list/create/update/assign), projects, teams, cycles/sprints, attachments, sync.
- "Auth Setup": `LINEAR_API_KEY`, link to Linear settings for key creation.
- "Notable Commands": curate from the capability list in current mirror (analytics, attachments, audit, cycles, issues, projects, reactions, roadmaps, teams, search, sync) - pick the 6-8 most-used.

**Patterns to follow:**
- `library/sales-and-crm/hubspot/SKILL.md` for CRM/issue-tracker-shaped triggers.

**Test scenarios:**
- Integration: verifier passes.
- Integration: key notable commands return valid --help (spot-check 3).

**Verification:**
- File exists and verifier green.

- [ ] **Unit 4: Write library/commerce/dominos-pp-cli/SKILL.md**

**Goal:** Ship real SKILL.md for `dominos-pp-cli`.

**Requirements:** R1, R2, R3, R4

**Dependencies:** Unit 1

**Files:**
- Create: `library/commerce/dominos-pp-cli/SKILL.md`

**Approach:**
- Binary on PATH. Run `dominos-pp-cli --help` and recurse.
- Read `internal/cli/` for the address-auth flow (how the CLI holds a store / address).
- Frontmatter description: trigger phrases "order dominos", "order a pizza", "track my domino's order", "domino's menu", "deals on domino's", "cheapest domino's pizza", "nearest domino's store".
- "When to Use" vs "Skip it": point to pagliacci-pizza for Seattle local. Note that this is real live ordering (placing orders costs money) and requires explicit --yes-gate style confirmation.
- "Auth Setup": explain the address + phone number / account setup flow.
- "Notable Commands": address (set store), menu (browse), cart (build), checkout (place order), track (delivery status), compare-prices if present, rewards.
- Include an explicit "Financial actions" caveat in "When to Use" per the repo's overall safety guidance: require user confirmation before placing a real order.

**Patterns to follow:**
- `library/commerce/instacart/SKILL.md` for commerce-domain shape with cart + checkout.

**Test scenarios:**
- Integration: verifier passes.
- Integration: `dominos-pp-cli menu --help` and `dominos-pp-cli cart --help` reachable with flags matching SKILL.md.

**Verification:**
- File exists and verifier green.

- [ ] **Unit 5: Write library/food-and-dining/pagliacci-pizza/SKILL.md**

**Goal:** Ship real SKILL.md for `pagliacci-pizza-pp-cli`.

**Requirements:** R1, R2, R3, R4

**Dependencies:** Unit 1, Unit 4 (to mirror the pizza-ordering template)

**Files:**
- Create: `library/food-and-dining/pagliacci-pizza/SKILL.md`

**Approach:**
- Binary not installed. Run `GOPRIVATE='github.com/mvanhorn/*' go install github.com/mvanhorn/printing-press-library/library/food-and-dining/pagliacci-pizza/cmd/pagliacci-pizza-pp-cli@latest` first.
- Run --help and mirror the dominos structure.
- "When to Use" MUST cross-reference dominos: "use this for Seattle-area pagliacci; use pp-dominos for national coverage".
- Trigger phrases: "order pagliacci", "seattle pizza", "pagliacci delivery", "order from pagliacci".
- Same financial-actions caveat as dominos.

**Patterns to follow:**
- Unit 4 output (dominos SKILL.md) - reuse its structure.

**Test scenarios:**
- Integration: verifier passes.

**Verification:**
- File exists and verifier green.

- [ ] **Unit 6: Write library/developer-tools/trigger-dev/SKILL.md**

**Goal:** Ship real SKILL.md for `trigger-dev-pp-cli`.

**Requirements:** R1, R2, R3, R4

**Dependencies:** Unit 1

**Files:**
- Create: `library/developer-tools/trigger-dev/SKILL.md`

**Approach:**
- Binary not installed; install first.
- Focus on Trigger.dev's background-job / run-monitoring shape: runs, tasks, schedules, alerts on failures.
- Trigger phrases: "my trigger.dev runs", "trigger.dev failures", "check trigger.dev status", "monitor my jobs on trigger.dev", "which tasks are running on trigger.dev", "schedule a trigger.dev task".
- "When to Use": surfaced when the user has a Trigger.dev workspace and wants to monitor runs, inspect failures, or trigger a task without opening the web UI.
- "Auth Setup": TRIGGER_API_KEY (verify actual env var name from source).
- "Notable Commands": list runs, describe-run, list-tasks, schedules, alerts on failures.

**Patterns to follow:**
- `library/developer-tools/agent-capture` once Unit 9's research is done (if shape is similar); otherwise use hubspot.

**Test scenarios:**
- Integration: verifier passes.

**Verification:**
- File exists and verifier green.

- [ ] **Unit 7: Write library/developer-tools/postman-explore/SKILL.md**

**Goal:** Ship real SKILL.md for `postman-explore-pp-cli`.

**Requirements:** R1, R2, R3, R4

**Dependencies:** Unit 1

**Files:**
- Create: `library/developer-tools/postman-explore/SKILL.md`

**Approach:**
- Install first. Run --help.
- This is likely an API-Network-search wrapper: find APIs by name, view OpenAPI specs, list Postman workspaces. Confirm shape during research.
- Trigger phrases: "search postman for", "find an API on postman", "browse postman network", "postman workspace for X".
- "When to Use": agents researching an unfamiliar API who want to see docs / examples from the Postman public network.
- "Auth Setup": POSTMAN_API_KEY if required; may be public-read without auth.
- "Notable Commands": search, describe (get API details), list-workspaces.

**Patterns to follow:**
- `library/media-and-entertainment/hackernews/SKILL.md` for search-oriented read-only CLIs.

**Test scenarios:**
- Integration: verifier passes.

**Verification:**
- File exists and verifier green.

- [ ] **Unit 8: Write library/media-and-entertainment/steam-web/SKILL.md**

**Goal:** Ship real SKILL.md for `steam-web-pp-cli`.

**Requirements:** R1, R2, R3, R4

**Dependencies:** Unit 1

**Files:**
- Create: `library/media-and-entertainment/steam-web/SKILL.md`

**Approach:**
- Install first. Run --help.
- Steam Web API: player lookups, game stats, achievements, friend lists.
- Trigger phrases: "my steam library", "steam achievements for X", "who's playing Y on steam", "steam game stats", "compare steam profiles", "check if friend is online on steam".
- "Auth Setup": `STEAM_WEB_API_KEY` (confirm). Link to `https://steamcommunity.com/dev/apikey`.
- "Notable Commands": player profile, recent-games, achievements, friends, owned-games.

**Patterns to follow:**
- `library/media-and-entertainment/movie-goat/SKILL.md` for media-library shape.

**Test scenarios:**
- Integration: verifier passes.

**Verification:**
- File exists and verifier green.

- [ ] **Unit 9: Write library/developer-tools/agent-capture/SKILL.md**

**Goal:** Ship real SKILL.md for `agent-capture-pp-cli`.

**Requirements:** R1, R2, R3, R4

**Dependencies:** Unit 1

**Files:**
- Create: `library/developer-tools/agent-capture/SKILL.md`

**Approach:**
- Install first. Run --help.
- This is a macOS screen-capture / evidence-recording CLI per the existing plugin description ("Record, screenshot, and convert macOS windows and screens for AI agent evidence").
- "Auth Setup" is likely macOS Screen Recording permission rather than an env var. Document the permission-grant dance (System Settings -> Privacy & Security -> Screen & System Audio Recording).
- Trigger phrases: "screenshot this window", "record the screen for evidence", "capture agent output", "attach a screenshot to PR", "diff before/after screenshots".
- "Notable Commands": screenshot, record, batch, diff, find (find window), evidence (compose into a package).

**Patterns to follow:**
- `library/media-and-entertainment/archive-is/SKILL.md` for evidence-oriented shape.

**Test scenarios:**
- Integration: verifier passes.
- Edge case: document the macOS permission prompt so agents know to expect it on first run.

**Verification:**
- File exists and verifier green.

- [ ] **Unit 10: Regenerate plugin mirrors, bump plugin version, verify**

**Goal:** Promote all 8 CLIs from registry-only to library-backed in the plugin, and confirm CI stays green.

**Requirements:** R5, R6, R7, R8

**Dependencies:** Units 2-9

**Files:**
- Modify (via generator): `plugin/skills/pp-slack/SKILL.md`, `plugin/skills/pp-linear/SKILL.md`, `plugin/skills/pp-dominos/SKILL.md`, `plugin/skills/pp-pagliacci-pizza/SKILL.md`, `plugin/skills/pp-trigger-dev/SKILL.md`, `plugin/skills/pp-postman-explore/SKILL.md`, `plugin/skills/pp-steam-web/SKILL.md`, `plugin/skills/pp-agent-capture/SKILL.md`
- Modify: `plugin/.claude-plugin/plugin.json` (version bump)
- If the sibling slash-commands plan (2026-04-20-002) has already landed: the generator ALSO re-emits `plugin/commands/pp-<slug>.md` for each of the 8 newly-library-backed CLIs. This is expected, not extra scope.

**Approach:**
- Run `go run ./tools/generate-skills/main.go` from repo root.
- Check the diff: the 8 affected `plugin/skills/pp-*/SKILL.md` files should transition from "registry-only" thin mirrors to "library-backed" full mirrors matching the new library SKILL.md source.
- Bump `plugin/.claude-plugin/plugin.json` version by one patch (current 1.1.21; next available bump depending on sibling plan's landing order).
- Update AGENTS.md: remove the "CLIs without one (today: ...)" list, since the list is now empty (or significantly shorter if this plan lands partially).
- Run `.github/scripts/verify-skill/verify_skill.py --dir library/<each>` for each of the 8 and confirm all pass.

**Patterns to follow:**
- PR #99 commit pattern: `chore(plugin): regenerate pp-* skills + bump to X.Y.Z` with a brief summary of what promoted.

**Test scenarios:**
- Integration: verify-skills CI passes on the branch.
- Integration: `.github/workflows/generate-skills.yml` would not emit a diff if re-run on the merged branch (idempotence).
- Integration: typing `/pp-slack`, `/pp-linear`, etc. in Claude Code (after plugin update + reload) surfaces the real command rather than registry-only boilerplate -- assuming sibling plan 2026-04-20-002 has landed.

**Verification:**
- All 8 `plugin/skills/pp-*/SKILL.md` files match their library source (diff between the library file and plugin mirror shows only headers the generator adds).
- `plugin/.claude-plugin/plugin.json` version bumped.
- AGENTS.md "CLIs without one" list is removed (or shortened if any unit was deferred).
- `python3 .github/scripts/verify-skill/verify_skill.py --dir library/<each>` exits 0 for each of the 8.

## System-Wide Impact

- **Interaction graph:** Unchanged. Each SKILL.md is a description + workflow prose; no new tool dispatch paths. The plugin-skill regeneration is a derived artifact, same pipeline as PR #99.
- **Error propagation:** Unchanged. Errors surface from each CLI's own execution, not from SKILL.md.
- **State lifecycle risks:** None. SKILL.md files are pure documentation.
- **API surface parity:** After this lands, all 22 CLIs in the library have an equal-quality SKILL.md. Parity achieved.
- **Integration coverage:** `.github/scripts/verify-skill/verify_skill.py` is the key CI gate. Each new SKILL.md must pass all four checks (flag-names, flag-commands, positional-args, unknown-command).
- **Unchanged invariants:** No CLI Go source changes. No registry.json changes. No changes to CLIs that already have a SKILL.md (contact-goat, kalshi, etc.). The sibling slash-commands plan (2026-04-20-002) operates independently; this plan only makes its output richer.

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| --help output doesn't surface flags that the SKILL.md should document (e.g. persistent flags inherited from root). | Cross-reference `internal/cli/root.go` for persistent flags; verify_skill.py has explicit rules for root-vs-leaf flag resolution. |
| Trigger phrases overlap between CLIs (e.g. "pizza" hits both dominos and pagliacci; "my issues" could hit linear and hubspot). | Include a "Skip it when" / "vs. other CLIs" hint in each SKILL.md; Claude's skill matcher already breaks ties on description specificity. |
| verify_skill.py catches a flag-mismatch late in the cycle, blocking regeneration. | Run verifier per-unit, not only at Unit 10. Each of units 2-9 has an integration test scenario for the verifier. |
| A CLI's --help output is incomplete or misleading because of a bug the CLI itself has. | Note the bug as a follow-up issue; do not patch the SKILL.md to paper over a broken command. Skip that command from the "Notable Commands" table. |
| Env var name differs between documentation and actual source (e.g. SLACK_API_TOKEN vs SLACK_BOT_TOKEN). | Grep `os.Getenv(` in each CLI's source before writing auth sections. |
| Five CLIs need `go install` and could be blocked by Go module proxy lag or network issues. | Use `GOPRIVATE` + `@main` fallback per repo convention; if install fails, skip that unit and come back to it. The plan allows partial landing. |

## Documentation / Operational Notes

- Update `AGENTS.md` "SKILL.md coverage is not universal" in Unit 10 to reflect the new coverage reality (likely: "all 22 CLIs ship SKILL.md as of 2026-04-XX").
- Add a note in AGENTS.md "Keeping plugin/skills in sync" that `library/**/SKILL.md` is authoritative and `plugin/skills/pp-*/SKILL.md` is a derived mirror (already implied but worth making explicit).
- No CHANGELOG exists today; if added later, note SKILL.md coverage milestone.
- No rollout staging needed.

## Sources & References

- User request 2026-04-20: "skills for those 7 based on what you find and how it should work" (note: actually 8 CLIs - agent-capture is also missing).
- Canonical SKILL.md template: `library/sales-and-crm/contact-goat/SKILL.md`.
- Sibling plan: `docs/plans/2026-04-20-002-feat-pp-slash-commands-plan.md`.
- Prior regeneration pattern: PR #99 (`docs/plans/2026-04-20-001-fix-contact-goat-enrichment-tool-mapping-plan.md` Unit 5).
- Verifier: `.github/scripts/verify-skill/verify_skill.py`.
- Repo rules: `AGENTS.md` sections "SKILL.md coverage is not universal" and "Keeping plugin/skills in sync".
