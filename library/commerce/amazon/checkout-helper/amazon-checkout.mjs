#!/usr/bin/env node
// amazon-checkout.mjs — Playwright-driven cart view + place-order for amazon-pp-cli.
//
// Why: the static-HTTP cart path doesn't render Amazon's JS-decrypted cart cells,
// and the static place-order POST trips robot-check. This shells out from the Go
// CLI to a real browser, runs JS, and reads the same DOM the user would see.
//
// Exit codes (must match the Go CLI's internal/cli/root.go):
//   0   success — JSON payload on stdout
//   2   usage error
//   7   transient/network failure
//   9   manual required (CAPTCHA, sign-in challenge, etc.) — JSON has deeplink
//
// Args: <action> <cookies.json> [--place-order]
//   action: "cart-show" | "checkout"
//   --place-order: only honored for action=checkout; clicks the actual button.
//                  Without it, checkout stops at the order-review page.

import { chromium } from "playwright";
import fs from "node:fs";

const [, , action, cookiesPath, ...rest] = process.argv;
const wantPlace = rest.includes("--place-order");

// add-to-cart args: <action> <cookies.json> <ASIN> [--quantity N]
const addAsin = action === "add-to-cart" ? rest.find((a) => /^[A-Z0-9]{10}$/.test(a)) : null;
const qtyArg = (() => {
  const i = rest.indexOf("--quantity");
  if (i >= 0 && rest[i + 1]) return parseInt(rest[i + 1], 10);
  return 1;
})();

if (!action || !cookiesPath) {
  console.error("usage: amazon-checkout.mjs <cart-show|checkout|add-to-cart|history-sync> <cookies.json> [<ASIN>] [--place-order] [--quantity N]");
  process.exit(2);
}
if (!["cart-show", "checkout", "add-to-cart", "history-sync"].includes(action)) {
  console.error(`unknown action: ${action}`);
  process.exit(2);
}
if (action === "add-to-cart" && !addAsin) {
  console.error("add-to-cart requires an ASIN as a positional arg (10 uppercase letters/digits)");
  process.exit(2);
}

const raw = JSON.parse(fs.readFileSync(cookiesPath, "utf8"));
const cookies = raw.cookies.map((c) => {
  const out = { name: c.name, value: c.value, domain: c.domain, path: c.path || "/" };
  if (c.expires && !c.expires.startsWith("0001")) {
    const t = Date.parse(c.expires);
    if (!Number.isNaN(t)) out.expires = Math.floor(t / 1000);
  }
  return out;
});

const SAFARI_UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 " +
  "(KHTML, like Gecko) Version/18.6 Safari/605.1.15";

const STEALTH_INIT = `
  // Hide webdriver
  Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
  // Realistic plugin/MIME types stubs (Safari doesn't expose Chrome's, but Amazon
  // primarily checks for the absence of webdriver and the presence of plausible
  // navigator state).
  if (!navigator.languages || navigator.languages.length === 0) {
    Object.defineProperty(navigator, 'languages', { get: () => ['en-US', 'en'] });
  }
  // Permissions API shim
  const _query = window.navigator.permissions?.query;
  if (_query) {
    window.navigator.permissions.query = (params) =>
      params.name === 'notifications'
        ? Promise.resolve({ state: Notification.permission })
        : _query(params);
  }
`;

function captchaSelectors() {
  return [
    'form[action*="validateCaptcha"]',
    "#captchacharacters",
    'img[src*="captcha"]',
  ];
}

async function detectManualGate(page) {
  // Returns { kind, deeplink } if a manual gate is up, else null.
  const url = page.url();
  if (/\/ap\/signin/.test(url) || /\/ax\/claim/.test(url)) {
    return { kind: "sign-in", deeplink: "https://www.amazon.com/gp/cart/view.html" };
  }
  for (const sel of captchaSelectors()) {
    const found = await page.$(sel);
    if (found) {
      return { kind: "captcha", deeplink: page.url() };
    }
  }
  const bodyText = (await page.textContent("body")) || "";
  if (/to discuss automated access/i.test(bodyText) ||
      /enter the characters you see below/i.test(bodyText)) {
    return { kind: "captcha", deeplink: page.url() };
  }
  return null;
}

function manualExit(kind, deeplink, extra = {}) {
  process.stdout.write(JSON.stringify({
    status: "manual_required",
    kind,
    deeplink,
    ...extra,
  }) + "\n");
  process.exit(9);
}

