# HANDOFF — suno-pp-cli reconcile + publish (2026-05-29)

**Read this first when resuming.** Context was cleared mid-task. This doc is self-contained.

## TL;DR of where we are

1. ✅ **DONE & committed:** Restored the 8 transcendence commands the 4.14.0 reprint dropped, fixed the `clips info`→`clips get` gap, fixed all publish-gate findings. The **live publish gate PASSED (139/139)**.
2. ⏸️ **PUBLISH IS ON HOLD.** Nothing has been pushed to the public library. A fork clone exists locally but no branch/PR was created there.
3. 🔜 **NEXT TASK (approved, not started): reconcile to a true superset, then publish.** At publish time we discovered the published upstream `suno` is an *older, differently-shaped* reprint (4.6.1) and my 4.14.0 would **drop ~17 capabilities**. Decision: extend mine into a true superset using the **old command names**, via **`/printing-press-reprint`**, then publish.

---

## The three working trees (important — don't confuse them)

| Tree | Path | What it is | Git? |
|---|---|---|---|
| **Git repo (source of truth)** | `/Users/smacdonald/homegit/suno-cli` | All my work, branch `feat/restore-dropped-commands` | yes |
| **Publish staging** | `/Users/smacdonald/printing-press/library/suno` | What `cli-printing-press`/publish operate on; kept in sync with the repo via rsync | no |
| **Publish fork clone** | `/Users/smacdonald/printing-press/.publish-repo-suno-cli-4f8a726c` | Clone of `horknfbr/printing-press-library` fork; `upstream` = `mvanhorn/printing-press-library`; on `main` synced to upstream (has the merged **OLD 4.6.1** suno) | yes |

**Sync command (repo → staging)** — run after any repo edit before gates/publish:
```bash
SRC=/Users/smacdonald/homegit/suno-cli; DST=/Users/smacdonald/printing-press/library/suno
rsync -a --exclude='.git/' --exclude='.gitignore' --exclude='.gitattributes' --exclude='.claude/' \
  --exclude='/build/' --exclude='/suno' --exclude='/suno-pp-cli' --exclude='/suno-test' --exclude='*.test' "$SRC"/ "$DST"/
```
Repo and staging are currently **in sync** (verified). Working tree is clean.

> ⚠️ rsync gotcha learned the hard way: a bare `--exclude='suno-pp-cli'` also matches the `cmd/suno-pp-cli/` **directory**. Always anchor binary excludes with a leading slash (`/suno-pp-cli`).

---

## What was completed (branch `feat/restore-dropped-commands`, 21 commits)

- Mirrored the generated 4.14.0 CLI into the git repo, baseline commit on `main`, then this feature branch.
- **Restored 8 commands** the 4.14.0 reprint dropped vs the prior build: `vibes`, `burn`, `budget`, `sessions`, `ship`, `tail`, `tree`, `custom-model` — ported from the old source + a shared `suno_transcendence_helpers.go`. Unit tests added (tree/sessions/burn/vibes/budget). They **coexist** with the new `grep`/`analytics`/`lineage`/`top`/`sql`/`credits`.
- **Budget enforcement** wired into the generate path (`suno_generate_run.go` → `budgetCapExceeded`).
- **`clips info` → `clips get`** renamed at spec.yaml + generated artifact + MCP tool + tools-manifest + docs (closes the dogfood naming gap; the old 4.6.1 also used `clips get`, so this is consistent).
- **Publish-gate fixes:** Examples on `budget set daily/monthly`, `burn`, `ship`, `vibes list/delete/use`; `vibes save` rejects empty recipe (usage error); `grep` exits non-zero (not-found) on zero matches; `burn` empty-store emits `[]` (hint→stderr); `generate --dry-run` emits JSON under `--json`.
- **Polish gate:** scorecard 94/A, verify 100%, 0 hand-authored gosec, publish-validate PASS.
- **Live gate: PASS, 139/139** (acceptance marker at `…/manuscripts/suno/20260528-222057/proofs/phase5-acceptance.json`).

---

## THE NEXT TASK: reconcile to a true superset, then publish

