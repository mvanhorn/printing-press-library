
=== verify ===
Fetching spec from https://ucp.dev/2026-04-08/specification/overview...
warning: spec parse error (attempt 1), cleaning: failed to unmarshal data: json error: invalid character '<' looking for beginning of value, yaml error: error converting YAML to JSON: yaml: line 237: mapping values are not allowed in this context
Error: running verify: loading spec: loading OpenAPI spec (even after cleanup): failed to unmarshal data: json error: invalid character '<' looking for beginning of value, yaml error: error converting YAML to JSON: yaml: line 237: mapping values are not allowed in this context
running verify: loading spec: loading OpenAPI spec (even after cleanup): failed to unmarshal data: json error: invalid character '<' looking for beginning of value, yaml error: error converting YAML to JSON: yaml: line 237: mapping values are not allowed in this context

=== validate-narrative ===
MISSING [quickstart]: ucp-pp-cli profile init → profile init
MISSING [quickstart]: ucp-pp-cli mock serve --port 8080 & → mock serve
MISSING [quickstart]: ucp-pp-cli check checkout.coffeecircle.com --json → check
MISSING [quickstart]: ucp-pp-cli search "coffee" --merchant localhost:8080 --limit 5 --json → search
MISSING [quickstart]: ucp-pp-cli cart add --merchant localhost:8080 --sku <sku> --qty 2 → cart add
MISSING [quickstart]: ucp-pp-cli checkout prep --cart $(ucp-pp-cli cart list --merchant localhost:8080 --json | jq -r '.[0].id') --json → checkout prep
MISSING [recipes]: ucp-pp-cli merchants diff checkout.coffeecircle.com localhost:8080 --json --select capabilities.added,capabilities.removed,transports.added,transports.removed → merchants diff
MISSING [recipes]: ucp-pp-cli search "french press" --merchants checkout.coffeecircle.com,localhost:8080 --agent --select results.title,results.price,results.merchant,results.url → search
MISSING [recipes]: ucp-pp-cli checkout preflight --cart cart_42 --ap2-dry-run --json → checkout preflight
MISSING [recipes]: ucp-pp-cli watch product --gtin 0123456789012 --merchants checkout.coffeecircle.com,localhost:8080 --threshold 10% --interval 6h --emit jsonl → watch product
MISSING [recipes]: ucp-pp-cli mock serve --port 8080 & ucp-pp-cli profile init && ucp-pp-cli check localhost:8080 --json → mock serve
DONE: 0 ok, 11 missing, 0 empty-words, 0 failed-examples, 0 unsupported
narrative validation failed

=== dogfood ===
dogfood: using spec /Users/dave/Work/openbrain/https:/ucp.dev/2026-04-08/specification/overview (--spec)
Using cached spec for https://ucp.dev/2026-04-08/specification/overview
warning: spec parse error (attempt 1), cleaning: failed to unmarshal data: json error: invalid character '<' looking for beginning of value, yaml error: error converting YAML to JSON: yaml: line 237: mapping values are not allowed in this context
Error: running dogfood: loading OpenAPI spec (even after cleanup): failed to unmarshal data: json error: invalid character '<' looking for beginning of value, yaml error: error converting YAML to JSON: yaml: line 237: mapping values are not allowed in this context
running dogfood: loading OpenAPI spec (even after cleanup): failed to unmarshal data: json error: invalid character '<' looking for beginning of value, yaml error: error converting YAML to JSON: yaml: line 237: mapping values are not allowed in this context

=== workflow-verify ===
Workflow Verification: ucp-pp-cli
================================

Overall Verdict: workflow-pass
  - no workflow manifest found, skipping

