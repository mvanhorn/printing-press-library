# Dub Polish Report

**Date:** 2026-05-02
**Skill:** `cli-printing-press:printing-press-polish` (forked context)
**Verdict:** `ship` (no fixes applied; all gates already passing)

## Delta

```
                         Before    After     Delta
  Scorecard:             86/100    86/100    0
  Verify pass rate:      100%      100%      0
  Dogfood:               PASS      PASS      —
  Verify-skill:          0 errors  0 errors  —
  Workflow-verify:       pass      pass      —
  Tools-audit findings:  0         0         0
  Go vet:                0         0         —
```

## Skipped findings (intentional, not defects)

- **`mcp_token_efficiency 4/10`** — structural. The spec does not opt into the new `mcp:` surface (intent tools, code-orchestration, `endpoint_tools: hidden`). Lifting requires generator/spec change, not polish.
- **`mcp_surface_strategy 2/10`, `mcp_remote_transport 5/10`, `mcp_tool_design 5/10`, `cache_freshness 5/10`, `type_fidelity 3/5`, `auth_protocol 8/10`** — same structural class.
- **`verify` `qr` and `since` scored 2/3** — environmental harness flakes (qr returns binary PNG; `since` optional-positional dry-run output doesn't contain the substring probe). Not CLI defects.
- **README install path uses `library/other/dub`** — the publish step rewrites this; not a polish target.
- **Public library divergence:** the public copy at `library/marketing/dub` has post-publish edits (`campaigns`, `domains report`, `tail`, `analytics_retrieve`, `timehelpers` commands plus new test files) that postdate the last publish. Polish chose not to sync because syncing public→internal would overwrite the fresh regen. Will surface again at `/printing-press publish` time.

## Remaining issues

None.

## Ship recommendation

`ship`.
