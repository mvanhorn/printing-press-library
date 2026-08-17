# WordPress CLI — Phase 3 Build Log

Manifest transcendence rows: 7 planned, 7 built. All shipping-scope rows shipped.

## Shipping-scope hand-code items (10)

| # | Item | Status |
|---|------|--------|
| T1 | `fleet` | **built** |
| T2 | `diagnose` | **built** |
| T3 | `caps` | **built** |
| T4 | `queue` | **built** |
| T5 | `audit` | **built** |
| T6 | `orphans` | **built** |
| T7 | `schema <type>` | **built** |
| I1 | `site add/list/use/remove` | **built** |
| I2 | `media upload <file>` (raw binary) | **built** |
| I3 | `--wp-fields` / `--embed` persistent flags | **built** |

## Generated baseline

53 resources / 174 typed endpoints, 41 API interfaces, 268 files in internal/cli.
Auth (Basic + base64 of WORDPRESS_USER:WORDPRESS_APP_PASSWORD), WORDPRESS_BASE_URL
override, learn seeds, and MCP code-orchestration all emitted correctly from the spec.

## Codex delegation notes

**Failure 1 (batch A, broad prompt).** A single prompt covering four items
(site registry, per-site DB path, media upload, global query flags) against the
391-file / 98k-line generated tree caused Codex to exhaust its budget reading
source — 18.5k log lines of file dumps — and exit with no marker and an empty
diff. Nothing was written, so nothing needed reverting.

Root cause: the prompt described the surrounding code instead of quoting it, so
Codex had to go find it. The delegation reference's "paste ACTUAL CODE in the
CURRENT CODE section — never descriptions of code" rule is not stylistic at this
tree size; it is what keeps the task inside budget.

Retry shape: one item per invocation, every referenced signature and struct
inlined, an explicit "do not explore the tree" instruction, and a named file
allowlist.

## Bugs found and fixed during foundation review

1. **Response-cache key collision (correctness, would have shipped).**
   `client.cacheKey(path, params)` hashes only per-command params. The new
   `--wp-fields` / `--embed` globals change the effective URL and the response
   body without changing `params`, so `posts list` and
   `posts list --wp-fields id,link` collided on one cache entry and the second
   call was served the first call's unprojected response. Verified: projection
   worked under `--no-cache` and silently did nothing without it. Fixed by
   folding the global params into the cache key. Per-command params still win
   on conflict.

2. **`site list` / `site current` printed help when run bare.** The
   verify-friendly `len(args)==0 && NFlag()==0 -> cmd.Help()` guard is correct
   for commands with required input, but wrong for zero-input list commands —
   the framework's own `profile list` does the work when run bare. Removed the
   guard from those two; kept it on `site add` and `site use`, which do take a
   required positional. `--dry-run` still exits 0 on all of them.

Live verification against wordpress.org/news: Link-header discovery resolved the
REST root, `wp/v2` namespace check passed, and the Application Password
authorization URL was surfaced from the root's `authentication` object.

3. **Local-store path incoherence (correctness, would have shipped).** The
   generated `sync` defaults to the framework data directory
   (`~/.local/share/wordpress-pp-cli/data.db`), but `wordpressDBPath` returned a
   per-site subdirectory. A plain `sync` wrote one file while every novel read
   command looked at another, so `fleet` reported `no-mirror` immediately after
   a successful 117-record sync. Fixed by preferring a per-site mirror only when
   one actually exists and otherwise falling back to the framework default, so
   single-site use is coherent out of the box and operators who isolate sites
   (via `--db` or a per-site `--home`) still get true separation.

4. **`fleet` / `queue` / `audit` / `orphans` printed help when run bare** — same
   zero-input class as `site list`. Guard removed from all four.

## Behavioral verification (real sites, not exit codes)

`diagnose` classified four live sites correctly, matching the independent
research predictions:

| Site | Verdict | Evidence |
|---|---|---|
| wordpress.org/news | `ok` | both request forms 200 |
| wpbeginner.com | `app-layer-block` | `rest_cannot_access` (Disable REST API plugin) |
| arstechnica.com | `path-blocked` | pretty 403, `?rest_route=` 200 |
| cloudways.com | `bot-challenge` | Cloudflare above WordPress |

Against a 117-record sync of wordpress.org/news: `fleet` reported 30 posts /
2 pages / 3 media / 30 users with a 1m sync age; `audit` found 31 of 32 posts
without a featured image plus 2 uncategorized and 2 with empty excerpts, each
with offending IDs; `orphans` found 2 unreferenced media totalling ~1.5 MB;
`schema post` surfaced `generated_slug`, `permalink_template`, `password`, and
`template` as never-populated (edit-context-only) plus that site's unexposed
plugin meta keys.

## Pre-existing generator issue (retro candidate — NOT patched here)

`internal/cliutil/credentials_test.go` fails at the **generated baseline**,
before any hand-code. The spec declares two-var HTTP Basic
(`WORDPRESS_USER` + `WORDPRESS_APP_PASSWORD`), and the emitted `AuthHeader()`
correctly returns empty unless both are set — but the emitted tests set only one
credential and assert a non-empty header. Four tests fail
(`TestCredentialsFileWinsWhenLegacyConfigAlsoHasSecrets`,
`TestCorruptCredentialsFallsBackToLegacyConfig`,
`TestCorruptCredentialsFallsBackToEnvCredential`,
`TestEmptyCredentialsFileDoesNotClearLegacyConfig`).

Left unpatched deliberately per the template-shape escape hatch: fixing it in the
printed CLI would hide the machine bug from the next two-var-auth CLI.