### Why
The published upstream `suno` is **4.6.1** (printer `mvanhorn`, run `20260514-184203`). Mine is **4.14.0** (printer `horknfbr`, run `20260528-222057`). They are *divergent reprints of the same reverse-engineered API*, not a clean upgrade. My top-level surface is a superset, but the two chose **different endpoint coverage**.

### User decisions (approved during brainstorming — DO NOT re-litigate)
1. **Reconcile to a true superset of 4.6.1 before publishing.**
2. **Adopt the OLD 4.6.1 command names** wherever mine diverged ("since this has never been published, rename to match the old so there's zero friction for current users; all functionality maintained; only extending").
3. **Build via `/printing-press-reprint`** — the official reprint flow that regenerates spec-driven commands from an updated spec **and preserves the hand-authored novel features** (transcendence, generate-flow, restored locals). (A plain regenerate would erase the ~30 hand-authored files; the reprint flow is the chosen tool precisely to avoid that.)

### The gap to close (precise — endpoint level)

**~17 genuinely-missing read-only endpoints** to ADD (present in 4.6.1, absent in 4.14.0):

| Old command | Method | Path |
|---|---|---|
| `clips attribution` | GET | `/api/clips/{id}/attribution` |
| `clips comments` | GET | `/api/gen/{id}/comments` |
| `clips parent` | GET | `/api/clips/parent` |
| `clips direct-children-count` | GET | `/api/clips/direct_children_count` |
| `clips similar` | GET | `/api/clips/get_similar/` |
| `generate video-status` | GET | `/api/video/generate/{id}/status/` |
| `billing eligible-discounts` | GET | `/api/billing/eligible-discounts` |
| `billing usage-plan` | GET | `/api/billing/usage-plan-web-table-comparison/` |
| `billing usage-plan-faq` | GET | `/api/billing/usage-plan-faq/` |
| `persona list` | GET | `/api/persona/me` |
| `project default` | GET | `/api/project/default` |
| `project pinned-clips` | GET | `/api/project/default/pinned-clips` |
| `user config` | GET | `/api/user/user_config/` |
| `user personalization` | GET | `/api/personalization/settings` |
| `user personalization-memory` | GET | `/api/personalization/memory` |
| `notification list` | GET | `/api/notification/v2` |
| `notification badge-count` | GET | `/api/notification/v2/badge-count` |

**Renames to apply (mine → old name)** so the surface matches what published users know:
- `clips set-metadata` → `clips edit` (same endpoint `POST /api/gen/{id}/set_metadata`)
- `trash` (top-level, `POST /api/feed/trash`) → old was `clips delete` (`POST /api/gen/trash/`). **Decide endpoint:** mine's `/api/feed/trash` passed the live gate; old's `/api/gen/trash/` may be stale. Keep the working endpoint under the name `clips delete`.
- `clips concat` → old `generate concat`; `lyrics create`→`generate lyrics`; `lyrics get`→`generate lyrics-status`. Old `generate` was a **group** (create/concat/lyrics/lyrics-status/video-status); mine's `generate` is a single runnable command (= old `generate create`). Reconciling the `generate` group shape is the trickiest structural piece — resolve during reprint spec design.
- `personas` (single) → old `persona` with `get`/`list`.
- `workspace` (create/get/list/rename/trash) → old `project` (me/default/pinned-clips). Note the structures differ; old `project` was read-centric, mine `workspace` is CRUD. Superset = keep CRUD + add the old read views under the `project`/old name.
- `custom-model` already restored (matches old).
- Consider `auth refresh` (old had it — a LOCAL re-login helper, ≈ `auth login --chrome`).

**Already-equivalent (no action):** `clips get` (both), `clips list`, `clips aligned-lyrics`, `clips set-visibility`, `billing info`, `clips stems`/`convert-wav`/`wav-url` (mine-only additions — keep), and all my novel features (keep & extend).

