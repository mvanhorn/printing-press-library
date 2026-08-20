# Live verification log — grants-pp-cli

Run: 20260705-183000 · originally executed 2026-07-06T17:19:29Z against live APIs, no mocks.
Re-executed 2026-08-10 against live APIs after the Hungarian-to-English string pass; the
transcripts below are from that re-run, so they show current output verbatim. Hit counts
differ from the original run because the upstream data changed, not the code.

## grants-pp-cli version
```
grants-pp-cli 1.0.0
exit: 0
```

## grants-pp-cli doctor
```
🩺 grants-pp-cli doctor — live API check
  ✔ Grants.gov   OK (605 open 'health' opportunities)
  ✔ NIH RePORTER OK (594649 'cancer' projects)
  ✔ NSF          OK (1 result(s) fetched)
  All sources up.
exit: 0
```

## grants-pp-cli search cancer --rows 3
```
🔎 "cancer" — 123 open opportunities total, 3 shown
  RFA-CA-27-020  HHS-NIH11 closes: 10/19/2026  Advanced Development of Informatics Technologies for Cancer Research …
  RFA-CA-27-019  HHS-NIH11 closes: 10/19/2027  Early-Stage Development of Informatics Technologies for Cancer Resear…
  PAR-25-444     HHS-NIH11 closes: 09/25/2028  Cancer Center Support Grants (CCSGs) for NCI-designated Cancer Center…
exit: 0
```

## grants-pp-cli search cancer --rows 3 --closing-before 2026-12-31
```
warning: results may be truncated (deadline filter applied to first 3 rows only)
🔎 "cancer" — 123 open opportunities total, 1 shown (deadline ≤ 2026-12-31)
  RFA-CA-27-020  HHS-NIH11 closes: 10/19/2026  Advanced Development of Informatics Technologies for Cancer Research …
exit: 0
```

## grants-pp-cli nih cancer --year 2025 --rows 3 --min-amount 1000000
```
🏥 NIH RePORTER "cancer" — 20987 awarded projects total, 3 shown (sorted by award, descending)
  1ZIHLM200888-17  $357,194,999  FY2025  NATIONAL LIBRARY OF MEDICINE National Biomedical Information Services
  75N91019D00024-0-759102500016-10  $82,503,495  FY2025  LEIDOS BIOMEDICAL RESEARCH,… NCI-Frederick Operational Support
  75N91019D00024-P00011-759101900139-1  $41,960,680  FY2025  LEIDOS BIOMEDICAL RESEARCH,… MD NET
exit: 0
```

## grants-pp-cli nsf "quantum computing" --rows 3 --min-amount 500000
```
  (warn: --min-amount applies only to the first 3 unsorted NSF results; increase --rows to fetch more)
🔬 NSF "quantum computing" — 2 awarded grants shown
  2535115       $636,000  03/15/2027→02/28/2030 Montana State University   EPSCoR Graduate Fellowship Program (EGFP): Grad…
  2620285     $2,000,000  01/01/2027→12/31/2032 University of Colorado at… S-STEM: Future STEM Professionals in Artificial…
exit: 0
```

## grants-pp-cli search (no keyword — error path)
```
a keyword is required: grants-pp-cli search <keyword>
exit: 2
```

## grants-pp-cli search cancer --rows 2 --json
```
{
  "keyword": "cancer",
  "opportunities": [
    {
      "id": 359855,
      "number": "RFA-CA-27-020",
      "title": "Advanced Development of Informatics Technologies for Cancer Research and Management (U24 Clinical Trial Optional)",
      "agencyCode": "HHS-NIH11",
      "agency": "National Institutes of Health",
      "openDate": "05/21/2026",
      "closeDate": "10/19/2026",
      "oppStatus": "posted"
exit: 0
```

## go test ./...
```
?   	github.com/mvanhorn/printing-press-library/library/health/grants/cmd/grants-pp-cli	[no test files]
ok  	github.com/mvanhorn/printing-press-library/library/health/grants/internal/cli	0.507s
?   	github.com/mvanhorn/printing-press-library/library/health/grants/internal/sources	[no test files]
exit: 0
```
