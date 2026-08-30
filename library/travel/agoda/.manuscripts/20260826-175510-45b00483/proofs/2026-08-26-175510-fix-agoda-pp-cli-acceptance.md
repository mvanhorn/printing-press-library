# Agoda CLI acceptance report

```
Acceptance Report: agoda
  Level: Full Dogfood
  Tests: 107/107 passed (73 skipped as not-applicable)
  Failures: 0
  Gate: PASS
```

Auth context: `none`. Agoda's public hotel surfaces need no credentials, so the
full matrix ran against the live API rather than mocks.

## Failures found and fixed during Phase 5
The first full run failed 3 of 107:

1. **`feedback --help` missing an Examples section.** A generated framework
   command: `feedback list` carried an example but the `feedback` parent did
   not. Added an Examples block to the parent.
2 & 3. **`vip delta` scored as a failure on exit 4** (both `happy_path` and
   `json_fidelity`). The command is behaving correctly - with no session cookie
   it reports that member pricing cannot be measured and exits with the
   auth-required code. The `pp:typed-exit-codes` annotation was present on the
   `delta` leaf, but the live matrix attributes results to the `vip` parent, so
   the parent needed the same declaration. Added it there.

Re-run after those fixes: **107 passed, 0 failed.**

## Behavioral assertions (content, not just exit codes)
These were run against live Tokyo data, checking output correctness rather than
merely that the command returned zero:

| Check | Assertion | Result |
|---|---|---|
| `hotels search` | every all-in price >= advertised price; no zero-priced rows | PASS (45/45, markups 10.0%-40.9%) |
| `hotels rank` | returned list is genuinely sorted ascending by all-in price | PASS (42 of 49 properties changed position vs Agoda's ordering) |
| `hotels fees` | every `outlier` flag matches the threshold rule; `excess_over_median` arithmetic exact | PASS (0 mislabeled, 0 math errors) |
| `prices cheapest` | reported `cheapest_price` equals the true minimum; list sorted ascending | PASS (46 days covered, min 80.00 matched) |
| `compare` | `cheapest_property_id` is the actual minimum of the returned set | PASS |
| `search` | every match contains the query term | PASS ("prince" -> 2/2 relevant) |
| `search` (negative) | a nonsense query returns zero matches, not noise | PASS (0 matches, honest note) |
| `search` (relevance) | "osaka" returns Osaka properties, not Tokyo ones | PASS |
| `watch run` (seeded) | with history seeded 25% above current price, exactly the seeded property is flagged | PASS (1 drop at -20.0%, 0 false positives across the other 48) |
| Error paths | missing destination, single-id compare, non-numeric ids, malformed window, unsupported `--sort` | PASS (all exit 2 with actionable messages) |

## Printing Press issues for retro
1. **Dead framework helpers.** `collectionItemsForOutput`,
   `paginatedGetWithResponsePath`, and `successfulNoop` are emitted
   unconditionally but unused for a spec without pagination or collection
   endpoints. Holds Dead Code at 2/5 through no fault of the printed CLI.
2. **`feedback` parent command ships without an Examples section**, which its
   own live-dogfood help check then fails.
3. **Novel-command scaffolds default to `mcp:read-only: "false"`** even for
   plainly read-only features, and every one had to be corrected by hand.
4. **Scaffold parents inherit the research.json `group` string as their Cobra
   `Short`**, producing help text like "Honest pricing" for a `hotels` parent.
5. **All scaffold flags are emitted as `StringVar`** regardless of the type
   implied by the example, so `--nights 2` arrived as a string flag.
6. **`pp:typed-exit-codes` is not inherited from a leaf to the parent the live
   matrix attributes results to**, so a correct typed exit is scored as a
   failure unless the annotation is duplicated on the parent.

## Observed once, not reproduced
A Go runtime `throw` appeared in one combined test+shipcheck run. Three
subsequent `go test -count=1 ./...` runs and two further full shipchecks were
clean, and it never recurred. Recorded here rather than claimed fixed, because
the cause was never identified.
