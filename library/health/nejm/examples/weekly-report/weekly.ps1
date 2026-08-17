# weekly.ps1  --  Weekly NEJM report (last 7 days)
#
# One-command workflow: fetch the last 7 days of NEJM articles through
# nejm-pp-cli, enrich each with full metadata, print a console summary, and
# write both a CSV and a formatted Excel (.xlsx) file.
#
# It only composes existing nejm-pp-cli commands (`sql` and `article --enrich`);
# it adds no new behavior to the CLI. See README.md for details.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File .\weekly.ps1

$ErrorActionPreference = 'Stop'

# --- Report window / output configuration (the only difference vs monthly.ps1) ---
$WindowDays  = 7
$WindowLabel = 'last 7 days'
$ReportTitle = 'WEEKLY'
$BaseName    = 'weekly_articles'

# --- Locate the nejm-pp-cli binary (PATH first, then a local build) ---
function Resolve-NejmCli {
    $cmd = Get-Command nejm-pp-cli -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }

    $candidates = @(
        (Join-Path $PSScriptRoot '..\..\bin\nejm-pp-cli.exe'),
        (Join-Path $PSScriptRoot '..\..\bin\nejm-pp-cli'),
        (Join-Path $PSScriptRoot '..\..\nejm-pp-cli.exe'),
        (Join-Path $PSScriptRoot '..\..\nejm-pp-cli')
    )
    foreach ($p in $candidates) {
        if (Test-Path $p) { return (Resolve-Path $p).Path }
    }

    throw "nejm-pp-cli not found. Add it to your PATH, or build it from the CLI root:`n  make build   (produces bin/nejm-pp-cli)"
}

$cli = Resolve-NejmCli
Write-Host "Using CLI: $cli" -ForegroundColor DarkGray

# --- List the DOIs published in the report window ---
# The `sql` command emits JSON when its output is piped (not a TTY), so parse
# the rows as JSON and pull the doi field rather than scraping text columns.
$sqlOut = & $cli --json sql "SELECT doi FROM article WHERE date >= date('now', '-$WindowDays days')"
$dois = @()
if ($sqlOut) {
    try {
        $dois = @(($sqlOut | Out-String | ConvertFrom-Json) |
            ForEach-Object { $_.doi } |
            Where-Object { $_ -and "$_".Trim() -ne "" })
    } catch {
        Write-Host "ERROR: could not parse 'nejm-pp-cli sql' output as JSON. $_" -ForegroundColor Red
    }
}

$total = $dois.Count
$i = 0
$results = @()

Write-Host ""
Write-Host "FETCHING $ReportTitle ARTICLES ($WindowLabel)" -ForegroundColor Cyan
Write-Host "========================================"
Write-Host "Total articles: $total"
Write-Host ""

if ($total -eq 0) {
    Write-Host "No articles in the $WindowLabel. Has the corpus been synced? Try: nejm-pp-cli sync" -ForegroundColor Yellow
}

foreach ($doi in $dois) {
    $i++
    $doi = $doi.Trim()
    if ([string]::IsNullOrEmpty($doi)) { continue }

    Write-Host "[$i / $total] $doi" -ForegroundColor Yellow

    $jsonOutput = & $cli --json article $doi --enrich 2>$null

    if ($LASTEXITCODE -eq 0 -and $jsonOutput) {
        try {
            $data = $jsonOutput | Out-String | ConvertFrom-Json
            $article = [PSCustomObject]@{
                doi          = $doi
                title        = $data.title
                date         = $data.date
                authors      = $data.authors
                abstract     = $data.abstract
                article_type = $data.article_type
                specialties  = $data.specialties
                is_free      = $data.is_free
            }
            Write-Host "  $($article.title)" -ForegroundColor Green
            Write-Host "    Author(s): $($article.authors)" -ForegroundColor Gray
            $results += $article
        } catch {
            Write-Host "  ERROR: JSON parsing failed for $doi" -ForegroundColor Red
        }
    } else {
        Write-Host "  ERROR: enrich request failed for $doi" -ForegroundColor Red
    }
    Start-Sleep -Milliseconds 500
}

Write-Host ""
Write-Host "$($results.Count) of $total articles enriched." -ForegroundColor Cyan

if ($results.Count -eq 0) {
    Write-Host ""
    Write-Host "No enriched articles to report. Run 'nejm-pp-cli sync' to populate the corpus, then re-run." -ForegroundColor Yellow
    return
}

# --- Console summary ---
Write-Host ""
Write-Host "$ReportTitle NEJM SUMMARY ($WindowLabel)" -ForegroundColor Cyan
Write-Host "=========================================="
Write-Host ""
Write-Host "Total articles: $($results.Count)"

$types = $results | Where-Object { $_.article_type -and $_.article_type -ne "" } | Group-Object article_type | Sort-Object Count -Descending
if ($types.Count -gt 0) {
    Write-Host ""
    Write-Host "Breakdown by type:"
    foreach ($t in $types) {
        Write-Host "   $($t.Name): $($t.Count)"
    }
} else {
    Write-Host ""
    Write-Host "No type information available."
}

$latest = $results | Where-Object { $_.date } | Sort-Object { [datetime]$_.date } -Descending | Select-Object -First 5
Write-Host ""
Write-Host "Top 5 newest articles:"
$n = 1
foreach ($a in $latest) {
    $date = $a.date
    if ($date.Length -ge 10) { $date = $date.Substring(0, 10) }
    Write-Host "   $n. $($a.title) ($date)"
    $n++
}

