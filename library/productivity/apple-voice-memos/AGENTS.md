# Apple Voice Memos Printed CLI Agent Guide

This directory is a local Printing Press-style CLI for Apple Voice Memos. It is not yet a published Printing Press Library entry.

Start with runtime truth:

```bash
bin/apple-voice-memos-pp-cli doctor --agent
bin/apple-voice-memos-pp-cli agent-context --pretty
```

Use `--agent` for JSON output and stable scripting:

```bash
bin/apple-voice-memos-pp-cli recent --limit 10 --agent
bin/apple-voice-memos-pp-cli transcript <id> --agent
```

Do not mutate the Apple Voice Memos database. The CLI is intentionally read-only except for `export`, which copies audio files to a user-selected destination.
