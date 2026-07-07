# Plane CLI — Workspace Targeting & Onboarding (design)

Date: 2026-06-10
Status: Approved (brainstorming), pending implementation plan

## Problem

The published `plane-pp-cli` (printingpress.dev, PR #1006) smuggles the workspace
slug *inside* `base_url` (`https://<host>/api/v1/workspaces/<slug>`). The generated
commands still advertise a positional `<slug>` argument, but after the v3 server
restructure the relative paths carry no `{slug}` placeholder — so the positional is
**inert**: it binds to nothing and is silently swallowed.

Consequence for every user (verified live 2026-06-10):

- `projects list bbm`, `projects list doctor-school`, `members bbm` **all** emit
  `GET …/workspaces/doctor-school/…` — the typed slug is ignored, `PLANE_BASE_URL`
  wins. Targeting a second workspace returns the wrong workspace's data as a silent
  **false positive**.
- There is no command to see which workspace is active or to switch.

## Empirical constraints (verified 2026-06-10, BBM self-hosted CE)

- **API key is user-scoped, not workspace-scoped.** One key returned `200` for both
  `…/workspaces/bbm/members/` (Anton, role 20) and `…/workspaces/doctor-school/members/`
  (different members). One key spans all the user's workspaces. → a single "slug axis"
  is sufficient; switching does not require swapping credentials.
- **Discovery by API key is impossible.** Every workspace-enumeration endpoint rejects
  the key: `/api/v1/workspaces/` → 404, `/api/v1/users/me/workspaces/` → 404,
  `/api/users/me/workspaces/` and `/api/workspaces/` → 401 under both `X-API-Key` and
  `Authorization: Bearer`. `/api/v1/users/me/` → 200 but profile only, no workspaces.
  The public v1 API is strictly workspace-scoped; membership enumeration lives behind
  session auth. **The slug must be user-supplied input.**
- **slug → workspace id is possible** once a slug is known: object payloads carry a
  `workspace` UUID (e.g. a project's `.workspace` = `03a696ba-…` for doctor-school).
- Self-hosted only: workspaces *can* be enumerated via the DB management shell
  (`ssh <host> docker exec … Workspace.objects.values('slug','id')`), the same escape
  hatch `relations unset` uses. Not viable for the public/cloud build (needs infra
  access). At most an opt-in self-hosted flag — out of the core architecture.

## Goals

1. Workspace targeting that works out-of-the-box for **any** user (cloud or self-hosted),
   not a hack tailored to one instance.
2. Slug is a first-class, separately-configured axis — **not** part of `base_url`.
3. An explicit enrollment step replaces the impossible discovery, with each slug
   validated by a live probe (kills the false positive).
4. `generate`/rebuild produces the new behaviour (fix the build, not a throwaway patch).

## Non-goals

- Faking a server-driven `workspaces list` (the API cannot enumerate by key).
- Session/JWT auth support (rejected: fragile on CE, separate auth path).
- An engine PR being a *blocker* (issue #2599 stays open as the eventual upstream home;
  we ship in the recipe+library layer now).

## Architecture

Separate the tenant identifier from the transport base URL.

### A. Transport — slug as a server template variable

- `base_url` / server = **host only**: `https://api.plane.so` (cloud default) or
  `https://<host>` (self-hosted). The API root `…/api/v1/workspaces/{slug}/…` keeps
  `{slug}` as a **template variable**, not a literal in `base_url`.
- Slug resolution precedence (per invocation):
  `--workspace <slug>` (new global flag) → `PLANE_WORKSPACE` (env) →
  `default_workspace` (config) → **loud error**:
  `no workspace set — run 'plane-pp-cli init' or pass --workspace`.
- The resolved slug feeds the client's `TemplateVars["slug"]`.
- The advertised positional `<slug>` is removed in favour of the global flag (or kept
  as an alias that feeds the same resolver — decided in the plan).
- Layer: **recipe + library** (survives regen, indexed in `.printing-press-patches.json`,
  same pattern as relations/module/attach-file). Engine issue #2599 remains the
  long-term home.

### B. Onboarding — explicit enrollment (novel, hand-written)

The generator does not produce interactive multi-slug onboarding; this is a novel
command set in the library, plus a recipe note documenting the setup step.

- `plane-pp-cli init` — interactive: prompt API key → host → workspace slug(s)
  (comma-separated, from the browser URL) → probe each `…/workspaces/<slug>/members/`
  → write config. Invalid/no-access slugs are rejected at enrollment, not silently.
- `workspaces add <slug> [<slug>…]` — non-interactive twin for agents/CI (probes +
  records). `init` calls it under the hood.
- `workspaces use <slug>` — probe + set as `default_workspace`.
- `workspaces current` — print resolved slug + source (flag/env/config).
- `workspaces list` — print the locally-recorded `[[workspaces]]` set, mark current,
  with an explicit banner that the public API cannot enumerate workspaces by key.
- `doctor` — when no workspaces are configured, emit
  `⚠ no workspaces configured — run 'plane-pp-cli init'` (installer-agnostic nudge;
  optional maintainer post-install message can echo the same line).
- Optional, self-hosted only, behind a flag: `workspaces discover --via-db` using the
  docker/management-shell escape hatch.

### Config schema (config.toml)

```toml
base_url = "https://plane.bbm.academy"   # host only — NOT …/workspaces/<slug>
default_workspace = "bbm"

[[workspaces]]
slug = "doctor-school"
id   = "03a696ba-5af3-4848-b228-5396b4d6fea4"   # cached from the enrollment probe

[[workspaces]]
slug = "bbm"
id   = "…"
```

`PLANE_BASE_URL` (env) must mean **host only** going forward; a value that still
contains `/workspaces/<slug>` is detected and a migration warning is printed.

## Build, rebuild & publish strategy

Three sources exist with known drift:

1. **Dogfood** `C:\Users\sidor\printing-press\library\plane` (not git) — builds the
   installed `~/go/bin/plane-pp-cli.exe`; currently has both `modules_novel.go` **and**
   `attach_file_novel.go`.
2. **Library publish clone** `…/.publish-repo-cli-printing-press-316e4ead/library/project-management/plane`
   (remote `sidorovanthon/printing-press-library`, branch `feat/plane`) — PR #1006
   MERGED; has `modules_novel.go` but **not** `attach_file_novel.go` (behind dogfood).
3. **Catalog/recipe** `C:\Users\sidor\repos\cli-printing-press/catalog/{plane.yaml,specs/plane-spec.yaml}`
   (remote `mvanhorn/cli-printing-press`, branch `feat/plane-catalog`) — PR #2598 MERGED.

Plan must: implement in dogfood, rebuild + live-verify the installed binary (rename-swap
for the running .exe, also rebuild `plane-pp-mcp.exe`), mirror into the library clone
(including reconciling the pre-existing attach-file drift), update the recipe, and open
fresh PRs against the now-merged baselines.

## Testing

- Unit: slug resolver precedence; config read/write round-trip; base_url migration
  detection; dry-run URL composition per resolution source.
- Live (dogfood, BBM): `init`/`workspaces add` probe success+failure; `use`/`current`/
  `list`; `--workspace` override; both `bbm` and `doctor-school` reachable and distinct;
  the original false positive no longer reproduces.
- Generate: a clean `generate` from the updated recipe compiles and exposes the flag.

## Risks

- `PLANE_BASE_URL` env still set to the old full prefix on the user's box shadows the new
  config → migration warning + `doctor` check must catch it.
- Engine cannot be made to emit the flag without #2599; the recipe+library patch must be
  regen-durable and indexed in `.printing-press-patches.json`.
- Drift between the three sources; the plan tracks each explicitly.
