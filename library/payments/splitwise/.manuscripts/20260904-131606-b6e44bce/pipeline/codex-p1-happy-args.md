# Task: verifier fixtures for four hand-written novel commands (splitwise-pp-cli, Go)

Repo: this working directory is a generated Go CLI (module `splitwise-pp-cli`). Only touch these files:
`internal/cli/fairness_nudge.go`, `internal/cli/ledger.go`, `internal/cli/settle_up.go`, `internal/cli/split.go`.
Do NOT change behavior, flags, or any other file. Do not run the CLI against `~/.local/share/splitwise-pp-cli/data.db`.

## Why
`cli-printing-press publish validate` fails with "phase5 acceptance has hollow coverage for: fairness nudge, ledger,
settle-up, split": the live dogfood matrix skips their happy paths because a non-id positional name at depth 0 gets no
fixture. The verifier reads `cmd.Annotations["pp:happy-args"]`: tokens separated by unescaped `;`, in order; positional
tokens MUST be `label=value` (the label is discarded, the value becomes the positional). A bare value without `=` is
SILENTLY IGNORED (that is exactly what `ledger` has today: `"pp:happy-args": "Tahoe Trip"`). `--flag=value` tokens add
flags; bare `--flag` means `--flag=true`. With `"pp:typed-exit-codes": "0,3"` already present, a graceful not-found
exit 3 counts as a pass, so an unknown example name is the right fixture — it exercises the resolver and never writes.

## Edits (exact)
1. `ledger.go`: change `"pp:happy-args": "Tahoe Trip"` → `"pp:happy-args": "<group>=Example Group"`.
2. `fairness_nudge.go` (Use `nudge <friend>`): add `"pp:happy-args": "<friend>=Example Friend"` to the existing
   Annotations map (keep `mcp:hidden`, `mcp:read-only`, `pp:typed-exit-codes` as they are).
3. `settle_up.go` (Use `settle-up <group-or-friend>`): add `"pp:happy-args": "<group-or-friend>=Example Group"`.
4. `split.go` (Use `split <group>`; requires `--amount` and a split mode): add
   `"pp:happy-args": "<group>=Example Group;--amount=84;--equal"`. Also this file has TWO `// pp:data-source`
   directive lines (`auto` near line 4 and `live` near line 21); the generator contract allows exactly one per
   command file. Read the RunE to decide which is true (does it honor --data-source auto|local|live, or only call the
   live API?) and delete the wrong line, keeping the other unchanged.

## Verify (must all pass before you finish)
- `go build ./... && go vet ./... && go test -count=1 ./internal/cli/`
- `go run ./cmd/splitwise-pp-cli ledger "Example Group" --json; echo exit=$?` → exit 3 (not found) with JSON on stdout.
- Same for `fairness nudge "Example Friend" --json`, `settle-up "Example Group" --json`,
  `split "Example Group" --amount 84 --equal --json` → each exit 3, no panic, no write. (No API key needed for a
  not-found on an empty local store; if a command needs sync first it should still exit 3 or print the
  missing-mirror empty result — record what you observed.)
Report each command's exit code and first stdout line in your final message.
