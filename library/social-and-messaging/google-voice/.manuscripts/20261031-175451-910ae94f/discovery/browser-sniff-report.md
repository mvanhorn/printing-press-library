# Google Voice read discovery

## User Goal Flow

Goal: read personal Google Voice conversations and their message history for a local searchable archive.
Verified account and conversation-list requests from a fresh signed-in tab, then replayed them outside the browser.
Fetched thread detail directly using a returned thread ID and the community source's verified request structure.
All three planned read operations succeeded.
No mutation controls were exercised.

## Pages & Interactions

The isolated Chrome profile redirected Voice to Google's product page and offered sign-in.
The existing personal Dia profile opened `https://voice.google.com/u/0/messages` successfully in a fresh tab.
Opened DevTools Network, temporarily enabled authenticated HAR export, reloaded the fresh tab, and saved the capture in the private session directory.
Restored the export preference and closed the fresh capture tab.
Existing user tabs were preserved.

## Browser-Sniff Configuration

Backend: native DevTools HAR export through the authorized computer-use interface.
Chrome CLI was available but its isolated profile was signed out; browser-use and agent-browser were absent.
Direct replay requests were paced at one second between calls.
No proxy-envelope pattern was observed.
The RPC payload format is protobuf JSON arrays (`application/json+protobuf`).

## Endpoints Discovered

| Method | Path | HTTP with auth | HTTP without auth | Content type | Evidence |
|---|---|---:|---:|---|---|
| POST | /voice/v1/voiceclient/account/get | 200 | 401 | application/json+protobuf | HAR and direct replay |
| POST | /voice/v1/voiceclient/api2thread/list | 200 | 401 | application/json+protobuf | HAR and direct replay |
| POST | /voice/v1/voiceclient/api2thread/get | 200 | Not repeated | application/json+protobuf | Direct replay using HAR-derived identity and source schema |

The page also fired other initialization reads, including cold sync and threading information.
Those are discovery evidence, not automatically approved product scope.

## Traffic Analysis

The analyzer emitted three endpoints but could not infer reliable named fields from positional protobuf JSON arrays.
It also treated the web client's query `key` as personal API-key auth.
Both are rough-scaffold limitations requiring repair before generation.
Real authentication is Google cookies plus a time-derived `SAPISIDHASH` header and an account selector.
Captured private values are not present in this report or the redacted analyzer input.
The live direct replay resolves runtime to standard HTTP with an explicit positional JSON codec and auth adapter.

## Coverage Analysis

Verified account access, text-thread listing, and 100-message thread detail with a continuation cursor.
Pagination across all history, archived/group coverage, media metadata, account isolation, expired-session handling, and repeated-sync deduplication still require CLI verification.
No claim of complete account history is made.

## Response Samples

Research capture samples preserve positional shape while replacing all string values and large numeric values with synthetic placeholders.
Raw private bodies, cookies, authorization values, and the original HAR remain only in the private session directory.
Thread list returned 20 rows.
Thread detail returned 100 messages and a nonempty older-history cursor.
Account response is one wrapped account array; no private account values are recorded here.

## Rate Limiting Events

No 429 was observed in the verified account/list/detail replay requests.
No rate-limit or completeness guarantee is implied.

## Authentication Context

Mike approved the capture and existing-account use in this chat.
Cookie-plus-signature replay succeeds outside the browser and fails without authentication for account/list.
The credential source is a private HAR; no resident browser transport is required.
Do not promise automatic Chrome cookie import based on this proof: the working profile was Dia and generic Chrome import has not been tested.
The CLI needs a private credential-import route and account verification.
