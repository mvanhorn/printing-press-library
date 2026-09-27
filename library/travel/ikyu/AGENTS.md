# Ikyu CLI maintenance

This is a Printing Press accommodation CLI. The product is anonymous, read-only Japan accommodation discovery, exact room/plan comparison and canonical booking handoff.

## Work locally

Preserve existing work and shared tool configuration. Keep updates proportional to meaningful milestones. Batch independent, targeted reads; inspect bounded output. Run the smallest relevant tests after an edit, then the full required checks before acceptance. Use the version/tooling declared in `go.mod` and `Makefile`.

Use `stay --help` and leaf help as the command source of truth. [SKILL.md](SKILL.md) is the agent workflow; [README.md](README.md) covers local build and human use. For price, occupancy, bath or cancellation changes, read [docs/source-contract.md](docs/source-contract.md) before editing.

## Source correctness

Keep property, room, plan and offer identities separate. Preserve Japanese names and source string IDs. Missing values stay explicit. Price integers come from Ikyu; percentage-based reconstruction cannot replace them. Room amenities require room evidence. Compare only known compatible conditions. Validate returned dates/party because upstream can silently normalize invalid inputs.

Source clients must bound requests, response bodies, retries and cache growth. Preserve typed transport/schema/rate-limit errors and item-level partial failures. Real source denial must never become an empty success. Keep stdout compact JSON and diagnostics stderr.

## Generated code

Keep source-specific behavior in hand-authored packages/files and register novel commands through the generated hook. Keep `pp:data-source` directives and matching annotations honest; declare read-only MCP hints for public read workflows. Dry runs stop before network/cache writes. Source callers honor the command context and generated adaptive limiter.

Record code-level customizations in `.printing-press-patches/` as durable behavior contracts. The [Printing Press patch schema](https://github.com/mvanhorn/printing-press-library/blob/main/AGENTS.md#printing-press-patches-records-library-side-customizations) is authoritative when adding or updating those records. Documentation-only edits need no patch entry. Preserve source tests, live proofs and the run's approved scope during regeneration.

## Acceptance

For a behavior change, verify the affected source/semantic case and a meaningful failure case. Live checks must be read-only and use bounded future stays. A successful build or structural score does not establish correct prices or plan conditions. The published library’s post-merge automation owns release versions and changelog entries. Publish only when explicitly requested.
