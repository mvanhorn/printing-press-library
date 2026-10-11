# Phase 4.95 local code review (direct review of hand-written code; go vet clean)
Autofix summary: 3 findings fixed in place across 1 round (hardcoded America/Los_Angeles replaced by property/env time zone; `today --dry-run` now emits a dry-run envelope instead of empty stdout; live fan-outs no longer throttled to 2 req/s by the auto pacer).
Reviewed: units.go (registry lock, env/file/store/live sources, error messages), hostfully_ops_register.go (sync PreRunE chaining), client GetNoRetry hook, jobs/jobs_list split, quote read-intent POST.
Retro candidates (generator):
- Generated internal/mcp tests depend on HOSTFULLY_API_KEY being unset.
- dogfood novel-feature scanner ignores addNovelCommandIfAbsent wiring for two-word novel commands.
- Seven endpoint positional-arg helpers are emitted but unused for specs without positional endpoints, then fail dogfood; regen re-emits them.
- spec global_params (env-backed agencyUid) are ignored, so sync needs a hook to scope requests.