function transientExit(message) {
  process.stderr.write(message + "\n");
  process.exit(7);
}

async function readCart(page) {
  return await page.evaluate(() => {
    function txt(el) { return (el && (el.innerText || el.textContent) || "").replace(/\s+/g, " ").trim(); }
    // Amazon often renders titles twice (visible + screen-reader copy);
    // collapse "X X" into "X" when both halves are equal.
    function dedupTitle(t) {
      if (!t) return t;
      const half = Math.floor(t.length / 2);
      const left = t.slice(0, half).trim();
      const right = t.slice(half).trim();
      if (left.length > 5 && (left === right || `${left} ${right.slice(0, left.length)}` === t.slice(0, left.length * 2 + 1))) {
        return left;
      }
      return t;
    }
    const items = [];
    // Find a saved-for-later boundary by document position. Any .sc-list-item
    // that appears AFTER this point is excluded. This works even if Amazon
    // changes the data-name attribute or class structure, because the boundary
    // text is the actual user-visible heading.
    const savedBoundary = (() => {
      // Restrict to actual section headings — links and small divs elsewhere
      // can contain "Saved for later" too (sidebar nav, account menu), and
      // those would incorrectly exclude active items that follow them.
      const candidates = [
        ...document.querySelectorAll("h1,h2,h3,h4"),
        ...document.querySelectorAll('[data-name*="Saved" i]'),
      ];
      for (const el of candidates) {
        const t = (el.innerText || el.textContent || "").trim();
        if (/^Saved for later(?:\s|$|\()/i.test(t) && t.length < 60) return el;
      }
      return null;
    })();
    const isAfterBoundary = (el) =>
      savedBoundary && (savedBoundary.compareDocumentPosition(el) & Node.DOCUMENT_POSITION_FOLLOWING);

    // Try the most-specific selector first; fall back to the broader one.
    let rows = document.querySelectorAll('[data-name="Active Items"] .sc-list-item');
    if (rows.length === 0) {
      rows = document.querySelectorAll(".sc-list-item, .sc-list-item-content");
    }
    // Active-cart positive identification: a row only counts as an actual cart
    // item if it has at least one of the controls that ONLY active cart rows
    // expose — a quantity selector, a delete action, or a "Save for later"
    // action. Recommendations, items-of-interest, buy-it-again, and
    // saved-for-later rows can share the same .sc-list-item-content class but
    // never have all of these.
    function isActiveCartRow(row) {
      const hasQtySelect = !!row.querySelector('select[name^="quantity"], .sc-quantity-textfield, [data-feature-id="quantity"] select');
      const hasDelete = !!row.querySelector('input[data-action="delete"], [data-action="delete-active"], [aria-label*="Delete" i], [value="Delete"]');
      const hasSaveForLater = !!row.querySelector('input[data-action="save-for-later"], [data-action="save-for-later"], [aria-label*="Save for later" i], [value*="Save for later" i]');
      return hasQtySelect || hasDelete || hasSaveForLater;
    }

    rows.forEach((row) => {
      if (isAfterBoundary(row)) return;
      // Also reject if any ancestor's data-name says "Saved..."
      let p = row;
      while (p && p !== document.body) {
        const name = p.getAttribute && p.getAttribute("data-name");
        if (name && /saved/i.test(name)) return;
        p = p.parentElement;
      }
      // POSITIVE check: must look like an active-cart row.
      if (!isActiveCartRow(row)) return;
      // Prefer a single anchor text (the product link's own visible text),
      // which is one DOM node and not subject to the visible+SR dupe.
      const link = row.querySelector('a.a-link-normal[href*="/dp/"]');
      let title = "";
      if (link) {
        title = txt(link);
      }
      if (!title) {
        const titleEl = row.querySelector('.sc-product-title, .a-truncate-cut');
        title = dedupTitle(txt(titleEl));
      } else {
        title = dedupTitle(title);
      }
      const priceEl = row.querySelector('.sc-product-price, [data-action="show-price-details"] .a-color-price');
      const price = txt(priceEl);
      // Qty parsing — Amazon's modern cart uses a styled a-dropdown widget,
      // not a plain <select>. Order matters: authoritative sources (data-quantity
      // on the row, .sc-quantity-textfield's actual value) come BEFORE
      // .a-dropdown-prompt because S&S items show frequency ("2 months") in a
      // separate dropdown-prompt and a naive \d+ would grab the "2".
      const qty = (() => {
        // 1. data-quantity on row or ancestor (most authoritative — set by Amazon's
        //    cart renderer directly from the server response).
        let p = row;
        while (p && p !== document.body) {
          const dq = p.getAttribute && p.getAttribute("data-quantity");
          if (dq && /^\d+$/.test(dq)) return parseInt(dq, 10);
          p = p.parentElement;
        }
        // 2. .sc-quantity-textfield / .sc-product-quantity (also authoritative)
        let el = row.querySelector('.sc-quantity-textfield, .sc-product-quantity');
        if (el) {
          const v = el.value || el.innerText || el.textContent || "";
          const m = v.match(/^\s*(\d+)/);
          if (m) return parseInt(m[1], 10);
        }
        // 3. Classic <select name="quantityN">
        el = row.querySelector('select[name^="quantity"]');
        if (el && el.value && /^\d+$/.test(el.value)) return parseInt(el.value, 10);
        // 4. Hidden input
        el = row.querySelector('input[type="hidden"][name^="quantity"]');
        if (el && el.value && /^\d+$/.test(el.value)) return parseInt(el.value, 10);
        // 5. Aria-label "Quantity 3" — REQUIRE the word "Quantity" so we don't
        //    grab numbers from unrelated labels.
        const ariaEl = row.querySelector('[aria-label*="Quantity" i]');
        if (ariaEl) {
          const m = (ariaEl.getAttribute("aria-label") || "").match(/Quantity[^0-9]*(\d+)/i);
          if (m) return parseInt(m[1], 10);
        }
        // 6. a-dropdown-prompt — ONLY if scoped to a qty feature container.
        //    Naked .a-dropdown-prompt also catches S&S "2 months" frequency.
        el = row.querySelector('[data-feature-id="quantity"] .a-dropdown-prompt');
        if (el) {
          const m = (el.innerText || "").match(/^\s*(\d+)\s*$/);
          if (m) return parseInt(m[1], 10);
        }
        return -1;
      })();
      // ASIN: try data-asin on the row first (always present on active items),
      // then fall back to parsing the /dp/ link (sometimes missing on
      // freshly-added rows or specific product types).
      let asin = "";
      let pAsin = row;
      while (pAsin && pAsin !== document.body) {
        const a = pAsin.getAttribute && pAsin.getAttribute("data-asin");
        if (a && /^[A-Z0-9]{10}$/.test(a)) { asin = a; break; }
        pAsin = pAsin.parentElement;
      }
      if (!asin && link?.href) {
        const m = link.href.match(/\/(?:gp\/product|dp)\/([A-Z0-9]{10})/);
        if (m) asin = m[1];
      }
      if (title || asin) items.push({ asin, title, quantity: qty, price });
    });

    // Subtotal — try the dedicated subtotal IDs in priority order. Never fall
    // back to .sc-price (which is the first item's price, not the cart total).
    let subtotal = "";
    for (const sel of [
      '#sc-subtotal-amount-buybox',
      '#sc-subtotal-amount-activecart',
      '[data-feature-id="proceed-to-checkout-buybox"] .a-price',
      '[data-feature-id="proceed-to-checkout-action"] .a-price-whole',
    ]) {
      const el = document.querySelector(sel);
      if (el && txt(el)) { subtotal = txt(el); break; }
    }
    return { items, subtotal };
  });
}

async function readDefaults(page) {
  // On checkout / order review page, the default address and payment are shown.
  return await page.evaluate(() => {
    function txt(el) { return (el && (el.innerText || el.textContent) || "").replace(/\s+/g, " ").trim(); }
    const addr = txt(document.querySelector('[data-testid="default-shipping-address"], .ship-to-this-address, .displayAddressDiv, #addressChangeLinkId'));
    // Card last-4 lives in patterns like "····1234" or "ending in 1234"
    const bodyText = document.body.innerText || "";
    let cardLast4 = "";
    const m1 = bodyText.match(/(?:ending in|·{2,}|⋯|•+)\s*(\d{4})/);
    if (m1) cardLast4 = m1[1];
    return { address: addr, card_last4: cardLast4 };
  });
}

async function main() {
  let browser;
  try {
    browser = await chromium.launch({
      headless: true,
      executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH || undefined,
      args: ["--no-sandbox", "--disable-blink-features=AutomationControlled"],
    });
  } catch (e) {
    transientExit(`failed to launch chromium: ${e.message}`);
  }

  const context = await browser.newContext({
    userAgent: SAFARI_UA,
    viewport: { width: 1280, height: 900 },
    locale: "en-US",
  });
  await context.addInitScript({ content: STEALTH_INIT });
  await context.addCookies(cookies);

  const page = await context.newPage();

  // ===== history-sync action =====
  // Walks Amazon's order-history across multiple year filters and emits JSONL
  // (one order per line) on stdout. Two-pass strategy:
  //   1. Modern URL /your-orders/orders?timeFilter=year-YYYY (year-aware,
  //      JS-rendered — wait for skeletons to clear before parsing).
  //   2. Legacy URL /gp/legacy/order-history (fallback for the current window;
  //      static HTML, but ignores the orderFilter param so it can only return
  //      the default view).
  // Years to walk are passed via --years (comma-separated, e.g. "2026,2025,2024").
  // Default: current year + 2 prior years.
  if (action === "history-sync") {
    const yearsArgIdx = rest.indexOf("--years");
    const yearsArg = yearsArgIdx >= 0 ? rest[yearsArgIdx + 1] : "";
    const now = new Date();
    const defaultYears = [now.getFullYear(), now.getFullYear() - 1, now.getFullYear() - 2];
    const years = yearsArg
      ? yearsArg.split(",").map((s) => parseInt(s.trim(), 10)).filter((n) => n > 2000 && n < 2100)
      : defaultYears;
    process.stderr.write(`history-sync walking years: ${years.join(", ")}\n`);

    const parseOrdersFromPage = async (sourceTag) => {
      // Returns array of orders parsed from current page. Handles both modern
      // and legacy DOM shapes.
      return await page.evaluate((src) => {
        function text(el) { return (el && (el.innerText || el.textContent) || "").replace(/\s+/g, " ").trim(); }
        function findHeaderValue(card, captionRegex) {
          const lis = card.querySelectorAll("li.order-header__header-list-item, [class*='order-header']");
          for (const li of lis) {
            const cap = li.querySelector(".a-color-secondary.a-text-caps") || li.querySelector(".a-row.a-size-mini");
            if (cap && captionRegex.test(text(cap))) {
              const rows = li.querySelectorAll(".a-row");
              for (const row of rows) {
                const t = text(row);
                if (t && !captionRegex.test(t) && !row.querySelector(".a-color-secondary.a-text-caps")) {
                  return t;
                }
              }
            }
          }
          // Modern shape: look for label/value spans.
          const labels = card.querySelectorAll(".a-size-mini.a-color-secondary, [class*='date-label']");
          for (const lbl of labels) {
            if (captionRegex.test(text(lbl))) {
              const sibling = lbl.parentElement?.querySelector(".a-size-base, [class*='value']");
              if (sibling && sibling !== lbl) {
                const t = text(sibling);
                if (t) return t;
              }
            }
          }
          return "";
        }
        const out = [];
        // Match BOTH legacy (.order-card) AND modern card shells (after hydration).
        const cards = document.querySelectorAll(".order-card, [data-component='order-card'], [data-yo-orders-order-id]");
        cards.forEach((card) => {
          // Skip skeleton placeholders.
          if (card.querySelector("[class*='Skeleton']") && !card.querySelector("a[href*='/dp/'], a[href*='/gp/product/']")) {
            return;
          }
          const idText = text(card.querySelector(".yohtmlc-order-id, [class*='order-id'], bdi"));
          const dataOrderId = card.getAttribute && card.getAttribute("data-yo-orders-order-id");
          const idMatch = (dataOrderId || idText).match(/(\d{3}-\d{7}-\d{7})/);
          const orderId = idMatch ? idMatch[1] : "";
          const placedAt = findHeaderValue(card, /Order\s*placed|placed/i);
          const total = findHeaderValue(card, /^Total$/i);
          const items = [];
          const seenAsins = new Set();
          const itemContainers = card.querySelectorAll(".a-fixed-left-grid, .item-box, .yohtmlc-item, [class*='product-image-container']");
          itemContainers.forEach((row) => {
            const link = row.querySelector("a[href*='/dp/'], a[href*='/gp/product/']");
            if (!link) return;
            const href = link.getAttribute("href") || link.href || "";
            const m = href.match(/\/(?:gp\/product|dp)\/([A-Z0-9]{10})/);
            const asin = m ? m[1] : "";
            if (!asin || seenAsins.has(asin)) return;
            seenAsins.add(asin);
            let title = "";
            row.querySelectorAll("a").forEach((a) => {
              if (!title) {
                const t = text(a);
                if (t && t.length > 3) title = t;
              }
            });
            items.push({ asin, title, quantity: 1 });
          });
          if (orderId) out.push({ order_id: orderId, placed_at: placedAt, total, items, _source: src });
        });
        return out;
      }, sourceTag);
    };

    const waitForOrdersToHydrate = async () => {
      // The modern /your-orders page renders ~1100 skeleton placeholders, then
      // JS replaces them with real cards. Wait for either real cards to appear
      // or the skeleton count to drop to ~zero.
      try {
        await page.waitForFunction(() => {
          const hasRealCard = !!document.querySelector(".order-card a[href*='/dp/'], [data-component='order-card'] a[href*='/dp/'], [data-yo-orders-order-id]");
          const skeletons = document.querySelectorAll("[class*='Skeleton']").length;
          return hasRealCard || skeletons < 10;
        }, { timeout: 30000 });
      } catch (e) { /* fall through with whatever we have */ }
      await page.waitForLoadState("networkidle", { timeout: 15000 }).catch(() => {});
      await page.waitForTimeout(1500);
    };

    const allOrders = [];

    for (const year of years) {
      const url = `https://www.amazon.com/your-orders/orders?timeFilter=year-${year}`;
      process.stderr.write(`fetching ${url}\n`);
      try {
        await page.goto(url, { waitUntil: "domcontentloaded", timeout: 60000 });
      } catch (e) {
        process.stderr.write(`navigation to year-${year} failed: ${e.message}\n`);
        continue;
      }
      const gate = await detectManualGate(page);
      if (gate) {
        await browser.close();
        manualExit(gate.kind, gate.deeplink, { stage: `history-sync-year-${year}` });
      }
      await waitForOrdersToHydrate();
      let pageNum = 1;
      while (pageNum <= 30) {
        const got = await parseOrdersFromPage(`modern-year-${year}-p${pageNum}`);
        process.stderr.write(`  page ${pageNum}: ${got.length} orders\n`);
        allOrders.push(...got);
        // Find next page link (modern pagination)
        const nextLink = await page.$(
          "ul.a-pagination li.a-last:not(.a-disabled) a, " +
          "[aria-label='Next page'], " +
          "a[class*='pagination-next']:not([class*='disabled'])"
        );
        if (!nextLink) break;
        try {
          await Promise.all([
            page.waitForLoadState("domcontentloaded", { timeout: 30000 }).catch(() => {}),
            nextLink.click(),
          ]);
        } catch (e) { break; }
        pageNum += 1;
        await waitForOrdersToHydrate();
      }
    }

    // Also do one legacy-page pass as a safety net for the very recent window
    // — Amazon sometimes shows orders on legacy that aren't yet on modern.
    try {
      await page.goto("https://www.amazon.com/gp/legacy/order-history",
        { waitUntil: "domcontentloaded", timeout: 30000 });
      await page.waitForLoadState("networkidle", { timeout: 15000 }).catch(() => {});
      await page.waitForTimeout(1500);
      const legacyOrders = await parseOrdersFromPage("legacy-default");
      process.stderr.write(`legacy fallback: ${legacyOrders.length} orders\n`);
      allOrders.push(...legacyOrders);
    } catch (e) {
      process.stderr.write(`legacy fallback failed (non-fatal): ${e.message}\n`);
    }

    // Dedupe by order_id (legacy + multiple year filters may overlap)
    const seen = new Set();
    const uniq = [];
    for (const o of allOrders) {
      if (!o.order_id || seen.has(o.order_id)) continue;
      seen.add(o.order_id);
      // Strip the _source debug tag before emitting
      const { _source, ...clean } = o;
      uniq.push(clean);
    }
    process.stdout.write(JSON.stringify({
      status: "ok",
      orders_count: uniq.length,
      years_walked: years,
      jsonl: uniq.map((o) => JSON.stringify(o)).join("\n"),
    }) + "\n");
    await browser.close();
    process.exit(0);
  }

  // Pre-warm: visit home for a brief touchpoint before /gp/cart.
  try {
    await page.goto("https://www.amazon.com/", { waitUntil: "domcontentloaded", timeout: 30000 });
    await page.waitForTimeout(1200);
  } catch (e) {
    // Non-fatal; continue.
  }

  // Navigate to cart.
  try {
    await page.goto("https://www.amazon.com/gp/cart/view.html", { waitUntil: "domcontentloaded", timeout: 45000 });
    await page.waitForLoadState("networkidle", { timeout: 20000 }).catch(() => {});
    await page.waitForTimeout(1500);
  } catch (e) {
    await browser.close();
    transientExit(`cart navigation failed: ${e.message}`);
  }

  let gate = await detectManualGate(page);
  if (gate) {
    await browser.close();
    manualExit(gate.kind, gate.deeplink);
  }

  const cart = await readCart(page);
  const defaults = await readDefaults(page);

  if (action === "cart-show") {
    process.stdout.write(JSON.stringify({
      status: "ok",
      items: cart.items,
      subtotal: cart.subtotal,
      default_address: defaults.address,
      default_card_last4: defaults.card_last4,
    }) + "\n");
    await browser.close();
    process.exit(0);
  }

  if (action === "add-to-cart") {
    // Already on /gp/cart from the navigation above; navigate to the product
    // page now and drive the real Add-to-Cart button.
    const beforeAsins = new Set(cart.items.map((it) => it.asin).filter(Boolean));

    try {
      await page.goto(`https://www.amazon.com/dp/${addAsin}?th=1&psc=1`,
        { waitUntil: "domcontentloaded", timeout: 45000 });
      await page.waitForLoadState("networkidle", { timeout: 20000 }).catch(() => {});
      await page.waitForTimeout(1500);
    } catch (e) {
      await browser.close();
      transientExit(`product page navigation failed: ${e.message}`);
    }

    gate = await detectManualGate(page);
    if (gate) {
      await browser.close();
      manualExit(gate.kind, gate.deeplink, { stage: "product-page" });
    }

    // Set quantity if not 1.
    if (qtyArg > 1) {
      try {
        const qtySel = await page.$('#quantity, select[name="quantity"]');
        if (qtySel) {
          await qtySel.selectOption(String(qtyArg)).catch(() => {});
        }
      } catch (e) { /* fall through; default qty 1 */ }
    }

    // Find Add-to-Cart button. Try multiple selectors + role-based fallback.
    let clicked = false;
    const candidates = [
      'input#add-to-cart-button',
      '#add-to-cart-button',
      'input[name="submit.add-to-cart"]',
      '[name="submit.add-to-cart"]',
    ];
    for (const sel of candidates) {
      const el = await page.$(sel);
      if (!el) continue;
      try {
        await Promise.all([
          page.waitForLoadState("domcontentloaded", { timeout: 45000 }),
          el.click(),
        ]);
        clicked = true;
        break;
      } catch (e) {
        process.stderr.write(`add click via ${sel} failed: ${e.message}\n`);
      }
    }
    if (!clicked) {
      try {
        const btn = page.getByRole("button", { name: /add to cart/i });
        if (await btn.count() > 0) {
          await Promise.all([
            page.waitForLoadState("domcontentloaded", { timeout: 45000 }),
            btn.first().click(),
          ]);
          clicked = true;
        }
      } catch (e) {
        process.stderr.write(`role-based add failed: ${e.message}\n`);
      }
    }
    if (!clicked) {
      await browser.close();
      transientExit(`could not find Add-to-Cart button for ${addAsin}`);
    }

    await page.waitForLoadState("networkidle", { timeout: 20000 }).catch(() => {});
    await page.waitForTimeout(2000);

    gate = await detectManualGate(page);
    if (gate) {
      await browser.close();
      manualExit(gate.kind, gate.deeplink, { stage: "post-add" });
    }

    // Verify by re-reading the cart and confirming the ASIN is now an ACTIVE item.
    try {
      await page.goto("https://www.amazon.com/gp/cart/view.html",
        { waitUntil: "domcontentloaded", timeout: 45000 });
      await page.waitForLoadState("networkidle", { timeout: 15000 }).catch(() => {});
      await page.waitForTimeout(1500);
    } catch (e) {
      await browser.close();
      transientExit(`post-add cart verify navigation failed: ${e.message}`);
    }
    const afterCart = await readCart(page);
    const beforeCount = cart.items.length;
    const afterCount = afterCart.items.length;

    // Per-ASIN quantity delta is the only honest verification. If the ASIN
    // was already in the cart, "presence of row" tells you nothing about
    // whether the click did anything — Amazon's silent-routing bug is
    // structurally indistinguishable from a no-op otherwise.
    const qtyByAsin = (rows) => {
      const m = new Map();
      for (const it of rows) {
        if (!it.asin) continue;
        m.set(it.asin, (m.get(it.asin) || 0) + (it.quantity || 1));
      }
      return m;
    };
    const beforeQty = qtyByAsin(cart.items).get(addAsin) || 0;
    const afterQty = qtyByAsin(afterCart.items).get(addAsin) || 0;
    const wasAlreadyThere = beforeAsins.has(addAsin);
    const expected = qtyArg;

    // Primary signal: per-ASIN qty went up by the requested amount.
    let confirmed = afterQty - beforeQty >= expected;
    let landed = afterCart.items.find((it) => it.asin === addAsin) || null;

    // Fallback: ASIN extraction failed on the freshly-added row. Only trust
    // this if (a) the item was NOT already in cart and (b) cart row count went
    // up by exactly one. Two conditions together prevent the silent-success
    // bug from masquerading as a fallback hit.
    if (!confirmed && !wasAlreadyThere && afterCount === beforeCount + 1) {
      const known = new Set(cart.items.map((it) => it.asin).filter(Boolean));
      const newRow = afterCart.items.find((it) => !it.asin || !known.has(it.asin));
      if (newRow) {
        confirmed = true;
        landed = newRow;
      }
    }

    if (!confirmed) {
      process.stdout.write(JSON.stringify({
        status: "add_failed",
        asin: addAsin,
        reason: wasAlreadyThere
          ? `ASIN was already in cart at qty ${beforeQty}; after click qty is ${afterQty} (Amazon silently dropped the add — likely items-of-interest routing)`
          : "click reported success but cart did not change (Amazon likely routed to items-of-interest)",
        cart_items: afterCount,
        cart_items_before: beforeCount,
        qty_before: beforeQty,
        qty_after: afterQty,
      }) + "\n");
      await browser.close();
      process.exit(7);
    }

    process.stdout.write(JSON.stringify({
      status: "added",
      asin: addAsin,
      title: landed ? landed.title : "",
      quantity: landed ? landed.quantity : qtyArg,
      cart_items: afterCount,
      cart_items_before: beforeCount,
      qty_before: beforeQty,
      qty_after: afterQty,
      was_already_in_cart: wasAlreadyThere,
    }) + "\n");
    await browser.close();
    process.exit(0);
  }

  // action === "checkout"
  // Click "Proceed to checkout" to reach order review. Amazon's cart DOM
  // changes frequently; try selector- and role-based locators in order, and
  // fall back to navigating directly to the SPC URL (works when the button is
  // hidden behind a JS handler we can't reach reliably).
  let proceeded = false;
  const proceedCandidates = [
    'input[name="proceedToRetailCheckout"]',
    '[data-feature-id="proceed-to-checkout-action"] input',
    '[data-feature-id="proceed-to-checkout-action"] button',
    'span#sc-buy-box-ptc-button input',
    'a[href*="/gp/buy/spc/handlers/display.html"]',
    'a[href*="/gp/buy/spc/"]',
  ];
  for (const sel of proceedCandidates) {
    const el = await page.$(sel);
    if (!el) continue;
    try {
      await Promise.all([
        page.waitForLoadState("domcontentloaded", { timeout: 45000 }),
        el.click(),
      ]);
      proceeded = true;
      break;
    } catch (e) {
      process.stderr.write(`proceed click failed via ${sel}: ${e.message}\n`);
    }
  }
  if (!proceeded) {
    // Try the role-based locator (Playwright walks accessibility tree).
    try {
      const btn = page.getByRole("button", { name: /proceed.*checkout/i });
      if (await btn.count() > 0) {
        await Promise.all([
          page.waitForLoadState("domcontentloaded", { timeout: 45000 }),
          btn.first().click(),
        ]);
        proceeded = true;
      }
    } catch (e) {
      process.stderr.write(`role-based proceed failed: ${e.message}\n`);
    }
  }
  if (!proceeded) {
    // Last resort: navigate directly to the SPC URL.
    try {
      await page.goto("https://www.amazon.com/gp/buy/spc/handlers/display.html?hasWorkingJavascript=1",
        { waitUntil: "domcontentloaded", timeout: 45000 });
      proceeded = true;
    } catch (e) {
      await browser.close();
      transientExit(`proceed-to-checkout failed via all paths: ${e.message}`);
    }
  }

  try {
    await page.waitForLoadState("networkidle", { timeout: 20000 }).catch(() => {});
    await page.waitForTimeout(2000);
  } catch (e) {
    // Non-fatal
  }

  // BYG / SSD ("Save a trip" grocery upsell) interstitial. When the user's cart
  // has an SSD-eligible item like a grocery good, Amazon routes the
  // proceed-to-checkout click through /checkout/byg/ before the real review
  // page. Detect and click through; gating happens again post-click.
  if (/\/checkout\/byg\//.test(page.url())) {
    process.stderr.write(`BYG interstitial detected at ${page.url()}; clicking through\n`);
    const bygBtn =
      (await page.$('#checkout-byg-ptc-button')) ||
      (await page.$('a[id*="byg-ptc"], a[name*="byg-ptc"]'));
    if (bygBtn) {
      try {
        await Promise.all([
          page.waitForLoadState("domcontentloaded", { timeout: 45000 }),
          bygBtn.click(),
        ]);
        await page.waitForLoadState("networkidle", { timeout: 20000 }).catch(() => {});
        await page.waitForTimeout(2000);
      } catch (e) {
        process.stderr.write(`BYG click-through failed: ${e.message}\n`);
      }
    } else {
      process.stderr.write("no #checkout-byg-ptc-button found on BYG interstitial\n");
    }
    // Re-check gate after BYG click — Amazon typically routes to /ap/signin
    // with a max_auth_age=900 challenge, which detectManualGate already
    // classifies as kind="sign-in" → exit 9 with deeplink.
    gate = await detectManualGate(page);
    if (gate) {
      await browser.close();
      manualExit(gate.kind, gate.deeplink, { stage: "post-byg" });
    }
  }

  gate = await detectManualGate(page);
  if (gate) {
    await browser.close();
    manualExit(gate.kind, gate.deeplink, { stage: "post-proceed" });
  }

  // Re-read defaults from the more-authoritative order-review page.
  const reviewDefaults = await readDefaults(page);
  const previewCart = await readCart(page).catch(() => ({ items: [], subtotal: "" }));

  if (!wantPlace) {
    process.stdout.write(JSON.stringify({
      status: "review_ready",
      items: previewCart.items.length ? previewCart.items : cart.items,
      subtotal: previewCart.subtotal || cart.subtotal,
      default_address: reviewDefaults.address || defaults.address,
      default_card_last4: reviewDefaults.card_last4 || defaults.card_last4,
      review_url: page.url(),
    }) + "\n");
    await browser.close();
    process.exit(0);
  }

  // Place the order.
  const placeBtn =
    (await page.$('input[name="placeYourOrder1"]')) ||
    (await page.$('#placeYourOrder input')) ||
    (await page.$('input[aria-labelledby*="placeYourOrder"]')) ||
    (await page.$('.place-order-button input'));

  if (!placeBtn) {
    await browser.close();
    transientExit("no place-order button found on review page");
  }

  try {
    await Promise.all([
      page.waitForLoadState("domcontentloaded", { timeout: 60000 }),
      placeBtn.click(),
    ]);
    await page.waitForLoadState("networkidle", { timeout: 30000 }).catch(() => {});
    await page.waitForTimeout(2500);
  } catch (e) {
    await browser.close();
    transientExit(`place-order click failed: ${e.message}`);
  }

  // Check for post-place gate.
  gate = await detectManualGate(page);
  if (gate) {
    await browser.close();
    manualExit(gate.kind, gate.deeplink, { stage: "post-place" });
  }

  // Extract the order ID from the confirmation page.
  const confirmation = await page.evaluate(() => {
    const bodyText = document.body.innerText || "";
    const m = bodyText.match(/(\d{3}-\d{7}-\d{7})/);
    return { order_id: m ? m[1] : "", url: window.location.href };
  });

  if (!confirmation.order_id) {
    // We placed the order but didn't see a confirmation marker — return the URL
    // so the agent can show the user the page.
    process.stdout.write(JSON.stringify({
      status: "placed_unconfirmed",
      confirmation_url: confirmation.url,
    }) + "\n");
    await browser.close();
    process.exit(0);
  }

  process.stdout.write(JSON.stringify({
    status: "placed",
    order_id: confirmation.order_id,
    confirmation_url: confirmation.url,
    default_address: reviewDefaults.address || defaults.address,
    default_card_last4: reviewDefaults.card_last4 || defaults.card_last4,
  }) + "\n");
  await browser.close();
  process.exit(0);
}

main().catch((e) => {
  process.stderr.write(`unhandled: ${e.stack || e.message}\n`);
  process.exit(7);
});
