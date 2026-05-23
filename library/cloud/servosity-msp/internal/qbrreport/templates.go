// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package qbrreport

// htmlTemplate is the self-contained QBR HTML document. All CSS is
// inlined so the file renders identically when opened directly in a
// browser or piped through headless Chrome for PDF conversion. The
// cover page uses `page-break-after: always` so the PDF starts the
// body content on page 2.
const htmlTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>QBR — {{.Company.Name}} — {{.Quarter}}</title>
<style>
  :root {
    --ink: #1f2937;
    --muted: #6b7280;
    --rule: #e5e7eb;
    --accent: #0f766e;
    --bg-soft: #f9fafb;
  }
  * { box-sizing: border-box; }
  html, body {
    margin: 0; padding: 0;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen, Ubuntu, sans-serif;
    color: var(--ink);
    font-size: 12pt;
    line-height: 1.55;
  }
  main { padding: 0.75in 0.75in 0.5in 0.75in; max-width: 7.5in; margin: 0 auto; }
  h1, h2, h3 { color: var(--ink); margin: 0 0 0.4em 0; font-weight: 600; }
  h1 { font-size: 28pt; letter-spacing: -0.02em; }
  h2 { font-size: 16pt; margin-top: 1.4em; padding-bottom: 0.2em; border-bottom: 1px solid var(--rule); }
  h3 { font-size: 13pt; color: var(--muted); }
  p { margin: 0 0 0.7em 0; }
  .muted { color: var(--muted); }
  .cover {
    min-height: 9.5in;
    padding: 1in 0.75in;
    display: flex; flex-direction: column; justify-content: space-between;
    page-break-after: always;
  }
  .cover-top h1 { font-size: 36pt; margin-bottom: 0.2em; }
  .cover-top .quarter { font-size: 18pt; color: var(--accent); margin-bottom: 0.4em; }
  .cover-top .prep { color: var(--muted); font-size: 13pt; }
  .cover-bottom { color: var(--muted); font-size: 11pt; border-top: 1px solid var(--rule); padding-top: 0.5em; }
  table { width: 100%; border-collapse: collapse; margin: 0.4em 0 1em 0; font-size: 11pt; }
  th, td { padding: 0.4em 0.55em; text-align: left; border-bottom: 1px solid var(--rule); vertical-align: top; }
  th { background: var(--bg-soft); font-weight: 600; color: var(--muted); text-transform: uppercase; font-size: 9.5pt; letter-spacing: 0.04em; }
  .check { text-align: center; font-size: 13pt; color: var(--accent); }
  .bar-outer { background: var(--bg-soft); height: 14px; border-radius: 7px; overflow: hidden; margin: 0.4em 0; border: 1px solid var(--rule); }
  .bar-inner { background: var(--accent); height: 100%; }
  .summary-card { background: var(--bg-soft); border-left: 4px solid var(--accent); padding: 0.8em 1em; border-radius: 4px; }
  .stat-row { display: flex; gap: 1.2em; margin: 0.4em 0 0.8em 0; }
  .stat { flex: 1; }
  .stat .label { font-size: 9.5pt; color: var(--muted); text-transform: uppercase; letter-spacing: 0.04em; }
  .stat .value { font-size: 18pt; font-weight: 600; }
  .empty { color: var(--muted); font-style: italic; }
</style>
</head>
<body>

<section class="cover">
  <div class="cover-top">
    <div class="quarter">{{.Quarter}}</div>
    <h1>Quarterly Backup Review</h1>
    <h3>{{.Company.Name}}</h3>
    <p class="prep">Prepared by {{.Reseller.Name}}</p>
  </div>
  <div class="cover-bottom">
    Generated {{fmtDate .GeneratedAt}} · Window {{fmtDate .From}} – {{fmtDate .To}}
  </div>
</section>

<main>

<h2>Executive summary</h2>
<div class="summary-card">{{summary .}}</div>

<h2>Backup coverage</h2>
{{if .Coverage}}
<table>
  <thead><tr>
    <th>Device</th><th>Classic</th><th>Restic</th><th>DR</th>
    <th>Last successful</th><th>Status</th>
  </tr></thead>
  <tbody>
    {{range .Coverage}}
    <tr>
      <td>{{.Device}}</td>
      <td class="check">{{check .Classic}}</td>
      <td class="check">{{check .Restic}}</td>
      <td class="check">{{check .DR}}</td>
      <td>{{fmtDate .LastSuccessful}}</td>
      <td>{{nonEmpty .Status "—"}}</td>
    </tr>
    {{end}}
  </tbody>
</table>
{{else}}
<p class="empty">No protected devices found in the local store.</p>
{{end}}

<h2>Job success rate</h2>
{{if .SuccessRate.Total}}
<div class="stat-row">
  <div class="stat"><div class="label">Success rate</div><div class="value">{{pct .SuccessRate.Rate}}</div></div>
  <div class="stat"><div class="label">Jobs run</div><div class="value">{{.SuccessRate.Total}}</div></div>
  <div class="stat"><div class="label">Hard fail</div><div class="value">{{.SuccessRate.HardFail}}</div></div>
  <div class="stat"><div class="label">Dirty</div><div class="value">{{.SuccessRate.Dirty}}</div></div>
  <div class="stat"><div class="label">Retried OK</div><div class="value">{{.SuccessRate.RetriedSucceeded}}</div></div>
</div>
{{successBar .SuccessRate}}
{{else}}
<p class="empty">No backup jobs recorded in this quarter.</p>
{{end}}

<h2>Restore tests</h2>
{{if .RestoreTests}}
<p><strong>{{len .RestoreTests}}</strong> test(s) recorded this quarter.</p>
<table>
  <thead><tr><th>Date</th><th>Device</th><th>Outcome</th><th>Notes</th></tr></thead>
  <tbody>
    {{range .RestoreTests}}
    <tr>
      <td>{{fmtDate .Date}}</td>
      <td>{{.Device}}</td>
      <td>{{nonEmpty .Outcome "—"}}</td>
      <td>{{.Notes}}</td>
    </tr>
    {{end}}
  </tbody>
</table>
{{else}}
<p class="empty">No restore tests recorded in this quarter.</p>
{{end}}

<h2>Open issues</h2>
{{if .OpenIssues}}
<table>
  <thead><tr><th>Severity</th><th>Opened</th><th>Title</th></tr></thead>
  <tbody>
    {{range .OpenIssues}}
    <tr>
      <td>{{nonEmpty .Severity "—"}}</td>
      <td>{{fmtDate .OpenedAt}}</td>
      <td>{{.Title}}</td>
    </tr>
    {{end}}
  </tbody>
</table>
{{else}}
<p class="empty">No open issues. Nice quarter.</p>
{{end}}

<h2>Storage trend</h2>
{{if hasTrend .StorageTrend}}
<p>{{len .StorageTrend}} data point(s) across the quarter.</p>
<table>
  <thead><tr><th>Date</th><th>Storage</th></tr></thead>
  <tbody>
    {{range .StorageTrend}}
    <tr><td>{{fmtDate .At}}</td><td>{{fmtBytes .Bytes}}</td></tr>
    {{end}}
  </tbody>
</table>
{{else}}
<p class="empty">No history yet — storage trend will populate as snapshots accumulate.</p>
{{end}}

</main>
</body>
</html>
`
