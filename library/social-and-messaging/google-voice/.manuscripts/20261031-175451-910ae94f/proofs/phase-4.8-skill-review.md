# Phase 4.8 — Agentic SKILL review

Final result: **PASS — no findings.** All five initial findings (four errors, one warning) were resolved and independently rechecked against the rebuilt executable's help and implementation source. No auth, HAR, session data, message database, or external service was opened. Only this proof was written by the reviewer.

Target: `working/google-voice-pp-cli/SKILL.md`; binary: `build/stage/bin/google-voice-pp-cli`.

The private-build notice at SKILL.md:21 adequately makes the required canonical public-install block conditional on a future published release; it is not a finding.

## Final recheck

- Doctor: `doctor --help` now advertises `--fail-on none|warn|error`; `internal/cli/voice_core.go:267–315` implements local setup warnings and explicit live-auth deferral to `auth status --live`. The documented invocation is supported.
- Credentials: SKILL.md:233 explicitly limits `credentials.toml` to framework credentials and identifies Voice's separate private `session.json`; SKILL.md:220 preserves the exact Voice paths.
- Envelope: SKILL.md:202,211 now scopes `{meta,results}` parsing to `--agent` and distinguishes direct `--json` output.
- No-local-data: `internal/cli/voice_core.go:22` registers `mirror sync` with the pre-run write guard in `internal/cli/root.go:419,995–998`; SKILL.md:255 accurately describes the block. The source regression test exercises this pre-I/O rejection.
- Purge: SKILL.md:254 and rebuilt `local-data purge --help` explicitly retain Voice profile mirrors, custom database paths and credentials. Runtime help customization is at `internal/cli/voice_core.go:33–38`.

The two focused source tests in `internal/cli/voice_core_test.go` cover pre-I/O collection blocking and one-document doctor warning output. The reviewer inspected their source; did not execute stateful commands or tests.

## Initial findings — resolved

These describe the initial review snapshot, not the current implementation.

1. **Auth narrative accuracy — error — SKILL.md:234.** `doctor --fail-on warn` is unsupported; the custom doctor only reports local configuration. **Fix:** use `doctor --agent` for configuration and `auth status --live --agent` for live authentication verification.
   Evidence: `doctor --help`; `internal/cli/voice_core.go:251–271`.

2. **Auth narrative accuracy — error — SKILL.md:233.** The unconditional `credentials.toml` claim conflicts with Voice's config-directory `voice-profiles/<profile>/session.json`. **Fix:** distinguish framework credentials from Voice session storage and remove the inapplicable migration claim.
   Evidence: `internal/voice/session.go:58–78,115–137`; SKILL.md:220,232–233.

3. **Recipe/output claims — warning — SKILL.md:202.** Only `--agent` wraps Voice results in `{meta,results}`; `--json` alone emits the direct payload. **Fix:** qualify the unconditional envelope/parsing rule and document the non-agent JSON shape separately.
   Evidence: SKILL.md:202–211; `internal/cli/helpers.go:1773–1781,2410–2444`.

4. **Limits / known-gap disclosure — error — SKILL.md:252.** `GOOGLE_VOICE_NO_LOCAL_DATA=1` does not block `mirror sync`: the guard matches only `sync`, while Voice collection still writes its database. **Fix:** extend the guard to `mirror sync` before retaining the guarantee, or explicitly disclose this limitation.
   Evidence: `internal/cli/root.go:975–998`; `internal/cli/voice_core.go:273–315`.

5. **Limits / known-gap disclosure — error — SKILL.md:252.** `local-data purge` scans top-level `data*.db` files, excluding the default nested Voice `messages.sqlite3` mirror. **Fix:** disclose its framework-only scope prominently, or safely include Voice mirrors before claiming local-store erasure.
   Evidence: `internal/cli/local_data.go:27–29,125–137,170–189`; `internal/voice/session.go:71–72`.

## Seven-check result

- Trigger phrases: PASS; read, sync, local search and coverage are implemented.
- Verified set: PASS; Unique Capabilities exactly matches the five `novel_features_built` commands in run-root `research.json`.
- Novel descriptions: PASS against all five command help outputs and implementations.
- Stub/gated disclosure: PASS; no novel stubs, and private import/local-sync prerequisites, bounded-history limits, purge exclusions and no-local-data behavior are accurately described.
- Auth narrative: PASS; signed session, HAR import, SAPISIDHASH, browser-free reads, Voice credential paths and local doctor/remote-auth distinction match the implementation.
- Recipes: PASS; all four worked recipes match their intent and output with `--agent`, and the broader envelope rule is now qualified.
- Marketing copy: PASS; concrete descriptions, no ad-copy findings.

Read sources: SOUL.md, ledger Google Voice entry (not treated as current build evidence), local AGENTS.md, phase-14 seven-check contract, SKILL.md, README.md, absorb manifest, research.json, relevant implementation source, root/help and all five novel command help outputs plus auth/import/status/doctor/mirror-sync/account/search help. Final recheck repeated all five novel command helps, auth import/status help, doctor help, purge help, verified-set comparison, SKILL reading and source checks for each fix.

Separate documentation handoff to the parent: README.md:191,195,199–200 still contains generic credentials/cache/purge prose. That is for the following README correctness audit; it does not change this SKILL-specific PASS.

