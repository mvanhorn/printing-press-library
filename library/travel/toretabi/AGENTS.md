# Toretabi Printed CLI Agent Guide

This generated CLI implements four read-only Japan rail-ticket paths. Follow the source and output contract in [SKILL.md](SKILL.md) and [README.md](README.md). Treat shared framework defects as Printing Press issues; keep source-specific patches narrow and recorded.

## Runtime truth

```bash
toretabi-pp-cli doctor --json
toretabi-pp-cli agent-context --pretty
toretabi-pp-cli which "ticket periods" --json
toretabi-pp-cli tickets get --help
toretabi-pp-cli tickets get tokai_043 --agent
```

Provider operations are anonymous GETs. Never add bookings, payments, credentials, account writes or messages. Ticket commands accept stable IDs, not arbitrary URLs. Domain live calls must use `boundCtx(cmd.Context(), flags)` and pass the context into the source client before any request.

## Evidence boundaries

Keep publisher and linked official operator observations separate, with Japanese snippets, source URLs and their original retrieval clocks. Explicit endpoints/validity can be corroborated or conflicting; HTTP200/title matching cannot confirm all rules. Preserve unknown/inaccessible/PDF/unsupported/archive states. Traveler eligibility, calendar exceptions, full train coverage and inventory remain unresolved. Vouchers are benefits, not fares. Do not replace missing source prices with estimates.

Use the normal bounded SQLite cache: 100 records, 32 KiB payloads, 50 output rows and an 8 MiB read bound. Reads do not create/migrate or refresh. Transactions must retain committed WAL visibility, and delayed older saves cannot replace newer observations. Avoid unrelated framework or adversarial filesystem work.

## Novel command data sources

Each handwritten domain command file declares one `// pp:data-source` strategy, and its Cobra annotation must agree. List is live, get/compare honor auto/local/live, cached is local. Reject incompatible choices. Test actual MCP tools through the sibling CLI; raw generated HTML handlers must not supersede normalization. Compare MCP input `ids` is a comma-separated string.

## Local learning

Optional recall/teach actions only touch the separate local framework store. Try returned IDs live before using saved guidance; candidates need a successful trial before confirmation. Use concrete ticket IDs and resource type `ticket`:

```bash
toretabi-pp-cli recall "Hokkaido free ticket conditions" --agent
toretabi-pp-cli teach --query "Hokkaido free ticket conditions" --resource-type ticket --resource hokkaido_028
toretabi-pp-cli learnings candidates --agent
```

Pass arbitrary user text as data via argv/MCP, never embed it into shell source. `--no-learn` or `TORETABI_NO_LEARN=true` disables journaling. The local schema stamp can prevent an older binary opening a newer store.

## Release ledger and customizations

Preserve `CHANGELOG.md` and `.printing-press-release.json` if they exist. Fresh prints keep runtime `0.0.0-dev`; the public library workflow owns version assignment after merge. Never hand-bump release files or version declarations.

Record source-specific changes under `.printing-press-patches/` using the public library AGENTS.md schema. These records are reprint guards: explain the enduring invariant and cover the actual changed files. Build final CLI/MCP peers and MCPB from the same pinned source, then prove their hashes and embedded dependency versions. Root Go minimum is 1.26.6; this run uses Go1.27.1, SQLite module1.60.1 and libc1.77.1.
