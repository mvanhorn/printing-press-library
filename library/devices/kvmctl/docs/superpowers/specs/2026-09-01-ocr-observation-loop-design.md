# OCR Observation Loop Design

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Give an LLM planner grounded, fail-closed screen perception and bounded KVM actions without embedding BIOS knowledge or a goal planner in `kvmctl`.

**Architecture:** The LLM owns intent and chooses each next step. `kvmctl` owns fresh screen capture, real OCR, immutable observation evidence, exact/unique text resolution, observation-bound HID execution, and post-action verification. It never translates a goal such as “enable IOMMU” into BIOS navigation or reports invented OCR/UI state.

**Tech Stack:** Go; KVMD streamer snapshot and HID APIs; existing `internal/ocr` contracts; optional external OCR executable invoked only when configured; CLI/MCP semantic dispatcher; SHA-256 evidence hashes.

---

## Scope

The first slice adds four primitives to the semantic CLI/MCP surface:

- `kvm_observe`: capture a fresh snapshot, perform real OCR, and return an observation ID, image hash, timestamp, image dimensions, and bounded OCR regions.
- `kvm_click_text`: accept `text`, `observation_id`, and `--yes`; recapture the current screen, reject it if its image hash differs from the referenced observation, require exactly one OCR match above threshold, then click the text region’s center.
- `kvm_press_key`: accept `key`, `observation_id`, and `--yes`; recapture and require the same current image before sending the bounded HID key tap.
- `kvm_verify_text`: capture a fresh observation and report whether one expected text assertion and optional forbidden assertions are satisfied. It does not mutate hardware.

All returned observations and actions are evidence envelopes with operation, timestamp, source observation ID, image hashes, candidate/match data, and failure reason. A mutation requires `--yes` through the existing CLI and `write_enabled` through the dispatcher/MCP path.

## Explicit non-goals

- No direct BIOS-setting API.
- No LLM API key, prompt, planner, or autonomous multi-step controller in `kvmctl`.
- No OCR fallback that fabricates text, coordinates, dimensions, or success when OCR is unavailable.
- No stale-screen coordinate action.
- No click selection heuristic when OCR has zero or multiple eligible matches.
- No remote write in an observe/verify command.

## OCR runtime contract

`internal/ocr` remains the pure analysis boundary. A production engine implementation invokes an explicitly configured OCR executable (`KVMCTL_OCR_COMMAND`) with a bounded input/output protocol, or returns a typed `ocr_unavailable` result when it is absent/fails. It must return image dimensions and word-level text, confidence, and bounding boxes; stdout is parsed as a documented JSON schema. Test engines are injected and do not stand in for live OCR claims.

This avoids choosing a platform-specific OCR library or silently depending on a globally installed Tesseract binary. `kvmctl observe` must expose the actual engine name/version (when available) and the source image hash. It must not return a placeholder such as `ocr-bytes-N`.

## Observation validity

An observation is canonical JSON, SHA-256 identified, and includes:

```json
{
  "observation_id": "sha256:<hash>",
  "captured_at": "RFC3339Nano UTC",
  "image_sha256": "<hex>",
  "width": 1920,
  "height": 1080,
  "ocr": {"engine": "…", "regions": [{"text":"Advanced","confidence":95.2,"box":[…],"pixel":[…]}]}
}
```

The process-local observation store is bounded (TTL ≤60 seconds, bounded entry count) and defensive-copies data. An action receives the observation ID; `kvmctl` retrieves it, captures a new image, hashes it, and refuses if it differs. The store does not contain credentials. An expired/missing observation is rejected.

For exact state stability, a byte-level image hash is used in v1. This deliberately fails closed on video noise or cursor movement. A future policy can add perceptual comparison only with separate safety evidence and tests.

## Click and key execution

`click-text` resolves case-insensitive substring matching only in the referenced observation. It uses the existing ≥30 confidence floor plus a configurable high-confidence default (80) for physical action. The match count must equal one. The click target is derived from the OCR box’s center; no center-of-screen default exists. The current-image preflight happens immediately before HID input. Mouse press and release errors are propagated; a release is attempted after a successful press.

`press-key` is intentionally smaller: accepted key name, fresh-observation preflight, HID down/up, and evidence. It does not infer hotkeys or navigation sequences.

## Verification

`verify-text` always obtains a new observation. It accepts a required `expect_text` and optional bounded `forbid_text[]`. Its output distinguishes `matched`, `missing_expected`, `forbidden_present`, `ocr_unavailable`, and `capture_failed`. It never calls HID APIs and does not claim that a BIOS value changed—only that observed OCR text meets the requested assertion.

## CLI and MCP surface

Expose dedicated human commands under `kvmctl-pp-cli observe`, `act click-text`, `act press-key`, and `verify-text`, plus equivalent semantic/MCP operations. Each command prints structured JSON with `--agent`/`--json`. Help includes examples showing the observation ID flow. `act` commands require `--yes`; the runtime MCP metadata must classify them as external writes.

## Testing and validation

Use strict TDD. Tests must first fail against the current placeholder implementation and exercise real package boundaries using `httptest` snapshots plus injected OCR engines/command fixtures.

Required cases:

1. observe emits OCR text/boxes and hashes from actual snapshot bytes; OCR absence/failure is explicit, never fabricated.
2. text click rejects missing, expired, altered, ambiguous, and low-confidence observations without emitting HID requests.
3. a unique text match produces the exact box-center HID mouse request only after a fresh-image hash match and explicit write authorization.
4. key press rejects stale observations and sends down/up only after authorization and hash match.
5. verify-text returns correct expected/forbidden states from a fresh observation and is never write-gated.
6. all evidence/journal output redacts secrets and bounds text/regions.
7. CLI help/flag behavior and MCP registration agree with semantic behavior.
8. live acceptance remains separate: run one read-only observation against KVMD only after source changes settle; no BIOS action or key/mouse write is performed as part of acceptance.

Full gates: `gofmt -l .`, focused package tests, `go test ./...`, `go vet ./...`, `go build ./...`, race test for the observation store, Windows/Linux/macOS cross-builds, `git diff --check`, Printing Press package/policy validators, and fresh Phase 5/live proof only if source fingerprint rules require it.

## Files expected to change

- `internal/ocr/`: observation model, production-engine boundary, store, and tests.
- `internal/semantic/dispatch.go`: replace placeholder OCR handlers; add observation/action/verification operations.
- `internal/semantic/*_test.go`: dispatcher and HTTP/HID integration tests.
- `internal/cli/semantic_novel.go` and/or dedicated novel command file: flags, help, and action gating.
- `internal/mcp/semantic_novel.go`: typed metadata and write classification.
- `README.md`, `SKILL.md`, and `.printing-press-patches/<id>.json`: documented contract and durable generated-tree patch record.

## Risks and trade-offs

- OCR quality is hardware/font dependent. The v1 contract fails closed; the LLM receives uncertainty and selects the next safe observation, not a guessed click.
- Screenshot byte hashes are strict. This may stop actions on animated/noisy screens, which is preferable to acting on stale evidence.
- A generic OCR command runner introduces an external dependency; it is opt-in, schema-validated, timeout-bounded, and never an implicit placeholder.
- Cross-process observation reuse is intentionally excluded in v1. The observation ID is a short-lived proof tied to the running process and current screen.
