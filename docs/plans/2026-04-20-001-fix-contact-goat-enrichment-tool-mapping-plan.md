---
title: "fix: Correct contact-goat enrichment tool mapping, add Deepline preflight, sharpen skill guidance"
type: fix
status: active
date: 2026-04-20
---

# fix: Correct contact-goat enrichment tool mapping, add Deepline preflight, sharpen skill guidance

**Target repo:** printing-press-library

## Overview

During a real agent session on 2026-04-20 (lookup for Mike Craig's Stripe email), the contact-goat CLI fumbled three of its four enrichment paths before finally succeeding via an undocumented direct call to `dropleads_email_finder`. The three failures were not environmental flakiness. They trace to a single root cause plus two adjacent bugs:

1. The Deepline tool IDs in `internal/deepline/types.go` point at `ai_ark_personality_analysis` and `ai_ark_find_emails`. Neither tool is an email enricher in the Deepline catalog. The first analyzes LinkedIn personality and requires a `url` field; the second is an async follow-on to a People Search that requires a `trackId`. Every `deepline find-email`, `deepline enrich-person`, and Deepline-leg `waterfall` call therefore dies with an HTTP 422 validation error.
2. The LinkedIn subprocess step in `waterfall.go` ships `{"linkedin_url": target}` to `get_person_profile`, but the upstream `stickerdaniel/linkedin-mcp-server` tool requires `linkedin_username`. Every waterfall with a LinkedIn URL target errors on step one.
3. The DEEPLINE_API_KEY check only fires deep inside each Deepline attempt. Commands like `dossier --enrich-email` and `waterfall` run the full free chain (LinkedIn subprocess, Happenstance) before surfacing the missing-key problem, wasting 15-30 seconds per invocation.

A fourth, lower-severity issue is that provider-entitlement 403s from `code.deepline.com` (e.g. a valid key that lacks contactout or datagma access) get rendered as `Auth error (status 403). Check DEEPLINE_API_KEY.` which is misleading.

This plan fixes the tool mapping, re-routes the waterfall to a real provider menu, adds command-level preflight for the Deepline key, sharpens the error message on provider-entitlement 403s, and updates SKILL.md so future agents know that email enrichment requires DEEPLINE_API_KEY before they burn time on free-step retries.

## Problem Frame

contact-goat positions itself as a cross-source enrichment tool (`deepline find-email`, `waterfall`, `dossier --enrich-email`). In practice the managed Deepline path has never worked because the tool IDs are wrong. Agents routing through the CLI learn of the brokenness only after running the whole free chain. Users with a valid Deepline key cannot retrieve a work email through the CLI without bypassing contact-goat and calling `deepline tools execute` directly.

Session trace (2026-04-20):

- `waterfall https://www.linkedin.com/in/mkscrg --enrich email` failed at LinkedIn (`linkedin_username` missing), failed at Happenstance (empty), and failed at Deepline (`ai_ark_personality_analysis` missing required `url`).
- `deepline find-email "Mike Craig" --company stripe.com` routed to `ai_ark_find_emails` and failed with `trackId: Missing required field`.
- `deepline enrich-person <url>` routed to `ai_ark_personality_analysis` and failed with `url: Missing required field`.
- Direct `deepline tools execute dropleads_email_finder --payload '{...}'` worked on first try.

The brokenness is a pure client-side wiring bug, not an upstream API limitation.

## Requirements Trace

- R1. `deepline find-email <name> --company <domain>` MUST succeed against a live Deepline account without the caller hand-crafting a payload. The command SHOULD route through a waterfall of real email-finder providers (dropleads, hunter, datagma, icypeas) rather than the ai_ark family.
- R2. `deepline enrich-person <linkedin_url>` MUST route to a person-enrichment provider that accepts a LinkedIn URL (apollo_people_match, hunter_people_find, or contactout_enrich_person) and return email/phone/title fields.
- R3. `waterfall` MUST run the LinkedIn step successfully when given a LinkedIn URL target. The subprocess call MUST send `linkedin_username` as required by the upstream MCP tool.
- R4. `waterfall --enrich email` MUST use the correct Deepline tool for the target kind (linkedin_url vs. name vs. email) and MUST NOT call `ai_ark_personality_analysis` or `ai_ark_find_emails` as direct email-enrichment steps.
- R5. Every enrichment command that requires `DEEPLINE_API_KEY` (waterfall with no BYOK, dossier with --enrich-email, deepline find-email, deepline enrich-person, etc.) MUST short-circuit with a clear, actionable error BEFORE running any other step when the key is missing or invalid.
- R6. Deepline HTTP 403 responses MUST distinguish between "key missing/invalid" and "key valid but provider not entitled on this account". The error message MUST tell the user which provider returned 403 and suggest trying a different provider rather than re-checking their key.
- R7. SKILL.md MUST contain an up-front preflight checklist for enrichment commands so that future agent runs check for DEEPLINE_API_KEY before invoking waterfall/dossier/find-email.
- R8. The plugin mirror of SKILL.md (`plugin/skills/pp-contact-goat/SKILL.md`) and `plugin/.claude-plugin/plugin.json` MUST be regenerated and version-bumped after SKILL.md changes, per the repo convention in `AGENTS.md`.

## Scope Boundaries

- Cookie-path behavior (Happenstance quota fallback, rate-limit backoff) is not changed here. The 15s retry delay observed in the session trace is real but tolerable; tuning it is a separate plan.
- No new Deepline providers are added to the catalog. This plan only re-wires existing Deepline tools.
- BYOK key handling (Hunter/Apollo personal keys) is preserved as-is; this plan does not change the BYOK surface except where the waterfall's tool selection changes.
- This plan does not implement a new cross-provider verifier (Hunter verify, icypeas verify). Verification remains a separate feature.

### Deferred to Separate Tasks

- Cookie-quota retry tuning (15s bucket delay): future plan.
- Email-verification waterfall (deliverability check): future feature.
- Icypeas async polling helper: future plan (current call returns an async job ID that the CLI does not poll).

## Context & Research

### Relevant Code and Patterns

- `library/sales-and-crm/contact-goat/internal/deepline/types.go`: tool ID constants. Root cause lives here.
- `library/sales-and-crm/contact-goat/internal/deepline/client.go`: `ValidateKey`, `Execute`, `EstimateCost`. Entitlement-vs-auth distinction needs to happen here.
- `library/sales-and-crm/contact-goat/internal/deepline/http.go`: HTTP layer that currently maps any 403 to a generic auth error.
- `library/sales-and-crm/contact-goat/internal/cli/waterfall.go`: `tryLinkedIn` (line 211, wrong key name) and `tryDeepline` (lines 267-341, wrong tool routing).
- `library/sales-and-crm/contact-goat/internal/cli/deepline.go`: `find-email` and `enrich-person` cobra commands that route through the broken constants.
- `library/sales-and-crm/contact-goat/internal/cli/dossier.go`: `--enrich-email` path.
- `library/sales-and-crm/contact-goat/internal/cli/doctor.go`: existing Deepline WARN surface. Reuse its validation logic for the new preflight.
- `library/sales-and-crm/contact-goat/SKILL.md`: agent-facing guidance doc.
- `plugin/skills/pp-contact-goat/SKILL.md`: generated mirror (must be regenerated after SKILL.md edits).
- `plugin/.claude-plugin/plugin.json`: version must bump when SKILL.md changes.
- `tools/generate-skills/main.go`: regeneration entry point.

### Institutional Learnings

- AGENTS.md, section "Keeping plugin/skills in sync": SKILL.md edits require running `go run ./tools/generate-skills/main.go` and a manual `plugin.json` version bump, because the generator's `maybeUpdatePluginVersion` does not auto-bump on SKILL content changes.
- AGENTS.md, section "SKILL.md verification": the flag-names verifier in `.github/scripts/verify-skill/verify_skill.py` will fail CI if new --flags are referenced in SKILL.md without being declared in `internal/cli/*.go`.
- Prior plan `docs/plans/2026-04-19-003-fix-contact-goat-usability-and-install-freshness-plan.md` addressed related DX gaps; this plan continues that thread but targets the enrichment subsystem specifically.
- Prior plan `library/sales-and-crm/contact-goat/docs/plans/2026-04-19-001-fix-bearer-api-mutuals-plan.md` showed the same class of bug (client-side wiring dropped upstream data); the lesson is that tool IDs and payload shapes need an integration test against a live Deepline account, not only a unit test against mocks.

### External References

- Deepline tools catalog (live): `deepline tools list | grep email_finder` enumerates the real email-enrichment tools. Verified 2026-04-20: `apollo_people_match`, `hunter_email_finder`, `hunter_people_find`, `dropleads_email_finder`, `datagma_find_email`, `icypeas_email_search`, `contactout_enrich_person`, `bettercontact_enrich`, `fullenrich_enrich`.
- Deepline schema for `dropleads_email_finder` (verified 2026-04-20): requires `first_name`, `last_name`; optional `company_domain`, `company_name`. Returns `{email, status, mx_record, mx_provider, credits_charged}`. Status values include `valid`, `catch_all`, `unknown`.
- Deepline schema for `apollo_people_match` (verified 2026-04-20): accepts `linkedin_url`, `email`, `hashed_email`, `first_name + last_name + domain`, `organization_name`, `reveal_personal_emails` (default true). Returns a full person record including `personal_emails[]`, `email`, `email_status`, `extrapolated_email_confidence`, `employment_history`, `organization`.
- stickerdaniel/linkedin-mcp-server `get_person_profile` tool schema (verified 2026-04-20): requires `linkedin_username` (string, the slug after `/in/`), NOT `linkedin_url`.

## Key Technical Decisions

- Decision: Replace the broken ai_ark tool IDs with a provider menu keyed by target kind, rather than keep the single-tool abstraction. Rationale: the three target kinds (linkedin_url, email, name) each have different best-fit providers and payload shapes. Pretending they share one tool ID is why the current code breaks.
- Decision: Keep `ToolPersonEnrich` and `ToolEmailFind` as named constants, but point them at correct tools (`apollo_people_match` and `dropleads_email_finder` respectively). Rationale: external callers (MCP server, direct constant consumers) can keep their symbols.
- Decision: Preflight for DEEPLINE_API_KEY happens in each command's `PreRunE`, not in a shared parent. Rationale: commands differ in whether the key is required (dossier without --enrich-email doesn't need it, dossier with it does). Per-command preflight keeps the logic local and testable.
- Decision: Provider-entitlement 403s map to a distinct error type (`ErrProviderNotEntitled`) that surfaces the provider name and suggests an alternative. Rationale: today a user with a valid key gets the same message as a user with no key. This blocks troubleshooting.
- Decision: The waterfall's Deepline step sequences through 2-3 providers internally before giving up, rather than a single call. Rationale: Apollo often has no verified work email while Dropleads has a catch-all guess; sequencing gives the user a real answer more often. Order: apollo_people_match (personal email + verified work) → dropleads_email_finder (pattern guess) → hunter_email_finder (pattern guess with confidence score). Each logs its own step.
- Decision: SKILL.md adds an "Enrichment preflight" section at the top of the argument-parsing block. Rationale: agents read SKILL.md sequentially; the gate needs to fire before they hit the command table.

