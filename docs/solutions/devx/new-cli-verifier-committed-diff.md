---
name: new-cli-verifier-committed-diff
date: 2026-06-22
problem_type: knowledge
category: devx
component: .github/scripts/verify-publish-package/verify_publish_package.py
applies_when: preparing a new library CLI PR and checking publish-shape validation locally
tags: [printing-press, verifier, manuscripts, new-cli]
---

# New CLI publish validation checks committed diffs and requires a shipcheck or acceptance proof filename

## Applies when

You are preparing a new `library/<category>/<slug>/` CLI package and running the published-library verifier before opening the PR.

## The pattern

Run `verify_publish_package.py` after the package exists in `HEAD`, not only while it is staged or untracked. The verifier selects changed CLI directories from the git diff against the base ref, so a staged-only new package can produce:

```text
No changed library CLI packages to validate.
```

Once committed, the same new package is selected and strict new-CLI checks run. For manuscripts, the verifier does not only require files under `.manuscripts/<run-id>/proofs/`; at least one proof filename must include `acceptance` or `shipcheck`.

Use a concrete proof path like:

```text
library/<category>/<slug>/.manuscripts/<run-id>/proofs/<run-id>-shipcheck.md
```

## Why

The verifier is designed for PR-time behavior, where the diff is a committed head SHA against a base SHA. Staged and untracked files are local working-tree state and are intentionally invisible to that comparison.

The proof filename rule prevents placeholder manuscript directories from satisfying publish shape. A file named `local-verification.md` can contain good evidence, but the verifier's artifact classifier specifically looks for `acceptance` or `shipcheck` in at least one proof filename.

## Example

```bash
# After committing the new package:
python3 .github/scripts/verify-publish-package/verify_publish_package.py --base-ref upstream/main
```

If it reports:

```text
new library CLI has manuscripts for run_id <run-id>, but proofs/ does not contain an acceptance or shipcheck artifact.
```

add a real proof:

```text
.manuscripts/<run-id>/proofs/<run-id>-shipcheck.md
```

Then amend or commit and rerun the verifier.

## Counter-cases

This does not apply to already-published CLI bugfixes unless the PR also adds a new `.printing-press.json`. Existing CLI edits have different expectations, such as patch accrual under `.printing-press-patches/` for hand-edited generated source.
