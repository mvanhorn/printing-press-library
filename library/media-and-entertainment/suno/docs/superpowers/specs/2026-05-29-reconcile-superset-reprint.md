# Reconcile-to-Superset Reprint — execution spec (2026-05-29)

Supersedes the "NEXT TASK" section of `HANDOFF-2026-05-29-suno-reconcile.md` with
verified ground truth and approved decisions. Read this first when resuming.

## Verified state (post-upgrade)

- **Printing Press upgraded 4.14.0 → 4.19.0** (binary `~/go/bin/cli-printing-press`, May 29 14:57).
- **Public upstream is still 4.6.1** (`mvanhorn`, run `20260514-184203`, gen 2026-05-15).
  Last touched 2026-05-28 by housekeeping only (attribution sweep, dep bump, gofmt) —
  **not** a republish. The reconcile-to-superset premise holds.
- **Local (this repo)** = 4.14.0, run `20260528-222057`, branch `feat/restore-dropped-commands`, clean.
- **Surface is large**: ~38 top-level commands / 77 `internal/cli` files. `spec.yaml` declares
  only **18 spec-driven endpoints**; the generation family (generate/cover/remaster/extend/
  describe) and all novel features are **hand-authored, not in the spec**. Regen-merge
  (printing-press Phase 5.6) preserves these on reprint.
- Patches: **0** queued. Prior research: manuscript `20260528-222057/research.json` (~1 day old).

## Authoritative endpoint source

**paperfoot/suno-cli** `API_INTELLIGENCE.md` (reverse-engineered Suno v5.5, captured 2026-04-06)
is the catalog of "every available Suno feature." Staged locally at
`docs/superpowers/specs/paperfoot-API_INTELLIGENCE.md`. Repo: https://github.com/paperfoot/suno-cli

## Approved decisions (do not re-litigate)

1. **Scope = maximal superset.** Add all of:
   - **Voice persona creation** (5 endpoints): `POST /api/uploads/audio/{id}/upload-finish/`,
     `GET /api/uploads/audio/{id}/`, `POST /api/processed_clip/voice-vox-stem`,
     `POST /api/voice-verification/`, `POST /api/persona/create/` (47KB base64-audio payload).
     **Caveat:** needs a user voice sample + recorded verification phrase; **excluded from the
     live dogfood gate** (would create real personas). Ships best-effort with unit tests.
   - **Playlists** `GET /api/playlist/me` · **Trending** `GET /api/trending/`
   - **Old-4.6.1 read endpoints** (~17 GETs): clips attribution/comments/parent/similar/
     direct-children-count, billing eligible-discounts/usage-plan/usage-plan-faq, persona list
     (`/api/persona/me`), project default/pinned-clips, user config/personalization/
     personalization-memory, notification list/badge-count.
   - **Old-name renames** (mine → old/paperfoot): `clips set-metadata`→`set`/`edit`,
     `clips set-visibility`→`publish`, `trash`→`clips delete` (keep working `/api/feed/trash`),
     `clips aligned-lyrics`→`timed-lyrics`, `personas`→`persona` group, `workspace`→`project`
     (keep CRUD, add old read views). `custom-model` already matches.
2. **Build path = full 4.19.0 reprint**, paperfoot intelligence folded into the research input
   so regeneration natively covers playlists/trending/persona-create. Via
   `/printing-press-reprint suno` → `/printing-press suno`.
3. **Auth = `SUNO_TOKEN` canonical** (match published 4.6.1 + paperfoot) **with `SUNO_JWT`
   accepted as an alias** (keeps existing live-gate scripts working).
4. **`generate` = group, old shape**: `create` (captcha-aware main gen) + `concat` + `lyrics`
   + `lyrics-status` + `video-status`, with `extend`/`cover`/`remaster`/`describe`/`evolve`
   under it. Never published, so the `generate` → `generate create` move has zero external friction.

## Build / verify / publish sequence

1. Drive `/printing-press-reprint suno` → hands to `/printing-press` with the handoff below.
2. `/printing-press` regenerates fresh in a work dir; Phase 5.6 `regen-merge` carries the
   60+ hand-authored files into the library. Re-run the **full live gate** (auth notes in the
   original handoff) until 0 failures. Persona-create excluded from live gate.
3. Mirror library staging → git repo, commit on `feat/restore-dropped-commands`.
4. `/printing-press-publish suno --from-polish` — replaces public 4.6.1. **Separate explicit
   gate; PII re-scan before any PR.**

## `/printing-press` handoff prompt (assembled)

> Regenerate the suno CLI. The user has already chosen to regenerate — Phase 0's library check
> should select "Generate a fresh CLI" and not re-prompt fresh-vs-improve.
>
> **Research mode:** redo, seeded with paperfoot reverse-engineering intelligence
> (`docs/superpowers/specs/paperfoot-API_INTELLIGENCE.md`, repo
> https://github.com/paperfoot/suno-cli). Treat it as the authoritative Suno v5.5 endpoint catalog.
>
> **## User Vision**
> Build a true superset of the published 4.6.1 suno CLI and of paperfoot's reverse-engineered
> v5.5 surface, under old/paperfoot command names for zero friction. Add voice persona creation
> (5-endpoint clone flow), playlists, trending, and the old-4.6.1 read endpoints. Preserve every
> existing novel feature (grep/analytics/lineage/top/sql/credits, vibes/burn/budget/sessions/
> ship/tail/tree/custom-model, the captcha-aware A/B generate flow, wav/stems/workspace).
>
> **## Reprint Spec Enrichment**
> - **Auth (auth_protocol):** canonical env var `SUNO_TOKEN`, accept `SUNO_JWT` as alias.
>   Auth stays `bearer_token` Clerk-minted JWT. (printing-press Phase 2 Pre-Generation Auth Enrichment.)
> - **`generate` group, old shape:** create/concat/lyrics/lyrics-status/video-status + extend/
>   cover/remaster/describe/evolve.
> - **New endpoints:** persona-create flow, playlists, trending, ~17 old-4.6.1 reads, per the
>   catalog above. Persona-create excluded from live verification.
> - **Renames** per decision 1 above.
> - **MCP surface:** lift weak MCP dimensions via Phase 2 Pre-Generation MCP Enrichment (intents
>   for multi-step flows).
>
> **## Prior Patches** — none (0 queued).
