# Printing Press Retro: DNS Made Easy

## Session Stats
- API: dnsmadeeasy (official V2.0 REST API)
- Spec source: hand-authored internal YAML from the official Go SDK (`DNSMadeEasy/dme-go-client`) + community wrappers; no OpenAPI exists
- Scorecard: 95/100 (Grade A, post-polish)
- Verify pass rate: 100%
- Fix loops: 2 (generation govulncheck gate; code-review fixes)
- Manual code edits: signing transport + 6 novel commands + sync-records + custom store migration (expected transcendence work) + 1 one-line hook in generated client.New
- Features built from scratch: 6 transcendence commands + sync-records (all hand-code, all shipped)

## Findings

### 1. Go quality-gate govulncheck pins the press's own build toolchain, false-failing on stdlib-only CVEs already fixed in an available newer patch (Bug / gate behavior)
- **What happened:** `generate --validate` (and later the `verify` leg) failed the `govulncheck ./...` gate on `GO-2026-5856` (crypto/tls ECH), reported as "Found in: crypto/tls@go1.26.4". The host had go1.26.5 available and `go env -w GOTOOLCHAIN=go1.26.5` set, yet the gate subprocess emitted `go: downloading go1.26.4` and analyzed against the older stdlib. The generated code was clean under go1.26.5 (build + vet + govulncheck "No vulnerabilities found"). Generation was blocked until go.mod was bumped to `go 1.26.5` + `toolchain go1.26.5`.
- **Scorer correct?** Partially. govulncheck correctly reports a real stdlib CVE, but the gate analyzed against a toolchain the machine pinned (its own build version, go1.26.4) rather than the newest patch available on the host — turning an already-fixed-and-available stdlib vuln into a hard, non-actionable generation block. The penalty is real; the toolchain selection is wrong.
- **Root cause:** The Go quality gates (generate `--validate` and `verify`'s govulncheck) appear to run with `GOTOOLCHAIN` pinned to the press binary's build version, ignoring both the host's newer available patch and the module's `go` directive. (Uncertainty: the exact mechanism — explicit `GOTOOLCHAIN=go<press-build>` on the subprocess vs. the generated go.mod directive driving exact selection — could not be confirmed from outside the binary. Disambiguate by checking how the govulncheck/verify subprocess sets `GOTOOLCHAIN`/`GOFLAGS`.)
- **Cross-API check:** Recurs on **every** CLI generated in the window between a stdlib CVE disclosure and the press being rebuilt on the patched toolchain — independent of API shape. Not API-specific; it is a gate-infra defect with universal blast radius on affected machines.
- **Frequency:** every API (on any host where the press's pinned toolchain has a since-fixed stdlib CVE, or is older than an available host patch).
- **Fallback if the Printing Press doesn't fix it:** The agent must diagnose an opaque "affected by 1 vulnerability from the Go standard library" gate failure, realize it is stdlib-only + already fixed, and manually bump go.mod / pin GOTOOLCHAIN. High-friction and easy to misread as a code defect; an agent may waste a fix loop or wrongly HOLD.
- **Worth a Printing Press fix?** Yes. It blocks generation entirely and is fully preventable.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Either (a) run the Go quality gates with `GOTOOLCHAIN=auto` (or the newest installed patch of the required minor) so an available fixed stdlib is used, or (b) classify a govulncheck finding that is stdlib-only AND fixed in a patch newer-or-equal to an available toolchain as WARN (with actionable "bump go directive / GOTOOLCHAIN" guidance) rather than a hard gate FAIL. Prefer (a); (b) is the fallback when the host genuinely has no fixed toolchain.
- **Test:** Positive — on a host whose default `go` carries a since-fixed stdlib CVE but a fixed patch is installed/available, `generate --validate` uses the fixed toolchain and passes. Negative — a real first-party (non-stdlib, non-fixed) vulnerability still fails the gate.
- **Evidence:** Session — repeated `generate --validate` failures citing `crypto/tls@go1.26.4`; `go: downloading go1.26.4` despite `GOTOOLCHAIN=go1.26.5`; clean govulncheck under go1.26.5.
- **Related prior retros:** None (first retro in this library).
- **Case-against (Step G) and why it fails:** Case-against — "the user's Go was a patch behind; works as designed, tell them to update." It fails because the host *did* have the fixed toolchain available and selected, and the gate overrode that with an older pinned toolchain — the machine ignored the fix that was already present.

### 2. No request-mutator / signing hook in the generated client; per-request signed-auth schemes require editing the DO-NOT-EDIT client (Template gap)
- **What happened:** DNS Made Easy signs every request with HMAC-SHA1 (`x-dnsme-apiKey` / `x-dnsme-requestDate` / `x-dnsme-hmac`). The generator has no signing auth mode (only `api_key | bearer_token | oauth2 | oauth2_refresh | none`) and no hook to install a request mutator that covers generated endpoint commands. I hand-built a signing `http.RoundTripper` in a separate file, but wiring it required a one-line edit inside the generated, DO-NOT-EDIT `client.New`.
- **Scorer correct?** N/A (not a score-penalty finding).
- **Root cause:** `internal/generator/` emits a client with `CheckRedirect` logic that *already anticipates* nonce-bound signed schemes but no mechanism to implement them, and no registration point for a per-request signer.
- **Cross-API check:** Generalizes to every per-request-signed API. The machine's own code names the class: `internal/client/client.go:119-125` — "nonce-bound schemes (OAuth 1.0a PLAINTEXT, SigV4, Hawk)". Named APIs with evidence: DNS Made Easy (`x-dnsme-hmac`, this run), AWS SigV4 (any AWS API; documented signing), and OAuth 1.0a/Hawk (named in the client's own comment). All require a fresh per-request signature the current auth modes cannot express.
- **Frequency:** subclass:signed-request-auth (HMAC/SigV4/OAuth1.0a/Hawk).
- **Fallback if the Printing Press doesn't fix it:** Agent hand-writes a signer AND edits generated `client.New` to wire it — a generated-file edit that is fragile across regen and easy to get subtly wrong (the official DME SDK itself computes the HMAC over a different timestamp than the header, a latent bug this run avoided).
- **Worth a Printing Press fix?** Yes — not by implementing every signing algorithm, but by providing the hook so hand-written auth code registers cleanly and covers all endpoint commands without editing generated code.
- **Inherent or fixable:** The signing algorithm is per-provider (inherent, belongs in hand code or a future auth mode); the *hook* is fixable and generalizable.
- **Durable fix:** Emit a request-mutator registration point in the generated client — e.g., `client.New` installs an optional package-level `RequestSigner func(*http.Request) error` (or a registered `http.RoundTripper`) that a separate hand-authored file sets, so signing applies to every request with no edit to generated code. Optionally add an `auth.type: signed` / `hmac` mode that wires a provided signer. Keep the hook algorithm-agnostic.
- **Test:** Positive — a spec/hand-file that registers a signer produces signed requests on both a generated endpoint command and a novel command, with no diff to generated `client.go`. Negative — APIs with static `api_key`/`bearer` auth emit no signer scaffolding and are unchanged.
- **Evidence:** Session — signing transport built in `internal/client/dnsmadeeasy_signing.go`; required editing generated `client.New`; client.go:119-125 names the anticipated-but-unimplemented schemes.
- **Related prior retros:** None.
- **Case-against (Step G) and why it fails:** Case-against — "signed auth is custom per-provider; a hand-built signer is expected SKILL-recipe work, not a generator feature." It fails because the *hook* (not the algorithm) is the machine gap: without it, even correct hand-written auth must edit a DO-NOT-EDIT file to cover generated endpoint commands, and the machine already advertises support for these schemes in its redirect handling.

### 3. Hierarchical child resources are silently skipped by sync and their emitted tables have no parent-id column, so cross-parent features are impossible without a fully custom mirror (Template gap)
- **What happened:** Records are hierarchical (`/dns/managed/{domainId}/records`). The generator emitted a `records` table and `records list` command, but the framework `sync` **skips** records (unresolved `{domainId}` placeholder) and the `records` table has **no zone/parent column**. Every cross-zone transcendence feature (where-used, drift, health, export) therefore required a hand-built custom sync (`sync-records`) plus custom store tables (`zone_records`, `record_snapshots`) with a parent-id column.
- **Scorer correct?** N/A.
- **Root cause:** `internal/cli/sync.go` skips resources whose path retains a `{key}` placeholder (sync.go:27-33), and the generator's typed child-resource tables omit a parent-id column — so the machine emits a table it can never populate and provides no parent tagging for the records it does fetch one-parent-at-a-time.
- **Cross-API check:** Generalizes to every parent/child API. The machine's own comment names them: sync.go:28-29 — "Hierarchical APIs (Yahoo Fantasy, Reddit pre-2024, YouTube Data v3, MLB Stats, etc.)". Named with evidence: GitHub (`/repos/{owner}/{repo}/issues`), Asana (`/projects/{id}/tasks`), DNS Made Easy (`/dns/managed/{id}/records`, this run). All emit a child table that flat-sync leaves empty.
- **Frequency:** subclass:hierarchical-child-resources (any spec with `/parents/{id}/children` list endpoints).
- **Fallback if the Printing Press doesn't fix it:** Agent notices the empty table only if they inspect it, then hand-writes a parent-iterating sync + custom tables + parent tagging — a substantial lift repeated per hierarchical API. Easy to miss that the emitted table never fills.
- **Worth a Printing Press fix?** Yes — at minimum stop emitting a table the machine cannot fill, or raise the floor by iterating known parents.
- **Inherent or fixable:** Fixable in two independent increments (either helps): (a) emit a `parent_id` column on child-resource tables and populate it whenever child records are fetched; (b) when a child-resource list path's only unresolved placeholder is a parent id of an already-synced parent resource, iterate the synced parents and populate the child table tagged with parent id (rate-limit-aware, bounded).
- **Test:** Positive — for a spec with `/parents/{id}/children`, `sync` populates the `children` table with a `parent_id` column filled from the parent iteration; a cross-parent query returns rows spanning parents. Negative — flat (non-hierarchical) resources are unchanged and gain no spurious parent column.
- **Evidence:** Session — framework sync skipped `records`; hand-built `sync-records` + `zone_records`/`record_snapshots`; sync.go:27-33; generated `records` table lacks any parent/zone column.
- **Related prior retros:** None.
- **Case-against (Step G) and why it fails:** Case-against — "cross-parent mirroring is custom work; a generic parent-iterating sync risks rate limits / wrong parents." It fails because the machine *already emits the child table* (signaling intent to store it) then silently never fills it; increment (a) alone (a parent-id column + tagging the records it does fetch) is safe, cheap, and unblocks the custom feature layer without any parent-iteration risk.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Go quality gate false-fails on stdlib-only CVE fixed in an available newer toolchain | generator | every API (affected hosts) | low (opaque, easy to misdiagnose) | medium | Only downgrade to WARN when the vuln is stdlib-only AND a fixed toolchain is available; real first-party vulns still FAIL |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | Request-mutator/signing hook in generated client | generator | subclass:signed-request-auth | low (fragile generated-file edit) | medium | No scaffolding for static api_key/bearer APIs |
| F3 | Hierarchical child-resource sync + parent-id column | generator | subclass:hierarchical-child-resources | medium (empty table easily missed) | medium (a: small, b: medium) | Flat resources gain no parent column; parent-iteration bounded + rate-limit-aware |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| F4 | `.preserve-*` snapshot strands `generate --force` retry after a post-emission gate (validate) failure on a first no-hand-edit run | Step G: case-against roughly even — coupled to F1 (only reached because validate failed), and the preserve mechanism is a deliberate data-safety feature; default direction is don't-file. Revisit only if it recurs decoupled from F1. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| MCP Tool Design 5/10 | Flat 42-endpoint MCP surface scored low; maps to `mcp.intents` spec enhancement | printed-CLI — SKILL already instructs "ask the user" for 30–50 endpoint surfaces |
| Cache Freshness 5/10 | Generator emitted no freshness helper for this read-through API shape | unproven-one-off — no cross-API evidence gathered; this-API scorecard dim |
| Two-credential api_key auth | DME needs API key + secret; modeled as `api_key` with two `env_vars` | iteration-noise — the existing Stytch two-env-var pattern handled it with zero friction |

## Work Units

### WU-1: Go quality gates must not false-fail on stdlib-only CVEs fixed in an available toolchain (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** `generate --validate` and `verify`'s govulncheck use the newest available fixed toolchain (or downgrade a stdlib-only, already-fixed finding to an actionable WARN) instead of hard-blocking on the press's pinned build toolchain.
- **Target:** The Go quality-gate invocation in the generate `--validate` path and the `verify` leg (govulncheck subprocess `GOTOOLCHAIN`/`GOFLAGS` handling).
- **Acceptance criteria:**
  - positive: on a host with a since-fixed stdlib CVE in the default `go` but a fixed patch installed, `generate --validate` selects the fixed toolchain and passes.
  - negative: a real first-party (module/dependency) vulnerability still fails the gate.
- **Scope boundary:** Does not change how non-Go gates run; does not suppress non-stdlib vulns.
- **Dependencies:** None.
- **Complexity:** medium

### WU-2: Algorithm-agnostic request-signer hook in the generated client (from F2)
- **Priority:** P2
- **Component:** generator
- **Goal:** Hand-written auth code can register a per-request signer/RoundTripper that covers all generated endpoint commands without editing the generated client.
- **Target:** Generated `internal/client/client.go` (`New`) — emit an optional registration point; optionally an `auth.type: signed`/`hmac` mode that wires a provided signer.
- **Acceptance criteria:**
  - positive: registering a signer in a separate file signs both a generated endpoint command and a novel command, with no diff to generated `client.go`.
  - negative: static `api_key`/`bearer` APIs emit no signer scaffolding and are byte-unchanged.
- **Scope boundary:** Does not implement SigV4/HMAC/OAuth1.0a algorithms; provides only the hook.
- **Dependencies:** None.
- **Complexity:** medium

### WU-3: Hierarchical child-resource support in sync + parent-id column (from F3)
- **Priority:** P2
- **Component:** generator
- **Goal:** Child resources of already-synced parents get mirrored (or at minimum carry a `parent_id` column filled when fetched), so cross-parent features don't require a fully custom mirror.
- **Target:** `internal/cli/sync.go` (parent-iteration for resolvable `{parentId}` placeholders) and the generator's typed child-resource table emission (parent-id column).
- **Acceptance criteria:**
  - positive: a spec with `/parents/{id}/children` populates the `children` table with a filled `parent_id` and a cross-parent query returns rows spanning parents.
  - negative: flat resources gain no parent column and are unchanged.
- **Scope boundary:** Increment (a) parent-id column is the minimum; (b) parent-iteration sync must be bounded and rate-limit-aware. No API-specific parent names.
- **Dependencies:** None.
- **Complexity:** medium (a: small, b: medium)

## Anti-patterns
- Emitting a typed resource table (`records`) that the framework sync can never populate, with no signal to the agent that the table will stay empty — silent dead scaffolding.
- Advertising support for signed schemes in redirect handling (client.go:119-125) without any mechanism to actually produce a signature.

## What the Printing Press Got Right
- The `api_key` + two-`env_vars` pattern cleanly modeled DME's API-key-plus-secret credential pair with zero custom config code.
- Novel-command scaffolds (drift/export/health/bulk-apply/acme-purge/where-used + tests) were pre-generated from `research.json`, giving a correct Cobra + dry-run + MCP-annotation skeleton to fill in.
- Polish caught a real defect the generation pass missed (orphaned `auth set-token` never wired into the `auth` group) and lifted MCP Desc Quality 7→10.
- Dogfood's `novel_features_check` deterministically confirmed 6/6 planned transcendence commands shipped.
- `verify-skill` + `validate-narrative` kept the SKILL/README honest against the real binary (all example commands resolve).
