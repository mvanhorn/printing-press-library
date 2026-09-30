<!-- slop-gate: off -->
# Phase 4.85 output review findings

Status: WARN (printing-press-output-review, forked). Review ran without credentials on an empty store plus a synthetic store.

1. recipient-domains ranking (warning, fixed): domains were compared against an account average that included themselves, so a domain with most of the volume could never be flagged. Fixed with a leave-one-out comparison (rest_of_account_bounce_rate_pct / rest_of_account_open_rate_pct) plus an absolute 5% hard-bounce floor; human table prints the account rate and the rule. Test: TestRecipientDomainsFlagsDominantDomain.
2. templates check with a missing --dir (minor, fixed): now a usage error that says to run `templates pull <dir>` first.
3. Retro candidate (machine): live_check isGracefulEmptyResponse treats OS "no such file or directory" usage errors as graceful empty passes, inflating live-check pass rates for commands whose examples take a path flag.
