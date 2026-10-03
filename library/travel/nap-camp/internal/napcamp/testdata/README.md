# Regression fixture provenance

The seven API fixtures are selected-field regression inputs derived from anonymous
Nap Camp public GET observations on 2026-10-02. They preserve the upstream nesting,
IDs, Japanese facility/pitch names, vehicle memo, capacity, area, power/pet fields,
calendar date/status/starting-price types, and catalog labels used by the tests.
Arrays used for list tests are reduced to a few entities. Addresses, telephone
numbers, images, unrelated metadata and marketing text are omitted. These are
partial test inputs, not a saved response archive or current planning evidence.

The source contracts are `/api/master`, `/api/locations`, `/api/search`,
`/api/campsite/11007`, `/api/campsite/11007/plans`,
`/api/campsite/11007/plans/20005062`, and its `/reservation?month=2026-10`
calendar at `https://www.nap-camp.com`. Native public category/list/pitch/calendar
workflows and observed first-party assets established the request shapes before
HTTP verification. No account, cookie, guest or booking data was collected.

`before.json` and `after.json` are tiny synthetic versioned observations for the
local comparison command. None of these fixtures establishes campervan clearance,
vacancy or a full dated price. Live publication acceptance is recorded separately.
