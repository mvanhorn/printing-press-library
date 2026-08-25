# Weekly & Monthly NEJM Report Scripts

These PowerShell scripts generate a formatted Excel report of recent NEJM
articles using the `nejm-pp-cli` (which must be installed and available).

They add **no new behavior to the CLI** — the scripts only call commands the CLI
already ships (`sql`, `article --enrich`). You can delete this folder and the CLI
is unchanged.

## Files

- `weekly.ps1` — last 7 days
- `monthly.ps1` — last 30 days

Both scripts produce:

- `weekly_articles.csv` / `monthly_articles.csv` — raw data (semicolon-delimited, UTF-8)
- `weekly_articles.xlsx` / `monthly_articles.xlsx` — formatted Excel with:
  - fixed column widths
  - abstract column wrapped
  - frozen header row
  - AutoFilter
  - centered `date` and `is_free` columns

## Requirements

- Windows with PowerShell 5.1+ (built-in)
- `nejm-pp-cli` on your `PATH`, or built locally (`make build` → `bin/nejm-pp-cli`).
  The scripts locate the binary automatically.
- Microsoft Excel (optional — without it you still get the CSV)

## Usage

Open PowerShell in this folder and run:

```powershell
# Weekly report
powershell -ExecutionPolicy Bypass -File .\weekly.ps1

# Monthly report
powershell -ExecutionPolicy Bypass -File .\monthly.ps1
```

The scripts will:

1. Fetch all NEJM articles from the last 7 / 30 days via `nejm-pp-cli sql`
2. Enrich each article with full metadata using `nejm-pp-cli article --enrich`
3. Print a summary to the console
4. Save the CSV and formatted XLSX files in the current directory

## Notes

- If you get "No articles found", run `nejm-pp-cli sync` first to populate the local corpus.
- If the output file is locked (open in Excel), the script writes a timestamped copy instead of failing.
- The scripts do not require any additional PowerShell modules.
