# Loops API research

Source: [Loops API introduction](https://loops.so/docs/api-reference/intro), [official OpenAPI 1.22.2](https://app.loops.so/openapi.json), and [official Loops CLI](https://loops.so/docs/cli), reviewed 2026-10-02.

The official CLI already exposes the broad API surface, JSON output, auth profiles, and an optional idempotency key. The public OpenAPI contains 68 operations for contacts, lists, segments, events, campaigns, email messages, transactional email, workflows, themes, metrics, and supporting resources. The printed CLI preserves that coverage and adds exact team verification, default redacted write previews, explicit send and publication intent, a duplicate-send journal, and compact lifecycle and campaign preflight reads.

The public API does not provide a bulk contact list or general segment membership export. Product usage cohorts require another authorized source, such as PostHog, and a private identity join. No account notes, contacts, credentials, or account identifiers are included in this manuscript.

Potential further agent workflows are listed in [WORKFLOWS.md](../../../WORKFLOWS.md). Only the three commands labeled implemented there are part of this print.
