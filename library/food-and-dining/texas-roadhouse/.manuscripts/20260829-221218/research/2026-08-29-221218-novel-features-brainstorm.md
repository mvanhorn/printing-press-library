# Texas Roadhouse novel features

Sanitized sniffed-waitlist facts that a reprint must keep. No customer PII, tokens, cookies, or traffic dumps.

## Sniffed waitlist join

`texasroadhouse submit <extref>` posts to `/api/texasroadhouse/waitlist/{extref}/submit`.

- Path id is store **extref**, not internal store id.
- `EmailAddress` is required.
- `PartySize` max is 6.
- `WaitMinutes` is the quote `MinQuote` for that party size.
- Live join requires `--yes`. `--dry-run` prints the body and does not POST.

## Numeric waitlist cancel

`texasroadhouse cancel` posts to `/api/texasroadhouse/waitlist/cancel`.

- Body `waitlistRequestId` is a JSON number (string 400s).
- `siteId` is the store extref.
- Query `clientid=texasroadhouse`.
- Live cancel requires `--yes`.

## Sniffed waitlist check-in (HERE)

`texasroadhouse checkin <extref>` posts to `/api/texasroadhouse/waitlist/{extref}/checkin`.

- Marks the party HERE at the host stand.
- Live check-in requires `--yes`.

## Quote MinQuote lookup

`texasroadhouse get-quote <extref>` reads per-party-size MinQuote/MaxQuote so submit can send WaitMinutes.

## Store extref lookup

`stores` lists nearby restaurants. Waitlist commands use `extref`, never the internal store id.
