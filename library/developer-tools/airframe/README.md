# airframe CLI

**Aircraft forensics from open public records — tail-number dossiers, fleet research, model-level safety, and NTSB event archaeology. Offline-first SQLite, no API key required.**

Given an N-number, `airframe` returns the registered owner, make/model, engine, and (with NTSB enabled) every NTSB-investigated event involving that airframe. Given an owner name, it returns the fleet. Given an aircraft model, it returns the model's fleet-wide accident profile.

All data is open public records. No registration. No API key for the local CLI.

> **Why "airframe"?** In aviation, *airframe* is the technical word for a specific, individual aircraft — distinct from its make/model or type. That's exactly the unit of analysis here: when you join the FAA registry to NTSB events, you're building a dossier on one specific tail-numbered airframe, not on a make/model in the abstract. The name also leaves room to grow without fighting itself — operator views, airworthiness directives, model-level aggregates — none of them clash with the single-word framing.

**Author:** Chris Drit · [github.com/ChrisDrit](https://github.com/ChrisDrit) · [@chrisdrit on X](https://x.com/chrisdrit)

---

## Example questions

Phrased the way real people ask them. Each shows the natural question and the airframe invocation that answers it. Tier tags (1 / 2 / +flight-goat) tell you what needs to be installed — see [Two sync tiers](#two-sync-tiers) below for the install matrix.

### Standalone airframe (no flight-goat)

**Curious / plane-spotter** *(Tier 1)*
- *"I saw a small plane fly over with N12345 on the tail. What is it and who owns it?"*
  → `airframe-pp-cli tail N12345`
- *"Whose plane just landed at the Aspen airport — N79RF?"*
  → `airframe-pp-cli tail N79RF --select results.aircraft.owner_name,results.aircraft.year_mfr`
- *"How old is the plane with this registration?"*
  → `airframe-pp-cli tail <N> --select results.aircraft.year_mfr`

**Anxious traveler — general, no booking yet** *(Tier 2)*
- *"How safe is a Cessna 172? Has it had a lot of accidents?"*
  → `airframe-pp-cli model "Cessna 172"` → aggregate by year + outcome
- *"My friend wants to take me up in a Robinson R44. Should I be worried?"*
  → `airframe-pp-cli model "Robinson R44" --since 2010` → fatal vs non-fatal breakdown
- *"Are Cirrus SR22s safer than Beechcraft Bonanzas? My uncle just bought one."*
  → two `airframe-pp-cli model` calls + comparison

**Following the news** *(Tier 2)*
- *"There was a small plane crash near Aspen yesterday. Look it up — what happened?"*
  → `airframe-pp-cli model --state CO --since 2026-05-12` → most recent matching event + narrative
- *"Find the fatal aviation accidents in Texas this year."*
  → `airframe-pp-cli model --state TX --since 2026-01-01 --highest-injury FATL`

**Investigative / journalism** *(Tier 1; +2 for incident lookups)*
- *"Who really owns this private jet — N79RF? I think it's a Delaware LLC."*
  → `airframe-pp-cli tail N79RF` *(shows the registrant; LLC dead-ends honestly — not a beneficial-ownership tool)*
- *"Find every aircraft registered to a Delaware LLC with 'aviation' in the name."*
  → `airframe-pp-cli owner "aviation" --state DE`
- *"What aircraft are registered to Berkshire Hathaway and what's their fleet make-up?"*
  → `airframe-pp-cli owner "Berkshire Hathaway"`
- *"Has this aircraft ever crashed or been in any reported incidents?"*
  → `airframe-pp-cli tail <N> --select results.history`

**Pre-purchase / aviation enthusiast** *(Tier 2; +FTS for narrative search)*
- *"I'm thinking about buying a 1978 Cessna 182. Anything bad in the accident history for that vintage?"*
  → `airframe-pp-cli model "Cessna 182" --since 1976 --until 1980`
- *"Are there any patterns in icing-related accidents for my plane's model (Piper Archer)?"*
  → `airframe-pp-cli search "icing"` *(needs `--with-fts` sync)*
- *"Show me incidents where the Cirrus parachute (CAPS) was deployed."*
  → `airframe-pp-cli search "CAPS OR parachute deployed"` *(needs `--with-fts` sync)*

### Day-of flight check (airframe + flight-goat)

> **Heads up — this works only within ~72 hours of departure.** Airlines don't assign a specific tail to a flight until 48–72 hours before takeoff. If you're shopping a ticket a week+ out, there is no "specific plane" to look up yet. For pre-purchase questions, use the **model-level lookup instead** — most airline booking sites already tell you the equipment type (e.g. "Boeing 737 MAX 8"), and you can run `airframe-pp-cli model "Boeing 737 MAX 8"` directly without needing flight-goat at all.

**Day-of curiosity** *(Tier 1 + flight-goat, must be ≤72 hours before departure)*
- *"I'm flying United 1234 in a few hours. What plane is it and has this specific tail had any incidents?"*
  → `airframe-pp-cli flight UA1234`
- *"I just checked in for AA200 — what's the tail and how old is it?"*
  → `airframe-pp-cli flight AA200 --select results.flight.registration,results.aircraft.year_mfr`

**Recently flown / post-incident research** *(Tier 1+2 + flight-goat)*
- *"The flight UA89 from EWR to TLV that was delayed last Tuesday — what plane was it, and what's the operator's safety record?"*
  → `airframe-pp-cli flight UA89` (returns the past 14 days of tails on that ident) + `airframe-pp-cli owner "<operator>"` drill-down
- *"Has the specific plane I flew on yesterday ever been in an incident?"*
  → `airframe-pp-cli flight UA89 --select results.history`

**Probabilistic route research** *(Tier 1+2 + flight-goat)*
- *"What tails has United been using on UA897 over the past two weeks? Any concerning patterns?"*
  → `airframe-pp-cli flight UA897` returns the pool of recent tails; airframe enriches each

#### What you can do without flight-goat

If the airline's website tells you the equipment type (almost all do), you don't need flight-goat at all — just look up the model:

- *"I'm booked on a 737 MAX-8 next month. Should I be worried?"*
  → `airframe-pp-cli model "Boeing 737 MAX 8"`
- *"I can fly JFK→LHR on a 787 or a 777-300ER. Which has the better safety record?"*
  → two `airframe-pp-cli model` calls + comparison

### Through Claude Code

The `pp-airframe` skill encodes the decision tree for these questions. When a user asks Claude Code any of the questions above, the skill runs `airframe-pp-cli doctor` first to figure out what's installed and synced, then either answers directly (if everything is ready) or walks the user through the missing piece (install mdbtools, sync NTSB, install flight-goat, set the AeroAPI key) before answering. Cryptic errors like *"mdb-export not found"* never reach the user — they see a plain-language explanation and an offer to fix it.

### Quick CLI cheatsheet

```bash
# Tier 1 only: who owns this Gulfstream?
airframe-pp-cli tail N628TS

# Tier 1: just the owner, as a JSON string
airframe-pp-cli tail N628TS --select results.aircraft.owner_name

# Tier 1: what aircraft does Falcon Landing LLC fly?
airframe-pp-cli owner "FALCON LANDING"

# Tier 2: model safety profile
airframe-pp-cli model "Cessna 172"

# Tier 1 + flight-goat: pre-flight safety check
airframe-pp-cli flight UA1234

# Tier 2 + FTS: full-text narrative search
airframe-pp-cli search "icing"
```

---

## Two sync tiers

> **The most important thing to understand before installing.**

`airframe` ingests **two separate datasets**. They have **different install requirements**.

### Tier 1 — FAA Aircraft Registry (default, zero dependencies)

```bash
airframe-pp-cli sync          # ← what you get out of the box
```

| | |
|---|---|
| **Provides** | Every US-registered aircraft: tail number → owner, make/model, year, engine, status |
| **Install** | Just `airframe-pp-cli` — **no system dependencies** |
| **Disk** | ~80 MB (FAA core) |
| **First sync** | ~30 seconds |
| **Commands that work** | `tail`, `owner`, `doctor`, `flight` (with flight-goat) |

If all you want is "who owns this plane" and "what aircraft does this owner have," **Tier 1 alone is enough** and you can stop reading here.

### Tier 2 — NTSB Accident History (opt-in, requires `mdbtools`)

```bash
airframe-pp-cli sync --source ntsb     # ← explicit opt-in
airframe-pp-cli sync --source all      # ← FAA + NTSB in one go
```

| | |
|---|---|
| **Provides** | Every NTSB-investigated US aviation event since 1982: tail → accident history, model-level safety stats |
| **Install requirement** | **`mdbtools` package** (see below) — the NTSB ships data as Microsoft Access `.mdb` |
| **Disk** | +50–100 MB on top of Tier 1 |
| **First sync** | ~60 seconds (after mdbtools is installed) |
| **Additional commands** | `model`, `event`, `search` (with `--with-fts`) |

#### Installing `mdbtools` (Tier 2 only)

| OS | Install |
|---|---|
| **macOS** | `brew install mdbtools` |
| **Debian / Ubuntu** | `sudo apt install mdbtools` |
| **Fedora / RHEL** | `sudo dnf install mdbtools` |
| **Arch** (via AUR) | `yay -S mdbtools` |
| **Windows** | Use WSL and follow the Ubuntu line |

`airframe-pp-cli doctor` reports whether `mdb-export` is detected and where.

#### Why isn't mdbtools bundled or replaced?

NTSB only publishes their accident database in Microsoft Access binary format. There is no public CSV/JSON mirror, and pure-Go Access readers are not production-grade. `mdbtools` is the open-source standard for this exact job and packages cleanly on every common Linux distribution and macOS. We ask you to install it once on your machine instead of bundling it into our binary (which complicates licensing and cross-platform builds).

---

## Install

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/cmd/airframe-pp-cli@latest
```

Verify:

```bash
airframe-pp-cli --version
airframe-pp-cli doctor
```

`doctor` shows you exactly what's wired up (Tier 1 data, Tier 2 data, mdbtools, flight-goat) and what's missing.

---

## Sync flags reference

```bash
airframe-pp-cli sync                # FAA only (default)
airframe-pp-cli sync --source ntsb  # NTSB only (requires mdbtools)
airframe-pp-cli sync --source all   # both
airframe-pp-cli sync --full         # ignore If-Modified-Since cache

# Tier 1 (FAA) options:
--include-dereg          # +100-200 MB historical deregistrations
--include-addresses      # +30-50 MB owner street/city/state/zip

# Tier 2 (NTSB) options (require --source ntsb or all):
--include-pre1982        # +30 MB events from before 1982
--full-narratives        # +120 MB full narratives, zstd-compressed

# Indexing:
--with-fts               # +50-100 MB FTS5 full-text index for `search`

# One-flag-all:
--everything             # everything above (requires mdbtools)
```

### Sync profile size matrix

| Profile | Tier | Disk |
|---|---|---|
| Default (FAA core) | 1 | 80–100 MB |
| `--source all` (FAA + NTSB core) | 1+2 | 150–200 MB |
| `--source all --with-fts` | 1+2 | 200–300 MB |
| `--source all --include-addresses --with-fts` | 1+2 | 250–350 MB |
| `--everything` | 1+2 | 800 MB – 1 GB |

---

## Commands

| Command | Tier | What it does |
|---|---|---|
| `sync [flags]` | both | Download bulk data, replace local tables, VACUUM |
| `tail <N-number>` | 1 + 2* | Full dossier: aircraft + make/model + engine + accident history (history empty without Tier 2) |
| `owner "<name>"` | 1 | List aircraft for a registered owner |
| `model "<make> [model]"` | 2 | Aggregate NTSB events for a model |
| `event <event-id>` | 2 | Single NTSB event with all aircraft involved |
| `flight <ident>` | 1 + 2* | Resolve flight ident via flight-goat, then enrich (needs `FLIGHT_GOAT_API_KEY_AUTH`) |
| `search "<query>"` | 1+2 + FTS | Full-text search; requires `--with-fts` sync |
| `doctor` | — | DB freshness, schema version, mdbtools detection, flight-goat detection |

`*` Tier 2 enriches but isn't required for the command to run.

Every command supports `--json` for machine output, `--select <dotted-path>` for field extraction, and `--db-path` to override the default DB location (`~/.local/share/airframe-pp-cli/data.db`).

---

## How it composes with `flight-goat`

`airframe-pp-cli flight UA1234` shells out to `flight-goat-pp-cli aircraft owner get-aircraft UA1234 --agent` to resolve the flight ident to a tail number (via FlightAware AeroAPI). That requires:

1. `flight-goat-pp-cli` installed (`go install github.com/mvanhorn/printing-press-library/library/travel/flight-goat/cmd/flight-goat-pp-cli@latest`)
2. `FLIGHT_GOAT_API_KEY_AUTH` set to a valid FlightAware AeroAPI key

If either is missing, `airframe flight` returns a precise hint and does not invoke flight-goat. Other airframe commands work without flight-goat.

---

## JSON envelope

Every command's `--json` output uses a stable `{meta, results}` envelope:

```json
{
  "meta": {
    "source": "local",
    "synced_at": "2026-05-13T00:53:08Z",
    "db_path": "/home/.../data.db",
    "query": { "registration": "N628TS" }
  },
  "results": { ... }
}
```

Use `--select` to extract a subtree:

```bash
airframe-pp-cli tail N628TS --select results.aircraft.owner_name
```

---

## Caveats

- **Owner data may be a Delaware LLC dead-end.** Many private jets register through shell companies. airframe reports what the FAA registry says — it is not a beneficial-ownership tool.
- **NTSB only investigates incidents meeting their reporting threshold.** "No NTSB events" is not the same as "never had a maintenance issue."
- **`make_model_code` for NTSB-side events is NULL in v1.** NTSB stores aircraft make/model as free text; the `model` aggregation joins through the FAA registry, which means events involving aircraft not in the FAA database (foreign tails, historical reg numbers no longer assigned) are undercounted.
- **`flight` requires a paid AeroAPI key** routed via flight-goat. FAA + NTSB are free; ident-to-tail resolution is what costs.

---

## License

Apache-2.0.
