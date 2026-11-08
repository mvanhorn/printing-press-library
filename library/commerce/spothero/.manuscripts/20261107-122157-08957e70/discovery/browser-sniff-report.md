# SpotHero browser sniff report

Backend: chrome-MCP in the owner's logged-in profile, read only, own tab, plus the public JS bundles for the consumer-web and consumer-checkout zones. Names and shapes only: no values were kept. No request was replayed that changes state.

Observed authenticated GETs on spothero.com/api/v1: reservations, reservations/{id}, reservations/{id}/refund-eligibility, users/me, users/{id}/vehicles, users/{id}/credit-cards, drive-up/users/me, airports, cities, states, facilities/{id}/operator-info, oip/item-add-ons/available.
Observed on api.spothero.com/v2: search/transient, search/transient/{id}, search/extension_rates/{id}.
Read from bundles, not executed: POST checkout/, POST reservations/{id}/refund/, POST hero/rebook/.
Cookie contract: AUTH_TOKEN alone authenticates. See the research brief.
