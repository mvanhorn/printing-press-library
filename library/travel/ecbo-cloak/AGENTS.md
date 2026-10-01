# ecbo cloak CLI maintenance

Read README.md for domain semantics and evidence/final.md for verified source coverage. Focused product commands live in internal/cli/ecbo_commands.go; internal/ecbo contains first-party read transport and normalization. Generated framework files retain their original attribution.

Changes to provider access must stay read-only: GET search/facility data, POST price/validate only. Preserve listed-vs-available distinctions, null missing cutoffs, source timestamps, Japanese identity and canonical URLs. Treat source shape changes as errors; never guess remaining capacity or totals.

Use targeted deterministic tests for consequential input/parsing/cache/state behavior and live reads for source correctness. Run go test -count=1 ./... and go vet ./... with isolated caches. Rerun live dogfood after source edits before local promotion. Keep global config unchanged. Regeneration is previewed in isolated staging; preserve hand-authored provider files and command adapters.
