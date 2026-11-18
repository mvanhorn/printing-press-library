# auth renew - lifecycle proof

`auth renew` only does real work when the REWE shop session cookie (`rstp`) has
expired while the single-sign-on cookies (`KEYCLOAK_IDENTITY`, `KEYCLOAK_SESSION`)
are still valid. A fresh live dogfood sandbox starts with a valid shop session, so
the renewal path cannot be exercised there without first destroying the session.

Lifecycle run on 2026-11-17 against www.rewe.de with a real customer account:

1. Imported cookies were reduced to the account.rewe.de single-sign-on cookies
   plus Cloudflare cookies (no `rstp`, `mtc`, `cso_jsid`).
2. `GET /api/marketselection/configuration` answered `isLoggedIn: false`.
3. The renewal sequence used by `auth renew` (`GET /mydata/login` -> Keycloak
   "continue with this account" form -> `POST login=` -> callback) ran over plain
   HTTP without a password.
4. `GET /api/marketselection/configuration` then answered `isLoggedIn: true` and
   `GET /shop/api/timeslots/pickup/checkout` answered 200.
5. The CLI's `Ensure` path (used by every account command and by `auth renew`)
   was exercised the same way in unit tests (`TestEnsureRenewsLoggedOutSession`)
   and live: `auth renew --json` returned `renewed: true, logged_in: true`.

A missing or expired single-sign-on cookie makes `auth renew` exit 4
("re-login needed"), covered by `TestShopErrors`.
