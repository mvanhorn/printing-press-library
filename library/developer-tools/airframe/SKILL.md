---
name: pp-airframe
description: "Aircraft forensics from open public records — tail-number dossiers, fleet research, model-level safety, and NTSB event archaeology. Trigger phrases: `look up tail number`, `who owns this plane N…`, `what aircraft does <company> own`, `safety record of <aircraft model>`, `accident history for`, `crash history for <aircraft model>`, `find aircraft registered to`, `NTSB event ERA…`, `use airframe`, `run airframe`. Secondary (day-of only, requires flight-goat + within 72hr of departure): `is UA<flight> safe`, `what plane is <flight>`."
author: "Chris Drit"
license: "Apache-2.0"
argument-hint: "<command> [args] | install"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - airframe-pp-cli
    install:
      - kind: go
        bins: [airframe-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/cmd/airframe-pp-cli
---

# airframe — Printing Press CLI

Aircraft forensics from open public data. **Two distinct data tiers — understand which one the user's question needs before doing anything.**

## Prerequisites: Install the CLI

This skill drives the `airframe-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install airframe --cli-only
   ```
2. Verify: `airframe-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/cmd/airframe-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

For Tier 2 (NTSB accident data) you'll also need the `mdbtools` package — `airframe-pp-cli doctor` reports whether it's detected and gives the install command for your OS. Tier 1 (FAA registry) works without it.

## The two-tier model (read this first, every invocation)

| Tier | Dataset | Install requirement | Answers questions like |
|------|---------|---------------------|------------------------|
| **1** | FAA Aircraft Registry | **none** — just `airframe-pp-cli` | "Who owns N12345?" • "How old is this plane?" • "What aircraft does Berkshire Hathaway own?" • "What type is this Mode-S code?" |
| **2** | NTSB CAROL accident database | **`mdbtools` package** | "How safe is a Cessna 172?" • "Crash history for the Boeing 737 MAX 8" • "Has tail N12345 ever been in an incident?" • "Probable cause of event ERA22LA001" |

`airframe flight <ident>` additionally requires `flight-goat-pp-cli` installed + `FLIGHT_GOAT_API_KEY_AUTH` set, **and only works within ~72 hours of departure** (airlines don't assign a specific tail to a flight before then). For pre-purchase or pre-booking "is my flight safe" questions, use a Tier 2 model-level lookup with the equipment type from the booking site.

## Step 0 — Always run doctor first

Before answering any user question, run:

```bash
airframe-pp-cli doctor --json
```

Parse the JSON to determine:

- `db_exists` — has the user ever synced?
- `sync_meta` — which sources (faa_master, ntsb_avall) are present and how fresh?
- `mdbtools.detected` — can we serve Tier 2 questions?
- `flight_goat.detected` — can we serve `flight <ident>` questions?

**Branch from here** based on what the user actually asked.

## Decision tree (use this to drive every user interaction)

### A) User asks a Tier 1 question and Tier 1 is missing
*Example: "Who owns N628TS?" but `airframe-pp-cli doctor` shows db_exists=false or sync_meta is empty.*

1. Tell the user: "I need to download the FAA registry first (~80 MB, ~30 seconds, no setup needed)."
2. Offer to run `airframe-pp-cli sync` for them.
3. After sync completes, answer their original question.

### B) User asks a Tier 1 question and Tier 1 is ready
Just answer it directly:
```bash
airframe-pp-cli tail N628TS
airframe-pp-cli owner "Berkshire" --limit 20
```

### C) User asks a Tier 2 question (safety / accident / NTSB) and **`mdbtools` is missing**
*This is the most important branch — handle it with care.*

Surface a clear explanation:

> "To answer that, I need NTSB accident data, which requires the `mdbtools` package (NTSB ships their data in Microsoft Access format). It's a one-time install — would you like me to install it for you?"

If the user says yes, detect their OS and run the right command:

```bash
# Detect OS
uname -s   # → "Darwin" for macOS, "Linux" for Linux
# Detect Linux distribution if Linux
cat /etc/os-release 2>/dev/null | grep -E '^ID='
```

Then propose **before running**:

| OS detected | Command to run |
|---|---|
| macOS (`Darwin`) | `brew install mdbtools` |
| Ubuntu / Debian (`ID=ubuntu`, `ID=debian`) | `sudo apt update && sudo apt install -y mdbtools` |
| Fedora (`ID=fedora`) | `sudo dnf install -y mdbtools` |
| RHEL / Rocky / Alma (`ID=rhel`/`ID=rocky`/`ID=almalinux`) | `sudo dnf install -y mdbtools` |
| Arch (`ID=arch`) | `yay -S --noconfirm mdbtools` *(AUR — needs `yay` or `paru`; if neither is present, tell the user to install one first)* |

**Important:** never run `sudo` commands without first showing the user what you plan to run and confirming. The `--noconfirm` for `yay` is OK because the user has already opted in once.

After `mdbtools` installs, run:

```bash
airframe-pp-cli sync --source all
```

Then answer the original question.

### D) User asks a Tier 2 question and `mdbtools` is present but NTSB hasn't been synced
*`doctor` shows `mdbtools.detected=true`, but `sync_meta` has no `ntsb_avall` row.*

Tell the user: "I have the NTSB ingest tool installed but haven't synced the data yet. Would you like me to run that now? It's ~90 MB and takes about a minute."

If yes:
```bash
airframe-pp-cli sync --source ntsb
```

### E) User asks a flight-ident question — **first, check the 72-hour gate**
*Examples: "Is UA1234 safe?", "What plane will I be on for AA200?", "Is my flight on DL5678 tomorrow OK?"*

**Critical context:** Airlines don't assign a specific tail to a flight until ~48–72 hours before departure. If the user is more than 72 hours from their flight (or shopping a ticket they haven't booked yet), looking up the specific tail is **impossible** — there is no tail yet. Don't burn a flight-goat call on it.

**Step 1 — establish when the flight is.** Ask the user (or infer from context):

> "When is the flight? Tail assignments are made 48–72 hours before departure — if you're shopping or it's further out than that, I'll point you at the model-level safety lookup instead, which is more reliable for that horizon."

**Step 2 — branch:**

- **Flight is >72 hours away or hypothetical** → recommend the model-level fallback. Most airline booking sites tell you the equipment type (e.g. "Boeing 737 MAX 8" or "Airbus A321neo"). Run:
  ```bash
  airframe-pp-cli model "Boeing 737 MAX 8"
  ```
  This answers the user's underlying question ("is the type of plane I'll be on safe?") and needs no flight-goat at all. (It still needs Tier 2 / mdbtools for safety data — fall through to branch C if missing.)

- **Flight is ≤72 hours away OR the user is asking about a flight that already happened (post-incident research)** → proceed with flight-goat resolution. Continue to step 3.

**Step 3 — flight-goat present? If not, install it.**

If `flight_goat.detected=false`, tell the user:

> "To resolve a specific flight ident (like UA1234) to a tail number, I need `flight-goat-pp-cli`. Would you like me to install it? It also requires a FlightAware AeroAPI key — that part you'll need to provide."

If yes:
```bash
go install github.com/mvanhorn/printing-press-library/library/travel/flight-goat/cmd/flight-goat-pp-cli@latest
```

**Step 4 — `FLIGHT_GOAT_API_KEY_AUTH` set? If not, explain the cost.**

If the env var is unset, tell the user that flight-goat routes through FlightAware's AeroAPI (paid service) and they'll need to sign up at flightaware.com/aeroapi to get a key. Don't push past without explicit direction.

**Step 5 — run the lookup.**

```bash
airframe-pp-cli flight UA1234
```

Note that the result may include **multiple recent tails** that have flown the same ident over the past 14 days — flight-goat returns the history, not just the next scheduled flight. For post-incident research, look at the date-aligned tail; for pre-departure curiosity, look at the most recent one.

## Commands

| Command | Tier | What it does |
|---|---|---|
| `sync [flags]` | both | Download bulk data, replace local tables, VACUUM |
| `tail <N-number>` | 1 (+2 enriches) | Aircraft dossier + accident history |
| `owner "<name>"` | 1 | Aircraft fleet by registered owner |
| `model "<make> [model]"` | 2 | Aggregate NTSB events by model |
| `event <event-id>` | 2 | One NTSB event |
| `flight <ident>` | 1 + flight-goat | Resolve ident → tail → enrich |
| `search "<query>"` | 1+2 + `--with-fts` | FTS5 search |
| `doctor` | — | Health check |

All commands accept `--json`, `--select <dotted-path>`, and `--db-path`.

## Trigger-phrase → tier mapping

For fast classification of what the user is asking. **Most user questions are answered by Tier 1 or Tier 2 alone** — the flight-goat path is the exception, not the rule.

**Tier 1 phrases** → just need FAA sync
- "who owns N…", "who owns the plane with tail N…"
- "what aircraft does <company> own", "<company> fleet"
- "how old is this plane", "when was this tail manufactured"
- "what type/model is N…", "what's the Mode-S for N…"
- "find aircraft registered to <name/state/LLC>"

**Tier 2 phrases** → need NTSB + mdbtools
- "how safe is a <aircraft model>", "is the <model> safe", "safety record of <model>"
- "accident history for <model>", "crash history of <model>"
- "ever had an incident with this tail" (when N-number is known)
- "NTSB events for", "show me event ERA…"
- "fatal accidents in <state> this year"
- "crashes in <region> last <period>"

**Tier 1 + flight-goat phrases** → require ident resolution and **only work within ~72 hours of departure**
- "is UA1234 safe" *(only useful if flight is within 72 hours)*
- "what plane is United 1234 today/tomorrow"
- "my flight is DL567 in a few hours — what aircraft"
- "what tails did UA89 fly last week" *(post-incident — always works)*

For any "is my flight safe" / "what plane will I be on" question, **always ask when the flight is first**. If the user is shopping or the flight is more than 72 hours out, redirect to a model-level lookup (`airframe-pp-cli model "<equipment-type>"`) using the equipment type from the user's booking site. Most airline booking pages display the aircraft type for every flight at search time.

## Output and agent mode

`--agent` (= `--json --compact --no-input --no-color --yes`) is the default mode for programmatic consumption. The envelope is `{meta, results}` matching `sec-edgar`. Use `--select results.aircraft.owner_name` to extract a single field.

## Caveats to surface to the user

- FAA owner data may be a Delaware LLC shell. airframe reports the registry value verbatim; it is not a beneficial-ownership tool.
- NTSB only investigates incidents meeting their reporting threshold — "no events" is not "perfect record."
- `flight` requires a paid AeroAPI key via flight-goat. FAA + NTSB are free; ident-to-tail resolution is what costs.

