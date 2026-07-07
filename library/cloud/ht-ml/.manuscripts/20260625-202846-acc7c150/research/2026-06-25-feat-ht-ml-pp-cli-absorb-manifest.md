# ht-ml.app CLI — Absorb Manifest

Sources that touch this API: the `nsmith/html` Agent Skill (MIT, ~3 stars; create/update/assets/password + 20 templates, no CLI, no local state) and the in-browser WebMCP tool `publish_html_site` (create-only). No competing CLI or MCP server exists. Our CLI is the only ht-ml.app CLI and the only tool with persistence.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Create/publish a site from HTML | nsmith/html Step 1 + WebMCP `publish_html_site` | `ht-ml-pp-cli publish` | Persists site_id/url/update_key/html/title to SQLite; file/stdin/string input; `--password`; `--assets` auto-upload; `--dry-run`; `--json` |
| 2 | Update a site's HTML | nsmith/html (PUT /v1/sites/{id}) | `ht-ml-pp-cli update` | Resolves update_key from local store by site_id (user never handles the key); records a new version; `--dry-run` |
| 3 | Upload a referenced asset | nsmith/html Step 2 (POST assets) | `ht-ml-pp-cli assets upload` | update_key auto-resolved; validates the path is referenced first (avoids 403); records asset status |
| 4 | Get site details + referenced/missing assets | GET /v1/sites/{id} | `(generated endpoint) sites <id>` | Mirrors API; caches status + asset list offline |
| 5 | Password protection (set/clear/leave) | nsmith/html password section | `(behavior in ht-ml-pp-cli password set)` and `password clear` | Stores the shared secret locally; `set`/`clear`/rotate; tells user it is a shared secret |
| 6 | Self-describing help / error codes | GET /v1/help | `(behavior in ht-ml-pp-cli doctor)` | Offline; folded into a local health report |
| 7 | Ready-made page templates | nsmith/html assets/templates (20 templates) | `ht-ml-pp-cli templates` (list) + `new --template <name>` | Built-in curated starter templates scaffolded locally and publishable in one step |

Notes: because the API has no global credential and writes use a per-site Bearer `update_key`, the write/workflow commands above (`publish`, `update`, `assets upload`, `password`) are hand-built so they resolve the per-site key from the local store. Only the no-auth `GET /v1/sites/{id}` and `POST /v1/sites` create endpoints come straight from the generator.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Site registry inventory | `list [--orphaned] [--missing-assets] [--sort age\|versions]` | 10 | hand-code | The API has no list endpoint and no accounts; the local `sites` table is the only inventory of what you've published that can exist | none |
| 2 | Key vault & recovery | `keys show <id> --reveal` / `keys export` / `keys import` | 10 | hand-code | The once-only `update_key` has no recovery endpoint; the local store is its sole holder. Passphrase-sealed export/import is the only disaster recovery and the only cross-machine path | none |
| 3 | One-shot asset reconcile | `assets sync <id> [--root <dir>]` | 10 | hand-code | Collapses the documented create→discover-missing→upload flow: parses HTML refs, diffs the API missing-list, uploads every referenced-but-missing local file, honoring the referenced-first 403 rule | Use to upload every referenced-but-missing asset for ONE site in a single pass. To find WHICH sites across your registry have broken assets, use `assets audit` instead. |
| 4 | Version rollback | `rollback <id> [<version>]` | 9 | hand-code | The API has no version concept; rollback reads a prior HTML snapshot from the local `versions` table, auto-resolves the key, and PUTs it as a new version | none |
| 5 | Broken-asset audit | `assets audit [--missing-only]` | 9 | hand-code | Cross-entity `sites`⋈`assets` join across every known site to surface publicly-visible broken images, an answer no single GET can produce | Use to find broken/missing referenced assets across ALL your sites (read-only). To upload the missing files for a given site, use `assets sync <site_id>` instead. |
| 6 | Living-doc republish by alias | `republish --as <name> <file\|->` | 8 | hand-code | A local alias map + upsert (PUT in place if the alias is known, else create once and bind) gives the deliberately accountless API a stable identity for cron/recurring documents | none |
| 7 | Pre-publish secret/PII scan | `scan <file\|-> [--rules secrets,pii]` | 7 | hand-code | Mechanical regex guard (AWS keys, bearer tokens, private keys, emails) before HTML becomes a public, crawlable, permanent URL; the API content-scans for safety (422) but not for secret leakage | none |

Hand-code count (transcendence rows tagged `hand-code`): **7**. Generator auto-emit (spec-emits): the no-auth create + get endpoints. Stubs: none.
