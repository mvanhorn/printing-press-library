# Phase 17 security review — round 1
Review path: direct security reviewer subagent; read-only inspection of custom catalog, SEC, local snapshot/cache code and reachable generated endpoint auth plumbing.

## Findings

### P1 — Federal key sent to California by generated raw endpoints
- Paths: internal/cli/california_find.go:29 (flags.newClient and absolute California path); internal/client/client.go:2094 (unconditionally resolves auth); internal/client/client.go:2178 (sets X-Api-Key without comparing initial destination to federal origin).
- Trigger: operator configures DATA_GOV_API_KEY and invokes `california find`, `california dataset`, or another generated California command (also exposed through generated MCP endpoint tools).
- The custom govdata client is safe, but these reachable raw endpoint commands bypass it. `no_auth: true` in the source spec did not produce per-endpoint suppression. Redirect protection does not protect the initial cross-source request.
- Fix: prevent federal auth from being attached to California at the generated endpoint path, preferably a durable authored client hook/origin-scoped transport or correct spec tier-auth routing; verify with a recording transport. Treat the ignored no_auth shape as generator retro evidence rather than hand-patching the template blindly.

### P2 — Resource URL userinfo can become an unmasked credential
- Paths: internal/govdata/catalog.go:315-317 accepts any HTTPS URL including userinfo; internal/cli/gov_catalog.go:180 returns original URL in normal output.
- Trigger: `resource download --url https://user:password@host/file ...` accepts the URL; net/http converts URL userinfo to Basic Authorization, while the command emits the literal password-containing URL and can include it in errors. This public-data downloader has no need for embedded URL credentials.
- Fix: reject URLs with parsed.User != nil and redact userinfo from any returned URL/error. Add a test proving rejection occurs before transport.

### P3 — Intake manifest failure leaves a partial destination
- Path: internal/cli/gov_local.go:146-156.
- Trigger: disk-full/write or close error after O_EXCL creates the final manifest path. Command returns failure but leaves an incomplete intake artifact; retry refuses the existing destination.
- Fix: same-directory temp file plus successful close and exclusive hard-link publication, as already used by both download paths; remove temporary output on error.

## Checks without findings
- Custom govdata federal credentials are loaded through config.Load/AuthHeader and attached only for federal requests; custom resource downloads carry no federal header. Cross-origin redirects are refused.
- SEC_USER_AGENT is read only from configuration environment at request time, restricted to exact approved SEC hostnames, and redirects cannot leave those hosts. No contact value is embedded in source.
- Download destinations use explicit user paths, atomic same-directory temp publication, exclusive no-overwrite links, and size bounds.
- govcache SQL values use placeholders; no interpolated user SQL. Snapshot membership inserts are transactional and IDs immutable.
- --save checks localDataDisabled; root writer policy blocks sync. Existing local reads are intentionally still permitted by that policy.
- Known --output paths are automatically write-blocked in MCP; SEC documents uses read-only-unless-flags, intake lock is local-write, and resource download is not read-only-hinted.
