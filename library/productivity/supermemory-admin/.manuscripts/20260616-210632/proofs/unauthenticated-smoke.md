# Supermemory Admin Unauthenticated Smoke

No real Supermemory API token was available in this runner, so Phase 5 live calls were skipped. The CLI still verified the two safe paths that do not require sending credentials.

## Doctor

Command:

```bash
go run ./cmd/supermemory-admin-pp-cli doctor --agent
```

Result:

- API base URL reachable.
- Config path resolved.
- Auth correctly reported as not configured.
- Missing credential hint uses `SUPERMEMORY_ADMIN_TOKEN`.

## Recall Dry Run

Command:

```bash
go run ./cmd/supermemory-admin-pp-cli supermemory-recall post-v4-search --q test --agent --dry-run
```

Result:

- Built `POST https://api.supermemory.ai/v4/search`.
- Body included `q`, `limit`, `searchMode`, and `threshold`.
- No request was sent.
- JSON envelope returned `{ "dry_run": true }`.
