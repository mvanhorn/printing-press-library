# Actual built CLI help catalog

Binary: `/Users/zjsng/printing-press/.runstate/zjsng-a9b2d4f4/runs/20260927-212939-86ef661f/working/tabelog-pp-cli/tabelog-pp-cli`

SHA-256: `47593f2acf789e75583e7693fc71ed2eecb8a27717f262e4d0107a640a3e8049`

Visible command help was recursively collected with local `--help` only. Inherited global flags are recorded once in root help. Intentional hidden generic sync/search/analytics surfaces are excluded.

## (root)

```text
Find Japan restaurants and bars, then keep a factual trip shortlist.

Resolve a location, find a few candidates, inspect selected details, and save trip lists. Source ratings, meal budgets, unknown facts and retrieval times remain explicit. Run a command with --help for its criteria and examples.

Usage:
  tabelog-pp-cli [command]

Available Commands:
  areas         Resolve typed prefecture, area and station choices
  cuisines      Browse verified English cuisine and bar taxonomy choices
  doctor        Check CLI health
  find          Find ranked restaurants within explicit location and meal criteria
  help          Help about any command
  lists         Keep notes and compare saved trip candidates
  show          Inspect one restaurant's source facts and practical details
  version       Print version

Flags:
      --agent                Set agent-friendly output defaults (--json --compact --no-input --no-color)
      --compact              Compact output; source commands preserve restaurant decision facts
      --csv                  Output as CSV (table and array responses)
      --data-source string   auto uses fresh cache or fetches; live bypasses cache; local makes no network requests (default "auto")
      --dry-run              Show request without sending
  -h, --help                 help for tabelog-pp-cli
      --home string          Root directory for config, data, state, and cache files
      --json                 Output as JSON
      --max-age duration     Age threshold for saved-evidence hints; 0 disables age checks (default 30m0s)
      --no-cache             Bypass response cache
      --plain                Output as plain tab-separated text
      --quiet                Bare output, one value per line
      --select string        Comma-separated fields to include in output (e.g. --select title,url)
      --timeout duration     Request timeout (default 20s)
  -v, --version              version for tabelog-pp-cli

Use "tabelog-pp-cli [command] --help" for more information about a command.
```

## areas

```text
Resolve typed prefecture, area and station choices

Usage:
  tabelog-pp-cli areas [QUERY] [flags]

Examples:
  tabelog-pp-cli areas Shinjuku --agent
  tabelog-pp-cli areas Ginza --kind station --agent

Flags:
  -h, --help          help for areas
      --kind string   Filter choices by area, station, or prefecture
      --limit int     Maximum typed source choices to return (1–100) (default 20)
```

## cuisines

```text
Browse verified English cuisine and bar taxonomy choices

Usage:
  tabelog-pp-cli cuisines [QUERY] [flags]

Examples:
  tabelog-pp-cli cuisines bar --agent
  tabelog-pp-cli cuisines sushi --agent

Flags:
  -h, --help        help for cuisines
      --limit int   Maximum cuisine/taxonomy choices to return (1–100) (default 20)
```

## doctor

```text
Check CLI health

Usage:
  tabelog-pp-cli doctor [flags]

Examples:
  tabelog-pp-cli doctor
  tabelog-pp-cli doctor --json
  tabelog-pp-cli doctor --fail-on warn
  tabelog-pp-cli doctor --fail-on stale

Flags:
      --fail-on string   Exit non-zero for selected health gates. stale: cache freshness plus errors; warn: path warnings plus errors; error: errors only. Default is never.
  -h, --help             help for doctor
```

## find

```text
Find ranked restaurants within explicit location and meal criteria

Usage:
  tabelog-pp-cli find [flags]

Examples:
  tabelog-pp-cli find --area tokyo --cuisine bar --limit 5 --agent
  tabelog-pp-cli find --area https://tabelog.com/en/tokyo/A1301/A130101/rstLst/ --meal lunch --budget-max 2000

Flags:
      --area string      Required prefecture, typed location selector, or English area URL
      --budget-max int   Source-supported maximum average-price threshold in JPY
      --budget-min int   Source-supported minimum average-price threshold in JPY
      --cuisine string   Verified cuisine slug or taxonomy choice (bar is broad category)
  -h, --help             help for find
      --keyword string   Free text within the verified source geography
      --limit int        Maximum candidates to return (1–50); no detail fan-out (default 5)
      --max-pages int    Maximum source pages to fetch (1–5), independently of result limit (default 1)
      --meal string      Source average-price meal: lunch or dinner
```

