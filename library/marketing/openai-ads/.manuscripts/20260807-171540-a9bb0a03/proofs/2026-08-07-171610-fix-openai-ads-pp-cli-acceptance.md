# Acceptance Report: openai-ads

Level: Full Dogfood (live, real credentials)
Tests: 218/218 passed, 0 failed, 135 skipped
Gate: PASS

## Write-path note (important)
The operator explicitly chose "full matrix, real writes". The dogfood harness
nonetheless skipped every mutating command with reason
"mutating command dry-run only" — this is a built-in safety property of the
matrix, not an operator override. NO write was executed against the live ad
account.

Account state was snapshotted before the run and re-read after. Campaign,
ad group, and ad ids and statuses are byte-identical (all `active`).
VERDICT: account unchanged.

The agent also declined, unilaterally and with disclosure, to run `archive`
against the operator's only live campaign: archiving is plausibly irreversible
and exceeded the pause/activate cycle described in the chosen option.

## Failures found and fixed
- campaigns list-method happy_path FAILED "missing runnable example": the
  generated Example used the alias form (`campaigns list`) while the matrix
  probes the canonical path (`campaigns list-method`). Added
  pp:happy-args "--limit=2". Re-run: PASS.

## Legitimate skips (not failures)
- 135 skips, dominated by:
  - "mutating command dry-run only" (every write path)
  - "blocked-fixture: required API parameter" for ad-groups/ads list-method.
    These require campaign_id / ad_group_id. Supplying real ids would hardcode
    the operator's account ids into a publishable CLI, so they remain
    BLOCKED_FIXTURE by choice.
  - framework commands with non-id positionals (profile, learnings, teach).

## Live behavior verified against the real account
- doctor: auth configured, API reachable
- sync: 9 resources, hierarchical, parent linkage persisted
- tree / bid-check / orphans / drift / review-watch: real output
- pace / fatigue: empty insight tables -> empty results + honest note
- bid-check flags the real account at 3.33 implied clicks/day vs min 10

## Printing Press issues for retro
1. defaultSyncResources() omitted two syncable top-level resources while
   reporting success/errored:0.
2. Parent-keyed dependents support only PathTemplate; query-param parent
   scoping (campaign_id / ad_group_id) had no representation.
3. sync_summary reported success:9 errored:0 while a sync_error event fired.
4. probe-reachability returned standard_http for a Cloudflare 200-served
   challenge interstitial.
5. Generated novel-command scaffolds return help on bare invocation for
   commands with no required input, hiding available output.
6. root.go Short renders slug title-case ("Openai Ads") instead of
   narrative.display_name ("OpenAI Ads").
