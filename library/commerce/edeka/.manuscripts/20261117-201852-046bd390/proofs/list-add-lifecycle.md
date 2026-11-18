# list add lifecycle proof (2026-11-17)

Sandbox: the test customer's own basket at one EDEKA market (doerflinger-ulm), session imported from the operator's browser; no order placed, no payment. The basket held one test item (1x Weide Milch) before and after.

Live dogfood ran without --allow-destructive on purpose (no checkout path may run destructively in this run), so the `list add` happy path is recorded here from a real run with the release binary:

```text
$ edeka-pp-cli basket show --json --select count,total
exit 0
{"count":1,"total":"1,59 €"}
$ edeka-pp-cli basket add milch --qty 1 --json
exit 0
{"query":"milch","product":{"id":"5915407005MXST","title":"Weide Milch","price_cents":159,"price":"1,59 €","on_offer":false,"weight_article":false,"in_basket":1},"quantity":1,"new_quantity":2,"added":true,"basket_total":"3,18 €","basket_pieces":2}
$ edeka-pp-cli list add butter 2x bananen --json --select matched,not_matched,results.query,results.product.title,results.new_quantity,results.added
exit 0
{"matched":2,"not_matched":0,"results":[{"added":true,"new_quantity":1,"product":{"title":"MinusL Butter 125 g"},"query":"butter"},{"added":true,"new_quantity":2,"product":{"title":"GUT&GÜNSTIG Bananen, WWF"},"query":"bananen"}]}
$ edeka-pp-cli basket show --json --select count,total,items.title,items.quantity
exit 0
{"count":5,"items":[{"quantity":2,"title":"Weide Milch"},{"quantity":1,"title":"MinusL Butter 125 g"},{"quantity":2,"title":"GUT&GÜNSTIG Bananen, WWF"}],"total":"8,05 €"}
$ edeka-pp-cli basket remove 658664001MXST --json --select action,basket_total
exit 0
{ "action": "remove", "basket_total": "5,76 €" } 
$ edeka-pp-cli basket remove 81384660MXKG --json --select action,basket_total
exit 0
{ "action": "remove", "basket_total": "3,18 €" } 
$ edeka-pp-cli basket set 5915407005MXST 1 --json --select quantity,basket_total
exit 0
{ "basket_total": "1,59 €", "quantity": 1 } 
$ edeka-pp-cli basket show --json --select count,total
exit 0
{ "count": 1, "total": "1,59 €" } 
```

Cleanup: the added lines were removed with `basket remove <article-id>` (exit 0 each) and Weide Milch was set back to 1 with `basket set 5915407005MXST 1` (exit 0); the final `basket show` reports 1 piece, 1,59 EUR, as before the run.