=== verify-skill ===
=== ucp-pp-cli ===
  ✘ 44 error(s), 11 likely false-positive(s)
    [flag-names] ucp-pp-cli search: --merchants is referenced in SKILL.md but not declared in any internal/cli/*.go
      evidence: SKILL.md
    [flag-names] ucp-pp-cli search: --rank is referenced in SKILL.md but not declared in any internal/cli/*.go
      evidence: SKILL.md
    [flag-names] ucp-pp-cli cart: --items is referenced in SKILL.md but not declared in any internal/cli/*.go
      evidence: SKILL.md
    [flag-names] ucp-pp-cli cart: --constraint is referenced in SKILL.md but not declared in any internal/cli/*.go
      evidence: SKILL.md
    [flag-names] ucp-pp-cli watch: --threshold is referenced in SKILL.md but not declared in any internal/cli/*.go
      evidence: SKILL.md
    [flag-names] ucp-pp-cli watch: --interval is referenced in SKILL.md but not declared in any internal/cli/*.go
      evidence: SKILL.md
    [flag-names] ucp-pp-cli checkout: --ap2-dry-run is referenced in SKILL.md but not declared in any internal/cli/*.go
      evidence: SKILL.md
    [flag-names] ucp-pp-cli mock serve: --variant is referenced in SKILL.md but not declared in any internal/cli/*.go
      evidence: SKILL.md
    [flag-names] ucp-pp-cli watch: --emit is referenced in SKILL.md but not declared in any internal/cli/*.go
      evidence: SKILL.md
    [flag-names] ucp-pp-cli search: --merchants is referenced in README.md but not declared in any internal/cli/*.go
      evidence: README.md
    [flag-names] ucp-pp-cli search: --rank is referenced in README.md but not declared in any internal/cli/*.go
      evidence: README.md
    [flag-names] ucp-pp-cli cart: --items is referenced in README.md but not declared in any internal/cli/*.go
      evidence: README.md
    [flag-names] ucp-pp-cli cart: --constraint is referenced in README.md but not declared in any internal/cli/*.go
      evidence: README.md
    [flag-names] ucp-pp-cli watch: --threshold is referenced in README.md but not declared in any internal/cli/*.go
      evidence: README.md
    [flag-names] ucp-pp-cli watch: --interval is referenced in README.md but not declared in any internal/cli/*.go
      evidence: README.md
    [flag-names] ucp-pp-cli checkout: --ap2-dry-run is referenced in README.md but not declared in any internal/cli/*.go
      evidence: README.md
    [flag-names] ucp-pp-cli mock serve: --variant is referenced in README.md but not declared in any internal/cli/*.go
      evidence: README.md
    [flag-names] ucp-pp-cli watch: --emit is referenced in README.md but not declared in any internal/cli/*.go
      evidence: README.md
    [flag-commands] ucp-pp-cli search: --merchants is not declared anywhere
      evidence: SKILL.md
    [flag-commands] ucp-pp-cli search: --rank is not declared anywhere
      evidence: SKILL.md
    [flag-commands] ucp-pp-cli cart: --items is not declared anywhere
      evidence: SKILL.md
    [flag-commands] ucp-pp-cli cart: --constraint is not declared anywhere
      evidence: SKILL.md
    [flag-commands] ucp-pp-cli watch: --gtin is declared elsewhere but not on watch
      evidence: SKILL.md
    [flag-commands] ucp-pp-cli watch: --threshold is not declared anywhere
      evidence: SKILL.md
    [flag-commands] ucp-pp-cli watch: --interval is not declared anywhere
      evidence: SKILL.md
    [flag-commands] ucp-pp-cli checkout: --ap2-dry-run is not declared anywhere
      evidence: SKILL.md
    [flag-commands] ucp-pp-cli mock serve: --variant is not declared anywhere
      evidence: SKILL.md
    [flag-commands] ucp-pp-cli watch: --merchants is not declared anywhere
      evidence: SKILL.md
    [flag-commands] ucp-pp-cli watch: --emit is not declared anywhere
      evidence: SKILL.md
    [flag-commands] ucp-pp-cli search: --merchants is not declared anywhere
      evidence: README.md
    [flag-commands] ucp-pp-cli search: --rank is not declared anywhere
      evidence: README.md
    [flag-commands] ucp-pp-cli cart: --items is not declared anywhere
      evidence: README.md
    [flag-commands] ucp-pp-cli cart: --constraint is not declared anywhere
      evidence: README.md
    [flag-commands] ucp-pp-cli watch: --gtin is declared elsewhere but not on watch
      evidence: README.md
    [flag-commands] ucp-pp-cli watch: --threshold is not declared anywhere
      evidence: README.md
    [flag-commands] ucp-pp-cli watch: --interval is not declared anywhere
      evidence: README.md
    [flag-commands] ucp-pp-cli checkout: --ap2-dry-run is not declared anywhere
      evidence: README.md
    [flag-commands] ucp-pp-cli mock serve: --variant is not declared anywhere
      evidence: README.md
    [flag-commands] ucp-pp-cli watch: --merchants is not declared anywhere
      evidence: README.md
    [flag-commands] ucp-pp-cli watch: --emit is not declared anywhere
      evidence: README.md
    [positional-args] ucp-pp-cli mock serve: got 4 positional args; Use: "serve" expects 0–0
      evidence: SKILL.md: & ucp-pp-cli profile init
    [positional-args] ucp-pp-cli mock serve: got 1 positional args; Use: "serve" expects 0–0
      evidence: README.md: &
    [positional-args] ucp-pp-cli mock serve: got 4 positional args; Use: "serve" expects 0–0
      evidence: README.md: & ucp-pp-cli profile init
    [unknown-command] ucp-pp-cli watch: command path not found in internal/cli/*.go (no matching Use: declaration)
      evidence: bash recipe (SKILL.md)
    [positional-args] ucp-pp-cli cart: got 1 positional args; Use: "cart" expects 0–0  [likely false positive]
      evidence: SKILL.md: optimize
    [positional-args] ucp-pp-cli merchants: got 3 positional args; Use: "merchants" expects 0–0  [likely false positive]
      evidence: SKILL.md: diff checkout.coffeecircle.com etsy.com
    [positional-args] ucp-pp-cli checkout: got 1 positional args; Use: "checkout" expects 0–0  [likely false positive]
      evidence: SKILL.md: preflight
    [positional-args] ucp-pp-cli merchants: got 3 positional args; Use: "merchants" expects 0–0  [likely false positive]
      evidence: SKILL.md: diff checkout.coffeecircle.com localhost:8080
    [positional-args] ucp-pp-cli checkout: got 1 positional args; Use: "checkout" expects 0–0  [likely false positive]
      evidence: SKILL.md: preflight
    [positional-args] ucp-pp-cli profile: got 1 positional args; Use: "profile" expects 0–0  [likely false positive]
      evidence: README.md: init
    [positional-args] ucp-pp-cli cart: got 1 positional args; Use: "cart" expects 0–0  [likely false positive]
      evidence: README.md: optimize
    [positional-args] ucp-pp-cli merchants: got 3 positional args; Use: "merchants" expects 0–0  [likely false positive]
      evidence: README.md: diff checkout.coffeecircle.com etsy.com
    [positional-args] ucp-pp-cli checkout: got 1 positional args; Use: "checkout" expects 0–0  [likely false positive]
      evidence: README.md: preflight
    [positional-args] ucp-pp-cli merchants: got 3 positional args; Use: "merchants" expects 0–0  [likely false positive]
      evidence: README.md: diff checkout.coffeecircle.com localhost:8080
    [positional-args] ucp-pp-cli checkout: got 1 positional args; Use: "checkout" expects 0–0  [likely false positive]
      evidence: README.md: preflight
  ✓ canonical-sections passed

=== scorecard ===
Using cached spec for https://ucp.dev/2026-04-08/specification/overview
Error: running scorecard: parsing OpenAPI YAML spec: yaml: line 237: mapping values are not allowed in this context
running scorecard: parsing OpenAPI YAML spec: yaml: line 237: mapping values are not allowed in this context

Shipcheck Summary
=================
  LEG               RESULT  EXIT      ELAPSED
  verify            FAIL    4         763ms
  validate-narrative  FAIL    1         35ms
  dogfood           FAIL    3         21ms
  workflow-verify   PASS    0         19ms
  verify-skill      FAIL    1         629ms
  scorecard         FAIL    3         76ms

Verdict: FAIL (5/6 legs failed)
