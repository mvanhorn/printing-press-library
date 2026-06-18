# Verification

- `./cli-printing-press generate --name linq --spec catalog/specs/linq-openapi.yaml --dry-run --json`: parsed 37 endpoints / 19 resources.
- generated CLI quality gates passed during generation.
- `go test ./...`: pass.
- `./cli-printing-press verify --dir /Users/knox/printing-press/library/linq --spec catalog/specs/linq-openapi.yaml --fix --cleanup --json`: 33/33 pass.
- `./cli-printing-press scorecard --dir /Users/knox/printing-press/library/linq --spec catalog/specs/linq-openapi.yaml --json`: Grade A, 89.
- `./cli-printing-press dogfood --dir /Users/knox/printing-press/library/linq --spec catalog/specs/linq-openapi.yaml --json`: PASS.

Sandbox live smoke is blocked pending human email/phone verification to obtain a sandbox key; no real patient numbers were used.