## Open Questions

### Resolved During Planning

- Q: Should `ai_ark_personality_analysis` be kept as a separate tool constant? A: Yes but renamed to `ToolPersonalityAnalysis`. It is a real Deepline tool for a real use case (personality-based outbound messaging); it just is not an email enricher. Preserving it as a named constant makes future work easier.
- Q: Should `ai_ark_find_emails` be removed entirely? A: No, but drop its alias to `ToolEmailFind`. Keep it as `ToolExportedEmailFinder` for the export workflow (People Search → trackId → find-emails chain) which is a real flow. Do not let the direct `find-email` command route to it.
- Q: Should preflight run even with `--dry-run`? A: Yes. `--dry-run` should tell the user what the call would be AND what's missing. Running the key check is cheap.

### Deferred to Implementation

- Q: Exact error type names (`ErrProviderNotEntitled`, `ErrDeeplineAuth`) and whether to use sentinel errors or a typed struct. Resolved during implementation based on what integrates cleanest with existing `internal/deepline/client.go`.
- Q: Whether to page apollo_people_match's `personal_emails[]` separately from its work `email`. Resolved when writing the field-merge logic: probably return both under `personal_email` and `email` keys in the Waterfall result, with the latter preferring work.

## High-Level Technical Design

> This illustrates the intended approach and is directional guidance for review, not implementation specification. The implementing agent should treat it as context, not code to reproduce.

