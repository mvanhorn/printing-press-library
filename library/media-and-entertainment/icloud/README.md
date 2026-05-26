# icloud-pp-cli

Query your iCloud data from the command line. Reads your Mac's local databases
directly — no Photos.app launch, no API token, no network calls.

**[icloudcli.com](https://icloudcli.com)** · macOS · Apache-2.0

---

## Install

The recommended path installs both the `icloud-pp-cli` binary and the `pp-icloud` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install icloud
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install icloud --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install icloud --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install icloud --agent claude-code
npx -y @mvanhorn/printing-press-library install icloud --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/icloud/cmd/icloud-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/icloud-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-icloud --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-icloud --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-icloud skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-icloud. The skill defines how its required CLI can be installed.
```

## Quick start

```bash
icloud-pp-cli doctor                            # verify Photos + Messages access
icloud-pp-cli photos top                        # top 25 heaviest files
icloud-pp-cli messages list-chats --limit 10    # 10 most-recently-active chats
icloud-pp-cli messages search "lunch"           # search your message history
```

Pipe any command for automatic JSON:

```bash
icloud-pp-cli photos top | jq '.[0:5]'
icloud-pp-cli messages list-chats --agent | jq '[.[] | select(.is_group)]'
```

---

## Commands

```
icloud-pp-cli
  photos
    top         Top N heaviest files (--limit, --type all|photo|video)
    videos      Largest videos (--limit, --year, --month)
    storage     Breakdown by media type and year
    stats       Total items and library size
    delete      Move items to Recently Deleted (requires --confirm)
    download    Export originals to a local folder
  messages
    list-chats  Chats ordered by most-recent activity (--limit, --since, --include-empty)
    search      Full-text search of message bodies (--chat, --handle, --from-me, --since, --until, --limit)
    stats       Total messages / chats / handles + by-year + top handles
    export      Export a chat or all chats to JSON (--chat, --out, --since, --until)
  doctor        Pre-flight: System / Library / Assets / Messages
```

All commands accept: `--json` `--compact` `--no-color` `--agent`.
`photos` commands also accept `--library PATH`; `messages` commands accept `--messages-db PATH`.

`--agent` sets `--json --compact --no-color` in one flag — use it in AI workflows.

## Messages: Full Disk Access required

Reading `~/Library/Messages/chat.db` requires macOS Full Disk Access for the
terminal app invoking the binary. If a messages command fails with
"Full Disk Access not granted," open System Settings > Privacy & Security >
Full Disk Access, add your terminal, quit and reopen the terminal, and rerun.

`doctor` reports this automatically — the Messages section shows green when
FDA is granted and a yellow warning (with remediation) when it is not.

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
