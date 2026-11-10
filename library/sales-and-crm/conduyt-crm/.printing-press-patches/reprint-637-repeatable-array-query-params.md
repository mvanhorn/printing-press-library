# Repeatable array query parameters, and no literal --cf-key (board #637 round 2, 2026-10-30)

Generalises contacts-list-email-param.md. Printing Press 4.28.0 prints an exploded form-style array query parameter as a plain string
flag sent once (`tag=a,b`), so `--tag a --tag b` kept only the last value and a list went out as one comma-joined value. Every public
query parameter whose schema is an array is now a repeatable `StringArrayVar` flag, flattened by `appendRepeatableArrayQueryParam`
(plain value, comma list or JSON array per occurrence; a JSON array keeps a value that contains a comma) and sent as one key per value;
a supplied-but-empty value is a usage error before any request.

On spec.yaml at reprint 637 that is 9 parameters on 3 commands: `email`, `tag`, `tagId` (flags `--email`, `--tag`, `--tag-id`) on
`contacts list` (GET /contacts), `contacts get-count` (GET /contacts/count) and `contacts export` (GET /contacts/export).
`TestEveryArrayQueryParamIsRepeatable` (internal/cli/array_query_params_test.go) reads every array query parameter from spec.yaml, finds
its command by the pp:method / pp:path annotations, and proves the flag is a stringArray and that three occurrences reach the API as
four repeated keys. A parameter a future spec adds fails it until it is made repeatable the same way.

The literal `--cf-key`: the document's deep-object parameter `cf[<key>]` (one parameter per custom-field key, `cf[<fieldKey>]=value`)
was printed as a flag named `cf-key` that sent the literal key `cf[<key>]`, which the API never reads. It is not registered on any
command, and the `cf[<key>]` bindings are removed from `internal/mcp/code_orch.go` and `tools-manifest.json`. The documented filter
stays `--cf` (`cf`: comma-separated `fieldKey=value` pairs or a JSON object; values exact, `__notempty__` or `__empty__`).
Pinned by `TestLiteralCfKeyFlagIsAbsent`. The MCP executor already sends array values as repeated keys (`codeOrchAddArrayQuery`).

Round 4: the spec scan resolves $ref chains with cycle protection and finds array schemas through oneOf / anyOf / allOf and type lists
(still 9 parameters on this spec.yaml).
