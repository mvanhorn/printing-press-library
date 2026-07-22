# Printing Press Retro: GL.iNet (gl-inet)

## Session Stats
- API: gl-inet (GL.iNet travel-router local API: JSON-RPC over /rpc + OpenWrt/UCI over SSH)
- Spec source: internal (authored from research + live device validation; no machine spec)
- Scorecard: 94/100 (A)
- Verify pass rate: 100%
- Fix loops: ~2 (shipcheck), plus iterative live validation
- Manual code edits: high (entire transport/auth/SSH + 14 novel commands hand-built — expected for this exotic target)
- Features built from scratch: 14 novel commands + 2 transport packages (glrpc, glssh)

## Findings

### 1. Publish skill stages all skill mirrors as line-ending churn on Windows (Skill instruction gap)
- **What happened:** Step 6 of the publish skill runs `go run ./tools/generate-skills/main.go` (regenerates the flat `cli-skills/` mirror) then `git add library/ cli-skills/`. On Windows, the generator writes files git sees as modified (LF→CRLF / trailing-newline), so `git add cli-skills/` staged ~290 *unrelated* skill mirrors — the first commit was 428 files instead of 138. Had to reset, `git checkout -- cli-skills/`, and re-stage only `library/<cat>/<slug>` + `cli-skills/pp-<slug>`.
- **Scorer correct?** N/A (not a scorer finding).
- **Root cause:** `skills/printing-press-publish/SKILL.md` Step 8 "Commit and push" instructs `git add library/ cli-skills/`. That broad add captures every mirror the regeneration touched, not just the published CLI's. The regeneration is required for parity, but only the new/changed CLI's mirror is a real change; the rest are line-ending noise on any platform where the generator's output line endings differ from the checked-out tree (Windows is the common case).
- **Cross-API check:** Recurs on **every publish run on Windows**, independent of API. Not API-shape-specific — it's a platform×skill-instruction interaction. (3-API evidence bar is N/A because the trigger is the platform + the publish flow, not the API; any CLI published from Windows hits it.)
- **Frequency:** every publish on Windows (and any platform where generate-skills output line endings differ from the working tree).
- **Fallback if the Printing Press doesn't fix it:** The agent must notice 290 spurious staged files and manually scope the commit every publish. Easy to miss → noisy PRs touching every other CLI's mirror, which fail/annoy library CI and review.
- **Worth a Printing Press fix?** Yes. Cheap, durable, removes a recurring cross-platform footgun.
- **Inherent or fixable:** Fixable. Scope the stage to the published CLI only.
- **Durable fix:** In `skills/printing-press-publish/SKILL.md` Step 8, change `git add library/ cli-skills/` to scope the add to the published CLI's paths: `git add library/<category>/<api-slug> cli-skills/pp-<api-slug>`. Parity is still satisfied (the new mirror is staged); unrelated mirrors that only differ by line endings are not committed. Optionally also note: regenerate with `core.autocrlf=false` or have `tools/generate-skills` emit LF, but the scoped `git add` is the platform-agnostic fix and lands in the skill (the generate-skills tool lives in the library repo, out of this repo's scope).
- **Test:** Positive — on Windows, publish a CLI and assert `git show --stat HEAD` lists only `library/<cat>/<slug>/*` and `cli-skills/pp-<slug>/*`. Negative — on Linux (no line-ending delta), the scoped add still commits the same files (no regression).
- **Evidence:** This session — first publish commit was "428 files changed"; ~290 were `cli-skills/pp-*/SKILL.md` with a single `1 +` line-ending diff each.
- **Related prior retros:** None found (no matching keyword hits in prior retro proofs).
- **Step G case-against, and why it fails:** Case-against: "Windows users should set `core.autocrlf=input`; this is a local git-config issue, not a skill bug." Why it fails: the skill should be robust cross-platform without depending on each user's global git config, and the broad `git add` is wrong regardless of line endings (it would also capture any other in-flight mirror change) — scoping the add is strictly better and costs nothing.

