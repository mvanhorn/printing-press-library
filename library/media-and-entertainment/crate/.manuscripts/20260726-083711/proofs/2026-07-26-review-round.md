# crate-pp-cli — adversarial review round

All six honesty claims from the shipcheck writeup verified as holding. Two bugs nonetheless falsified the numbers those claims describe.

## Bugs fixed
1. **`floor` summed mixed currencies and labelled the total with whichever arrived first.** Seeded GBP 10 + USD 20 + JPY 5000 produced `floor total 5030.00 GBP`, with the per-row table printing all three currencies beneath it. The cache was keyed on `release_id` alone, so `--currency` was also silently ignored for anything already cached. The key is now `(release_id, req_currency)`, and totals are reported per returned currency — never added across them.
2. **An unrecognised 200 wiped the local collection and recorded it as a successful sync.** Because auto-sync only fires when a sync record is absent, the user was then permanently stuck on "collection is empty" with no error. A response carrying none of `releases`/`wants`/`results` is now an error, not an empty collection.
3. **The username was interpolated into the URL path unescaped** — the delivery mechanism for #2. Now `url.PathEscape`d in both `loadShelf` and `shelf-sync`.
4. **`--any` on `spin` was a no-op that printed a false explanation.** `f.Unrated` was set from the flag and then unconditionally overwritten with `false`, while `Pick` always preferred unrated. The preference is now an explicit argument to `Pick`.
5. **Auto-sync discarded the truncation flag**, so a 6,000-record collection silently became 5,000 and every downstream count described part of the shelf. Now warned, with the command to fix it.
6. **`--format LP` and `--format 7"`, both advertised in help, could never match.** Discogs puts those in `formats[].descriptions`; only `formats[].name` was read.
7. **`dig`'s truncation warning went only to stderr** while the headline count on stdout read as complete. Now on both.
8. **`dig --country` / `--format` alone errored as directionless.**
9. **`Breakdown(nil, "bogus")` returned no error** — the validation sat inside the range loop.

## Not found
No SQL injection: every query is parameterized. `ReplaceRecords`' transaction and rollback are correct. The `internal/crate` unit tests were judged genuinely meaningful; both wrong-answer bugs shipped in the untested CLI layer.

## After
7/7 shipcheck legs · 12 Go test packages green, with regression tests added for the currency scoping and the `--any` no-op.
