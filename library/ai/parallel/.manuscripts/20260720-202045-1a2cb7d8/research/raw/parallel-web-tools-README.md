# Parallel-Web-Tools

CLI and data enrichment utilities for the [Parallel API](https://docs.parallel.ai).

> **Note:** This package provides the `parallel-cli` command-line tool and data enrichment utilities in the `parallel-web-tools` package.
> It depends on [`parallel-web`](https://github.com/parallel-web/parallel-sdk-python), the official
> Parallel Python SDK, but does not contain it. Install `parallel-web` separately if you need
> direct SDK access.

## Features

- **CLI for Humans & AI Agents** - Works interactively or fully via command-line arguments
- **Web Search** - AI-powered search with domain filtering and date ranges
- **Content Extraction** - Extract clean markdown from any URL
- **Data Enrichment** - Enrich CSV, JSON, DuckDB, and BigQuery data with AI
- **Follow-up Context** - Chain research and enrichment tasks using `--previous-interaction-id`
- **AI-Assisted Planning** - Use natural language to define what data you want
- **Multiple Integrations** - Polars, DuckDB, Snowflake, BigQuery, Spark

## Installation

Requires **Python 3.10+**.

### Standalone CLI (Recommended)

Install the standalone `parallel-cli` binary for search, extract, enrichment, and deep research (no Python required):

```bash
# macOS / Linux (Homebrew)
brew install parallel-web/tap/parallel-cli

# macOS / Linux (shell script)
curl -fsSL https://parallel.ai/install.sh | bash
```

The shell script automatically detects your platform (macOS/Linux, x64/arm64) and installs to `~/.local/bin`.

> **Note:** The standalone binary supports `search`, `extract`, `research`, and `enrich run` with CLI arguments, CSV files, and JSON files. For YAML config files, interactive planner, DuckDB/BigQuery sources, or deployment commands, use pip install.

### npm

```bash
npm install -g parallel-web-cli
```

This downloads the pre-built binary for your platform. No Python or Go required.

### Python Package

For programmatic usage or additional features:

```bash
# Minimal CLI (search, extract, enrich with CLI args)
pip install parallel-web-tools

# + YAML config files and interactive planner
pip install parallel-web-tools[cli]

# + Data integrations
pip install parallel-web-tools[duckdb]       # DuckDB (includes cli, polars)
pip install parallel-web-tools[bigquery]     # BigQuery (includes cli)
pip install parallel-web-tools[spark]        # Apache Spark

# Full install with all features
pip install parallel-web-tools[all]
```

## CLI Overview

```
parallel-cli
├── auth                    # Check authentication status
├── login                   # Device OAuth login (single command)
├── logout                  # Remove stored credentials
├── search                  # Web search
├── extract / fetch         # Extract content from URLs
├── research                # Deep research commands
│   ├── run                 # Run deep research on a question or topic
│   ├── status              # Check status of a research task
│   ├── poll                # Poll until completion
│   └── processors          # List available research processors
├── enrich                  # Data enrichment commands
│   ├── run                 # Run enrichment
│   ├── status              # Check status of a task group
│   ├── poll                # Poll until completion and collect results
│   ├── plan                # Create YAML config
│   ├── suggest             # AI suggests output columns
│   └── deploy              # Deploy to cloud systems (requires pip install)
├── findall                 # Web-scale entity discovery
│   ├── run                 # Discover entities matching a natural language objective
│   ├── ingest              # Preview the schema before running
│   ├── entity-search       # Fast entity search (best-effort optimized for low latency)
│   ├── status              # Check status of a FindAll run
│   ├── poll                # Poll until completion
│   ├── result              # Fetch results of a completed run
│   ├── enrich              # Enrich existing FindAll results with new columns
│   ├── extend              # Request additional candidates for a run
│   ├── schema              # Get the schema for a FindAll run
│   └── cancel              # Cancel a running FindAll
└── monitor                 # Continuous web change tracking
    ├── create              # Create a new web monitor (event_stream or snapshot)
    ├── list                # List monitors (cursor paginated)
    ├── get                 # Get monitor details
    ├── update              # Update frequency, webhook, metadata
    ├── cancel              # Cancel a monitor (irreversible)
    ├── events              # List events for a monitor
    └── trigger             # Trigger an immediate one-off run
```

## Quick Start

### 1. Authenticate

```bash
# Device OAuth login (shows code and opens browser)
parallel-cli login

# Or set environment variable
export PARALLEL_API_KEY=your_api_key
```

### 2. Search the Web

```bash
# Natural language search
parallel-cli search "What is Anthropic's latest AI model?" --json

# Keyword search with filters
parallel-cli search -q "bitcoin price" --after-date 2026-01-01 --json

# Search specific domains
parallel-cli search "SEC filings for Apple" --include-domains sec.gov --json
```

### 3. Extract Content from URLs

```bash
# Extract content as markdown
parallel-cli extract https://example.com --json

# Extract with a specific focus
parallel-cli extract https://company.com --objective "Find pricing info" --json

# Get full page content
parallel-cli extract https://example.com --full-content --json
```

### 4. Enrich Data

```bash
# Let AI suggest what columns to add
parallel-cli enrich suggest "Find the CEO and annual revenue" --json

# Create a config file (interactive)
parallel-cli enrich plan -o config.yaml

# Create a config file (non-interactive, for AI agents)
parallel-cli enrich plan -o config.yaml \
    --source-type csv \
    --source companies.csv \
    --target enriched.csv \
    --source-columns '[{"name": "company", "description": "Company name"}]' \
    --intent "Find the CEO and annual revenue"

# Run enrichment from config
parallel-cli enrich run config.yaml

# Run enrichment directly (no config file needed)
parallel-cli enrich run \
    --source-type csv \
    --source companies.csv \
    --target enriched.csv \
    --source-columns '[{"name": "company", "description": "Company name"}]' \
    --intent "Find the CEO and annual revenue"

# Enrich a JSON file
parallel-cli enrich run \
    --source-type json \
    --source companies.json \
    --target enriched.json \
    --source-columns '[{"name": "company", "description": "Company name"}]' \
    --enriched-columns '[{"name": "ceo", "description": "CEO name"}]'
```

### 5. Deploy to Cloud Systems

```bash
# Deploy to BigQuery for SQL-native enrichment
parallel-cli enrich deploy --system bigquery --project my-gcp-project
```

## Non-Interactive Mode (for AI Agents & Scripts)

All commands support `--json` output and can be fully controlled via CLI arguments.

### Key patterns for agents

```bash
# Every command supports --json for structured output
parallel-cli search "query" --json
parallel-cli auth --json
parallel-cli research processors --json

# Read input from stdin with "-"
echo "What is the latest funding for Anthropic?" | parallel-cli search - --json
echo "Research question" | parallel-cli research run - --json

# Async: launch then poll separately
parallel-cli research run "question" --no-wait --json   # returns run_id + interaction_id
parallel-cli research status trun_xxx --json             # check status
parallel-cli research poll trun_xxx --json               # wait and get result

# Follow-up: reuse context from a previous task
parallel-cli research run "follow-up question" --previous-interaction-id trun_xxx --json
parallel-cli enrich run --data '[...]' --previous-interaction-id trun_xxx --json

# Exit codes: 0=ok, 2=bad input, 3=auth error, 4=api error, 5=timeout
```

### Follow-up research with context reuse

Tasks return an `interaction_id` that can be passed as `--previous-interaction-id` on a subsequent research or enrichment run. The new task inherits the context from the prior one, so follow-up questions can reference earlier results without repeating them.

```bash
# Step 1: Run initial research (interaction_id is in the JSON output)
parallel-cli research run "What are the top 3 AI companies?" --json --processor lite-fast
# → { "run_id": "trun_abc", "interaction_id": "trun_abc", ... }

# Step 2: Follow-up research referencing the first task's context
parallel-cli research run "What products does the #1 company make?" \
    --previous-interaction-id trun_abc --json

# Step 3: Use research context for enrichment
parallel-cli enrich run \
    --data '[{"company": "Anthropic"}, {"company": "OpenAI"}]' \
    --target enriched.csv \
    --source-columns '[{"name": "company", "description": "Company name"}]' \
    --enriched-columns '[{"name": "products", "description": "Main products"}]' \
    --previous-interaction-id trun_abc --json
```

The `interaction_id` is shown in both human-readable and `--json` output for `research run`, `research status`, and `research poll`.

### More examples

```bash
# Search with JSON output
parallel-cli search "query" --json

# Extract with JSON output
parallel-cli extract https://url.com --json

# Suggest columns with JSON output
parallel-cli enrich suggest "Find CEO" --json

# FindAll: discover entities
parallel-cli findall run "AI startups in healthcare" --json

# FindAll entity-search: fast, low-latency ranked entities (synchronous)
parallel-cli findall entity-search "AI startups in healthcare" -t companies -n 25

# Monitor: track web changes
parallel-cli monitor create "Track Tesla SEC filings" --frequency 1d --json

# Plan without prompts (provide all args)
parallel-cli enrich plan -o config.yaml \
    --source-type csv \
    --source input.csv \
    --target output.csv \
    --source-columns '[{"name": "company", "description": "Company name"}]' \
    --enriched-columns '[{"name": "ceo", "description": "CEO name"}]'

# Or use --intent to let AI determine the columns
parallel-cli enrich plan -o config.yaml \
    --source-type csv \
    --source input.csv \
    --target output.csv \
    --source-columns '[{"name": "company", "description": "Company name"}]' \
    --intent "Find CEO, revenue, and headquarters"
```

## Integrations

| Integration | Type | Install | Documentation |
|-------------|------|---------|---------------|
| **Polars** | Python DataFrame | `pip install parallel-web-tools[polars]` | [Setup Guide](docs/polars-setup.md) |
| **DuckDB** | SQL + Python | `pip install parallel-web-tools[duckdb]` | [Setup Guide](docs/duckdb-setup.md) |
| **Snowflake** | SQL UDF | `pip install parallel-web-tools[snowflake]` | [Setup Guide](docs/snowflake-setup.md) |
| **BigQuery** | Cloud Function | `pip install parallel-web-tools[bigquery]` | [Setup Guide](docs/bigquery-setup.md) |
| **Spark** | SQL UDF | `pip install parallel-web-tools[spark]` | [Demo Notebook](notebooks/spark_enrichment_demo.ipynb) |

### Quick Integration Examples

**Polars:**
```python
import polars as pl
from parallel_web_tools.integrations.polars import parallel_enrich

df = pl.DataFrame({"company": ["Google", "Microsoft"]})
result = parallel_enrich(
    df,
    input_columns={"company_name": "company"},
    output_columns=["CEO name", "Founding year"],
)
print(result.result)
```

**DuckDB:**
```python
import duckdb
from parallel_web_tools.integrations.duckdb import enrich_table, findall_table

conn = duckdb.connect()

# Enrich an existing table
conn.execute("CREATE TABLE companies AS SELECT 'Google' as name")
result = enrich_table(
    conn,
    source_table="companies",
    input_columns={"company_name": "name"},
    output_columns=["CEO name", "Founding year"],
)
print(result.result.fetchdf())

# Discover entities with FindAll
result = findall_table(
    conn,
    "countries that have won the FIFA World Cup and their capital cities",
    match_limit=10,
)
result.result.show()
```

## Programmatic Usage

```python
from parallel_web_tools import run_enrichment, run_enrichment_from_dict

# From YAML file
run_enrichment("config.yaml")

# From dictionary
run_enrichment_from_dict({
    "source": "data.csv",
    "target": "enriched.csv",
    "source_type": "csv",
    "source_columns": [{"name": "company", "description": "Company name"}],
    "enriched_columns": [{"name": "ceo", "description": "CEO name"}]
})
```

### Device Authorization (RFC 8628)

For headless environments (SSH, containers, CI), use the device authorization flow:

```python
from parallel_web_tools import request_device_code, poll_device_token

# Step 1: Request a device code
device_info = request_device_code()
print(f"Go to: {device_info.verification_uri_complete}")

# Step 2: Poll until the user authorizes
token = poll_device_token(device_info.device_code)
```

### FindAll

Discover entities from the web using natural language:

```python
from parallel_web_tools import run_findall

# Discover entities (auto-enriches by default)
result = run_findall("AI startups in healthcare", match_limit=20)

# Post-run operations
from parallel_web_tools import enrich_findall, extend_findall, get_findall_schema

schema = get_findall_schema(result.run_id)
enriched = enrich_findall(result.run_id, ["funding amount", "number of employees"])
extended = extend_findall(result.run_id, additional_matches=10)


# Fast, low-latency entity search (synchronous)
from parallel_web_tools import entity_search_findall

entities = entity_search_findall(
    "AI startups in healthcare",
    entity_type="companies",  # "companies" or "people"
    match_limit=25,
)
```

### Monitor

Track web changes programmatically:

```python
from parallel_web_tools import create_monitor, list_monitors, get_monitor

# Create an event_stream monitor (the default)
monitor = create_monitor(query="Track Tesla SEC filings", frequency="1d")

# List all monitors (cursor paginated)
monitors = list_monitors()

# Get monitor details
details = get_monitor(monitor["monitor_id"])
```

## YAML Configuration Format

```yaml
source: input.csv
target: output.csv
source_type: csv  # csv, json, duckdb, or bigquery
processor: core-fast  # lite, base, core, pro, ultra (add -fast for speed)

source_columns:
  - name: company_name
    description: The name of the company

enriched_columns:
  - name: ceo
    description: The CEO of the company
    type: str  # str, int, float, bool
  - name: revenue
    description: Annual revenue in USD
    type: float
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `PARALLEL_API_KEY` | API key for authentication (alternative to `parallel-cli login`) |
| `DUCKDB_FILE` | Default DuckDB file path |
| `BIGQUERY_PROJECT` | Default BigQuery project ID |

## Related Packages

- [`parallel-web`](https://github.com/parallel-web/parallel-sdk-python) - Official Parallel Python SDK (this package depends on it)

## Development

```bash
git clone https://github.com/parallel-web/parallel-web-tools.git
cd parallel-web-tools
uv sync --all-extras
uv run pytest tests/ -v
```

## License

MIT
