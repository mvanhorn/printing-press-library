Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

# Artistly CLI — Build Log

## Discovery basis
- No public API. Laravel + Inertia.js app, session-cookie + CSRF auth. Contracts captured from: authenticated browser-sniff (Ziggy route registry, read endpoints), JS chunk extraction (generate payload fields), and a user-provided HAR of one real generation (definitive generate→poll→download sequence).
- Generate contract (verified from HAR): `POST /ai/{feature}/store` (feature `image-designer-v6`), JSON body {prompt, negative_prompt, checkpointId, width, height, aspect_ratio, quantity, seed, quality, tool_used, category, ...}, headers X-XSRF-TOKEN + X-Requested-With, returns 302 (queued); design appears in `/fetch-personal-designs` with status processing→private and real CDN images.

## Built & verified (compiles, vet clean, tests pass, dry-run exit 0)
### Transcendence (all 7, hand-code)
- `generate` — POST generate + --wait (poll) + --download; --preset, --tool, --style, --aspect-ratio, --quality, --quantity, --seed, etc.
- `batch <file>` — sequential quota-paced generation from a prompt file; --wait/--download; per-prompt failure accounting.
- `redo <id>` — re-run a past design's settings (live fetch + resubmit); --seed/--quantity/--prompt-append overrides.
- `export` — bulk-download images of designs matching --query/--folder/--since to --to, templated filenames; no quota.
- `preset save/use/list/remove` — local settings presets (verified roundtrip, no auth).
- `quota [--for <file>]` — read generation budget from Inertia props; batch-fit preflight.
- `search` — framework FTS over synced designs (offline).

### Absorbed (verified contracts)
- `designs list` (GET /fetch-personal-designs), `designs by-folder` (GET /designs-by-folder) — generator-emitted, verified 200.
- `designs download <id|uuid>` — hand-built; downloads CDN images.
- `styles list [--match]` — hand-built; reads illustratorStyles from Inertia props.
- `sync` (designs), `doctor`, `auth login --chrome` — framework/cookie-auth.

### Transport layer (hand-authored, internal/cli/artistly.go)
- CSRF header construction (XSRF-TOKEN cookie → X-XSRF-TOKEN), design fetch/parse, generate submit (302-tolerant), poll-until-rendered, CDN download, Inertia shared-props extraction (quota + styles).

## Deferred (NOT built) — unverified request contracts
These were in the Phase 1.5 absorbed list but their **request-body shapes were not captured** in the HAR (the user exercised only the generate flow), and several are mutating/destructive. Shipping guessed bodies for destructive ops is irresponsible:
- `designs delete` (POST /delete-designs) — destructive; body shape unknown.
- `designs move` (POST /move-designs, /change-folder) — body shape unknown.
- `folders` create/rename/remove (POST/PUT/DELETE /designs/folder) — body shape unknown.
- `prompt enhance` (POST /prompt/enhance), `prompt extract` (POST /prompt/extract) — body shape unknown.

Resolution options recorded for the user (scope decision): ship the verified set now and add these via a follow-up amend with a targeted HAR, OR capture another HAR exercising delete/move/folder/prompt now. The 7 approved transcendence features (the differentiators) are all built.

## Generator limitations found
- Internal-YAML `auth.type` doc enum omits `cookie`/`composed`, but the generator DOES accept `auth.type: cookie` (emitted working `auth login --chrome`). Doc gap — retro candidate.
- No cookie-extraction tool installed (pycookiecheat/cookie-scoop/press-auth), so `auth login --chrome` and live dogfood of authed commands require installing one first.