Provider menu, keyed by target kind:

    target kind        primary provider          fallback 1                fallback 2
    -----------        ----------------          ----------                ----------
    linkedin_url   ->  apollo_people_match   ->  hunter_people_find    ->  contactout_enrich_person
    email          ->  apollo_people_match   ->  hunter_people_find    ->  (stop)
    name+domain    ->  dropleads_email_finder -> hunter_email_finder   ->  datagma_find_email

Preflight gate on enrichment commands:

    command invoked
       |
       v
    PreRunE:
       if requires_deepline(cmd, flags):
         key = env.DEEPLINE_API_KEY or --deepline-key
         if missing: return ErrMissingKey with actionable message
         if invalid_shape: return ErrMalformedKey
       (do NOT call code.deepline.com here; a ping costs credits)
       |
       v
    Run command

Error disambiguation on 403:

    HTTP 403 from code.deepline.com
       |
       response body matches { "error_category": "auth", "code": "AUTH_*" }?
       |-- yes -> ErrDeeplineAuth ("key is missing/invalid/expired")
       |
       response body matches provider-forbidden signals?
       |-- yes -> ErrProviderNotEntitled(provider) ("key is valid, but provider X is not enabled on this account; try provider Y")
       |
       else  -> wrap as generic 403 with response body included

