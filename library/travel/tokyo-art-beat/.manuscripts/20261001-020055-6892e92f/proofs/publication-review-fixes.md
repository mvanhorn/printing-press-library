# Publication review fixes

All four first-round Greptile findings are addressed. Selection-aware detail testing retains a hidden venue closure flag and reports schedule uncertainty. The artist scan regression distinguishes all 52 fetched candidates from continuation offset 4. Dry-run regressions reject invalid local inputs/cache modes while a validation-only client stops before all cache and source I/O. The unverified optional Homebrew tap configuration is removed and its unavailability documented.

Focused regressions reproduced the original failures and pass after correction. Final Go build, vet, tests, both documentation verifiers, reachable govulncheck and publish validation pass. Full live matrices ran after source edits; the final packaged source passed 52/52 executed checks with 0 failures and 14 inapplicable/skipped probes. No credential or upstream-access skip was used. Raw publish transcripts were deleted from private temporary storage.
