# `score` field on `api hpn search` results

The `score` field on each row is what the Happenstance public API
returns plus a small CLI-side adjustment. This doc explains what
contact-goat does with that score so users know when to use
`--min-score` versus `--first-degree-only`.

## What the CLI does with the score

`--min-score N` drops rows where the rendered score is below `N`. It
runs after the API call and bridge-affinity adjustment, so the
threshold applies to what you actually see in the output. `0` (the
default) is a no-op; `--min-score 5` typically drops weak-signal
public-graph entries (rows where the bearer surface returned no graph
affinity at all but kept the row in the result set anyway).

`--first-degree-only` drops rows lacking any `self_graph` bridge after
the per-row `currentUUID` retag. This is the right tool when you want
"only my 1st-degree connections" - it filters by graph membership, not
by score. Use `--min-score` for noise reduction; use
`--first-degree-only` for tier filtering.

The two filters compose. The intersection is the SF-task case: only
1st-degree, with public-graph noise dropped.

## Score range, observationally

Per `internal/cli/flagship_helpers.go` (the file with the bridge-affinity
math), typical scores sit between 10 and 100. The strongest signals
observed in captured traffic reach around 300. Public-graph rows with
no graph affinity show as 0 to 2.

Do not encode "high-confidence 1st-degree" as a fixed threshold in
your scripts. Real 1st-degree bridges sit at medium-affinity values
(observed 49.99 in fixtures) and a high blanket cutoff would drop
them.

## Where the score actually comes from

The score is a Happenstance product detail, not a CLI behavior. The
canonical reference is Happenstance's own documentation. This doc is
intentionally narrow: it describes only what the CLI does with the
value (the `--min-score` filter, the bridge-affinity adjustment). If
Happenstance changes the underlying scoring model, the CLI keeps
working - the threshold semantics in the help text just become less
useful until you re-tune.
