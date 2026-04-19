// Instacart dumped-orders exporter. Run this after extract-one.js has
// walked every pending order (check dumper.js output for pending_extract
// reaching 0). Triggers a browser download of instacart-orders.jsonl to
// the user's default Downloads directory.
//
// Then run:
//     instacart history import ~/Downloads/instacart-orders.jsonl
//
// Skip records (from extract-one.js failures) are filtered out of the
// JSONL but counted in the return summary so the caller can warn the
// user about coverage gaps.

(() => {
  const DUMPED_KEY = '__ic_dumped';

  let data;
  try {
    data = JSON.parse(localStorage.getItem(DUMPED_KEY) || '[]');
  } catch (e) {
    return JSON.stringify({ err: 'dumped_store_corrupt', detail: String(e) });
  }
  if (!Array.isArray(data) || data.length === 0) {
    return JSON.stringify({ err: 'no_dumped_records', hint: 'run dumper.js + extract-one.js first' });
  }

  const valid = data.filter((r) => r && !r.skipped && r.order_id);
  const skipped = data.length - valid.length;

  if (valid.length === 0) {
    return JSON.stringify({
      err: 'only_skip_records',
      skipped_count: skipped,
      hint: 'every extract-one.js run failed; check cache key and re-run',
    });
  }

  const jsonl = valid.map((r) => JSON.stringify(r)).join('\n') + '\n';
  const blob = new Blob([jsonl], { type: 'application/x-ndjson' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = 'instacart-orders.jsonl';
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);

  return JSON.stringify({
    downloaded: true,
    filename: 'instacart-orders.jsonl',
    bytes: jsonl.length,
    orders: valid.length,
    skipped_count: skipped,
    next_command: 'instacart history import ~/Downloads/instacart-orders.jsonl',
  });
})();
