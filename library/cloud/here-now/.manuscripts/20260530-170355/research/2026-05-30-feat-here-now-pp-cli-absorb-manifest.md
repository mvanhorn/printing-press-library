# here.now CLI — Absorb Manifest

**API:** here.now (OpenAPI 3.1.0, 56 ops, 75 schemas) | **Auth:** Bearer, env `HERENOW_API_KEY` | **Free-plan-first** is shipping scope.

No competing CLI / MCP / SDK exists. "Absorb" = full correct coverage of the official API + the capabilities of here.now's own hosted skill, beaten with an offline SQLite mirror, agent-native output, and free-plan-aware behavior.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Publish a directory to a live URL (one command) | official here.now skill | (behavior in here-now-pp-cli publish <dir>) walks dir, inlines small text files, presigns + PUTs large/binary assets, finalizes, prints live URL | Single command for the whole inline+presign+finalize dance; records the publish locally for resume |
| 2 | Anonymous publish (no account) | official skill | (behavior in here-now-pp-cli publish --anon) | Captures the one-time claimToken into a local vault so the 24h-expiring site can be made permanent later |
| 3 | Claim an anonymous site | API claimAnonymousSite | here-now-pp-cli sites claim | Auto-reads the claimToken from the local vault — no manual token paste |
| 4 | Update / republish a site | API updateSite | (generated endpoint) sites update | Offline record, --dry-run, typed exit codes |
| 5 | Finalize a site version | API finalizeSiteVersion | (generated endpoint) sites finalize | Used by publish orchestration + resume |
| 6 | Refresh presigned upload URLs | API refreshSiteUploadUrls | (generated endpoint) sites refresh-uploads | Recovers expired upload targets during a long publish |
| 7 | Patch site metadata (title, password, visibility) | API patchSiteMetadata | (generated endpoint) sites metadata | --dry-run preview of the change |
| 8 | List sites | API listSites | (generated endpoint) sites list | Offline from local mirror; --json/--select/--csv |
| 9 | Get a site | API getSite | (generated endpoint) sites get | Offline; bounded --compact output |
| 10 | Delete a site | API deleteSite | (generated endpoint) sites delete | --dry-run, idempotent |
| 11 | Search sites (server q=) | API searchSites | (behavior in here-now-pp-cli search) | Offline FTS over the local mirror; cross-entity (sites + drive files + site-data), not just server-side site title match |
| 12 | Publish from a drive version | API publishFromDrive | (generated endpoint) sites from-drive | Pairs with drive sync to publish straight from synced content |
| 13 | List / read / delete / move drive files | API + official scripts/drive.sh | (generated endpoint) drives files ... | Beaten by the rsync-style drive sync/diff transcendence commands |
| 14 | Create / list / get / patch / delete drives | API | (generated endpoint) drives ... | Offline mirror, --json/--select |
| 15 | Drive presigned upload create + finalize, batch apply | API | (generated endpoints) used by drive sync | Orchestrated automatically by sync/push |
| 16 | Drive share tokens (list / create) | API | (generated endpoint) drives tokens ... | Scriptable token creation for agent-to-agent sharing |
| 17 | Site Data records CRUD | API | (generated endpoint) site-data ... | Mirrored locally for cross-site search + CSV export |
| 18 | Domains CRUD | API | (generated endpoint) domains ... | Offline list, --dry-run on create/delete |
| 19 | Handle (subdomain) CRUD | API | (generated endpoint) handle ... | Typed; paid-tier-aware messaging |
| 20 | Short links CRUD | API | (generated endpoint) links ... | Offline list of routes |
| 21 | Variables list / set / delete | API | (generated endpoint) variables ... | Secret values redacted in output |
| 22 | Profile + username + profile-listed sites | API | (generated endpoint) profile ... | Offline profile view |
| 23 | Account analytics (PAID) | API getAccountAnalytics | (generated endpoint) analytics account | Graceful paid-gate: clean "requires a paid plan" message + typed exit, never a raw 402 |
| 24 | Per-site analytics (PAID) | API getSiteAnalytics | (generated endpoint) analytics site | Same graceful paid-gate handling |
| 25 | Auth email-code login → API key | API request-code/verify-code | (behavior in here-now-pp-cli auth login) | Runs the two-step email-code flow and stores the key locally |
| 26 | Support request | API createSupportRequest | (generated endpoint) support create | Typed; --dry-run preview |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Score | Persona | Why Only We Can Do This |
|---|---------|---------|--------------|-------|---------|-------------------------|
| 1 | Claim-token vault + auto-claim | claims | hand-code | 9/10 | Mira | The claimToken is returned once at anonymous-publish time and never again by any API; only a local store can persist it to make the site permanent later |
| 2 | Expiry radar | claims expiring --within 6h | hand-code | 8/10 | Mira, Theo | Anonymous-expiry countdown is a pure local time-window query over the vault — no API endpoint reports "expiring soon" |
| 3 | Drive sync (sha256 diff push/pull) | drive sync ./dir --drive <id> | hand-code | 9/10 | Devin | Computes local sha256 vs the synced DriveFile.sha256 to upload only drift; the API offers per-file PUT but no diff |
| 4 | Drive diff (dry-run drift) | drive diff ./dir --drive <id> | hand-code | 7/10 | Devin | Read-only local-vs-remote comparison over the synced drive table; no single API call expresses "what differs from this folder" |
| 5 | Cross-site Site Data search | search "<q>" --type site-data | hand-code | 8/10 | Sara | FTS across every site's every collection requires the local mirror; the API only lists records one collection at a time |
| 6 | Free-plan usage meter | usage | hand-code | 8/10 | Theo + all free users | Rolls up synced site count, drive bytes, and recent-publish cadence against free-tier ceilings — the paid-analytics-free health signal, computed locally |
| 7 | Stale-site finder | sites stale --days 30 | hand-code | 7/10 | Theo | Time-window query over the local sites mirror to reclaim free-plan slots; no free API surface ranks sites by staleness |
| 8 | Publish resume (finish a half-done publish) | publish resume <slug> | hand-code | 7/10 | Mira | Detects a locally-recorded publish that uploaded but never finalized and completes it; recovery needs the in-flight state only the CLI persisted |

**Hand-code commitment: 8 of 8 transcendence features are `hand-code`** (each ~50–150 LoC + root.go wiring). 0 are spec-emitted. No stubs.

## Killed candidates (audit trail)
Site Data CSV export (folded into `site-data list --csv`), duplicate-site detector (unsynced version hash), plan-aware doctor (folded into framework `doctor`), password-gated audit (thin `--select`), backup-everything (scope creep), "what changed" feed (overlaps claims/stale/search), link/handle router map (paid+single on free), anonymous→permanent migrate (composite, handle is paid).
