# Phase 4.85 Output Review Findings — sarvam-pp-cli

Status: WARN (1 finding)

- check: aggregation/error propagation
  severity: warning
  description: voices preview exits 0 even when every speaker request fails, so scripted pipelines cannot detect total failure from exit status
  suggestion: exit non-zero when all speakers failed (applied: partialFailureErr when zero files written)
  resolution: FIXED in polish — voices_preview.go now returns exit 6 when no audio was generated
