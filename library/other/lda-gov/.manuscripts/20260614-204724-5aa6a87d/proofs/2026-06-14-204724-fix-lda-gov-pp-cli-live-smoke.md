Live smoke passed against the public LDA.gov API and a tiny local SQLite mirror.

Live sync commands:

```bash
./lda-gov-pp-cli --agent sync --resources filings,contributions --latest-only --max-pages 1 --db /var/folders/g8/s5fyqf0j19911vk1dx9kjml00000gn/T/opencode/lda-gov-dogfood.db
./lda-gov-pp-cli --agent sync --resources registrants,clients,lobbyists --latest-only --max-pages 1 --db /var/folders/g8/s5fyqf0j19911vk1dx9kjml00000gn/T/opencode/lda-gov-dogfood.db
```

Results:

- filings: 25 records synced
- contributions: 25 records synced
- registrants: 25 records synced
- clients: 25 records synced
- lobbyists: 25 records synced
- sync warnings: 0
- sync errors: 0

Novel command dogfood:

- `audit filings --limit 5`: returned an empty risk list on the sampled page, valid for a risk-only command.
- `audit spend`: returned grouped client/registrant/issue spend totals from live filings.
- `graph export --limit 10`: returned bounded de-duplicated relationship edges.
- `contributions totals`: returned LD-203 item totals from live contribution reports.
- `reports quarter`: returned period summaries combining filings and contribution totals.
- `lobbyists covered-positions --limit 5`: returned covered-position rows with source URLs.
- `entities resolve GENENTECH --limit 5`: returned the synced official client record.

Local-only contract:

- `--data-source live audit filings` rejected with the expected "no live equivalent" error.
- Missing local mirror produced `[]` on stdout plus a sync hint on stderr.
