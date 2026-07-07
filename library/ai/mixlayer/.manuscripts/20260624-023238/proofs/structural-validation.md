# Structural Validation

Run ID: `20260624-023238`
Validated at: `2026-06-24T02:49:18Z`

Local validation completed without using a Mixlayer API key:

```text
go test ./...
```

Result: pass.

Representative smoke checks exercised the novel local layer:

```text
mixlayer-pp-cli shield scan
mixlayer-pp-cli shield redact
mixlayer-pp-cli shield restructure
mixlayer-pp-cli vault list
mixlayer-pp-cli models query "ctx>=128k tools reasoning"
mixlayer-pp-cli sql
```

The model query smoke confirmed the seeded cache includes documented Qwen rungs plus console-visible Qwen 3.6 IDs. Live `/models` refresh is gated on `MIXLAYER_API_KEY` and should be exercised during Phase 5 live dogfood.