## help

```text
Help provides help for any command in the application.
Simply type tabelog-pp-cli help [path to command] for full details.

Usage:
  tabelog-pp-cli help [command] [flags]

Flags:
  -h, --help   help for help
```

## lists

```text
Save fetched restaurants in named trip lists. Show, compare, alternatives and audit use saved facts without network requests; refresh explicitly retrieves current facts.

Usage:
  tabelog-pp-cli lists [flags]
  tabelog-pp-cli lists [command]

Examples:
  tabelog-pp-cli lists add tokyo-bars 13005012 --note 'Ginza bar option' --agent

Available Commands:
  add          Save a fetched restaurant and optional personal note
  alternatives Find factual alternatives within the saved set
  audit        Inspect missing and old evidence without fetching
  compare      Compare saved facts and notes without fetching
  note         Set a personal note on a saved candidate
  refresh      Refresh selected saved facts and preserve personal notes
  remove       Remove membership while retaining fetched source facts
  show         Show saved candidates, or list the notebooks

Flags:
  -h, --help   help for lists


Use "tabelog-pp-cli lists [command] --help" for more information about a command.
```

## show

```text
Inspect one restaurant's source facts and practical details

Usage:
  tabelog-pp-cli show [URL_OR_CACHED_ID] [flags]

Examples:
  tabelog-pp-cli show https://tabelog.com/en/tokyo/A1301/A130103/13294162/ --agent
  tabelog-pp-cli show 13294162 --data-source local

Flags:
  -h, --help   help for show
```

## version

```text
Print version

Usage:
  tabelog-pp-cli version [flags]

Flags:
  -h, --help   help for version
```

## lists add

```text
Save an already fetched restaurant in a named trip list. Re-adding preserves its position and note unless --note is supplied. Use lists refresh to update source facts.

Usage:
  tabelog-pp-cli lists add <list> <id> [flags]

Flags:
  -h, --help          help for add
      --note string   Personal note; preserved on repeat add unless this flag is supplied
```

## lists alternatives

```text
Match saved candidates to an anchor's verified source area and exact normalized category labels. Optional meal budget checks the cached source bracket upper bound. Unknown matching fields are unevaluable; results preserve list order. Use lists compare for general comparison.

Usage:
  tabelog-pp-cli lists alternatives <list> [flags]

Flags:
      --budget-max int   Maximum known cached meal-bracket upper bound in JPY
      --for string       Saved anchor restaurant ID
  -h, --help             help for alternatives
      --match string     Comma-separated anchor fields: area,category
      --meal string      Meal for cached budget comparison: lunch or dinner
```

## lists audit

```text
Inspect saved evidence states and snapshot age. Findings distinguish detail_not_fetched, source_unknown and older_than_threshold. Source-unknown facts may stay unknown after another fetch. Use lists refresh to retrieve current facts.

Usage:
  tabelog-pp-cli lists audit <list> [flags]

Flags:
  -h, --help             help for audit
      --max-age string   Snapshot age threshold; 0 disables age checks; supports h, d and w (default "24h")
      --require string   Required evidence fields, comma-separated (default "hours,payment,reservation,dinner_budget")
```

## lists compare

```text
Compare known source facts, personal notes and snapshot ages in list order. Missing details remain not fetched; source-unknown facts remain unknown. Use lists alternatives to find a backup for one candidate.

Usage:
  tabelog-pp-cli lists compare <list> [id...] [flags]

Flags:
  -h, --help   help for compare
```

## lists note

```text
Set a personal note on a saved candidate

Usage:
  tabelog-pp-cli lists note <list> <id> [flags]

Flags:
  -h, --help          help for note
      --note string   Personal note; an explicitly empty note clears it
```

## lists refresh

```text
Fetch current detail facts for up to 20 saved restaurants, at most two at a time within a 20-second operation deadline. Report changed source facts separately from newly obtained evidence. A failed record retains its last valid snapshot and note; any per-record failure gives a non-success exit. Use lists audit for a network-free evidence check.

Usage:
  tabelog-pp-cli lists refresh <list> [id...] [flags]

Flags:
  -h, --help   help for refresh
```

## lists remove

```text
Remove membership while retaining fetched source facts

Usage:
  tabelog-pp-cli lists remove <list> <id> [flags]

Flags:
  -h, --help   help for remove
```

## lists show

```text
Show saved candidates, or list the notebooks

Usage:
  tabelog-pp-cli lists show [list] [flags]

Flags:
  -h, --help   help for show
```
