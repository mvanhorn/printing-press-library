# icloud-pp-cli

Query your iCloud data from the command line. Reads your Mac's local databases
directly — no Photos.app launch, no API token, no network calls.

**[icloudcli.com](https://icloudcli.com)** · macOS · Apache-2.0

---

## Install

```bash
go install github.com/matysanchez/icloudcli/cmd/icloud-pp-cli@latest
```

Or via [Printing Press](https://github.com/mvanhorn/printing-press-library):

```bash
npx -y @mvanhorn/printing-press install icloud
```

**Requires:** macOS (Sonoma / Sequoia), Go 1.23+

---

## Quick start

```bash
icloud-pp-cli doctor              # verify library is readable
icloud-pp-cli photos top          # top 25 heaviest files
icloud-pp-cli photos storage      # breakdown by type and year
icloud-pp-cli photos stats        # total size + item count
```

Pipe any command for automatic JSON:

```bash
icloud-pp-cli photos top | jq '.[0:5]'
```

---

## Commands

```
icloud-pp-cli
  photos
    top        Top N heaviest files (--limit, --type all|photo|video)
    videos     Largest videos (--limit, --year, --month)
    storage    Breakdown by media type and year
    stats      Total items and library size
  doctor       Verify Photos library is readable
```

All commands accept: `--json` `--compact` `--no-color` `--agent` `--library PATH`

`--agent` sets `--json --compact --no-color` in one flag — use it in AI workflows.

---

## Repository layout

```
icloudcli/
  cmd/icloud-pp-cli/   Go binary entry point
  internal/cli/        Command implementations and Photos SQLite reader
  web/                 Landing page (deployed to icloudcli.com via Cloudflare Pages)
  go.mod               module: github.com/matysanchez/icloudcli
```

### Submitting to Printing Press

To submit a snapshot to [printing-press-library](https://github.com/mvanhorn/printing-press-library):

1. Fork the library repo
2. Copy `cmd/`, `internal/`, `go.mod`, `go.sum`, `LICENSE`, `SKILL.md`, `.printing-press.json` into `library/media/icloud/`
3. Update `go.mod` module to `github.com/mvanhorn/printing-press-library/library/media/icloud`
4. Update the import in `cmd/icloud-pp-cli/main.go` to match
5. Open a PR with commit message: `feat(icloud): add icloud-pp-cli`

---

## Contributing

Issues and PRs welcome. This repo is the source of truth — the Printing Press
submission is a periodic snapshot with its module path updated.

## License

Apache-2.0
