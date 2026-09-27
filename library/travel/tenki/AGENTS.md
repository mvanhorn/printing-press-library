# tenki.jp Agent Guide

This is a generated Printing Press foundation with preserved hand-authored product code. Keep source parsing in `internal/tenki`, deterministic criteria in `internal/planning`, and command validation/rendering in `internal/cli`. Show controllers live in `places_show.go`, `mountain_show.go` and `seasonal_show.go`; `tenki_commands.go` owns groups, search and forecast controllers. Register novel commands through `registerNovelCommand`; preserve the generated root registration contract.

Read [SKILL.md](SKILL.md) when interpreting source times, hourly windows, mountain levels, seasonal years or comparison outcomes. It is the single interpretation reference. Use `<command> --help` for current runtime flags; [README.md](README.md) covers installation, local build, cache paths and troubleshooting. Read [VERIFICATION.md](VERIFICATION.md) when evaluating dated validation, scanner findings or reproduction steps.

Serialize source calls across CLI and MCP invocations. The provider limiter is per client; separate invocations do not coordinate. `--rate-limit` lowers only that invocation's ceiling.

Novel command files carry exactly one `// pp:data-source auto` (or deliberately narrower source) directive and matching command annotation. Read-only leaves use `mcp:read-only=true`. Help-only groups use `pp:parent-group=true` so MCP excludes the parent tool while traversing its children; unrelated generated utility paths use `mcp:hidden=true`. Use `boundCtx` before typed source calls and generated JSON selectors. Dry-run guards precede required-input validation and IO; real invalid inputs use typed usage errors. Source `429` errors propagate, and partial comparisons preserve each failed place separately.

Record code customizations under `.printing-press-patches/` using the [public library's guard contract](https://github.com/mvanhorn/printing-press-library/blob/main/AGENTS.md). Preserve those guards and `.manuscripts/` during reprints. Keep the permanent creator attribution from `.printing-press.json` and `NOTICE` unchanged.

Publish new CLIs and reprints through `/printing-press-publish`, with the generator's source-bound acceptance and manuscript checks. The published tree is `library/travel/tenki/`; catalog `registry.json` and `cli-skills/` mirrors are generated after merge. `CHANGELOG.md`, `.printing-press-release.json` and runtime release stamps belong to the post-merge release workflow; preserve existing ledger files on reprints and let that workflow assign versions.