$specialties = $results |
    Where-Object { $_.specialties -and $_.specialties -ne "" } |
    ForEach-Object { $_.specialties -split "\|" } |
    ForEach-Object { $_.Trim() } |
    Group-Object | Sort-Object Count -Descending
if ($specialties.Count -gt 0) {
    Write-Host ""
    Write-Host "Top specialties:"
    $specialties | Select-Object -First 5 | ForEach-Object {
        Write-Host "   $($_.Name): $($_.Count)"
    }
} else {
    Write-Host ""
    Write-Host "No specialty data available."
}

$free = ($results | Where-Object { $_.is_free -eq $true }).Count
Write-Host ""
Write-Host "Open-access articles: $free"

# --- CSV export (semicolon-delimited, UTF-8; timestamped temp if locked) ---
$timestamp    = Get-Date -Format "yyyyMMdd_HHmmss"
$finalCsvPath = Join-Path $PWD "$BaseName.csv"
$tempCsvPath  = Join-Path $PWD "${BaseName}_temp_$timestamp.csv"
$csvPath      = $finalCsvPath
$useTemp      = $false

if (Test-Path $finalCsvPath) {
    try {
        Remove-Item $finalCsvPath -Force -ErrorAction Stop
    } catch {
        Write-Host "WARNING: $finalCsvPath is locked. Using temp: $tempCsvPath" -ForegroundColor Yellow
        $csvPath = $tempCsvPath
        $useTemp = $true
    }
}

try {
    $results | Select-Object doi, title, date, authors, article_type, specialties, is_free, abstract |
        Export-Csv -Path $csvPath -NoTypeInformation -Encoding UTF8 -Delimiter ';' -Force
    Write-Host ""
    Write-Host "CSV exported to $csvPath" -ForegroundColor Green

    if ($useTemp -and (Test-Path $tempCsvPath)) {
        try {
            Move-Item -Path $tempCsvPath -Destination $finalCsvPath -Force -ErrorAction Stop
            $csvPath = $finalCsvPath
        } catch {
            Write-Host "WARNING: Temp CSV remains at $tempCsvPath" -ForegroundColor Yellow
        }
    }
} catch {
    Write-Host "ERROR: Failed to export CSV. $_" -ForegroundColor Red
    $csvPath = $null
}

# --- Formatted XLSX export via Excel COM (requires Microsoft Excel) ---
Write-Host ""
Write-Host "Creating formatted Excel file..." -ForegroundColor Yellow

if ($csvPath -and (Test-Path $csvPath)) {
    try {
        $excel = New-Object -ComObject Excel.Application -ErrorAction Stop
        $excel.Visible       = $false
        $excel.DisplayAlerts = $false

        # Open the CSV as a workbook so we can format and save as xlsx.
        $workbook  = $excel.Workbooks.Open((Resolve-Path $csvPath).Path)
        $worksheet = $workbook.Worksheets.Item(1)

        # Column order: 1=doi 2=title 3=date 4=authors 5=article_type 6=specialties 7=is_free 8=abstract
        $colWidths = @(22, 45, 12, 28, 22, 30, 8, 90)
        for ($c = 1; $c -le $colWidths.Count; $c++) {
            $worksheet.Columns($c).ColumnWidth = $colWidths[$c - 1]
        }

        # Abstract (col 8): wrap text so rows grow tall, not wide.
        $worksheet.Columns(8).WrapText = $true

        # Data rows (row 2+): left + top alignment.
        $lastRow = $worksheet.UsedRange.Rows.Count
        if ($lastRow -gt 1) {
            $dataRange = $worksheet.Range($worksheet.Cells.Item(2, 1), $worksheet.Cells.Item($lastRow, $colWidths.Count))
            $dataRange.HorizontalAlignment = -4131  # xlLeft
            $dataRange.VerticalAlignment   = -4160  # xlTop
        }

        # date (col 3) and is_free (col 7): centered.
        $worksheet.Columns(3).HorizontalAlignment = -4108  # xlCenter
        $worksheet.Columns(7).HorizontalAlignment = -4108

        # Header row (row 1): bold + centered + frozen.
        $headerRange = $worksheet.Rows(1)
        $headerRange.Font.Bold           = $true
        $headerRange.HorizontalAlignment = -4108
        $headerRange.VerticalAlignment   = -4108

        $worksheet.Activate()
        $excel.ActiveWindow.SplitRow    = 1
        $excel.ActiveWindow.FreezePanes = $true

        # AutoFilter on the header row.
        $worksheet.Rows(1).AutoFilter() | Out-Null

        # Save as xlsx (timestamped copy if the target is locked).
        $xlsxPath = Join-Path $PWD "$BaseName.xlsx"
        if (Test-Path $xlsxPath) {
            try {
                Remove-Item $xlsxPath -Force -ErrorAction Stop
            } catch {
                $xlsxPath = Join-Path $PWD "${BaseName}_$timestamp.xlsx"
                Write-Host "WARNING: existing XLSX locked. Saving as: $xlsxPath" -ForegroundColor Yellow
            }
        }

        $workbook.SaveAs($xlsxPath, 51)  # 51 = xlOpenXMLWorkbook
        $workbook.Close($false)
        $excel.Quit()

        [System.Runtime.InteropServices.Marshal]::ReleaseComObject($excel) | Out-Null
        [GC]::Collect()
        [GC]::WaitForPendingFinalizers()

        Write-Host "Formatted Excel file saved to: $xlsxPath" -ForegroundColor Green
    } catch {
        Write-Host "Excel not available or formatting failed - CSV is still usable. Error: $_" -ForegroundColor Yellow
    }
} else {
    Write-Host "CSV file not available - Excel formatting skipped." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "ALL DONE." -ForegroundColor Green
