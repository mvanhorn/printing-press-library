# ClickHouse Cloud Research

## Contribution Fit

ClickHouse Cloud is a data engineering platform for operating ClickHouse services, ingestion pipelines, backups, private endpoints, API keys, members, and usage reporting. The generated CLI fits the Printing Press public library as a cloud category print that supports data platform operations.

## API Source

- Official spec URL: https://api.clickhouse.cloud/v1
- Spec title: OpenAPI spec for ClickHouse Cloud
- OpenAPI version: 3.1.2
- Server: https://api.clickhouse.cloud
- Archived spec path: spec.json
- Spec checksum: sha256:4d19c18f1583a4edde4d9623dd1b08201269a9b7e14fddd9de21bca93b5765ab

## Generated Surface

- Cobra command tree: 175 defined commands, 175 registered commands
- MCP tools: 109
- Auth model: Basic auth using ClickHouse Cloud API key id and API key secret
- Primary data-engineering workflows:
  - Service inventory and lifecycle inspection
  - ClickPipes ingestion pipeline operations
  - Backup bucket and backup metadata operations
  - Private endpoint and reverse private endpoint inspection
  - Organization usage cost and Prometheus metrics access

## Post-Generation Notes

The official spec includes two backup-bucket request bodies with oneOf/anyOf shapes. Printing Press generated safe `--body-json` fallbacks for those operations. Two generated mutation commands also carried unused helper imports after this fallback handling; those imports were removed and recorded in `.printing-press-patches/`.

The generated auth command was adjusted to persist the official Basic auth pair (`<key-id> <key-secret>`) instead of a one-token placeholder. This keeps `auth setup`, `auth set-token`, env-var setup, doctor output, and `AuthHeader()` behavior aligned with the ClickHouse Cloud API.

Two Slack webhook-shaped example URLs in the archived spec were replaced with `https://example.invalid/slack-webhook` so the public package does not carry secret-scanner-triggering sample values. The API schema shape is otherwise unchanged.
