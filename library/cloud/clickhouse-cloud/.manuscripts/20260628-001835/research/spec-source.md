# Spec Source

The ClickHouse Cloud CLI was generated from the official OpenAPI document served by ClickHouse:

- URL: https://api.clickhouse.cloud/v1
- Retrieval date: 2026-06-28
- Archived file: spec.json
- SHA-256: 4d19c18f1583a4edde4d9623dd1b08201269a9b7e14fddd9de21bca93b5765ab

Two Slack webhook-shaped example URLs from the official document were redacted to `https://example.invalid/slack-webhook` in the archived spec to avoid publishing secret-scanner-triggering sample values. The endpoint paths, schemas, auth model, and generated command surface are unchanged.

The spec declares Basic authentication. The CLI therefore accepts either:

- `CLICKHOUSE_CLOUD_USERNAME` plus `CLICKHOUSE_CLOUD_PASSWORD`
- `clickhouse-cloud-pp-cli auth set-token <key-id> <key-secret>`

No live ClickHouse Cloud credentials are included in the package.