## Implementation Units

- [ ] **Unit 1: Correct the Deepline tool constants**

**Goal:** Re-point the ai_ark tool constants to real email-enrichment providers and add the provider menu used by the waterfall.

**Requirements:** R1, R2, R4

**Dependencies:** None

**Files:**
- Modify: `library/sales-and-crm/contact-goat/internal/deepline/types.go`
- Test: `library/sales-and-crm/contact-goat/internal/deepline/types_test.go`

**Approach:**
- Rename `ToolPersonEnrich` to point at `apollo_people_match`. Keep the old symbol as a deprecated alias pointing at the new value for one release so MCP callers don't break instantly.
- Rename `ToolEmailFind` to point at `dropleads_email_finder`. Drop the alias to `ToolPersonSearchToEmailWaterfall`.
- Keep `ToolPersonSearchToEmailWaterfall = "ai_ark_find_emails"` as-is, but add a comment explaining this tool requires a trackId from `ai_ark_people_search` and should not be called directly from `find-email`.
- Add a new constant `ToolPersonalityAnalysis = "ai_ark_personality_analysis"` preserving the old value for callers that genuinely want personality analysis.
- Add provider-menu constants: `ToolDropleadsEmailFinder`, `ToolHunterEmailFinder`, `ToolHunterPeopleFind`, `ToolApolloPeopleMatch`, `ToolContactOutEnrichPerson`, `ToolDatagmaFindEmail`, `ToolIcypeasEmailSearch`.
- Extend the `toolMetadata` map with cost and payload-hint info for each new provider.

**Patterns to follow:**
- Existing `toolMetadata` shape in `internal/deepline/types.go` (see the entries for `ai_ark_find_emails` and `ai_ark_personality_analysis`).

**Test scenarios:**
- Happy path: constants resolve to the expected upstream IDs (`apollo_people_match`, `dropleads_email_finder`, etc.).
- Happy path: `toolMetadata` contains an entry for every new constant, with non-zero cost and a descriptive label.
- Edge case: deprecated alias `ToolPersonEnrich` still compiles and resolves to the new value.

