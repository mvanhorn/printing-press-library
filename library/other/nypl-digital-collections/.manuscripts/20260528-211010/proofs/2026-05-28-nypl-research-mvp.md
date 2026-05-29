# NYPL Digital Collections research MVP proof

Commands verified locally without exposing credentials:

```text
go test ./...
go vet ./...
go build ./...
go build -o ./bin/nypl-digital-collections-pp-cli ./cmd/nypl-digital-collections-pp-cli
stories dossier "Anne Boleyn" --dry-run --markdown --per-cluster 1
stories discover "Anne Boleyn" --dry-run --json --per-cluster 1
search run-plan /tmp/nypl-plan.json --dry-run --json --limit 2
workspace init tudor --dir /tmp/nypl-workspaces --json
workspace add-run tudor /tmp/nypl-run.json --dir /tmp/nypl-workspaces --json
```

Live NYPL calls were not run in this proof because the API token is intentionally kept out of committed artifacts and tool logs.
