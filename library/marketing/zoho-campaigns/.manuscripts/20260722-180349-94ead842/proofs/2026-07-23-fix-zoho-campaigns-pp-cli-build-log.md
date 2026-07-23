Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

# zoho-campaigns Build Log

## Built
- Priority 0 (data layer): hand-authored history tables in `internal/store/zoho_campaigns_history.go` — campaign_report_snapshots, list_count_snapshots, recipient_actions, recipient_action_syncs. Lazy-init from the commands that need them (regen-durable).
- Priority 1 (absorbed): all 30 endpoint commands generator-emitted from the corrected, live-verified spec (campaigns 11, contacts 14, tags 5). Framework sync/search/sql/analytics included.
- Priority 2 (transcendence, all 6/6 hand-coded, live-verified against the Kontur org):
  - delta (pp:data-source local) — snapshot diff per campaign; honest one-snapshot note on day one
  - digest (auto) — org rollup + snapshot writer for reports and list counts; verified vs 3 real campaigns (44-campaign org)
  - growth (local) — list count trends from snapshots
  - engagement (auto) — cross-campaign contact ranking; verified real ranks (received/opened/clicked over 6 campaigns)
  - bounce-audit (auto) — 67 real bounced contacts found; pipeable into contacts do-not-mail
  - journey (local) — 14-campaign chronological history for a real contact, fully named/dated
- Registered `auth set-token` (generated constructor was emitted unregistered); aligned root.Short/spec cli_description with narrative.headline.

## Live API truths discovered during build (encoded in spec + code)
- `getcampaignrecipientsdata` silently truncates to 20 rows; undocumented `fromindex`/`range` params paginate (live-verified). Spec updated; ensureRecipientActions paginates at 500/page with a 4000-row cap.
- Framework `sync` does not apply spec param defaults, so bare sync got XML and silently parsed 0 records. Fixed by embedding `?resfmt=JSON` in the five sync-eligible spec paths (client query-merge handles override cleanly). Bare `sync` now returns 44 campaigns.
- Zoho error envelopes come back HTTP 200 + status:"error"; zohoErrorCode() guards every hand-coded fetch.

## Windows machine bugs found and fixed in-place (all retro candidates)
1. `AtomicWritePrivateFile` Chmod(0600) is a DACL no-op on Windows → `VerifyCredsPerms` rejected the credentials file the CLI itself wrote → every command 401'd after any token refresh. Fixed: hand-authored `internal/cliutil/private_file_windows.go` / `_unix.go` (RestrictPrivateFile — protected owner-only DACL applied to the temp file pre-rename) + 1-line hook in generated paths.go (re-apply after regen).
2. Generated tests sandbox `HOME` but not `USERPROFILE`; on Windows os.UserHomeDir() reads USERPROFILE, so tests leaked into the real config/state and failed (mcp, learn, cliutil, cli/teach). Fixed: USERPROFILE added beside every HOME setenv; credential fixtures also RestrictPrivateFile'd.
3. Generated `newAuthSetTokenCmd` emitted but never registered for oauth2_refresh specs (dogfood: unregistered command).

## Deferred / known gaps
- Mailing lists don't cache into the generic resources table (no extractable ID field — Zoho's key is `listkey`; the syncer's ID heuristics miss it). Live queries and all novel commands unaffected (list data lives in list_count_snapshots). Retro: spec-level `id_field` hint.
- `contacts fields` uses `type=json` (Zoho inconsistency), left as documented.
- delta/growth need ≥2 snapshots to trend; both report the single-snapshot state honestly.

## Inline edits to generated files (re-apply after any regen; scripts in session)
- internal/cliutil/paths.go — RestrictPrivateFile hook
- 7 test files — USERPROFILE sandbox + credential fixture ACLs
- internal/cli/auth.go — set-token registration
- internal/cli/root.go — Short aligned to narrative.headline (also fixed at spec source)
