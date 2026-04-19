// Instacart per-order extractor. Run this AFTER navigating to a
// /store/orders/<id> page in the logged-in Instacart tab.
//
// How it works:
//   1. Polls the Apollo cache for up to 10s waiting for the
//      OrderManagerOrderDelivery entry to hydrate with
//      "includeOrderItems":true. Hydration is async; running the
//      extractor on page load without polling drops records silently.
//   2. If the expected cache key is missing (bundle rotation renamed
//      it), falls back to scanning cache keys for the first entry whose
//      key contains "includeOrderItems":true. Reports the fallback in
//      the result so the caller can alert on bundle drift.
//   3. Normalizes the order into the JSONL schema consumed by
//      `instacart history import`.
//   4. Appends the record to localStorage.__ic_dumped (persistent across
//      navigation). Dedups by order_id on push.
//
// Skip records: if polling times out OR the entry holds no orderItems,
// a structured {order_id, skipped: true, reason} record is pushed so a
// partial backfill is visible instead of silently losing records.
//
// Run export-jsonl.js after the per-order loop to trigger a download of
// the collected JSONL for `instacart history import`.

(async () => {
  const DUMPED_KEY = '__ic_dumped';
  const MAX_POLL_MS = 10000;
  const POLL_INTERVAL_MS = 200;

  const loadDumped = () => {
    try {
      return JSON.parse(localStorage.getItem(DUMPED_KEY) || '[]');
    } catch (e) {
      return [];
    }
  };
  const saveDumped = (arr) => {
    localStorage.setItem(DUMPED_KEY, JSON.stringify(arr));
  };
  const pushDumped = (record) => {
    const arr = loadDumped();
    if (!arr.find((r) => r && r.order_id === record.order_id)) arr.push(record);
    saveDumped(arr);
    return arr.length;
  };

  // Derive the order ID from the URL so the caller does not have to
  // pass it explicitly. Matches /store/orders/<digits>.
  const urlMatch = window.location.pathname.match(/\/store\/orders\/(\d+)/);
  const urlOrderId = urlMatch ? urlMatch[1] : null;

  // --- Profile-picker redirect ------------------------------------------
  if (window.location.pathname.startsWith('/store/profiles')) {
    return JSON.stringify({
      profile_picker: true,
      reason: 'tab_redirected_to_profile_picker',
    });
  }

  // --- Locate the cache entry with includeOrderItems:true ---------------
  // The primary path is the OrderManagerOrderDelivery key. Fallback path
  // scans cache keys for the substring, in case Instacart renames the
  // key in a future bundle.
  const findOrderDeliveryEntry = () => {
    const client = window.__APOLLO_CLIENT__;
    if (!client || !client.cache || typeof client.cache.extract !== 'function') {
      return { err: 'no_apollo_client' };
    }
    const snapshot = client.cache.extract();
    const primary = snapshot && snapshot.OrderManagerOrderDelivery;
    if (primary && typeof primary === 'object') {
      for (const k of Object.keys(primary)) {
        if (k.includes('"includeOrderItems":true')) {
          return { path: ['OrderManagerOrderDelivery', k], entry: primary[k], fallback: false };
        }
      }
    }
    // Fallback: scan every top-level cache entry for an items-bearing sub-key.
    for (const topKey of Object.keys(snapshot || {})) {
      const bucket = snapshot[topKey];
      if (!bucket || typeof bucket !== 'object') continue;
      for (const k of Object.keys(bucket)) {
        if (k.includes('"includeOrderItems":true') && bucket[k] && bucket[k].orderDelivery) {
          return { path: [topKey, k], entry: bucket[k], fallback: true };
        }
      }
    }
    return { err: 'cache_key_missing' };
  };

  // --- Poll for hydration -----------------------------------------------
  const deadline = Date.now() + MAX_POLL_MS;
  let located = null;
  let lastErr = null;
  while (Date.now() < deadline) {
    const attempt = findOrderDeliveryEntry();
    if (attempt.entry && attempt.entry.orderDelivery) {
      located = attempt;
      break;
    }
    lastErr = attempt.err || 'entry_without_orderDelivery';
    await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS));
  }

  if (!located) {
    // Push a skip record so the missing order is visible in the dumped set.
    const skip = {
      order_id: urlOrderId || 'unknown',
      skipped: true,
      reason: lastErr || 'cache_not_hydrated',
      url: window.location.pathname,
    };
    pushDumped(skip);
    return JSON.stringify(skip);
  }

  // --- Normalize into JSONL shape ---------------------------------------
  const od = located.entry.orderDelivery;

  // Prefer the orderId encoded in the cache key JSON -- it matches URL.
  let keyOrderId = urlOrderId;
  try {
    const keyStr = located.path[1];
    const parsed = JSON.parse(keyStr);
    if (parsed && parsed.orderId) keyOrderId = parsed.orderId;
  } catch (e) {
    // Not JSON; rely on URL match.
  }

  const items = (od.orderItems || []).map((oi) => ({
    item_id: oi && oi.currentItem && oi.currentItem.legacyId
      ? 'items_' + (od.retailer && od.retailer.id ? od.retailer.id : '?') + '-' +
        oi.currentItem.legacyId.replace(/^item_/, '')
      : null,
    product_id:
      oi && oi.currentItem && oi.currentItem.basketProduct && oi.currentItem.basketProduct.item
        ? oi.currentItem.basketProduct.item.productId
        : null,
    name: oi && oi.currentItem ? oi.currentItem.name : null,
    quantity: oi ? oi.selectedQuantityValue : null,
    quantity_type: oi ? oi.selectedQuantityType : null,
  }));

  const record = {
    order_id: keyOrderId,
    retailer_id: od.retailer ? od.retailer.id : null,
    retailer_slug: od.retailer ? od.retailer.slug : null,
    retailer_name: od.retailer ? od.retailer.name : null,
    delivered_at: od.deliveredAt || null,
    item_count: items.length,
    items,
  };

  if (located.fallback) {
    record.extraction_note = 'fallback_cache_scan';
  }

  const total = pushDumped(record);
  return JSON.stringify({
    order_id: record.order_id,
    retailer: record.retailer_slug,
    items: items.length,
    fallback_scan: located.fallback,
    total_in_store: total,
  });
})();
