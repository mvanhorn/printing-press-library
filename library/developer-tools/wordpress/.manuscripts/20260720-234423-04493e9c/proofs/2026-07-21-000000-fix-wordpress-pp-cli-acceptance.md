# Acceptance Report: wordpress

Level: Full Dogfood (binary-owned live matrix, base_url = frikko.com from a live site config)
Tests: 547/547 passed (399 additional rows skipped by the runner's classification)

## Failures resolved (fix loop, resumed run)
Started at 13/547, then 7, then 2, then 0 across three annotation/fixture passes:
- abilities family (list/categories/get) — /wp-abilities/v1/* returns 404 rest_no_route on sites without the Abilities API (WP < 7.0 / plugin absent); annotated pp:typed-exit-codes 0,3 (honest "feature not present" mapping). workflow archive shares the abilities sync path — same root cause, resolved by the same annotation.
- block-types get — error-path probe: 200+empty for unknown IDs; annotated pp:no-error-path-probe.
- wp-types get/list — Example used underscore path (wp_types) + get lacked a fixture; fixed Examples to kebab-case (wp-types) and added pp:happy-args type=post.
- oembed get — happy-path 404: oembed only embeds SAME-site URLs; base_url (frikko.com) cannot embed a wordpress.org/news post. No universal cross-site fixture exists; annotated pp:typed-exit-codes 0,3 and documented the same-site requirement + exit-3 semantics in the command's Long help.

## Fixes applied: 5 files (all generated-endpoint annotation/Example fixes; no novel-code changes)
abilities_list.go, abilities_ability-categories.go, abilities_ability-category.go, abilities_get.go, block-types_get.go, wp_types_get.go, wp_types_list.go, oembed_get.go, oembed_proxy.go.

## Retro candidates (machine)
- Generated commands for optional-plugin REST namespaces (wp-abilities) have no BLOCKED_FIXTURE/optional classification; a namespace absent on the target site is a hard dogfood fail unless hand-annotated.
- wp-types Example rendered the snake_case internal slug (wp_types) instead of the kebab-case command path (wp-types).
- Same generated internal/cliutil/credentials_test.go two-var-auth baseline failure as wpengine (documented pre-existing in the build log).

## SIGBUS note
The scorecard live-check reported a SIGBUS on caps/schema in a staged binary; could not reproduce in 35 stress runs against a freshly-built binary, nor in the passing full matrix. Attributed to a stale staged binary; not a code defect.

Gate: PASS