### Suggested resume path
1. Re-read this doc + the two design docs in `docs/superpowers/specs/` (the restore design + plan).
2. Decide whether to drive `/printing-press-reprint` (chosen method) — it works from the manuscripts/spec at `~/printing-press/manuscripts/suno/20260528-222057/` and the staging CLI. Confirm it can ingest the merged spec AND preserve the novel features (grep/analytics/lineage/top/sql/credits, generate-flow, vibes/burn/budget/sessions/ship/tail/tree). If the reprint flow can't cleanly preserve them, fall back to "sync spec + apply in place" (hand-edit spec.yaml + add the 17 read-command files, matching the existing generated `clips_*`/`billing` style — exactly how `clips info`→`get` was done).
3. Build the reconciled spec.yaml: old 4.6.1 endpoint set (old names) ∪ my new endpoints, + novel features preserved.
4. `go build/vet/test`, then re-run the **publish live gate** (see auth notes below) until 0 failures.
5. Sync repo → staging, commit on the branch.
6. Resume publish: `/printing-press-publish suno --from-polish` → it will detect the merged 4.6.1 (Reprint/replace path), package, open the cross-repo PR from the `horknfbr` fork. **It will replace 4.6.1, so the superset must be real first.**

### Reference: old 4.6.1 source for porting
Full old command source is in the fork clone:
`/Users/smacdonald/printing-press/.publish-repo-suno-cli-4f8a726c/library/media-and-entertainment/suno/`
(its `spec.yaml`, `internal/cli/*.go`). A binary built from it is at `/tmp/suno-old46` (may be gone after reboot; rebuild with `go build -o /tmp/suno-old46 ./cmd/suno-pp-cli` from that dir).

---

## Auth + live-gate notes (critical for re-running the gate)

- Auth type is `bearer_token` via `SUNO_JWT`, but the real credential is a **Clerk session** in `~/.config/suno-pp-cli/config.toml` (captured via `auth login --chrome`). The CLI mints a short-lived JWT per call; **JWT lifetime is ~1 hour** (`exp - iat = 3600`).
- The session may be **expired** when resuming. Refresh: have the user run
  `! /Users/smacdonald/printing-press/library/suno/suno-pp-cli auth login --chrome`
  (must be logged into Suno in Chrome), then `… doctor` should show **Credentials: valid**.
- **dogfood runs commands in an isolated HOME**, so it can't see `config.toml`. To authenticate the gate you MUST pass `--auth-env SUNO_JWT` **and** export a fresh `SUNO_JWT` in the same shell:
  ```bash
  CLI=/Users/smacdonald/printing-press/library/suno/suno-pp-cli
  $CLI credits >/dev/null 2>&1   # mints+saves a fresh jwt into config
  export SUNO_JWT=$(sed -nE "s/^jwt = '(.*)'\$/\1/p" ~/.config/suno-pp-cli/config.toml)
  cli-printing-press dogfood --dir /Users/smacdonald/printing-press/library/suno \
    --live --level full --timeout 120s --auth-env SUNO_JWT \
    --write-acceptance <proofs>/phase5-acceptance.json --json > <proofs>/publish-live-gate.json
  ```
- Binary: `cli-printing-press` is at `/Users/smacdonald/go/bin/cli-printing-press` (v4.14.0). `gh` is authed as `horknfbr` (fork access to the library, not push).

## PII reminders (public repo)
- The minted JWT contains the user's email/handle/user_id — **never** print it or let it reach the PR.
- `publish-live-gate.json` (~265KB) contains live account data — it was **deleted** from the proofs dir; do not republish it. The canonical proof is `phase5-acceptance.json` (clean metadata).
- Credit balance was genericized in `…/proofs/…-live-smoke.md`. Re-scan manuscripts for PII before any PR.

## Publish config (already written)
`/Users/smacdonald/printing-press/.publish-config-suno-cli-4f8a726c.json` — access `fork`, gh_user `horknfbr`, clone at the fork path above. Scope = `suno-cli-4f8a726c`.

---

## Open structural decision to make during reconcile
The `generate` shape: old = group (`generate create/concat/lyrics/lyrics-status/video-status`); mine = single command + separate `extend`/`cover`/`remaster`/`describe`. A true superset under old names needs a decision: make `generate` a group with subcommands (old shape) while keeping mine's captcha-aware main generation as `generate create`, OR keep mine's `generate` and add the missing subcommands elsewhere. Resolve this first when designing the reconciled spec.
