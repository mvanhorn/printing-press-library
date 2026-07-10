# Apple Voice Memos CLI

A private-by-default, read-only command-line interface for Apple Voice Memos on macOS.

It lists and searches recordings, extracts Apple’s embedded transcripts, exports selected audio, and refreshes the local iCloud-backed store without leaving the Voice Memos app running.

> [!WARNING]
> Apple does not publish the Voice Memos database or embedded transcript format as a public API. This project detects incompatible schemas and fails safely, but a future macOS update may require a compatibility release.

## Why this exists

Apple Voice Memos has no official CLI and no interface on iCloud.com. Existing tools usually re-transcribe audio or export files without metadata. This CLI works locally with the data already synchronized by macOS:

```text
~/Library/Group Containers/group.com.apple.VoiceMemos.shared/Recordings/CloudRecordings.db
~/Library/Group Containers/group.com.apple.VoiceMemos.shared/Recordings/*.{m4a,qta}
```

## Privacy and safety

- The Voice Memos database is opened in SQLite read-only and query-only mode.
- The CLI makes no application-level network requests.
- Fresh refreshes trigger `voicememod` and may launch a hidden Voice Memos instance. The CLI cleans up only a single process whose PID, executable, and start identity match the instance observed after its own launch request. Ambiguous process ownership is treated as an error rather than broad cleanup.
- Apple’s own `voicememod` process performs iCloud synchronization.
- No recording is modified or deleted.
- `export` is the only command that writes user data. It copies one selected recording to a destination you choose.
- Voice Memos may be launched hidden during a freshness operation. The CLI terminates only the process instance it launched.

See [SECURITY.md](SECURITY.md) for the complete security model.

## Requirements

- macOS with Voice Memos opened at least once
- iCloud synchronization enabled for Voice Memos if cross-device recordings are needed
- Full Disk Access for the terminal or agent invoking the CLI when macOS requires it

## Install

### Release archive

Download the archive for Apple Silicon (`darwin_arm64`) or Intel (`darwin_amd64`) from GitHub Releases, then place the binary on your `PATH`:

```bash
install -m 0755 apple-voice-memos-pp-cli ~/.local/bin/apple-voice-memos-pp-cli
```

### From source

```bash
git clone https://github.com/0xble/apple-voice-memos-pp-cli.git
cd apple-voice-memos-pp-cli
make install
```

## Quick start

```bash
# Verify access and schema compatibility
apple-voice-memos-pp-cli doctor --json

# Refresh iCloud-backed state and list the newest recordings
apple-voice-memos-pp-cli recent --limit 5

# Search cached historical metadata
apple-voice-memos-pp-cli list --search "meeting" --json

# Extract Apple’s embedded transcript
apple-voice-memos-pp-cli transcript 293

# Copy one recording to another directory
apple-voice-memos-pp-cli export 293 --out ~/Downloads
```

## Commands

| Command | Behavior |
|---|---|
| `doctor` | Checks database access and schema compatibility |
| `sync` | Refreshes through `voicememod`, with a hidden-app fallback |
| `recent` | Refreshes by default, then lists newest recordings |
| `list` | Lists or searches cached metadata. Pass `--fresh` to refresh first |
| `transcript` | Extracts Apple’s embedded transcript from a recording’s `tsrp` atom |
| `export` | Copies a selected recording media file to an output directory |
| `which` | Maps a capability description to a command |
| `agent-context` | Emits machine-readable CLI capabilities |

Run `apple-voice-memos-pp-cli <command> --help` for command-specific flags.

## Machine-readable output

`--agent` enables JSON, disables color, and avoids interactive behavior. JSON from `list` and `recent` is wrapped in a versioned envelope so callers can distinguish cached reads from refresh attempts:

```json
{
  "schema_version": 1,
  "freshness": {
    "mode": "fresh",
    "result": {
      "refreshed": true,
      "changed": false,
      "freshness_confirmed": false,
      "warning": "store did not change during the sync window; it may already be current"
    }
  },
  "memos": []
}
```

`freshness_confirmed: false` is not proof of stale data. It means the local store did not visibly change during the observation window, so the CLI cannot prove that iCloud had nothing newer.

## Freshness and synchronization

`recent` treats “recent” as a freshness promise:

1. Trigger the built-in `com.apple.voicememod` LaunchAgent.
2. Watch the SQLite database and WAL for changes.
3. If the daemon does not refresh the store, launch Voice Memos hidden.
4. Wait for synchronization.
5. Terminate only the Voice Memos process launched by the CLI.

Use local cached data immediately when that is explicitly acceptable:

```bash
apple-voice-memos-pp-cli recent --cached --limit 10
```

Historical `list` queries remain cached by default:

```bash
apple-voice-memos-pp-cli list --fresh --after 2026-07-01
```

Synchronization timing is configurable:

```bash
apple-voice-memos-pp-cli sync \
  --daemon-wait 3s \
  --app-wait 12s \
  --settle-wait 2s \
  --poll-interval 500ms
```

A store that does not change may already be current. The CLI reports this uncertainty instead of claiming that new data does not exist.

## Transcript extraction

Recent Apple Voice Memos recordings may contain an on-device transcript in an ISO-BMFF `tsrp` atom. Apple currently uses media filenames including `.m4a` and `.qta`. `transcript` extracts that JSON and formats timestamped text. It does not upload or re-transcribe audio.

If Apple has not embedded a transcript, the command reports:

```text
tsrp transcript atom not found
```

## Agent usage

Use `--agent` for JSON, no color, and non-interactive defaults:

```bash
apple-voice-memos-pp-cli recent --limit 5 --agent
apple-voice-memos-pp-cli transcript 293 --agent
apple-voice-memos-pp-cli agent-context --pretty
```

The repository also includes a focused `SKILL.md` for compatible agents.

## Development

```bash
make check
```

The test suite creates synthetic SQLite databases and synthetic ISO-BMFF/M4A atoms. Real recordings, transcripts, titles, UUIDs, and databases must never be committed.

## Acknowledgments

The embedded transcript extraction approach was informed by Jesse Collis’s [`apple-voice-memos`](https://github.com/jessedc/apple-voice-memos). See [Third-Party Notices](THIRD_PARTY_NOTICES.md).

## License

MIT © Brian Le

Apple and Voice Memos are trademarks of Apple Inc. This project is independent and is not affiliated with or endorsed by Apple.
