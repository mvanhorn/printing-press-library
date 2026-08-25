# Browser-backed iFood transport

The Browser-backed workflow splits responsibility deliberately:

- `@Browser` owns the signed-in iFood page, cookies, browser fingerprint, CAPTCHA
  boundary, visible searches, and cart UI.
- `ifood-pp-cli browser ...` owns requirements, normalized observations,
  coverage validation, market selection, totals, and the confirmation boundary.

The CLI does not connect to the Browser plugin itself. An AI agent composes the
two tools. This keeps browser credentials out of files, shell history, process
arguments, logs, and model-visible output.

## 1. Produce the workflow

```bash
ifood-pp-cli --json browser plan \
  --markets 3 \
  --min-rating 4.5
```

Repeat `--item TERM[:QTY]` to override the six default items. The output is a
stable JSON plan with `transport: "browser"`, the requirements, read/write stage
labels, CAPTCHA behavior, unsupported actions, and the exact confirmation
boundary.

## 2. Record visible observations

Run `ifood-pp-cli --json browser schema` for the current schema and a redacted
example. Write one JSON object with:

- `schema_version: 1`
- every requested term and quantity under `requested_items`
- at least three market objects under `markets`
- visible market ID when available, name, rating, and optional delivery fee
- one product observation per requested term with visible name, price,
  availability, and optional product ID

Use each requested term verbatim as `items[].term`; matching is deterministic,
not fuzzy. The file must not contain cookies, authorization headers, addresses,
payment information, or unrelated page content. Unknown JSON fields are rejected.

## 3. Validate the quote

```bash
ifood-pp-cli --json browser validate-quote --input ./ifood-quote.json
```

A complete market must:

- have a non-empty visible name;
- have a rating between the configured minimum and 5;
- contain exactly one valid observation for every requested term;
- expose an addable, named product with a positive finite price.

The quote is complete only when the configured number of complete markets is
available. Complete markets are sorted by items plus known delivery fee. When a
delivery fee was not visible, the result labels the selection basis as an item
subtotal with unknown delivery fee.

## 4. Build the cart plan

```bash
ifood-pp-cli --json browser cart-plan --input ./ifood-quote.json
```

Use `--merchant <id>` to pin a validated merchant; otherwise the validator's
lowest complete estimate is selected. The command emits the exact product names,
quantities, observed prices, expected totals, and verification checklist. It
never performs a remote write.

An agent may begin clicking add-to-cart controls only after the user explicitly
authorizes that exact plan. After every addition, verify the authoritative cart
line and quantity. Stop if the product is substituted, the price changes
materially, availability changes, the merchant changes, or a CAPTCHA appears.
Never proceed to checkout, payment, or order submission.
