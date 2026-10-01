# Pocket Concierge CLI

Keep this CLI public and read-only. Use only pocket-concierge.jp first-party English/Japanese data. No arbitrary GraphQL, mutation, booking, payment, authentication, browser-launch or unrelated providers.

For source/schema changes, read evidence/research.md and preserve source restaurant/course/session IDs, null missing fields, JPY guest/group units, policy text, waitlists, reservation requests and instant confirmation separately. English page language is not evidence of staff language. Never calculate a payable total from conflicting statements.

Run `go test -count=1 ./...`, `go vet ./...`, and `go build ./...` after consequential changes. For live source changes run `python3 tools/live_e2e.py` using a built CLI and an isolated cache. Live proofs must state dates/IDs and cannot substitute fixtures. Update evidence/final.md with limits and measured cached/uncached requests, bytes, latency and peak memory.

Printing Press provenance and live acceptance are in .manuscripts/. Keep changes scoped to this CLI; leave generated registry/catalog artifacts and release accounting to library automation. Reviewers make no edits.
