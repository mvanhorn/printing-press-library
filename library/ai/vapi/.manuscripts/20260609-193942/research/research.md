# Vapi Private CLI Adopted-Source Research Note

This private Vapi CLI was adopted from the proven `feat/private-vapi-cli`
source that previously lived only in the private generator worktree on knox.
No archived `research.json` or full manuscripts were available locally or on
knox for the original 2026-06-09 print.

The implementation was therefore intentionally adopted and extended rather
than reprinted from the Vapi OpenAPI spec. Reprinting without prior research
would degrade to a fresh spec-only print and risk dropping the bespoke
`dial` and `juno` workflows.

Preserved and extended private capabilities:

- `dial`
- `juno call`
- `juno reservation`
- `juno order`
- `juno quote`
- `juno followup`
- `juno status`
- `juno setup`
- `juno test-call`
- `juno report`
- `juno phone-numbers`
- `juno assistant`
- recording enabled by default for `dial` and Juno outbound calls, with
  `--no-record` as the explicit opt-out

The `.printing-press-patches/` entry records the durable customization contract
that future reprints must preserve.
