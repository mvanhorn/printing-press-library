# 1Password Agent Secrets CLI Brief

## API Identity
- Domain: 1Password developer tooling for vault items, documents, Environments, service accounts, and secret references.
- Backing: official `op` CLI v2.18+ and the official Go SDK; not 1Password Connect.
- Auth: `OP_SERVICE_ACCOUNT_TOKEN` for least-privilege automation, or desktop-authenticated `op` for local operator approval.

## Reachability Risk
- Low for metadata and secret-reference workflows when `op` is installed and authenticated.
- Service-account caveat: 1Password documents that `OP_CONNECT_HOST` and `OP_CONNECT_TOKEN` take precedence over `OP_SERVICE_ACCOUNT_TOKEN`; this CLI must warn and refuse to behave like a Connect wrapper.
- Rate-limit caveat: service accounts have hourly and daily request limits. `op item list`, `op item get`, and `op read` can make multiple requests; broad audits should prefer `--vault` and IDs where possible.

## Official Surface Notes
- Service-account supported commands include `op read`, `op inject`, `op run`, `op service-account ratelimit`, `op vault create`, plus `op item` and `op document` with `--vault` when multiple vaults are accessible.
- Secret references use `op://<vault>/<item>/[section/]field[?attribute=...]`. OTP and SSH key format query parameters are documented.
- SDK item management supports create, get, update, delete, archive, list, batch operations, file/document operations, and item sharing. File/document SDK operations have a 50 MB message-size limit.
- SDK item sharing can create share links with expiry, recipient validation, one-time-view settings, and account-policy validation; service accounts need share permission.
- Environments can be read by SDK or CLI. `op run --environment` and `op run --env-file` load secrets into subprocess environments with masking by default.

## Top Workflows
1. Resolve fuzzy agent requests to exact `op://` references without values.
2. Check policy before any value reveal, file injection, process execution, or share.
3. Audit vault metadata for duplicates, ownership gaps, stale items, misplaced cards/documents/API keys, and suspicious document names.
4. Summarize current service-account scope and quota before broad actions.
5. Plan `op inject` / `op run` execution from env files and commands before executing.

## Product Thesis
- Name: 1Password Agent Secrets.
- Why it should exist: normal `op` is excellent for humans and scripts, but agents need a safety layer that favors exact references, metadata-only planning, explicit reveal gates, and service-account least privilege.

## Build Priorities
1. `op` auth/status and Connect-env conflict detection.
2. Metadata-only reference resolution and audits.
3. Policy/preflight gates before values, injection, sharing, or subprocess runs.
4. Service-account scope and rate-limit awareness.
5. Clear unsupported-state reporting where the current CLI/SDK cannot inspect existing share links.

## Sources
- https://www.1password.dev/sdks.md
- https://github.com/1Password/onepassword-sdk-go
- https://www.1password.dev/service-accounts/use-with-1password-cli.md
- https://www.1password.dev/service-accounts/rate-limits.md
- https://www.1password.dev/cli/reference.md
- https://www.1password.dev/cli/reference/commands/run.md
- https://www.1password.dev/cli/reference/commands/inject.md
- https://www.1password.dev/cli/reference/commands/read.md
- https://www.1password.dev/cli/reference/management-commands/item.md
- https://www.1password.dev/cli/reference/management-commands/document.md
- https://www.1password.dev/cli/reference/management-commands/service-account.md
