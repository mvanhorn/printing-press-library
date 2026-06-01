# HANDOFF — suno superset (apply-in-place) — 2026-05-29 PM

**Read this first when resuming.** Supersedes the reprint section of the morning handoff.
The design is DONE and validated; only the build execution remains, and the path changed.

## PROGRESS (resume here)
- **Decision made: Option B (apply-in-place on 4.14.0).**
- **Step 1/4 DONE** — additive endpoints committed `75aa252`: clips attribution/comments/parent/similar/direct-children-count, billing eligible-discounts/usage-plan/usage-plan-faq, and playlist/trending/user/notification groups. All resolve, build+vet green.
- **NEXT: step 2/4 — renames + persona/project reads** (task #7). Then generate-group reshape (#8), new features (#9), gates (#3), sync repo→staging (#4), publish (#5).
- **Gotcha learned:** always rebuild the binary fresh before testing commands — reused `/tmp` binaries gave false "command not found" failures. Group parents are intentionally `Hidden: true` (matches clips/billing); they resolve but don't list in top-level `--help`.
- Git: `feat/restore-dropped-commands`, commits `2703d70`→`82ef451`→`67d9e90`→`75aa252`. No remote (intentional).

## TL;DR
- Goal unchanged: ship a **superset** of the published 4.6.1 + paperfoot v5.5, publish over 4.6.1.
- We attempted the full `/printing-press` reprint under 4.19.0. **The regen-merge into the git repo failed** two ways: (1) it removed the repo's `.git` (regen-merge is meant for the non-git library staging, not a git repo), and (2) mixing 4.19.0 fresh framework files with the 4.14.0 hand-edited framework produced a **cascading skew** (`paginatedGet` 8→11 args, `resolveReadWithStrategy` missing, wiring drift). The CLI is too hand-authored for a clean machine reprint.
- **Recovered:** `.git` re-initialized fresh (no remote ever existed for the Go repo — `horknfbr/suno-cli` is the *Rust* paperfoot fork, unrelated). Repo reverted to **known-good 4.14.0** code (builds + vets green). Commits on `feat/restore-dropped-commands`: `2703d70` (broken-merge baseline), `82ef451` (this stable checkpoint).
- **Decision needed:** how to build the superset — Option A (fresh 4.19.0 base + port novels) or Option B (apply-in-place on 4.14.0). See below.

## What is DONE and valid (reuse, don't redo)
- **Superset spec** (621 lines, `generate --validate` passed end-to-end): currently staged at repo `spec.yaml`. Source copy: `~/printing-press/.runstate/suno-cli-4f8a726c/runs/20260529-180339/suno-superset-spec.yaml`.
- **Absorb manifest** + **research.json** + **novel-features brainstorm**: in `~/printing-press/.runstate/suno-cli-4f8a726c/runs/20260529-180339/{research/,research.json}`.
- **Paperfoot intelligence seed**: `docs/superpowers/specs/paperfoot-API_INTELLIGENCE.md`.
- **Approved scope** (Phase 1.5 gate APPROVED): 9 novel features (credits --forecast, grep, lineage, sql, analytics, persona usage [NEW], vibes, top, skill install [NEW]) + ~25 new endpoints + renames + SUNO_TOKEN auth + generate-group + workspace-in-listings + generate --project alias.

## Current repo state
- `~/homegit/suno-cli` on `feat/restore-dropped-commands`, NO remote. HEAD `82ef451`.
- Code = **4.14.0 known-good** (77 cli files, builds/vets green). `spec.yaml` = **superset target** (621 lines). 4.14.0 base spec saved at `docs/superpowers/specs/spec-4.14.0-base.yaml`.
- Reference trees: fresh 4.19.0 generated tree (builds, has new endpoints+renames+4.19.0 framework) at `~/printing-press/.runstate/suno-cli-4f8a726c/runs/20260529-180339/working/suno-pp-cli`; broken-merge backup at `~/homegit/suno-cli-SUPERSET-WIP`; clean 4.14.0 mirror at `~/printing-press/library/suno`.

## THE DECISION: how to build the superset
- **Option A — fresh 4.19.0 base + port novels.** Start from the fresh generated tree (correct 4.19.0 framework + new endpoints + renames already done), port the 45 hand-authored novels (suno_*.go: grep/analytics/lineage/top/sql/credits, vibes/burn/budget/sessions/ship/tail/tree/custom-model, the captcha-aware generate-flow, generate-types). Delivers the 4.19.0 machine lift (original reprint motivation). Risk: reverse-skew porting novels onto 4.19.0 helpers; the generate-flow + persona-create are the hard ports.
- **Option B — apply-in-place on 4.14.0 (RECOMMENDED, the morning handoff's named fallback).** Keep the known-good 4.14.0 (builds), and additively hand-write the new endpoint commands in 4.14.0 style (matching `clips_get.go`), apply renames in place, build the 4 new features, reshape generate/persona groups. Zero framework skew, lowest risk. Forgoes the 4.19.0 framework lift. This is normal additive feature dev — the reprint machinery isn't needed; the validated superset spec is the contract.

## Apply-in-place plan (Option B)
Use `spec.yaml` (the superset spec) as the contract. Build in `~/homegit/suno-cli`, gates via `cli-printing-press <leg> --dir ~/homegit/suno-cli`.

1. **New endpoint commands** (write in existing generated style, register in root.go):
   - clips: attribution, comments, parent, similar, direct-children-count, delete (POST /api/feed/trash)
   - generate group: video-status (concat/lyrics already exist as clips.concat/lyrics — move under generate)
   - billing: eligible-discounts, usage-plan, usage-plan-faq
   - persona: list (/api/persona/me)
   - project: default, pinned-clips
   - playlist: list (/api/playlist/me); trending: list (/api/trending/)
   - user: config, personalization, personalization-memory
   - notification: list, badge-count
2. **Renames** (rename command + keep endpoint): clips set-metadata→set, set-visibility→publish, aligned-lyrics→timed-lyrics; top-level trash→clips delete; personas→persona group; workspace→project group (keep CRUD + add default/pinned-clips). Add `--project` alias to generate's `--workspace`.
3. **generate group reshape**: make `generate` a group; current single `generate` runnable → `generate create`; add concat/lyrics/lyrics-status/video-status as subcommands; keep extend/cover/remaster/describe/evolve under it.
4. **New hand-code features**: `persona usage` (join personas vs clips for usage/orphans), `skill install` (--agent claude|codex|cursor|all, --print/--path/--force; writes SKILL.md to ~/.claude/skills/suno/SKILL.md, Codex skills loc, ./.cursor/rules/suno.mdc), workspace-membership in listings (sync clip↔workspace index; clips list/top/grep show workspace; analytics --group-by project), `--variation` param (high/normal/subtle) on generate create + describe.
5. **Auth**: make SUNO_TOKEN canonical env var, keep SUNO_JWT as alias (config.go).
6. Update README/SKILL/AGENTS, build/vet/test green.
7. **Live gate**: see auth notes below. Persona-create + generation are hCaptcha-gated → EXCLUDE from the hard live gate; ship persona-create best-effort + unit tests.
8. Sync repo→staging, then `/printing-press-publish suno --from-polish` (replaces public 4.6.1; PII re-scan first).

## Auth + live-gate notes (carry-over)
- Auth: Clerk session in `~/.config/suno-pp-cli/config.toml` via `auth login --chrome`; CLI mints ~1hr JWT. Reachability probe = 401 unauth (healthy). For the live dogfood gate: `$CLI credits >/dev/null` to mint, then `export SUNO_TOKEN=$(sed -nE "s/^jwt = '(.*)'\$/\1/p" ~/.config/suno-pp-cli/config.toml)` and run dogfood with `--auth-env SUNO_TOKEN`. Session may be expired on resume — have user run `! ~/printing-press/library/suno/suno-pp-cli auth login --chrome`.

## Lessons (for next time)
- NEVER point `regen-merge --apply` at a git repo — it swaps the whole tree and removes `.git`. It targets the non-git library staging; sync staging→repo + commit afterward.
- A CLI with ~45 hand-authored files + heavily hand-edited framework (config/client/store/helpers/root) is past the point where a clean machine reprint works — additive apply-in-place is the right tool.
