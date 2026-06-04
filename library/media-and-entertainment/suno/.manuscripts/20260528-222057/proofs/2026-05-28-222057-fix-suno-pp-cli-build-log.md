# Suno CLI — Phase 3 Build Log

Manifest transcendence rows: 6 planned, 6 built. Phase 3 Completion Gate PASS (per-row Cobra resolution + dogfood planned 6 = found 6).

(Transcendence rows, all `hand-code`, no stubs: grep, analytics, lineage, top, sql, credits --forecast.)

## Generator output (Phase 2)
- Spec: 5 resources, 18 typed endpoints. All gates passed (go mod tidy, govulncheck, go vet, go build, runnable binary, --help, version, doctor). MCP bundle built.
- Generated commands: api (clips list/info/concat/stems/set-metadata/set-visibility/aligned-lyrics/convert-wav/wav-url, lyrics create/get, workspace list/get/create/rename/trash), promoted billing + personas, framework (sync, search, import, which, doctor, workflow, profile, agent-context, MCP), auth stub.
- NOT emitted by generator (full hand-code): sql, analytics, tail. Generic syncer is GET-only (Suno list is POST /api/feed/v3 opaque-cursor) → sync hand-coded.
- Auth: config reads SUNO_JWT (bearer_token). Client exposes Get/Post + *WithHeaders variants for dynamic Device-Id/Browser-Token.

## Phase 3 plan (hand-code)
P0 foundation (agent 1, in progress): Clerk `auth login --chrome` (cookie→session→JWT+refresh via kooky), dynamic header injection (Device-Id + Browser-Token), Suno cursor `sync` into SQLite.
P1 absorb: generate/describe/extend/cover(+title)/remaster, download (mp3+wav convert/poll + ID3 lyric embed), trash, status (poll), models, workspace add/remove --clip.
P2 transcend: grep, analytics, lineage, top, sql, credits --forecast.

## Intentionally deferred / boundaries
- Generation hCaptcha: ships as replayable HTTP taking --token/--no-captcha (no resident browser solver, per Printing Press rule). Documented in README/SKILL.
