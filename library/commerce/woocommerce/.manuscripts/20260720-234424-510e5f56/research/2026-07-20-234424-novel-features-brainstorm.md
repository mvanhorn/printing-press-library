# WooCommerce CLI — Novel Features Brainstorm (audit trail)

Subagent run: 3 passes (customer model → 16 candidates → adversarial cut). 8 survivors, 8 kills.
Survivors and kills are reproduced in the absorb manifest; the customer model below is the
grounding that produced them and is preserved here.

## Customer model

### Marisol — solo owner-operator, ~400-SKU apparel store
**Today:** Lives in `wp-admin` with two tabs pinned all day — Orders filtered to Processing, and
Products sorted by stock. A third tab holds WooCommerce → Reports, which she opens and closes
because it only gives gross sales and a top-sellers list. To find out why Tuesday was down she
eyeballs the order list and guesses. To decide what to reorder she opens each product page and
compares the stock number against her memory of how fast it moves. She cannot answer: how many
days of stock SKU X has left, which products get returned most, what a repeat customer is worth,
or how much of last week's revenue drop came from fewer orders vs. smaller baskets.

**Weekly ritual:** Every morning, triage the overnight order queue — clear Processing, chase
On-hold, look at Failed and decide whether it's a card problem or a gateway problem. Every Monday,
a restock pass. Once a week, a revenue gut-check against the prior week.

**Frustration:** The restock pass. The number she needs — units sold per day over 30 days, per
variation, divided into current stock — does not exist anywhere in WooCommerce. She reconstructs
it by hand from an order export in a spreadsheet, so she does it badly and late, and stocks out
on her best sellers.

### Tomás — ops lead at an agency running 12 client WooCommerce stores
**Today:** Twelve sets of credentials, twelve `wp-admin` tabs, twelve slightly different plugin
stacks. He keeps a Postman collection and re-runs the same four requests per store, swapping the
base URL each time. When a client says "orders stopped coming in," he has no historical baseline —
the API only tells him what is true right now. When a client asks "is our catalog ready for the
campaign," he clicks through product pages looking for missing images. He cannot answer: which of
my 12 stores has a degrading payment gateway, or which store's catalog regressed since last month.

**Weekly ritual:** A Monday sweep across all 12 stores — did anything break, is anything stuck, is
any catalog obviously broken before a client notices. Then a client-facing weekly note per store.

**Frustration:** Everything is per-store and stateless. Twelve manual passes, no memory between
them, and every "is this worse than last week" question is unanswerable because nothing was
recorded last week.

### Priya — category merchandiser doing competitive catalog research
**Today:** She has no credentials to any competitor store and never will. So she does it by hand:
a browser tab per competitor collection page, a screenshot folder, and a spreadsheet where she
retypes prices once a week. She knows three competitors run WooCommerce because `/wp-json/`
responds, but her tooling stops there — every WooCommerce client assumes she owns the store. She
cannot answer: when did they drop that price, how long do their sales run, what did they add or
delist this month, what does their price distribution look like across a category.

**Weekly ritual:** Monday price sweep across competitor catalogs, retyped into a sheet; a monthly
"what's new / what's gone" pass before merchandising decisions.

**Frustration:** She is a human diffing tool. The retyping is the job, and because it is manual she
samples ~40 products out of thousands and misses every sale that starts and ends between Mondays.

## Candidate pool (16 generated, pre-cut)

C1 orders triage · C2 stock velocity · C3 revenue explain · C4 refund-rate · C5 customers ltv ·
C6 catalog audit · C7 catalog watch · C8 catalog diff · C9 coupon roi · C10 store fingerprint ·
C11 catalog compare · C12 catalog sale-cadence · C13 price integrity · C14 orders sla ·
C15 customers at-risk · C16 catalog attributes coverage

Survivors: C1–C8. Kills: C9–C16 (reasons recorded in the absorb manifest's killed-candidates table).

Notable kill: **C14 `orders sla`** — the REST API exposes only `date_created`, `date_modified`, and
`date_completed`. There is no status-transition history, so "time entered processing" cannot be
derived without inventing a timestamp the API never provided.
