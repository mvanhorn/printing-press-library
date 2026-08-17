# Forgejo CLI Brief

## API Identity
- Domain: Self-hosted Git forge (Forgejo is a community-owned Gitea fork). Targets teams running private/org infrastructure on any Forgejo instance.
- Users: Developers, DevOps engineers, OSS contributors using Codeberg, self-hosted Forgejo instances; Gitea users who have or will migrate.
- Data profile: Repos, issues, PRs, releases, branches, tags, organizations, notifications, runners, packages. Heavy on repo + issue/PR; notifications are real-time. OAuth2 app management is first-class.
- Spec: Forgejo API v15.0.2+gitea-1.22.0, Swagger 2.0, 491 operations, 314 paths

## Reachability Risk
- Low. Official Swagger spec served directly from the target instance. Every endpoint authenticated by the token provided. No reverse-engineering needed.

## Auth Architecture
- Token (Bearer via `Authorization: token <tok>` header) — the primary form
- BasicAuth — username:password, available
- SudoHeader — admin impersonation (`Sudo: <username>`)
- TOTPHeader — 2FA passthrough (`X-FORGEJO-OTP`)
- OAuth2 — `/user/applications/oauth2` CRUD for apps; CLI can implement device flow
- Security definitions in spec: AuthorizationHeaderToken, BasicAuth, SudoHeader, SudoParam, TOTPHeader

## Top Workflows
1. **PR lifecycle** — list → create → review → merge; `fj pr list`, `fj pr create`, `fj pr merge`
2. **Issue triage** — list by label/assignee, create, close, comment; `fj issue list --label bug --state open`
3. **Release management** — create release, upload assets; `fj release create --tag v1.2.0 --assets ./dist/*.tar.gz`
4. **Repo management** — clone, fork, create, search across any instance; `fj repo clone owner/repo`
5. **Notifications** — check inbox, mark read, filter by repo; `fj notification list --unread`
6. **Runner management** — register/list Actions runners (org, repo, user scoped) — unique Forgejo feature with no gh analog

## Table Stakes (from competitors)
- `tea` (Gitea official CLI): login, repo list/clone/fork, issue list/create, PR list/create, milestone, label, token management. Authentication via token stored in config. Lacks OAuth2 device flow, no SQLite cache.
- `forge` (git-pkgs): multi-forge repo/issue/pr/release/ci/label/notification commands. Auth via `forge auth login` with per-host config. Broad but shallow — no Forgejo-specific features (runners, packages, activitypub).
- `gh` (GitHub CLI): the UX benchmark. Repo, issue, PR, release, auth, gist, browse, api passthrough, search, codespace, workflow, secret management. Per-host auth with OAuth2 device flow. `--json` everywhere. Fuzzy filtering.

## Data Layer
- Primary entities: repos, issues, PRs (with their comments/reviews), releases, notifications
- Sync cursor: `updated_after` timestamp on list endpoints; `limit`/`page` pagination throughout
- FTS/search: repo full-text search (`/repos/search?q=`), issue search, user search — all spec-native
- Local store value: offline `fj issue search --repo owner/name --label bug` without hitting API; notification count badge; multi-repo issue triage dashboard

## Novel Features (no existing tool has these)
1. **Multi-instance profile management** — `fj auth login https://codeberg.org` + `fj auth login https://git.company.com` with per-host profiles (like gh, unlike tea which is single-host)
2. **OAuth2 device flow** — `fj auth login` prompts for device code; stores access+refresh tokens; refreshes automatically
3. **Runner lifecycle commands** — `fj runner list --org myorg`, `fj runner register --scope repo`, registration token retrieval; no other CLI exposes this
4. **ActivityPub endpoints** — `fj activitypub actor show owner/repo`, `fj activitypub inbox` — unique to Forgejo's federation design
5. **Sudo mode** — `fj --sudo username <cmd>` for admin impersonation flows
6. **Package registry** — `fj package list owner`, `fj package link owner/pkg/ver repo`
7. **Local notification cache** — `fj notification sync` + `fj notification list` from SQLite; bell icon + count with `fj notification count`
8. **Offline issue/PR search** — `fj sync repos/issues` to populate SQLite; fast local grep/filter

## Product Thesis
- Name: `fj` (maps to Forgejo exactly as `gh` maps to GitHub; two-letter, muscle memory)
- Why it should exist: `tea` is unmaintained upstream and Gitea-branded; `forge` treats Forgejo as one of four backends without exposing Forgejo-native features (runners, activitypub, admin sudo). A Forgejo-native CLI with `gh` UX parity and multi-instance auth gives Forgejo operators and Codeberg users what they've been missing.
- Positioning: "The `gh` for Forgejo — works with any instance, stores your data locally, and speaks Forgejo natively."

## Codebase Intelligence
- Spec version: `15.0.2+gitea-1.22.0` — current Forgejo stable
- Auth header: `Authorization: token <token>` (not `Bearer`)
- Rate limiting: Not explicitly documented in spec; Forgejo inherits Gitea's configurable rate limiting
- Pagination: `?page=&limit=` on all list endpoints, `X-Total-Count` response header

## User Vision
- L&F like `gh` — familiar command tree, `--json` output, `--web` to open in browser, pager support
- Works with any Forgejo instance (multi-host config)
- Token auth AND OAuth2 device flow
- Binary name: `fj`

## Source Priority
- Single source: `https://<forgejo-instance>/swagger.v1.json` (Forgejo API v15.0.2 spec)
- No multi-source ordering needed

## Build Priorities
1. Auth subsystem: `fj auth login/logout/status/token` — token storage + OAuth2 device flow; multi-host config at `~/.config/fj/config.yml`
2. Repo commands: `fj repo list/view/clone/fork/create/search/delete`
3. Issue commands: `fj issue list/view/create/close/reopen/comment/edit`
4. PR commands: `fj pr list/view/create/merge/review/checkout/diff`
5. Release commands: `fj release list/view/create/upload/download/delete`
6. Notification commands: `fj notification list/read` + SQLite cache
7. Runner commands: `fj runner list/register/delete` (org/repo/user scope)
8. Org commands: `fj org list/view/create/members`
9. `fj api` — raw passthrough with auth headers injected
10. `fj browse` — open repo/issue/PR in browser
