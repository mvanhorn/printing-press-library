// Instacart order-history dumper. Paste this into DevTools console on a
// logged-in Instacart tab (starting from
// https://www.instacart.com/store/account/orders) or run it from Chrome
// MCP via the /pp-instacart backfill skill flow.
//
// What it does:
//   1. Scrolls the orders page to force infinite-scroll / clicks "Load
//      more" until no new order IDs appear.
//   2. Accumulates the IDs into localStorage.__ic_backfill_state.seen_ids
//      so that re-runs after a tab crash or interrupted session resume
//      instead of re-walking.
//   3. Returns a JSON summary the skill (or a human) can read to decide
//      whether to proceed to the per-order extract loop.
//
// This script does NOT pull order contents. The per-order detail (items,
// retailer, delivery timestamp) lives in the Apollo cache on each order's
// /store/orders/<id> page and is captured by extract-one.js.
//
// Profile-picker handling: if Instacart redirected the tab to
// /store/profiles (multi-profile accounts do this before serving private
// data), the script exits early and returns a sentinel. The caller is
// expected to prompt the user to pick a profile and re-run.
//
// See docs/patterns/authenticated-session-scraping.md for why Chrome MCP
// or interactive DevTools is the right tier for this type of scraping.

(async () => {
  const STATE_KEY = '__ic_backfill_state';
  const MAX_SEEN_IDS = 2000; // hard cap to prevent runaway localStorage

  // --- Profile-picker short-circuit --------------------------------------
  if (window.location.pathname.startsWith('/store/profiles')) {
    return JSON.stringify({
      profile_picker: true,
      reason: 'tab_redirected_to_profile_picker',
      action: 'pick a profile manually, then re-run the dumper',
    });
  }

  // --- Load / init resume state ------------------------------------------
  const loadState = () => {
    try {
      const raw = localStorage.getItem(STATE_KEY);
      if (!raw) return { seen_ids: [], started_at: new Date().toISOString() };
      const parsed = JSON.parse(raw);
      if (!Array.isArray(parsed.seen_ids)) parsed.seen_ids = [];
      return parsed;
    } catch (e) {
      return { seen_ids: [], started_at: new Date().toISOString() };
    }
  };
  const saveState = (s) => {
    // Trim to MAX_SEEN_IDS most-recent to prevent unbounded growth over
    // months of backfill use. Order is preserved (most-recently-added last).
    if (s.seen_ids.length > MAX_SEEN_IDS) {
      s.seen_ids = s.seen_ids.slice(-MAX_SEEN_IDS);
    }
    s.updated_at = new Date().toISOString();
    localStorage.setItem(STATE_KEY, JSON.stringify(s));
  };

  const state = loadState();
  const resumedFromCount = state.seen_ids.length;
  const seen = new Set(state.seen_ids);

  // --- ID collection helpers ---------------------------------------------
  const collect = () => {
    const found = new Set();
    for (const a of document.querySelectorAll('a[href^="/store/orders/"]')) {
      const href = a.getAttribute('href') || '';
      const m = href.match(/\/store\/orders\/(\d+)/);
      if (m) found.add(m[1]);
    }
    let added = 0;
    for (const id of found) {
      if (!seen.has(id)) {
        seen.add(id);
        added++;
      }
    }
    return added;
  };

  // --- Infinite-scroll phase ---------------------------------------------
  // Loop until scroll position stabilizes for 3 consecutive iterations
  // OR we hit the page cap. Each iteration waits 1.4s for lazy-load.
  collect();
  let stableScroll = 0;
  const SCROLL_ITERS = 40;
  for (let i = 0; i < SCROLL_ITERS && stableScroll < 3; i++) {
    window.scrollTo(0, document.body.scrollHeight);
    await new Promise((r) => setTimeout(r, 1400));
    const added = collect();
    if (added === 0) stableScroll++;
    else stableScroll = 0;
  }

  // --- "Load more" click phase -------------------------------------------
  // Bounded retries with linear backoff. CDP gets upset if you click the
  // same button too aggressively -- 400ms scroll-into-view, 2s post-click
  // wait, and a backoff factor of +500ms per successful click.
  const LOAD_MORE_ITERS = 40;
  let loadMoreDelay = 2000;
  for (let i = 0; i < LOAD_MORE_ITERS; i++) {
    const btn = Array.from(document.querySelectorAll('button')).find((b) =>
      /^load more/i.test((b.innerText || '').trim())
    );
    if (!btn) break;
    btn.scrollIntoView({ block: 'center' });
    await new Promise((r) => setTimeout(r, 400));
    try {
      btn.click();
    } catch (e) {
      // Button was removed between find and click. Not fatal; loop will
      // refind on the next iteration (or exit if it is truly gone).
      continue;
    }
    await new Promise((r) => setTimeout(r, loadMoreDelay));
    const added = collect();
    if (added === 0) break;
    loadMoreDelay = Math.min(loadMoreDelay + 500, 5000);
  }

  // --- Persist state -----------------------------------------------------
  state.seen_ids = Array.from(seen);
  state.last_page_height = document.body.scrollHeight;
  saveState(state);

  // --- Cross-check dumped set to report what is still pending ------------
  let dumpedIds = new Set();
  try {
    const dumped = JSON.parse(localStorage.getItem('__ic_dumped') || '[]');
    for (const r of dumped) if (r && r.order_id) dumpedIds.add(r.order_id);
  } catch (e) {
    // corrupt __ic_dumped; leave dumpedIds empty so the caller treats
    // everything as pending and re-extracts.
  }
  const pending = state.seen_ids.filter((id) => !dumpedIds.has(id));

  return JSON.stringify({
    total_ids: state.seen_ids.length,
    new_ids_this_run: state.seen_ids.length - resumedFromCount,
    already_dumped: dumpedIds.size,
    pending_extract: pending.length,
    resumed: resumedFromCount > 0,
    state_key: STATE_KEY,
    pending_sample: pending.slice(0, 5),
  });
})();
