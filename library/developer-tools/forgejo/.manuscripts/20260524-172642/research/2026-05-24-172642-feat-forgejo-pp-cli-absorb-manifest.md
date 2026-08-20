# Forgejo CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Multi-server auth login (token) | tea | fj auth login --token <tok> --host <url> | Per-host config at ~/.config/fj/config.yml, multiple profiles |
| 2 | OAuth2 device flow login | forge | fj auth login (device flow, no --token) | Auto-refresh tokens, no password needed |
| 3 | Auth status / token output | forge, gh | fj auth status / fj auth token | Machine-readable token for scripting |
| 4 | Auth logout | gh | fj auth logout [--host <h>] | Removes stored credentials |
| 5 | Issue list (filter by state/label/assignee/milestone) | tea, gh | (generated endpoint) issue list | --json, pager, --web, all filter params |
| 6 | Issue view | tea, gh | (generated endpoint) issue view | Rich rendering with comment count |
| 7 | Issue create | tea, gh | (generated endpoint) issue create | --title --body --label --assignee --milestone --dry-run |
| 8 | Issue edit (close/reopen/edit fields) | tea, gh | (generated endpoint) issue edit | State transitions + field updates |
| 9 | Issue comment list/create/edit/delete | tea, gitea-mcp | (generated endpoint) issue comment | Full comment lifecycle |
| 10 | Issue search (cross-repo) | gitea-mcp | fj issue search <query> [--all] | Searches /repos/issues/search across all accessible repos |
| 11 | PR list | tea, forge, gh | (generated endpoint) pr list | --state --head --base filters |
| 12 | PR view | tea, gh | (generated endpoint) pr view | Diffs, review status, CI checks |
| 13 | PR create | tea, forge, gh | (generated endpoint) pr create | --title --body --base --head --draft |
| 14 | PR merge | tea, gh | (generated endpoint) pr merge | --merge/--squash/--rebase strategy |
| 15 | PR review (approve/request-changes) | forge, gh | (generated endpoint) pr review | With optional comment body |
| 16 | PR checkout locally | tea, gh | fj pr checkout <number> | Sets up local branch tracking PR head, injects auth |
| 17 | PR diff/patch download | gh | (generated endpoint) pr diff | Raw patch or unified diff output |
| 18 | PR reviewer request | forge, gh | (generated endpoint) pr reviewer add | Adds reviewers by username |
| 19 | PR update base (merge base into head) | Forgejo-specific | (generated endpoint) pr update | Forgejo's /pulls/{index}/update |
| 20 | Release list | tea, forge, gh | (generated endpoint) release list | Sorted by tag date |
| 21 | Release view (latest) | gh | (generated endpoint) release view | Shows assets, changelog body |
| 22 | Release create | tea, gh | (generated endpoint) release create | --tag --title --notes --draft --prerelease |
| 23 | Release asset upload | tea, gh | fj release upload <tag> <files...> | Progress + retry (see transcendence) |
| 24 | Release asset download | tea, gh | fj release download <tag> [--asset <n>] | Downloads to --dir, selective by name |
| 25 | Release delete | tea | (generated endpoint) release delete | With confirmation prompt |
| 26 | Repo list (user/org) | tea, forge, gh | (generated endpoint) repo list | --owner, --type fork/source/mirror, --json |
| 27 | Repo view | forge, gh | (generated endpoint) repo view | Stars, forks, description, language, clone URL |
| 28 | Repo create | tea, gh | (generated endpoint) repo create | --private --org --init --license --gitignore |
| 29 | Repo fork | tea, gh | (generated endpoint) repo fork | Into personal namespace or --org |
| 30 | Repo clone (auth token injected) | tea | fj repo clone owner/repo [--ssh] | Injects token into HTTPS git URL automatically |
| 31 | Repo delete | tea | (generated endpoint) repo delete | With confirmation prompt |
| 32 | Repo search | gh | (generated endpoint) repo search | --topic --language --visibility |
| 33 | Repo migrate (from remote git) | Forgejo-specific | (generated endpoint) repo migrate | No other CLI exposes this; mirrors/imports from GitHub, GitLab etc. |
| 34 | Branch list | tea, forge | (generated endpoint) branch list | With default branch highlighted |
| 35 | Branch create | tea | (generated endpoint) branch create | From --sha or HEAD |
| 36 | Branch delete | tea | (generated endpoint) branch delete | With --force flag |
| 37 | Branch update (rename/protection) | Forgejo-specific | (generated endpoint) branch update | PATCH /branches/{branch} |
| 38 | Label list/create/edit/delete | tea, gitea-mcp | (generated endpoint) label | Full label CRUD, --repo or --org scope |
| 39 | Milestone list/create/edit/delete | tea, gitea-mcp | (generated endpoint) milestone | Full milestone CRUD |
| 40 | Wiki list/view/create/update | gitea-mcp | (generated endpoint) wiki | Page list + content read/write |
| 41 | Notification list | tea, gitea-mcp | (generated endpoint) notification list | --unread, --all |
| 42 | Notification mark-read | tea, gitea-mcp | (generated endpoint) notification mark | By ID or all |
| 43 | Notification count (unread) | gitea-mcp | (behavior in fj notification list --count) Returns integer unread count | Shell prompt / badge integration |
| 44 | Org list | tea | (generated endpoint) org list | My orgs + public orgs |
| 45 | Org view | tea | (generated endpoint) org view | Members, repos, description |
| 46 | Org create/delete/edit | tea | (generated endpoint) org create | --description --website --visibility |
| 47 | Org member list/add/remove | tea | (generated endpoint) org member | Role filter |
| 48 | Runner list (repo/org/user scope) | Forgejo-specific | fj runner list --scope [user\|org=<o>\|repo=<r>] | No other CLI exposes this |
| 49 | Runner register | Forgejo-specific | fj runner register --scope ... [--print-token] | Retrieves token + prints pre-filled act_runner command |
| 50 | Runner delete | Forgejo-specific | (generated endpoint) runner delete | |
| 51 | Package list/view/delete | Forgejo-specific | (generated endpoint) package | Registry package management |
| 52 | ActivityPub actor (federation) | Forgejo-specific | (generated endpoint) activitypub | No other CLI exposes this |
| 53 | Browse (open in browser) | gh | fj browse [owner/repo] [--issue N] [--pr N] [--branch b] | Opens correct URL for current context |
| 54 | API passthrough | gh, forge | fj api /repos/{owner}/{repo} [--method POST] [--field k=v] | Injects auth headers, --jq filter |
| 55 | Sudo mode (admin impersonation) | Forgejo admin feature | (behavior in fj --sudo <user> <command>) Any command impersonated as another user | For admin workflows |
| 56 | User whoami | tea | (generated endpoint) user | Returns current authenticated user info |
| 57 | User search | gh | (generated endpoint) user search | --limit --json |
| 58 | Admin user management | tea | (generated endpoint) admin | User CRUD, org admin, email management |
| 59 | Time tracking list/add/delete | tea, gitea-mcp | (generated endpoint) issue time | Add time entries to issues |
| 60 | Webhook list/create/delete | tea | (generated endpoint) webhook | Per-repo webhook management |
| 61 | Gitignore/license/label templates | Forgejo-specific | (generated endpoint) template | Lists available gitignore, license, label templates |
| 62 | Offline issue/PR sync (SQLite) | novel | fj sync --resources issues,pulls [--since 7d] | Local cache for cross-repo dashboards |
| 63 | Shell completion | tea | (behavior in fj completion --shell bash/zsh/fish/powershell) Shell completion scripts | |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|-------------|--------------|----------|
| T1 | Multi-repo triage dashboard | `fj issue dashboard [--all-hosts] [--assignee @me] [--label <l>]` | 9/10 | hand-code | Reads from SQLite (populated by `fj sync`); joins repos × issues × labels; renders sorted terminal table cross-repo, optionally cross-host. No single API call produces this view. | Devika's Monday ritual; no competitor does cross-repo issue aggregation |
| T2 | Unified notification inbox (cross-host) | `fj notification inbox [--all-hosts] [--unread] [--since <duration>] [--json]` | 8/10 | hand-code | Syncs from all configured host profiles into SQLite keyed by `(host, notification_id)`; renders unified inbox with host column; `fj notification count --all-hosts` returns summed integer for prompt badge. | Marcus's multi-instance frustration; single-host only in absorb manifest |
| T3 | Runner health sweep (cross-org) | `fj runner sweep [--all-orgs] [--watch] [--json]` | 8/10 | hand-code | Iterates org/repo/user runner scopes in parallel, joins results, flags offline runners with last-seen timestamp; `--watch` polls every N seconds. | Barend's Friday sweep; no competitor CLI has runner commands at all |
| T4 | Release changelog generation | `fj release changelog <from-tag> <to-tag> [--format md\|json] [--section bug,feature,break]` | 8/10 | hand-code | Resolves tag timestamps; queries closed issues and merged PRs in window; groups by label. Three-entity join: tags × issues × PRs. | Priya posts changelog every sprint; no competitor does this |
| T5 | Release upload with progress + retry | `fj release upload <tag> <files...> [--retry <n>] [--progress]` | 7/10 | hand-code | Wraps generated upload endpoint: chunked transfer, per-file progress bar, retry-with-backoff, size verification post-upload. | Priya lost two uploads to silent curl failures; `gh` has this for GitHub |
| T6 | Stale issue sweeper | `fj issue sweep --stale-after <duration> [--label stale] [--dry-run] [--comment <text>] [--close]` | 7/10 | hand-code | Pre-filters via SQLite; fetches fresh state for candidates; presents confirmation list; applies label → comment → close in order. Agent-shaped read→confirm→write loop. | Devika's most tedious Monday task; stale-bot is GitHub-only today |
| T7 | PR review queue | `fj pr queue [--all-repos] [--review-requested @me] [--json]` | 7/10 | hand-code | Queries synced PR table for auth user's pending review requests; joins with check-run status and approval counts; renders age-sorted table. Three-entity join: PRs × review_requests × checks. | Devika's Monday PR scan; absorb `pr list` is per-repo and unfiltered |
| T8 | CI check status summary | `fj ci status [--branch <b>] [--all-repos] [--watch] [--json]` | 6/10 | hand-code | For each watched repo: fetches latest commit on branch, then check runs; joins branch → commit → check_run; renders pass/fail/pending per workflow; `--watch` tails at 10s interval. | Priya checks CI before cutting releases; no competitor surfaces cross-repo check status |
