# Live verification log — grants-pp-cli

Run: 20260705-183000 · executed 2026-07-06T17:19:29Z against live APIs, no mocks.

## grants-pp-cli version
```
grants-pp-cli 1.0.0
exit: 0
```

## grants-pp-cli doctor
```
🩺 grants-pp-cli doctor — élő API-ellenőrzés / live API check
  ✔ Grants.gov   OK (746 nyitott 'health' kiírás)
  ✔ NIH RePORTER OK (591424 'cancer' projekt)
  ✔ NSF          OK (1 találat lekérve)
  Minden forrás él. / All sources up.
exit: 0
```

## grants-pp-cli search cancer --rows 3
```
🔎 "cancer" — 132 nyitott kiírás összesen, 3 mutatva
  RFA-CA-27-020  HHS-NIH11 zárás: 10/19/2026  Advanced Development of Informatics Technologies for Cancer Research …
  RFA-CA-27-019  HHS-NIH11 zárás: 10/19/2027  Early-Stage Development of Informatics Technologies for Cancer Resear…
  PAR-25-444     HHS-NIH11 zárás: 09/25/2028  Cancer Center Support Grants (CCSGs) for NCI-designated Cancer Center…
exit: 0
```

## grants-pp-cli search cancer --agency HHS-NIH11 --rows 3
```
🔎 "cancer" — 80 nyitott kiírás összesen, 3 mutatva
  RFA-CA-27-020  HHS-NIH11 zárás: 10/19/2026  Advanced Development of Informatics Technologies for Cancer Research …
  RFA-CA-27-019  HHS-NIH11 zárás: 10/19/2027  Early-Stage Development of Informatics Technologies for Cancer Resear…
  PAR-25-444     HHS-NIH11 zárás: 09/25/2028  Cancer Center Support Grants (CCSGs) for NCI-designated Cancer Center…
exit: 0
```

## grants-pp-cli search cancer --rows 3 --closing-before 2026-12-31
```
warning: results may be truncated (deadline filter applied to first 3 rows only)
🔎 "cancer" — 132 nyitott kiírás összesen, 1 mutatva (határidő ≤ 2026-12-31)
  RFA-CA-27-020  HHS-NIH11 zárás: 10/19/2026  Advanced Development of Informatics Technologies for Cancer Research …
exit: 0
```

## grants-pp-cli search microbiome --rows 3 --min-award 500000
```
🔎 "microbiome" — 34 nyitott kiírás összesen, 0 mutatva
  (nincs találat a szűrőkkel / no results with these filters)
exit: 0
```

## grants-pp-cli search microbiome --rows 3 --eligibility university
```
🔎 "microbiome" — 34 nyitott kiírás összesen, 0 mutatva
  (nincs találat a szűrőkkel / no results with these filters)
exit: 0
```

## grants-pp-cli nih cancer --year 2025 --rows 3 --min-amount 1000000
```
🏥 NIH RePORTER "cancer" — 20987 megítélt projekt összesen, 3 mutatva (award szerint csökkenő)
  1ZIHLM200888-17  $357,194,999  FY2025  NATIONAL LIBRARY OF MEDICINE National Biomedical Information Services
  75N91019D00024-0-759102500016-10  $82,503,495  FY2025  LEIDOS BIOMEDICAL RESEARCH,… NCI-Frederick Operational Support
  75N91019D00024-P00011-759101900139-1  $41,960,680  FY2025  LEIDOS BIOMEDICAL RESEARCH,… MD NET
exit: 0
```

## grants-pp-cli nsf "quantum computing" --rows 3 --min-amount 500000
```
🔬 NSF "quantum computing" — 0 megítélt grant mutatva
  (nincs találat / no results)
exit: 0
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
ok  	github.com/mvanhorn/printing-press-library/library/health/grants/internal/cli	0.505s
?   	github.com/mvanhorn/printing-press-library/library/health/grants/internal/sources	[no test files]
exit: 0
```
