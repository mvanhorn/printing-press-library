# Artistly CLI — Shipcheck Report

## Shipcheck umbrella: PASS (6/6 legs)
- verify PASS, validate-narrative PASS (7/7), dogfood PASS, workflow-verify PASS, verify-skill PASS, scorecard PASS.
- Scorecard: 80/100 — Grade A. Strong: output modes, auth, error handling, README, doctor, agent-native, MCP quality, local cache, workflows (all 9-10). Weaker: cache freshness 5/10, insight 4/10, auth_protocol 2/10 (cookie auth is non-standard), type fidelity 2/5.

## Fixes applied this session
1. Implemented all 7 transcendence features + generate flow (Inertia POST + CSRF + poll + download) + cookie/CSRF transport layer.
2. Built designs list/by-folder/download/delete/move, folders CRUD, styles list (all verified contracts from HAR + props).
3. Phase 4.95 code review (subagent): fixed HIGH cache-staleness bug (poll used 5-min cached GET → never saw completion; switched to GetNoCache), batch ctx-aware pacing, dead-code removal. Path-traversal checked SAFE.
4. Phase 4.9: corrected README/SKILL "read-only" misclassification (generator inferred read-only from GET-only spec; CLI actually mutates). 

## Known Gaps (ship-with-gaps)
- `prompt enhance` / `prompt extract`: NOT built. Their result is delivered via an Inertia flash whose response body was empty in the capture, so the result contract could not be determined. Deferred to a follow-up amend with a capture that preserves the enhance XHR response body.
- Image-to-image edit tools (upscale/bg-remove/inpaint/outpaint): out of v1 (unverified upload payload). Reachable conceptually via `generate --tool <feature>`.
- **Live behavioral verification pending**: no Chrome cookie-extraction tool was installed during the run, so `auth login --chrome` and live dogfood of authenticated commands were not exercised. Local-only commands (preset save/use/list/remove) verified working. Authed commands are structurally verified (build, vet, tests, shipcheck, dry-run, clean auth-error handling) against the HAR-proven contracts.

## Generator retro candidates
- Internal-YAML auth.type doc enum omits `cookie`/`composed` though the generator accepts `cookie`.
- A spec with only GET endpoints + hand-built mutating commands causes the generator to mislabel the CLI "read-only" in README/SKILL/agent_context. Generator should not assert read-only when novel commands carry mcp:read-only=false.

## Verdict: ship-with-gaps
Promotable. Headline generate + 7 transcendence features + organize/read surface built and verified against captured contracts. Gaps documented above; live smoke pending auth-tool setup (see Phase 5).
