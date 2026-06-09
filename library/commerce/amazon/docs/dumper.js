// docs/dumper.js — browser-side Amazon order-history dumper.
//
// Paste this into the DevTools console while logged into amazon.com. It
// crawls /your-orders pagination and emits one JSON object per order to the
// console, formatted as JSONL that `amazon-pp-cli history import` accepts.
//
// Usage:
//   1. Open https://www.amazon.com/your-orders
//   2. Open DevTools (Cmd-Opt-I), Console tab
//   3. Paste this file's contents, press Enter
//   4. Wait for "DONE: N orders" — right-click the console output, "Save as..."
//   5. Strip the leading "VM####:N" prefixes — usually `pbpaste | jq -c '.' > orders.jsonl`
//
// Notes
// - This only reads what's on the page; it does NOT scrape product images or reviews.
// - Amazon's order-history HTML is verbose and brittle. This dumper handles the
//   2025-era layout. If Amazon redesigns /your-orders, update the selectors below.

(async () => {
  const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
  const orders = [];

  function parseOrderCard(card) {
    const orderId = (card.querySelector("bdi")?.innerText
      || card.querySelector(".yohtmlc-order-id .value")?.innerText
      || "").trim();
    const placedAt = (card.querySelector(".yohtmlc-order-date .value")?.innerText
      || card.querySelector(".order-info .a-col-left .a-row .a-column:nth-child(1) .value")?.innerText
      || "").trim();
    const total = (card.querySelector(".yohtmlc-order-total .value")?.innerText
      || card.querySelector(".order-info .a-col-left .a-row .a-column:nth-child(2) .value")?.innerText
      || "").trim();

    const items = [];
    card.querySelectorAll(".a-fixed-left-grid").forEach((row) => {
      const link = row.querySelector("a.a-link-normal[href*='/gp/product/']");
      const titleEl = row.querySelector(".yohtmlc-item .a-link-normal")
        || row.querySelector(".a-fixed-left-grid-col.a-col-right .a-link-normal");
      const qtyEl = row.querySelector("span.item-view-qty");
      if (!link) return;
      const m = link.href.match(/\/gp\/product\/([A-Z0-9]{10})/);
      const asin = m ? m[1] : "";
      const title = (titleEl?.innerText || "").trim();
      const qty = qtyEl ? parseInt(qtyEl.innerText, 10) || 1 : 1;
      items.push({ asin, title, quantity: qty });
    });
    return { order_id: orderId, placed_at: placedAt, total, items };
  }

  let page = 1;
  while (true) {
    console.log("scraping page", page);
    document.querySelectorAll(".order-card, .order").forEach((card) => {
      const o = parseOrderCard(card);
      if (o.order_id) orders.push(o);
    });
    const next = document.querySelector("li.a-last:not(.a-disabled) a");
    if (!next) break;
    next.click();
    page += 1;
    await sleep(1200);
    if (page > 30) break; // safety cap
  }

  console.log("DONE:", orders.length, "orders");
  console.log(orders.map((o) => JSON.stringify(o)).join("\n"));
})();
