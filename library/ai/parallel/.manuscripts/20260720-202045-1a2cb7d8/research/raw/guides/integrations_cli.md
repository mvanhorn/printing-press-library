> ## Documentation Index
> Fetch the complete documentation index at: https://docs.parallel.ai/llms.txt
> Use this file to discover all available pages before exploring further.

# Parallel CLI

> Command-line tool for web search, content extraction, data enrichment, deep research, entity discovery, and web monitoring

<div className="sr-only" aria-hidden="false">
  For AI agents: a documentation index is available at [https://docs.parallel.ai/llms.txt](https://docs.parallel.ai/llms.txt). The full text of all docs is at [https://docs.parallel.ai/llms-full.txt](https://docs.parallel.ai/llms-full.txt). You may also fetch any page as Markdown by appending `.md` to its URL or sending `Accept: text/markdown`.
</div>

The `parallel-cli` is a command-line tool for interacting with the Parallel API. It works interactively or fully via command-line arguments, making it the recommended way to use Parallel in standalone agents. For best results, pair the CLI with [Agent Skills](/integrations/agent-skills) or [Claude Code](/integrations/claude-code-marketplace) to give your agent structured access to search, extract, enrich, and research capabilities.

<Tip>View the source and full README on [GitHub](https://github.com/parallel-web/parallel-web-tools).</Tip>

<Note>Already have `parallel-cli` installed? Run `parallel-cli update` to get the latest features and improvements.</Note>

## Installation

<Tabs>
  <Tab title="pipx">
    Install in an isolated environment so `parallel-cli` is on your PATH:

    ```bash theme={"system"}
    pipx install "parallel-web-tools[cli]" && pipx ensurepath
    ```

    `pipx ensurepath` is only required on first-ever pipx use; including it here makes the command safe to copy-paste either way. For the minimal CLI without YAML configs / interactive planner, use `pipx install parallel-web-tools`.
  </Tab>

  <Tab title="uv">
    [`uv`](https://docs.astral.sh/uv/) is Astral's faster modern alternative to pipx. It writes a PATH-aware shim on first run, so no `ensurepath` step is needed:

    ```bash theme={"system"}
    uv tool install "parallel-web-tools[cli]"
    ```

    Requires `uv` to be installed first — see [Astral's install guide](https://docs.astral.sh/uv/getting-started/installation/).
  </Tab>

  <Tab title="Homebrew">
    ```bash theme={"system"}
    brew install parallel-web/tap/parallel-cli
    ```
  </Tab>

  <Tab title="pip">
    Use `pip` when you're embedding `parallel-web-tools` in a Python project (rather than using `parallel-cli` as a standalone tool — see the **pipx** tab for that):

    ```bash theme={"system"}
    # Minimal SDK
    pip install parallel-web-tools

    # With YAML config files and interactive planner
    pip install parallel-web-tools[cli]

    # With data integrations
    pip install parallel-web-tools[duckdb]    # DuckDB
    pip install parallel-web-tools[bigquery]  # BigQuery
    pip install parallel-web-tools[spark]     # Apache Spark

    # Everything
    pip install parallel-web-tools[all]
    ```
  </Tab>

  <Tab title="npm">
    ```bash theme={"system"}
    npm install -g parallel-web-cli
    ```
  </Tab>

  <Tab title="Shell Script">
    Install the standalone binary (no Python or Node required):

    ```bash theme={"system"}
    curl -fsSL https://parallel.ai/install.sh | bash
    ```

    This detects your platform (macOS/Linux, x64/arm64) and installs to `~/.local/bin`.

    <Note>Some agent skill registries flag the `curl | bash` pattern as a supply-chain risk. If you're installing `parallel-cli` for use with [Agent Skills](/integrations/agent-skills), prefer **pipx** or **Homebrew**.</Note>
  </Tab>
</Tabs>

## Authentication

```bash theme={"system"}
# Interactive OAuth login (opens browser)
parallel-cli login

# Device authorization flow — for SSH, containers, CI, or headless environments
parallel-cli login --device

# Or set environment variable
export PARALLEL_API_KEY="your_api_key"

# Check auth status
parallel-cli auth
```

<Tip>Get your API key from [Platform](https://platform.parallel.ai).</Tip>

## Commands

### Search

Search the web with natural language objectives or keyword queries.

```bash theme={"system"}
# Natural language search
parallel-cli search "What is Anthropic's latest AI model?" --json

# Keyword search with date filter
parallel-cli search -q "bitcoin price" --after-date 2026-01-01 --json

# Search specific domains
parallel-cli search "SEC filings for Apple" --include-domains sec.gov --json

# Set search mode
parallel-cli search "latest AI research" --mode turbo --json
```

| Option              | Description                                         |
| ------------------- | --------------------------------------------------- |
| `-q, --query`       | Keyword search query (repeatable)                   |
| `--mode`            | `turbo` (fastest), `basic` (default), or `advanced` |
| `--max-results`     | Maximum results (default: 10)                       |
| `--include-domains` | Only search these domains                           |
| `--exclude-domains` | Exclude these domains                               |
| `--after-date`      | Only results after this date (YYYY-MM-DD)           |
| `--json`            | Output as JSON                                      |
| `-o, --output`      | Save results to file                                |

### Extract

Extract clean markdown content from URLs.

```bash theme={"system"}
# Basic extraction
parallel-cli extract https://example.com --json

# Extract with a specific focus
parallel-cli extract https://company.com --objective "Find pricing info" --json

# Get full page content
parallel-cli extract https://example.com --full-content --json
```

| Option           | Description                         |
| ---------------- | ----------------------------------- |
| `--objective`    | Focus extraction on a specific goal |
| `-q, --query`    | Keywords to prioritize (repeatable) |
| `--full-content` | Include complete page content       |
| `--no-excerpts`  | Exclude excerpts from output        |
| `--json`         | Output as JSON                      |
| `-o, --output`   | Save results to file                |

### Research

Run deep research on open-ended questions.

```bash theme={"system"}
# Run deep research
parallel-cli research run "What are the latest developments in quantum computing?" --json

# Use a specific processor tier
parallel-cli research run "Compare EV battery technologies" --processor ultra --json

# Read query from file
parallel-cli research run -f question.txt -o report

# Async: launch then poll separately
parallel-cli research run "question" --no-wait --json    # returns run_id
parallel-cli research status trun_xxx --json              # check status
parallel-cli research poll trun_xxx --json                # wait and get result

# List available processors
parallel-cli research processors --json
```

| Option            | Description                                                                            |
| ----------------- | -------------------------------------------------------------------------------------- |
| `-p, --processor` | Processor tier: `lite`, `base`, `core`, `pro` (default), `ultra`, and `-fast` variants |
| `--no-wait`       | Return immediately after creating task                                                 |
| `--timeout`       | Max wait time in seconds (default: 3600)                                               |
| `-o, --output`    | Save results (creates `.json` and `.md` files)                                         |
| `--json`          | Output as JSON                                                                         |

### Enrich

Enrich CSV or JSON data with AI-powered web research.

```bash theme={"system"}
# Let AI suggest output columns
parallel-cli enrich suggest "Find the CEO and annual revenue" --json

# Run enrichment directly
parallel-cli enrich run \
    --source-type csv \
    --source companies.csv \
    --target enriched.csv \
    --source-columns '[{"name": "company", "description": "Company name"}]' \
    --intent "Find the CEO and annual revenue"

# Enrich with inline data (no file needed)
parallel-cli enrich run \
    --data '[{"company": "Google"}, {"company": "Apple"}]' \
    --target output.csv \
    --intent "Find the CEO"

# Enrich a JSON file
parallel-cli enrich run \
    --source-type json \
    --source companies.json \
    --target enriched.json \
    --source-columns '[{"name": "company", "description": "Company name"}]' \
    --enriched-columns '[{"name": "ceo", "description": "CEO name"}]'

# Run from YAML config
parallel-cli enrich run config.yaml

# Async: launch then poll
parallel-cli enrich run config.yaml --no-wait --json
parallel-cli enrich status tgrp_xxx --json
parallel-cli enrich poll tgrp_xxx --json
```

| Option               | Description                                        |
| -------------------- | -------------------------------------------------- |
| `--source-type`      | `csv` or `json`                                    |
| `--source`           | Source file path                                   |
| `--target`           | Target file path                                   |
| `--source-columns`   | Source columns as JSON                             |
| `--enriched-columns` | Output columns as JSON                             |
| `--intent`           | Natural language description (AI suggests columns) |
| `--processor`        | Processor tier (e.g. `core-fast`, `pro`, `ultra`)  |
| `--data`             | Inline JSON data array                             |
| `--no-wait`          | Return immediately                                 |
| `--dry-run`          | Preview without making API calls                   |
| `--json`             | Output as JSON                                     |

<Accordion title="YAML configuration format">
  You can also define enrichment jobs in YAML:

  ```yaml theme={"system"}
  source: input.csv
  target: output.csv
  source_type: csv
  processor: core-fast

  source_columns:
    - name: company_name
      description: The name of the company

  enriched_columns:
    - name: ceo
      description: The CEO of the company
      type: str
    - name: revenue
      description: Annual revenue in USD
      type: float
  ```

  Create YAML configs interactively or programmatically:

  ```bash theme={"system"}
  # Interactive
  parallel-cli enrich plan -o config.yaml

  # Non-interactive (for scripts/agents)
  parallel-cli enrich plan -o config.yaml \
      --source-type csv \
      --source companies.csv \
      --target enriched.csv \
      --source-columns '[{"name": "company", "description": "Company name"}]' \
      --intent "Find the CEO and annual revenue"
  ```

  <Note>YAML config files and the interactive planner require `pip install parallel-web-tools[cli]`.</Note>
</Accordion>

### FindAll

Discover entities from the web using natural language.

```bash theme={"system"}
# Discover entities
parallel-cli findall run "AI startups in healthcare" --json

# Control generator tier and match limit
parallel-cli findall run "Find roofing companies in Charlotte NC" -g base -n 25 --json

# Exclude specific entities
parallel-cli findall run "Find AI startups" \
    --exclude '[{"name": "Example Corp", "url": "example.com"}]' --json

# Preview schema before running
parallel-cli findall run "Find YC companies in developer tools" --dry-run --json

# Async workflow
parallel-cli findall run "AI startups" --no-wait --json
parallel-cli findall status frun_xxx --json
parallel-cli findall poll frun_xxx --json
parallel-cli findall result frun_xxx --json

# Cancel a running job
parallel-cli findall cancel frun_xxx
```

| Option              | Description                                                |
| ------------------- | ---------------------------------------------------------- |
| `-g, --generator`   | Generator tier: `preview`, `base`, `core` (default), `pro` |
| `-n, --match-limit` | Max matched candidates, 5-1000 (default: 10)               |
| `--exclude`         | Entities to exclude as JSON array                          |
| `--no-wait`         | Return immediately                                         |
| `--dry-run`         | Preview schema without creating the run                    |
| `--json`            | Output as JSON                                             |

### Monitor

Continuously track the web for changes.

```bash theme={"system"}
# Create a monitor
parallel-cli monitor create "Track price changes for iPhone 16" --json

# Set check frequency
parallel-cli monitor create "New AI funding announcements" --frequency 1h --json

# Track changes to a Task Run output
parallel-cli monitor create --type snapshot \
    --task-run-id trun_xxx --frequency 1d --json

# With webhook delivery
parallel-cli monitor create "SEC filings from Tesla" \
    --webhook https://example.com/hook --json

# Manage monitors
parallel-cli monitor list --json
parallel-cli monitor get mon_xxx --json
parallel-cli monitor update mon_xxx --frequency 1w --json
parallel-cli monitor cancel mon_xxx --json

# View events
parallel-cli monitor events mon_xxx --json
parallel-cli monitor events mon_xxx --event-group-id mevtgrp_xxx --json

# Run an off-schedule check
parallel-cli monitor trigger mon_xxx --json
```

`trigger` starts a real off-schedule run without changing the monitor's regular frequency. It is not a synthetic webhook test, and cancelled monitors cannot be triggered. Cancellation is irreversible and permanently stops future runs; create a new monitor to resume monitoring.

| Option             | Description                                                                                                                                                                                         |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `-f, --frequency`  | Frequency in `<n><unit>` format using `h`, `d`, or `w`, such as `1h`, `1d`, or `1w` (`monitor create` default: `1d`). Aliases `hourly`, `daily`, `weekly`, and `every_two_weeks` are also accepted. |
| `--type`           | Monitor type for `monitor create`: `event_stream` tracks a search query (default); `snapshot` tracks a Task Run output                                                                              |
| `--task-run-id`    | Task Run to track when creating a `snapshot` monitor                                                                                                                                                |
| `--processor`      | Monitor processor for `monitor create`: `lite` (default) or the more thorough, higher-cost `base`                                                                                                   |
| `--webhook`        | Webhook URL for event delivery                                                                                                                                                                      |
| `--output-schema`  | Output schema as a JSON string (`event_stream` only)                                                                                                                                                |
| `--status`         | Filter `monitor list` by `active` or `cancelled`; repeat to include both (default: active only)                                                                                                     |
| `--event-group-id` | Filter `monitor events` to a specific Monitor execution                                                                                                                                             |
| `--json`           | Output as JSON                                                                                                                                                                                      |

## Non-Interactive Mode

All commands support `--json` output and can be fully controlled via CLI arguments, making the CLI ideal for use in scripts and by AI agents.

```bash theme={"system"}
# Structured JSON output
parallel-cli search "query" --json

# Read input from stdin
echo "What is the latest funding for Anthropic?" | parallel-cli search - --json
echo "Research question" | parallel-cli research run - --json

# Exit codes
# 0 = success, 2 = bad input, 3 = auth error, 4 = API error, 5 = timeout
```

## Updating

The standalone binary automatically checks for updates and will notify you when a new version is available. To update:

```bash theme={"system"}
# pipx
pipx upgrade parallel-web-tools

# uv
uv tool upgrade parallel-web-tools

# Standalone binary
parallel-cli update

# Check for updates without installing
parallel-cli update --check

# Homebrew
brew upgrade parallel-cli

# npm
npm update -g parallel-web-cli

# pip
pip install --upgrade parallel-web-tools
```

To disable automatic update checks:

```bash theme={"system"}
parallel-cli config auto-update-check off
```
