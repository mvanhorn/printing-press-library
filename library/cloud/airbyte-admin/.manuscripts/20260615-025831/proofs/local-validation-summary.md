# Airbyte Admin Validation Summary

Validation used Airbyte's official Public API OpenAPI specification from `airbytehq/airbyte-platform` and local/mock-safe commands. No Airbyte Cloud token, self-managed username/password, workspace ID, customer name, or private endpoint was committed.

## Checks

- Generated from `airbyte-api/server-api/src/main/openapi/public_api.yaml`.
- `go mod tidy` completed after generation.
- `go build ./...` passed.
- `go vet ./...` passed.
- `go test ./internal/config` passed for Authorization header loading.
- Full `go test ./...` was rerun with local listener permission because generated `httptest` tests bind loopback ports.
- `airbyte-admin-pp-cli --help` returned the command tree.
- `airbyte-admin-pp-cli --version` returned the runtime version.
- `airbyte-admin-pp-cli public get-health-check --dry-run --agent` produced the expected read-only request shape.
- `airbyte-admin-pp-cli public list-sources --dry-run --agent` produced the expected read-only request shape.
- `airbyte-admin-pp-cli public list-destinations --dry-run --agent` produced the expected read-only request shape.
- `airbyte-admin-pp-cli public list-jobs --dry-run --agent` produced the expected read-only request shape.

## Auth Notes

The upstream OpenAPI spec does not declare a security scheme. The CLI keeps the generated read-only endpoint surface and adds optional Authorization support through:

- `AIRBYTE_ADMIN_TOKEN` for bearer-token auth.
- `AIRBYTE_ADMIN_AUTH_HEADER` for a complete header such as `Basic ...`.
- `auth_header` in the config file for local profiles.

This lets Airbyte Cloud and protected self-managed Airbyte deployments work without storing credentials in the repository.
