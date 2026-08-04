# nisifilters-pp-cli Build Log

Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

Built (hand-code, read-only over local SQLite mirror; 5 ported from francescogola-pp-cli, 1 new):
1. shop    — products joined to categories, priced, stock, sorted (ported)
2. filters — NEW: aggregates product attributes (sizes/types) + finds products by value
3. read    — agent-clean reader, HTML stripped (ported; portfolio type removed)
4. digest  — catalog + site summary, price range, top categories (ported)
5. since   — recently-changed posts/pages (ported; products excluded — Store API has no timestamps)
6. image   — featured-image/product-image resolver (ported; portfolio removed)

## Generator gap reused-fix
- syncResourcePath base_url override gap (from francescogola run) avoided up front:
  WooCommerce resources use absolute wc/store/v1 URLs in the spec, so sync hit them
  directly. All 10 resources synced clean (0 errors).

## Note
- product_attributes resource stores 0 rows: NiSi per-product attributes carry id:0,
  so ExtractResourceID skips them. The `filters` command reads embedded product
  attributes from the products table instead, so it is unaffected.

## Live verification (synced store: 461 items, 100 products)
- filters: 31 attributes discovered (Dimensione → 77mm×9, 67mm×8 …); value filter returns real 82mm products with prices — correct
- shop: priced + categorized, sorted by price — correct
- digest: 461 items, price range 0–11466.78 EUR, top categories — correct
- read: clean stripped Italian content (6750 chars) — correct
- image: post featured image + size variants; product inline images (5) — correct
- since 3650d: posts/pages by modified, scanned count surfaced — correct
- dry-run exit 0, help exit 0, missing-arg exit 2 — correct
