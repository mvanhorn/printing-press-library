# LawHub PP CLI

This package is a Printing Press-style Go/Cobra CLI for local-first LSAT analytics from LawHub.

Rules for future agents:

- Do not store LSAT question stems, passages, answer-choice text, or official explanations.
- Store only user-owned performance metadata, timing, correctness, question type/difficulty, links, and user-authored notes.
- Use `review open` to view official content in LawHub.
- Keep the public CLI Go-native; do not reintroduce silent Python fallbacks.
- Run `make test vet smoke` before claiming production readiness.