**Verification:**
- `go build ./...` succeeds.
- `go test ./internal/deepline/...` passes.

- [ ] **Unit 2: Route waterfall Deepline step through the provider menu**

**Goal:** Replace `tryDeepline` in `waterfall.go` so it picks the correct provider chain based on target kind and payload shape.

**Requirements:** R1, R2, R4

**Dependencies:** Unit 1

**Files:**
- Modify: `library/sales-and-crm/contact-goat/internal/cli/waterfall.go`
- Test: `library/sales-and-crm/contact-goat/internal/cli/waterfall_test.go`

**Approach:**
- Replace the switch on `r.TargetKind` (currently lines 287-299) with a call to a new helper `deeplineProviderChain(targetKind, target, enrichFields, byok) []providerAttempt`.
- For `linkedin_url` targets, sequence `apollo_people_match` -> `hunter_people_find` -> `contactout_enrich_person`.
- For `email` targets, sequence `apollo_people_match` (by email) -> `hunter_people_find` (by email).
- For `name` targets, require a company domain (from `--company` flag or `CONTACT_GOAT_COMPANY` env), then sequence `dropleads_email_finder` -> `hunter_email_finder` -> `datagma_find_email`.
- Run providers sequentially; stop the chain as soon as the target fields are filled or max-cost is exceeded. Log each attempt as its own `WaterfallStep`.
- Payload mapping per provider:
  - apollo_people_match: `{linkedin_url | email | first_name+last_name+domain, reveal_personal_emails: true}`
  - hunter_email_finder: `{first_name, last_name, domain}`
  - dropleads_email_finder: `{first_name, last_name, company_domain, company_name}` (NOT `domain`)
  - datagma_find_email: `{first_name, last_name, company_domain}`
  - contactout_enrich_person: `{linkedin_url}`
