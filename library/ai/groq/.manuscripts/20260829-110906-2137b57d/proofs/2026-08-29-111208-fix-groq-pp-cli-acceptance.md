# Groq CLI Live Dogfood Acceptance Report

Level: Full Dogfood
API: Groq Cloud (live, user-provided API key)

## Result

- **Gate: PASS** — `phase5-acceptance.json` status `pass`, level `full`, matrix 122/122 passed, 0 failed, pass_rate 100%.

## What was exercised live

- `doctor`, auth flow, config resolution.
- All read endpoints: models list/retrieve, files list/retrieve/download, batches list/get, fine-tunings list/get (tier-gated → skipped as BLOCKED_FIXTURE).
- Write-side happy paths: chat completions (real completion), embeddings, reranking, audio speech (dry-run binary guard verified), audio transcribe (real transcription of fixture tone), files upload (multipart).
- Novel features: rate-limits (reads real x-ratelimit-* headers), costs (real ledger aggregation), batch validate (fixture validation, 2 valid / 1 invalid), audio batch (real transcription), compare (real multi-model run, verified manually), batch diagnose (plan-gated → skipped as BLOCKED_FIXTURE).

## Fixtures used

- `testdata/sample-audio.wav` — 1s 440Hz tone; whisper transcription succeeds.
- `testdata/sample-batch.jsonl` — 3 lines (2 valid, 1 missing model).

## Known plan limitations (not CLI defects)

- **Batch API** (batches create/list/get/cancel + `batch diagnose`) requires a Groq **Developer plan**; the test account is Free tier → `not_available_for_plan`. Commands behave correctly (surface the real API error). Classified BLOCKED_FIXTURE in dogfood.
- **Fine-tuning endpoints** are closed beta → same plan gating; marked `x-live-dogfood-requires-tier`.

## Fixes applied during dogfood

1. Added `x-live-dogfood-requires-tier` to batch + fine-tuning spec operations so the runner classifies them as BLOCKED_FIXTURE instead of hard-failing.
2. Fixed fine-tuning example command paths (`fine_tunings` → `fine-tunings`) via `x-pp-example`.
3. Added testdata fixtures + updated audio batch / batch validate examples.
4. Guarded `audio speech` binary-render rejection under `--dry-run` (dry-run returns a JSON envelope, not audio).
5. Added `Example:` to the `feedback` parent command (was missing → dogfood help check failed).
6. Regenerated a proper non-silent WAV tone fixture (silent audio was 400-rejected by whisper).

## Printing Press issues for retro

- Generator example for `x-pp-resource`-renamed resources uses the operationId-based path (`fine_tunings`) instead of the renamed resource — examples must be re-authored via `x-pp-example`.
- Framework `feedback` parent command lacks an `Example:` section (dogfood help check requires one).
- Generated binary endpoints (audio speech) reject `--json` even under `--dry-run` (dry-run envelope is JSON, not binary).