## Prioritized Improvements

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | Scope publish `git add` to the published CLI's paths | skill | every Windows publish | Low (easy to miss 290 staged files) | small | none needed |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| govulncheck timing | Dependency vuln (pion/dtls via pion/stun) surfaced only at publish, not shipcheck/polish | Step B: single-occurrence process gap, can't name 3 APIs with evidence. Step G: govulncheck is intentionally a publish-time gate; adding it to every shipcheck slows the common no-new-deps path. Reasonable case-against. The agent caught and fixed it (stdlib STUN) within the run. |
| JSON-RPC `call`-envelope template | Generator has no template for JSON-RPC challenge-auth APIs; whole transport hand-built | Step B: can't name 3 catalog APIs with JSON-RPC + challenge/crypt auth. Exotic target; expected hand-build. session_handshake already covers the closest stock case. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| sha256-vs-md5 auth drift | Read `hash-method` from challenge to pick final digest | printed-CLI (GL-specific auth) |
| `wg-client` hyphenation | VPN modules hyphenated, underscore 404s | API-quirk |
| `/ubus` not exposed | nginx 302s ubus; used SSH for UCI | API-quirk |
| version-drifted method names | vpn-policy/macclone/get_status renamed on 4.8.1 | API-quirk |
| vpn toggle dry-run read | Read short-circuited by global DryRun | iteration-noise (own bug, fixed) |
| statusLooksConnected substring | "disconnected" matched "connect" | iteration-noise (own bug, fixed) |
| snapshot delete missing | Store had DeleteSnapshot, no command | printed-CLI |
| narrative --full-examples needs `home` snapshot | Stateful local-store example failed after I deleted the snapshot | printed-CLI (my narrative referenced a user-state entity) |

## Work Units

### WU-1: Scope the publish commit to the published CLI's paths
- **Priority:** P2
- **Component:** skill
- **Goal:** Publish PRs contain only the published CLI's files, never line-ending churn across all other skill mirrors.
- **Target:** `skills/printing-press-publish/SKILL.md`, Step 8 "Commit and push".
- **Acceptance criteria:**
  - positive: after a Windows publish, `git show --stat HEAD` lists only `library/<category>/<api-slug>/**` and `cli-skills/pp-<api-slug>/**`.
  - negative: on Linux, the same scoped add commits the identical file set (no regression; parity still satisfied because the new mirror is staged).
- **Scope boundary:** Only the `git add` line; do not change the mirror-regeneration step itself (parity regeneration must still run). The generate-skills line-ending behavior lives in the library repo and is out of scope here.
- **Dependencies:** none.
- **Complexity:** small.

## Anti-patterns
- None observed in the run worth machine action beyond WU-1.

## What the Printing Press Got Right
- **Novel-feature stubs from research.json were a major head start.** The generator emitted `newNovel<Name>Cmd` stubs (with correct `Use`/`Short`/`Example`/annotations) for all 13 declared novel features, plus promoted endpoint commands and `_test.go` scaffolds. Phase 3 was "fill RunE bodies," not "write 14 command files from scratch."
- **The framework carried the exotic target.** SQLite store + migrations, `cliutil` (output/select/flags/verify-env), MCP cobratree mirror, search/SQL/sync, README/SKILL skeletons, doctor scaffold, agent-native output — all reused unchanged even though the transport was fully custom (JSON-RPC + SSH).
- **dogfood synced README/SKILL/root-help/MCP from `novel_features_built`** correctly after I added `vpn verify`, and `validate-narrative --full-examples` caught a genuinely broken example (a deleted snapshot) — the scorer did its job.
- **Polish lifted MCP description quality 0→10 and removed dead code** without touching the device.
- **publish validate's govulncheck blocked a real transitive vuln** (pion/dtls) before it shipped — exactly the publish gate working as intended.
