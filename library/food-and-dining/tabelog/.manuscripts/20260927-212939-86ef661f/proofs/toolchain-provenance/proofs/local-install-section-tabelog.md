## Prerequisites: Install the CLI

This skill drives the `tabelog-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

This CLI uses a local Go module. Run the following from the root of this source checkout (requires Go 1.26.6 or newer):

```bash
go install ./cmd/tabelog-pp-cli
```

Go installs the binary into `$GOBIN` when set, otherwise `$GOPATH/bin` (default `$HOME/go/bin`). Add that directory to `$PATH` for the agent/runtime that will invoke this skill, or invoke the installed binary by its full path.

Verify: `tabelog-pp-cli --version`

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.
