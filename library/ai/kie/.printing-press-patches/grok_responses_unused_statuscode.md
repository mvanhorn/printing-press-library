# Patch: unused statusCode in grok_responses.go

Generated code declared `statusCode` from `PostWithParamsAndHeaders` but never used it,
failing `go vet`/`go build` with "declared and not used". Changed to `_` at the
`data, _, err := c.PostWithParamsAndHeaders(...)` call in `newGrokResponsesCmd`.
Reprint-safe: re-apply this same one-line change after any regen until Printing Press
fixes the generator template upstream.
