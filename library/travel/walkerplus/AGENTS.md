# Walkerplus contributor guide

The approved product is five bounded, read-only Walkerplus workflows. Keep Japanese source facts separate from derived trip matches. Use `README.md` for user behavior and `SKILL.md` for the agent discovery workflow.

Core domain, transport, parsing, catalogs and date logic live in `internal/walkerplus`. Commands, flags, output and error mapping live in `internal/cli/walkerplus_*.go`; the generated Printing Press framework remains for build and verification support. Custom command hooks replace sample endpoint discovery and hide unrelated framework commands. Keep custom HTTP commands annotated `pp:data-source live`, use `boundCtx`, and preserve offline dry-run before I/O and required-input validation.

Event projections retain query, coverage and provenance. Unknown facts are null and empty collections are arrays. Tests must assert semantic results, negative matches and bounded requests; a successful exit alone is insufficient. Schedule confidence requires source evidence, not a JSON-LD envelope.

Run `go test ./...` and `go vet ./...` after integrated changes. Live acceptance and cold/warm measurements are documented in the README. Publication is now user-authorized through Printing Press; shared global credentials/configuration remain outside scope, and the publication workflow owns release-version stamping.
