---
name: pp-orcarouter
description: "Agent-first OrcaRouter gateway access — model catalog, chat completions, auth health, and doctor diagnostics for the OpenAI-compatible OrcaRouter endpoint. Trigger phrases: `orcarouter models`, `use orcarouter`, `check orcarouter auth`, `orcarouter doctor`, `run orcarouter`."
author: "martinzudergaming-a11y"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - orcarouter-pp-cli
---

# OrcaRouter — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `orcarouter-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install orcarouter --cli-only
   ```
2. Verify: `orcarouter-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/orcarouter/cmd/orcarouter-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an AI agent needs to reach the OrcaRouter gateway from a shell: discover the model namespace, run an OpenAI-compatible chat completion, or verify auth and connectivity before a pipeline. The gateway exposes `orcarouter/*` routing models plus the underlying provider models behind one endpoint, so this CLI is the drop-in replacement for treating OrcaRouter as an anonymous custom base URL — every command speaks the same OpenAI-compatible surface the gateway already documents.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Gateway health before a run
- **`doctor`** — Verify auth, config, and live connectivity to the OrcaRouter gateway in one command. Probes `/models` and reports reachability plus model count.

  _Use this as a pre-flight gate in a cron or agent pipeline before dispatching chat work._

  ```bash
  orcarouter-pp-cli doctor --json
  ```

### Model namespace discovery
- **`models`** — List the full OrcaRouter model catalog sorted by id, with context length where advertised.

  _Use this when an agent needs to pick a valid model id without hallucinating the namespace._

  ```bash
  orcarouter-pp-cli models
  ```

- **`models get`** — Show a single model by id, including supported endpoint types and context length.

  _Use this when an agent needs to confirm a specific model's capabilities before routing to it._

  ```bash
  orcarouter-pp-cli models get orcarouter/fusion
  ```

### OpenAI-compatible chat
- **`chat`** — Send a chat completion to any OrcaRouter model through the OpenAI-compatible endpoint. Pass the model id and the prompt; `--max-tokens` and `--temperature` tune the call, `--agent` gives JSON output.

  _Use this when an agent needs one inference call through the gateway with adaptive routing and failover, without wiring up an SDK._

  ```bash
  orcarouter-pp-cli chat orcarouter/fusion-flash "summarize this PR in one line" --max-tokens 80
  ```

### Auth state
- **`auth`** — Show whether the CLI is configured, the auth source (env or config), and the target base URL. `auth set` persists an API key to the local config file.

  _Use this when an agent needs to confirm which key and endpoint a pipeline will use, or to persist a key once for non-env setups._

  ```bash
  orcarouter-pp-cli auth
  ```

## Command Reference

**auth** — Show the current auth state

- `orcarouter-pp-cli auth` — Show whether the CLI is configured, the auth source (env or config), and the target base URL.

  ```bash
  orcarouter-pp-cli auth --json
  ```

**auth set** — Save the OrcaRouter API key to the local config file

- `orcarouter-pp-cli auth set <api-key>` — Persist an API key to the local config file.

  ```bash
  orcarouter-pp-cli auth set sk-orca-example
  ```

**chat** — Send an OpenAI-compatible chat completion

- `orcarouter-pp-cli chat <model-id> <prompt>` — Send a chat completion to the given model.

  ```bash
  orcarouter-pp-cli chat orcarouter/free "hello" --max-tokens 20
  ```

**doctor** — Check CLI health

- `orcarouter-pp-cli doctor` — Check auth and connectivity to the OrcaRouter gateway.

  ```bash
  orcarouter-pp-cli doctor --fail-on warn
  ```

**models** — List the OrcaRouter model catalog

- `orcarouter-pp-cli models` — List the full OrcaRouter model namespace sorted by id.

  ```bash
  orcarouter-pp-cli models
  ```

**models get** — Show a single model by id

- `orcarouter-pp-cli models get <model-id>` — Show a single model by id.

  ```bash
  orcarouter-pp-cli models get orcarouter/fusion --json
  ```

**version** — Print version

- `orcarouter-pp-cli version` — Print the CLI version.

  ```bash
  orcarouter-pp-cli version
  ```
