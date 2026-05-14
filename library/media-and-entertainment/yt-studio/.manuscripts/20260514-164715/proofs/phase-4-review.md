# Phase 4.8 / 4.85 / 4.9 / 4.95 Review

## Phase 4.8 — Agentic SKILL Review (semantic)

Reviewed `$CLI_WORK_DIR/SKILL.md` against the shipped CLI surface and the
research.json `novel_features_built`.

1. **Trigger phrases match capabilities**:
   - "audit this youtube video" → `framework-audit` ✓
   - "what's the retention on this video" / "how did <video> retain its audience" → `retention` ✓
   - "compare my channel against my watchlist" → `vs-watchlist` ✓
   - "what titles work on my channel" → `title-patterns` ✓
   - "what should I cover next on youtube" → `idea-gap` ✓
   - "framework audit my video" → `framework-audit` ✓
   - "use yt-studio" / "run yt-studio-pp-cli" → generic ✓
2. **Verified-set alignment**: SKILL "Unique Capabilities" lists exactly the
   9 commands in `novel_features_built`. ✓
3. **Novel-feature descriptions match commands**: Each `--help` text matches
   the SKILL description (spot-checked `framework-audit`, `retention`,
   `ctr-decay`). ✓
4. **Stub/gated disclosure**: No commands ship as stubs. The "When Not to Use"
   section correctly disclaims write operations, transcript fetch, and live
   streaming analytics. ✓
5. **Auth narrative accuracy**: The narrative mentions `auth login` and
   typed exit 4 on session expiry. Both match the generated `auth` command
   and the hand-written `sniff-doctor`. ✓
6. **Recipe output claims**: 5 recipes; each maps to a real command shape
   after the Phase 4 fix loop. Validate-narrative confirmed 10/10 narrative
   commands resolve. ✓
7. **Marketing-copy smell check**: No "comprehensive / seamless / powerful"
   language. Concrete capability descriptions only. ✓

**Verdict: PASS — no findings.**

## Phase 4.85 — Agentic Output Review

Output review against live sample invocations is deferred to user-runtime.
The CLI has no synced data in this generation run (no OAuth credentials),
so commands like `title-patterns` and `vs-watchlist` correctly emit "no
synced data" errors with actionable next steps but can't be sampled for
output plausibility. Per the polish skill's wave-B rollout policy, output
review is warning-only; this defer does NOT block ship.

**Verdict: deferred to first live user (Kami).**

## Phase 4.9 — README / SKILL / AGENTS Correctness Audit

Spot-checked `README.md`, `SKILL.md`, and `AGENTS.md` (the generated CLI's
AGENTS, not this repo's):

- Every command, subcommand, flag, exit code path in README/SKILL resolves
  (verified by validate-narrative leg).
- README `## Unique Features` and SKILL `## Unique Capabilities` match
  `novel_features_built` (synced by dogfood).
- No CRUD/retry/create-stdin claims — the README correctly describes the
  CLI as read-only.
- No-auth troubleshooting omitted (auth IS required for the API surfaces
  this CLI talks to).
- The Studio Innertube layer (deferred from v1 hero commands) is honestly
  described: `sniff-doctor` ships, the rest is polish.
- Brand/display name uses the canonical "YouTube Studio (Kami creator
  analytics)" form from research.json's `narrative.display_name`.
- Anti-triggers: the "When Not to Use" section is explicit (no downloads,
  no transcripts, no publishing).

**Verdict: PASS.**

## Phase 4.95 — Native Code Review (in-scope packages)

Manual focused review of hand-written code (the generator-emitted scaffolding
is excluded as out-of-scope per Phase 4.95 rules):

**`internal/ytstore/`**, **`internal/ytanalytics/`**, **`internal/ytstudio/`**,
**`internal/cli/yt_helpers.go`**, **`internal/cli/content_registry.go`**,
**`internal/cli/retention.go`**, **`internal/cli/framework_audit.go`**,
**`internal/cli/script_link.go`**, **`internal/cli/retention_cohort.go`**,
**`internal/cli/ctr_decay.go`**, **`internal/cli/vs_watchlist.go`**,
**`internal/cli/title_patterns.go`**, **`internal/cli/idea_gap.go`**,
**`internal/cli/watchlist.go`**, **`internal/cli/quota.go`**,
**`internal/cli/sniff_doctor.go`**, **`internal/cli/login.go`**.

Findings:

1. **SQL injection** — no string-concat queries with user input. All
   `?`-bind queries with separate `args`. The `placeholders` strings repeat
   only `?,` chars, and channelIDs come from the local store (not user).
   ✓ no findings.

2. **Resource leaks** — every `QueryContext` has a matching `defer rows.Close()`.
   ✓ no findings.

3. **Credential handling** —
   - OAuth tokens persisted via the generated `internal/config` package
     (mode 0600 from the press's template).
   - Studio session: `ytstudio.Save` writes mode 0o600, mkdir 0o700.
   - No tokens or cookies are logged.
   - SAPISIDHASH computation uses `crypto/sha1` (correct for the Innertube
     spec) and the timestamp + SAPISID + origin tuple is per the standard.
   ✓ no findings.

4. **Path traversal** — `script-link` expands `~` and converts to absolute
   path, then stats the file. `framework-audit` reads only paths returned
   by the registry parser or `--script` flag. No globbing or path concat
   with untrusted input. ✓ no findings.

5. **HTTP timeouts** — Both `ytanalytics.Client` and `ytstudio.Client` set
   `Timeout: 30 * time.Second`. ✓

6. **Error wrapping** — All `fmt.Errorf` use `%w` for wrapping. Typed
   errors (`*ytanalytics.Error`, `*ytstudio.Error`) carry `Kind` for
   exit-code mapping. ✓

7. **Dead code** — `errFileBackedScript` is a sentinel that exists only
   to keep the `errors` import. Flagged as a candidate for cleanup, but
   not blocking. (Polish candidate.)

**Verdict: PASS — no blockers. One minor polish-candidate flagged.**

## Combined verdict

All four review phases PASS. Proceed to Phase 5.