- Add a `--company` flag on the waterfall command itself (currently it reads `CONTACT_GOAT_COMPANY` from env; promote to an explicit flag per the existing help text which already documents it but doesn't declare it).
- Field merging: apollo's `personal_emails[0]` fills a new `personal_email` field in the Waterfall result; apollo's `email` fills `email` when non-empty; dropleads' `catch_all` status fills a new `email_confidence = "catch_all"` annotation rather than being treated as verified.

**Patterns to follow:**
- Existing `tryDeepline` step-logging shape in `waterfall.go`.
- `applyEnrichFields` helper for field copying.

**Test scenarios:**
- Happy path: `linkedin_url` target with a mock Deepline client returning apollo hit on first try - only one step recorded, `email` and `personal_email` filled.
- Happy path: `name` target + `--company stripe.com` with apollo returning empty, dropleads returning `mike@stripe.com` status catch_all - two steps recorded, `email` filled with `email_confidence = "catch_all"`.
- Edge case: `name` target without `--company` or `CONTACT_GOAT_COMPANY` returns a user-facing error "name targets require --company".
- Edge case: max-cost exhausted after first provider - chain short-circuits and records remaining providers as `skipped`.
- Error path: first provider returns provider-entitlement 403 - step recorded as `error` with provider name, chain continues to fallback.

**Verification:**
- Integration test using a mocked `deepline.Client` exercises all three target-kind chains.
- Live run: `waterfall https://www.linkedin.com/in/mkscrg --enrich email` returns a filled `personal_email` field (session trace shows this is available via apollo_people_match).

- [ ] **Unit 3: Fix LinkedIn subprocess payload key + add command-level Deepline preflight**

**Goal:** Make the LinkedIn step actually run when given a LinkedIn URL, and stop running free-chain steps when the Deepline key is missing.

**Requirements:** R3, R5

**Dependencies:** None (can ship in parallel with Unit 2)

**Files:**
- Modify: `library/sales-and-crm/contact-goat/internal/cli/waterfall.go`
- Modify: `library/sales-and-crm/contact-goat/internal/cli/deepline.go`
- Modify: `library/sales-and-crm/contact-goat/internal/cli/dossier.go`
- Modify: `library/sales-and-crm/contact-goat/internal/linkedin/mcp.go` (if the key mapping is abstracted there) or waterfall.go directly
- Test: `library/sales-and-crm/contact-goat/internal/cli/waterfall_test.go`
- Test: `library/sales-and-crm/contact-goat/internal/cli/preflight_test.go` (new)

**Approach:**
- LinkedIn key fix: in `tryLinkedIn` (waterfall.go line 211), derive a `linkedin_username` from the URL (split on `/in/`, take the next path segment, strip trailing slash) and pass `{"linkedin_username": username}` to `get_person_profile`. Keep the original URL available in the result snippet for debugging.
- Preflight helper: create `requireDeeplineKey(cmd *cobra.Command) error` in a shared spot (e.g. `internal/cli/preflight.go`) that reads `DEEPLINE_API_KEY` (or the `--deepline-key` flag where present), validates shape with `dlp_` prefix + length check, and returns a typed error with a one-line fix: "Set DEEPLINE_API_KEY or pass --deepline-key. Get a key at https://code.deepline.com/settings/api-keys".
- Wire preflight into:
  - `waterfall` PreRunE: required unless `--byok` and BYOK providers cover the requested fields.
  - `dossier` PreRunE: required only when `--enrich-email` is set.
  - `deepline` subcommand root PreRunE: always required (all deepline subcommands need it).
- Update `waterfall` help text to mention the flag promotion and the preflight behavior.

**Patterns to follow:**
- Existing `PersistentPreRunE` wiring in `internal/cli/root.go`.
- Existing `newClientRequireCookies` helper in `flags` for the Happenstance cookie preflight.

**Test scenarios:**
- Happy path: `tryLinkedIn` called with `https://www.linkedin.com/in/satyanadella/` sends `{"linkedin_username": "satyanadella"}` to the mock MCP client.
- Happy path: `waterfall <linkedin_url>` with DEEPLINE_API_KEY unset and no BYOK errors with the preflight message before any step runs. No LinkedIn or Happenstance calls are made.
- Happy path: `dossier <url>` with no `--enrich-email` and no DEEPLINE_API_KEY succeeds (preflight is not required).
- Happy path: `dossier <url> --enrich-email` with no DEEPLINE_API_KEY errors via preflight.
- Edge case: `waterfall "Brian Chesky" --company airbnb.com --byok` with HUNTER_API_KEY set succeeds past preflight (BYOK satisfies it).
- Edge case: URL with no trailing slash or with trailing query params still extracts the right username.
- Error path: malformed key (e.g. `DEEPLINE_API_KEY=foo`) errors with "invalid key shape" distinct from "missing key".

**Verification:**
- `contact-goat-pp-cli waterfall <linkedin_url> --enrich email --dry-run` returns the preflight error instantly when key is missing.
- `contact-goat-pp-cli waterfall <linkedin_url>` with both keys set shows a `linkedin` step status `ok` in the result.

- [ ] **Unit 4: Disambiguate Deepline 403 auth errors from provider-entitlement errors**

**Goal:** Stop reporting every 403 as "Check DEEPLINE_API_KEY" when the real problem is that a specific provider is not enabled on the account.

**Requirements:** R6

**Dependencies:** None

**Files:**
- Modify: `library/sales-and-crm/contact-goat/internal/deepline/http.go`
- Modify: `library/sales-and-crm/contact-goat/internal/deepline/client.go`
- Test: `library/sales-and-crm/contact-goat/internal/deepline/http_test.go`

**Approach:**
- Inspect the 403 response body for signals that distinguish the two cases:
  - Auth failure: `error_category: "auth"`, `code` starting with `AUTH_`, or message containing "API key".
  - Provider entitlement: `error_category: "provider"` or `authorization`, `provider` field set, message containing "not enabled" or "not authorized for this integration".
- Define sentinel/typed errors: `ErrDeeplineAuth` for auth failures and `ErrProviderNotEntitled{Provider string}` for entitlement failures.
- Update the caller in `tryDeepline` to surface the provider name and continue to the next provider in the chain (not fall out of the whole waterfall) when the error is entitlement-related.
- Update the human-friendly error message on auth failures: include whether the key is unset, malformed, or rejected by upstream, plus the fix command.

**Patterns to follow:**
- Existing error-wrapping in `internal/deepline/http.go` that already parses the JSON error body for `message`.

**Test scenarios:**
- Happy path: 403 with body `{"error_category": "auth", "code": "AUTH_INVALID_KEY"}` maps to `ErrDeeplineAuth` with "key is rejected by upstream; check validity at https://code.deepline.com/settings/api-keys".
- Happy path: 403 with body `{"provider": "contactout", "message": "Integration not enabled"}` maps to `ErrProviderNotEntitled{Provider: "contactout"}` with a suggestion to try a different provider.
- Edge case: 403 with unrecognized body shape falls back to a generic wrapper that includes the raw body.
- Error path: non-403 statuses (422, 500) retain their current behavior.
- Integration: waterfall with a key that has only dropleads enabled skips apollo (entitlement 403) and lands on dropleads successfully, with apollo step recorded as `error` not `auth_failure`.

**Verification:**
- `go test ./internal/deepline/...` passes with new 403 variants.
- Live repro: direct `deepline tools execute contactout_enrich_person` call surfaces a provider-entitlement error, not "Check DEEPLINE_API_KEY".

- [ ] **Unit 5: SKILL.md enrichment preflight + plugin mirror regeneration**

**Goal:** Update the agent-facing skill doc so future agents know DEEPLINE_API_KEY is required before they waste time on a waterfall, and ship the regenerated plugin mirror.

**Requirements:** R7, R8

**Dependencies:** Units 1-4 (docs should match shipped behavior)

**Files:**
- Modify: `library/sales-and-crm/contact-goat/SKILL.md`
- Modify: `plugin/skills/pp-contact-goat/SKILL.md` (regenerated, not hand-edited)
- Modify: `plugin/.claude-plugin/plugin.json` (version bump)

**Approach:**
- Add a new section "Enrichment preflight" after the argument-parsing block in SKILL.md:
  - Lists the commands that need `DEEPLINE_API_KEY`: `waterfall` (unless `--byok`), `dossier --enrich-email`, `deepline find-email`, `deepline enrich-person`, `deepline phone-find`, `deepline email-find`, `deepline search-people`, `deepline search-companies`, `deepline enrich-company`.
  - Tells the agent: "If the user's task needs email/phone enrichment and DEEPLINE_API_KEY is not set, ask for it (or for a BYOK Hunter/Apollo key) BEFORE invoking any enrichment command. Do not run waterfall and watch it fail through three free steps."
  - Documents the provider chain by target kind so agents know what the tool will do.
- Clarify the `waterfall` command row in the command table: mention the new `--company` flag for name targets.
- Clarify the `dossier` command row: mention `--enrich-email` requires DEEPLINE_API_KEY.
- Regenerate plugin mirror: `go run ./tools/generate-skills/main.go` from the repo root.
- Bump plugin version in `plugin/.claude-plugin/plugin.json` by one patch level (current version TBD at implementation time; follow AGENTS.md guidance).

**Execution note:** The generator's `maybeUpdatePluginVersion` only bumps on directory-set changes. SKILL content changes require manual bump. See AGENTS.md "Keeping plugin/skills in sync".

**Patterns to follow:**
- Existing SKILL.md structure (sections "Argument Parsing", "CLI Installation", "MCP Server Installation", "Commands").
- Prior plugin version bumps in git history: `git log --oneline plugin/.claude-plugin/plugin.json` for the cadence.

**Test scenarios:**
- Test expectation: none for SKILL.md content. The verify-skill CI covers flag-reference drift; the generator test covers mirror regeneration.
- Integration: `.github/scripts/verify-skill/verify_skill.py` passes against the updated SKILL.md. Any new `--flag` mentions (e.g. `--company`) must be declared in `internal/cli/*.go` from Unit 2 or the verifier fails CI.
- Integration: `go run ./tools/generate-skills/main.go` produces a `plugin/skills/pp-contact-goat/SKILL.md` whose content matches the library source.

**Verification:**
- `.github/workflows/verify-skills.yml` passes on the branch.
- `plugin/skills/pp-contact-goat/SKILL.md` is a byte-identical regeneration (no hand edits).
- `plugin/.claude-plugin/plugin.json` version field is higher than main.

## System-Wide Impact

- **Interaction graph:** The waterfall command currently feeds into the MCP server's `waterfall` tool; the MCP tool wraps the CLI constant mappings so Unit 1's constant changes propagate automatically. No MCP manifest edit required.
- **Error propagation:** The new `ErrProviderNotEntitled` error must not abort the whole waterfall run; it should record a step error and continue. Callers outside the waterfall (e.g. `deepline enrich-person` direct command) should surface the error as a user-facing hint, not a hard failure that hides the provider name.
- **State lifecycle risks:** None. No persistent state changes. The SQLite cache stores results but not error states.
- **API surface parity:** The `ToolPersonEnrich` and `ToolEmailFind` constants are consumed by the MCP server package. Preserving the old symbol names while pointing them at the new tool IDs keeps the MCP manifest valid without a rebuild.
- **Integration coverage:** Every new provider call needs at least one mock test AND at least one live smoke test against a real Deepline key before release. Unit tests alone missed the current breakage.
- **Unchanged invariants:** The cookie-first / bearer-fallback Happenstance routing is unchanged. The BYOK surface is unchanged. The `coverage`, `hp people`, `prospect`, `warm-intro` commands are unchanged. The `waterfall` JSON result shape is backward-compatible (new fields added: `personal_email`, `email_confidence`; no existing fields removed).

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| A Deepline provider changes its payload shape upstream between plan and implementation. | Re-verify schemas with `deepline tools get <id> --json` immediately before writing Unit 2's payload builders. Pin the verified schemas in code comments. |
| Apollo `reveal_personal_emails` costs more credits than expected on some accounts. | Log the reported cost per step. The existing `EstimateCost` surface already honors per-tool cost metadata from Unit 1. |
| Provider entitlement varies per account, so the default chain fails for users whose key doesn't include apollo. | Unit 4's error disambiguation lets the chain continue past entitlement failures. Document the expected provider list in SKILL.md so users can request access before running. |
| Plugin version bump is forgotten, causing the mirror SKILL.md to ship without the new plugin version. | Unit 5 makes the bump explicit; AGENTS.md already documents the manual step; consider adding a CI check in a future plan. |
| Live smoke test consumes real Deepline credits. | Use a dedicated low-balance test key; cap the max-cost flag at 2 during CI. |
| The deprecated alias for `ToolPersonEnrich` leaves the broken tool still reachable via the old name. | The alias is a one-release grace period only. Add a TODO with a removal target (e.g. next minor version bump). |

## Documentation / Operational Notes

- Update the `contact-goat/README.md` enrichment section to match the new provider chain.
- Consider adding a `docs/solutions/` entry after implementation that documents the pattern "integration tool IDs must be verified against the live catalog, not assumed from naming conventions" so future CLI generations don't repeat the ai_ark mis-identification.
- No rollout staging needed; this is a pure client-side fix.

## Sources & References

- Origin: live session trace 2026-04-20 (Mike Craig Stripe email lookup) captured in this conversation.
- Related code: `library/sales-and-crm/contact-goat/internal/deepline/types.go`, `waterfall.go`.
- Related prior plan: `docs/plans/2026-04-19-003-fix-contact-goat-usability-and-install-freshness-plan.md`.
- Related prior plan: `library/sales-and-crm/contact-goat/docs/plans/2026-04-19-001-fix-bearer-api-mutuals-plan.md`.
- External: Deepline tools catalog (`deepline tools list`), verified 2026-04-20.
- External: stickerdaniel/linkedin-mcp-server `get_person_profile` tool schema, verified 2026-04-20.
- Repo conventions: `AGENTS.md` sections "Keeping plugin/skills in sync" and "SKILL.md verification".
