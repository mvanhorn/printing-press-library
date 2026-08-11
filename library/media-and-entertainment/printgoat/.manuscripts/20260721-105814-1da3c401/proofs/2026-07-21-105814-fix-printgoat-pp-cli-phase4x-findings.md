# Phase 4.7-4.95 Findings and Fixes

## Phase 4.7 (sync param-drop): skipped — no traffic-analysis.json exists (no browser-sniff was used; all 3 sources were hand-authored from research).

## Phase 4.8 (Agentic SKILL review) + 4.9 (README/SKILL/AGENTS audit)

Autofix summary: 7 error-severity findings fixed in place (fabricated `auth set-token` command replaced with real env-var instructions across SKILL.md/README.md; fabricated `credentials.toml` persistence claim removed; incomplete exit-code table completed with codes 4/6; broken `--select results.name,...` example fixed; `job download`/`job resume` Thingiverse-only limitation disclosed; frontmatter/README/root.go headline overpromise on Cults3D download qualified; root.go brand-casing and truncated banner fixed). Source-of-truth `research.json` narrative.headline updated to match.

Template-shape retro candidate: the generic auth-narrative template text ("OAuth2 app token", "credentials.toml", legacy config.toml migration) appears to be boilerplate not fully aligned with a CLI whose auth is entirely env-var/host-dispatched (no generated `auth` command family exists for this spec shape) — worth flagging to the Printing Press maintainers via retro, since any CLI using the `host_auth.go`-style custom auth pattern would hit the same doc/reality mismatch.

## Phase 4.85 (Agentic output review)

1 finding: JSON output HTML-escaped `<`/`>` as `<`/`>` in placeholder hints (e.g. `<model-key>`). Root cause: Go's `encoding/json` HTML-escapes by default; the CLI's output pipeline had ~15 separate `json.Marshal`/`json.NewEncoder` call sites across `helpers.go` and 8 command files, several of which re-marshal after an earlier step had already produced clean output. Fixed by introducing `marshalJSONNoEscape` (SetEscapeHTML(false)) and replacing every output-producing `json.Marshal(` call with it — global fix, not per-site patching, since any single missed intermediate re-marshal reintroduces the bug for callers of that path. Verified via `history diff`/`designer stats` after fix.

## Phase 4.95 (Local code review)

Review path: direct Agent tool dispatch (single agent covering correctness + security + maintainability lenses; no working-dir-shaped review skill was scoped for this session's harness).

**Autofix summary:** 8 findings fixed in place in round 1.
- HIGH (security): path traversal via unsanitized upstream file name in `job_download.go`'s job-file path construction — fixed by routing `ModelID`/`FileName` through the existing `sanitizeFilename` helper.
- MEDIUM (security): `sanitizeFilename`'s `".."` guard was incomplete (only checked `""`/`"."`/separator) — fixed to also reject a literal `".."` base name.
- MEDIUM-HIGH (correctness): Thingiverse's proxied `download_url` never actually received the Bearer token in either download path (`download.go`, `job_download.go` both used a bare `*http.Client`) — fixed by exporting `Client.AuthHeaderForURL` and attaching it in both `downloadResumable` and `downloadFileResumable`. Verified live: a real `download thingiverse:763622` pulled 24 files through the auth-requiring `api.thingiverse.com/v2/files/.../download` URL successfully.
- MEDIUM (correctness): `job download`'s file-completion check didn't verify transferred bytes against the known size, so a clean-but-short connection close could mark a truncated file "done" — fixed by threading the source's reported file size through to `downloadFileResumable` and treating `total < expectedSize` as an incomplete-transfer error.
- LOW (maintainability): dead code (`novelResult.key()`, `getBool`) removed.
- LOW (maintainability): truncated mid-sentence CLI banner fixed.
- Maintainability: divergent Thingiverse file-URL preference order between `models_shared.go` (direct_url first) and `novel_shared.go` (download_url first) — aligned both to prefer the unauthenticated CDN `direct_url` first, matching the documented/correct convention.

**Not fixed (accepted as low-risk for a single-user CLI):** `nextJobID`'s TOCTOU race between concurrent `job download` invocations computing the same job id — low likelihood for a personal CLI's typical usage pattern; would need a transaction or PK-conflict retry to close fully. Documented here rather than silently dropped.

Convergence: findings cleared after round 1 (build/vet/test/gofmt clean; live download re-verified; full shipcheck re-run PASS 7/7 after all fixes).
