# JMA CLI maintenance

Read [source research](evidence/RESEARCH.md) before changing endpoint paths, warning codes, units, forecast time interpretation or typhoon joins. Current warnings use `/warning/data/r8/`; the legacy endpoint can return HTTP 200 with stale data.

Keep JMA-only read access, compact bounded JSON and provenance under projection. Resolution follows first-party IDs/hierarchy. Missing/unknown warning records remain incomplete; explicit no-warning records apply per product and municipality. Analysis, estimate and uncertain forecast remain distinct. Update deterministic domain tests for consequential parsing/state changes and verify affected behavior against live JMA; label synthetic fixtures separately.

Author domain code in `internal/jma` and the preserved command hook `internal/cli/jma_commands.go`. Generated framework utilities are hidden from focused help. Use run-local GOCACHE and isolated `--cache-dir` / absolute `--home` for verification; keep shared config unchanged. Read [canonical paths](evidence/PATHS.md) before staging or promotion and preserve receipt sequencing. No publishing/PR workflow is authorized by this build.
