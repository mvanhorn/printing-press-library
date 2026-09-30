<!-- slop-gate: off -->
# Postmark API docs inventory (vs official Swagger 2.0 specs)

Fetched 2026-09-30 with `fetch-docs.sh` (raw HTML, no summarization). All 16 requested pages returned HTTP 200. Raw captures live in `$TMPDIR/printing-press-fetch-docs/postmarkapp.com-developer-api-<page>-<hash>.html`.

Coverage check (scripted diff of `METHOD path`, path params normalized, case-insensitive):
- Docs document **87** operations. Specs contain **66** (server 43 + account 23).
- **21** documented operations are in neither spec (Section 2).
- **0** spec operations are missing from the docs. Three spec operations are marked deprecated in the docs (Section 3).

Extra fetches:
- `https://postmarkapp.com/developer/api/smtp-tokens-api` returned **404**, and so did `/developer/api/smtp-tokens`. The overview error table has an "SMTP Tokens — new" group (codes 1450–1460), but no public reference page exists, so no SMTP-token endpoints are recorded.
- User-guide pages `sandbox-mode`, `send-email-with-api`, and `managing-your-account` returned 200. None of them gives a numeric rate limit.

---

## 1. Global contract

### Base URL and transport
- `https://api.postmarkapp.com` (HTTPS enforced: "We enforce TLS encryption by issuing requests via HTTPS").

### Auth
| Header | Token | Used by |
|---|---|---|
| `X-Postmark-Server-Token` | Server token ("API Tokens tab under your Postmark server") | email, bulk-email, bounce, templates (except push), server, message-streams, messages, stats, inbound-rules-triggers, webhooks, suppressions |
| `X-Postmark-Account-Token` | Account token ("API Tokens tab of your Postmark account"; Account Owner and Account Admins) | servers, domains, signatures (senders), `PUT /templates/push`, data-removals |

- "The header name and value are case insensitive."
- A wrong or missing header returns HTTP 401. ErrorCode `10` means "Request does not contain a valid Server or Account token, or the wrong token type was used for the endpoint."
- Both specs model auth as a required per-operation `in: header` parameter. Neither has `securityDefinitions`.

### Request headers
- `Accept: application/json` is marked required on every endpoint.
- `Content-Type: application/json` is marked required on endpoints that take a body. Documented exceptions and oddities:
  - `POST /message-streams/{stream_ID}/archive` and `/unarchive` require `Content-Length` "Set to 0" instead, with no Content-Type.
  - These list only `Accept`: `PUT /messages/inbound/{messageid}/bypass`, `PUT /messages/inbound/{messageid}/retry`, `POST /senders/{signatureid}/resend`, `POST /data-removals` (its curl still sends Content-Type), `PUT /domains/{domainid}/verifyDkim`, `PUT /domains/{domainid}/verifyReturnPath`, `POST /webhooks/{Id}/verify`.
  - `DELETE /webhooks/{Id}` lists `Content-Type` as required.
- HTTP 415 means "The API request is missing the expected request headers."

### Pagination model
- Offset style: `count` + `offset`, both required. Casing varies by endpoint: templates document `Count`/`Offset`; everything else uses lowercase.
- Max `count` = 500 where documented: bounces, messages outbound/inbound, opens, clicks, senders, domains. Servers cap at 500 per error 600 ("up to 500 servers per call"). Inbound rules cap at 500 per error 800 ("You may only request up to 500 triggers per call").
- Deep-paging cap: "Count + Offset cannot exceed 10,000" applies to `/bounces`, `/messages/outbound`, `/messages/inbound`, `/messages/outbound/opens`. Docs: "use parameters like `todate` and `fromdate` to filter the messages" to get past it. The clicks page does not state the 10,000 cap.
- Templates list: no max count documented. Page intro: "a Server may have up to 100 templates."
- Cursor style (bulk only): `GET /email/bulk?count=&paginationKey=`. The response has `PaginationKey`, "Absent once you reach the last page." The key is opaque base64 with padding removed. An altered key returns 422 ErrorCode `13` ("Invalid pagination key.").
- Unpaginated lists: `/message-streams`, `/webhooks`, `/message-streams/{stream_id}/suppressions/dump`.
- Date filters: messages and bounces take date/time values "using Eastern Time Zone" (e.g. `2021-01-01T12:00:00`). Stats take dates (`2014-01-01`) and "All stats use EST timezone."

### Error envelope
```json
{ "ErrorCode": 403, "Message": "Invalid request field(s): 'From'." }
```
- The code is echoed in the `X-PM-ApiErrorCode` response header.
- Bulk validation adds `Errors`, an object keyed by field name, each value an array of `{ErrorCode, Message}` (top-level ErrorCode `11`).
- `/email/batch` and `/email/batchWithTemplates` return HTTP 200 with a per-message `ErrorCode` array, in the same order as the request.

### HTTP status codes (overview, verbatim names)
| Code | Meaning |
|---|---|
| 200 | Success |
| 401 | Unauthorized: missing or incorrect API token in header |
| 404 | Entity doesn't exist |
| 413 | Payload Too Large: "size limit of 10 MB for our Email API and 50 MB total payload size for our Batch Email API" |
| 415 | Unsupported Media Type: missing expected request headers |
| 422 | Unprocessable Entity: body has `ErrorCode` + `Message` |
| 429 | Rate Limit Exceeded: "reduce the rate at which you query the API" |
| 500 | Internal Server Error |
| 503 | Service Unavailable (planned outages) |

### ErrorCode table (overview, complete)
| Group | Code | HTTP | Message |
|---|---|---|---|
| Authentication | 10 | 401 | Request does not contain a valid Server or Account token, or the wrong token type was used for the endpoint. |
| Global | 100 | 503 | The Postmark API is offline for maintenance. |
| Global | 101 | 500 | You encountered an error that shouldn't have occurred. The response includes an `Error ID` for support. |
| Sending (batch sends return per-message codes inside an HTTP 200 response) | 11 | 422 | Multiple errors occurred. Inspect the `Errors` property for more information. |
| Sending | 12 | 404 | Bulk send not found. |
| Sending | 13 | 422 | Invalid pagination key. |
| Sending | 14 | 422 | This endpoint requires approval to access. Contact support to use the Bulk API. |
| Sending | 300 | 422 | Send validation. Covers many messages — zero recipients, invalid address, missing `TextBody`/`HtmlBody`, and recipient, metadata, attachment, or header limits. |
| Sending | 402 | 422 | Invalid JSON. |
| Sending | 403 | 422 | Invalid request field(s). |
| Sending | 406 | 422 | Inactive recipient. |
| Sending | 410 | 422 | You may only send up to 500 messages in a single batched request. |
| Sending | 411 | 422 | Attachment file type not allowed. |
| Sending | 412 | 422 | While your account is pending approval, all recipient addresses must share the same domain as the From address. |
| Sending | 413 | 422 | This account is not approved to send email. |
| Sending | 422 | 422 | Invalid Server or Account. |
| Sending | 1235 | 422 | The stream provided does not exist on this server. |
| Sending | 1236 | 422 | Sending is not supported for this stream type. |
| Sending | 1480 | 422 | You are not authorized to send emails from your current IP address: 'IP Address'. |
| Templates | 601 | 422 | The source or destination server was not found (template push). |
| Templates | 1100 | 422 | Template list paging, or an invalid `TemplateType` or `editorType` query parameter. |
| Templates | 1101 | 422 | The request specifies neither `TemplateId` nor `TemplateAlias`, or the referenced template, alias, or layout was not found. |
| Templates | 1105 | 422 | A server's active-template limit would be exceeded by this request. |
| Templates | 1109 | 422 | No template data received. |
| Templates | 1120 | 422 | A required field is missing — `Name`, one of `TextBody`/`HtmlBody`, `Subject`, or `TemplateModel`. |
| Templates | 1121 | 422 | A field is too long — `Name`, `Alias`, `HtmlBody`, `TextBody`, `Subject`, or `TemplateModel`. |
| Templates | 1122 | 422 | Invalid `TemplateType`, alias empty/invalid/in-use, unparseable body, or a reserved top-level `TemplateModel` key. |
| Templates | 1123 | 422 | Template/send mutual-exclusion rules (layout vs. subject/body, templated vs. non-templated). |
| Templates | 1124 | 422 | No templates with aliases found to push, or the per-request push limit was exceeded. |
| Templates | 1125 | 422 | The template types don't match on the source and destination servers. |
| Templates | 1130 | 422 | The layout template cannot be deleted because dependent templates use it. |
| Templates | 1131 | 422 | Layout content-placeholder rules were not met. |
| Servers | 600 | 422 | Server-list paging — `offset`/`count` required or integer; up to 500 servers per call. |
| Servers | 602 | 422 | The specified inbound domain is already registered or in use on another server. |
| Servers | 603 | 422 | This server name already exists. |
| Servers | 604 | 422 | You do not have permission to delete servers using the API. |
| Servers | 605 | 422 | Unable to remove this server. Please contact support. |
| Servers | 606 | 422 | A supplied hook URL (Inbound, Bounce, Open, Delivery, or Click) is not valid. |
| Servers | 607 | 422 | Invalid server color. |
| Servers | 608 | 422 | Server name is invalid or missing, or an inbound domain containing postmarkapp.com was used. |
| Servers | 609 | 422 | No server data received. |
| Servers | 610 | 422 | We could not find an MX record pointing to the expected domain. |
| Servers | 611 | 422 | InboundSpamThreshold value is invalid. Use a number between 0 and 30. |
| Servers | 612 | 422 | The supplied `TrackLinks` option is not valid. |
| Servers | 613 | 422 | The supplied `DeliveryType` option is not valid. |
| Servers | 614 | 422 | Entitlement limit reached (inbound, stats, users, servers, streams, or domains). |
| Servers | 615 | 422 | Action is not supported. |
| Message activity, messages & bounces | 700 | 422 | Paging/parameter validation for messages, opens, clicks, and activity. |
| Message activity | 701 | 422 | This message was not found, or cannot be bypassed or retried. |
| Message activity | 702 | 422 | Could not bypass this blocked message. Please contact support. |
| Message activity | 703 | 422 | Could not retry this failed message. Please contact support. |
| Message activity | 1000 | 422 | Bounces query validation (non-negative, up to 500, count+offset, illegal bounce type). |
| Message activity | 1001 | 422 | The bounce was not found, or its dump is no longer available. |
| Message activity | 1002 | 400 | A `bounceID` parameter is required. |
| Message activity | 1003 | 422 | Due to the type of bounce, this address cannot be reactivated. |
| Inbound rules / triggers | 800 | 422 | You may only request up to 500 triggers per call, plus parameter validation. |
| Inbound rules | 809 | 422 | No trigger data received. |
| Inbound rules | 810 | 422 | This inbound rule already exists. |
| Inbound rules | 811 | 422 | Unable to remove this inbound rule. Please contact support. |
| Inbound rules | 812 | 422 | This inbound rule was not found. |
| Message Streams | 1220 | 422 | You do not have permission to use the message streams API. |
| Message Streams | 1221 | 422 | The `MessageStreamType` associated with this request was invalid. |
| Message Streams | 1222 | 422 | A valid `ID` must be provided. |
| Message Streams | 1223 | 422 | A valid `Name` must be provided. |
| Message Streams | 1224 | 422 | The `Name` is too long. |
| Message Streams | 1225 | 422 | You have reached the maximum number of message streams for this server. |
| Message Streams | 1226 | 422 | The message stream for the provided `ID` was not found. |
| Message Streams | 1227 | 422 | The `ID` must be a non-empty string starting with a letter, up to 30 characters. |
| Message Streams | 1228 | 422 | A server can only have one inbound stream. |
| Message Streams | 1229 | 422 | You cannot archive the default transactional and inbound streams. |
| Message Streams | 1230 | 422 | The `ID` provided already exists for this server. |
| Message Streams | 1231 | 422 | The `Description` is too long. |
| Message Streams | 1232 | 422 | You cannot unarchive this message stream anymore. |
| Message Streams | 1233 | 422 | The `ID` must not start with the `pm-` prefix. |
| Message Streams | 1234 | 422 | The `Description` must not contain HTML tags. |
| Message Streams | 1237 | 422 | The `ID` is reserved. |
| Message Streams | 1238 | 422 | You do not have permission to use Custom Unsubscribe Handling for this stream. |
| Message Streams | 1239 | 422 | The `UnsubscribeHandlingType` provided is not supported for this stream type. |
| Message Streams | 1240 | 422 | The `UnsubscribeHandlingType` associated with this request is invalid. |
| Message Streams | 1241 | 422 | Stream is unable to be archived at this time. |
| Suppressions | 1400 | 422 | Parameter `count` should be an integer within the allowed range. |
| Suppressions | 1401 | 422 | Parameter `count` is required but was left out. |
| Suppressions | 1402 | 422 | Parameter `offset` should be an integer greater than or equal to zero. |
| Suppressions | 1403 | 422 | Parameter `offset` is required but was left out. |
| Suppressions | 1404 | 422 | Parameter `SuppressionReason` is invalid. |
| Suppressions | 1405 | 422 | Parameter `Origin` is invalid. |
| Suppressions | 1406 | 200 body | You do not have the required authority to change this suppression (per-item result). |
| Suppressions | 1407 | 422 | Something went wrong when processing the request. |
| Suppressions | 1408 | 422 / 200 body | An invalid email address was provided. |
| Suppressions | 1409 | 422 | A proper request body must be provided. |
| Suppressions | 1410 | 422 | You cannot provide more than the maximum number of suppressions for this request. |
| Suppressions | 1411 | 422 | Parameter `emailAddress` is required but was left out. |
| Sender Signatures & Domains | 500 | 422 | Signature/domain list paging (`count`/`offset` required or integer, up to 500). |
| Sig & Domains | 501 | 422 / 404 / 500 | Signature not found (422/404), or the signature has no DKIM info (500). |
| Sig & Domains | 502 | 422 | No update data or signature data received. |
| Sig & Domains | 503 | 422 | You can't use public domain emails or public domains. |
| Sig & Domains | 504 | 422 | This signature already exists, or a similar signature already exists. |
| Sig & Domains | 505 | 422 | This DKIM is already being renewed. |
| Sig & Domains | 506 | 422 | This Sender Signature has already been confirmed. |
| Sig & Domains | 507 | 422 | You do not own this Sender Signature. |
| Sig & Domains | 508 | 422 | This DKIM is not being renewed, or a key in a failed state cannot be rotated. |
| Sig & Domains | 510 | 422 | This domain was not found. |
| Sig & Domains | 511 | 422 | Invalid fields supplied. |
| Sig & Domains | 512 | 422 | Domain already exists. |
| Sig & Domains | 513 | 422 | You do not own this Domain. |
| Sig & Domains | 514 | 422 | Name is a required field to create a Domain. |
| Sig & Domains | 515 | 422 | Name field must be ≤ 255 characters. |
| Sig & Domains | 516 | 422 | Name format is invalid. |
| Sig & Domains | 520 | 422 | FromEmail is a required field to create a Sender Signature. |
| Sig & Domains | 521 | 422 | A field is too long (confirmation note, Name, FromEmail, ReplyToEmail, ReturnPathDomain, or CustomTrackingDomain). |
| Sig & Domains | 522 | 422 | A value is not a valid email address, domain, or subdomain. |
| Sig & Domains | 523 | 422 | You need to add a CNAME record that points to the expected value. |
| Sig & Domains | 614 | 422 | Signature/domain entitlement limit reached. |
| Sig & Domains | 709 | 500 | DKIM verification failed due to invalid configuration. Please contact support. |
| SMTP Tokens (new) | 1450 | 422 | Parameter `serverId` is required but was left out. |
| SMTP Tokens | 1451 | 422 | This token could not be found. |
| SMTP Tokens | 1452 | 422 | A request body must be provided. |
| SMTP Tokens | 1453 | 422 | This server was not found. |
| SMTP Tokens | 1454 | 422 | A valid `MessageStream` is required, or the specified stream does not exist. |
| SMTP Tokens | 1455 | 422 | The request must contain a valid and existing `ServerID`. |
| SMTP Tokens | 1456 | 422 | Token length must be within the allowed range. |
| SMTP Tokens | 1457 | 422 | Tokens cannot be used with inbound streams. |
| SMTP Tokens | 1458 | 422 | A message stream's token limit would be exceeded by this request. |
| SMTP Tokens | 1459 | 422 | A token cannot be issued for an archived stream scheduled for deletion. |
| SMTP Tokens | 1460 | 422 | SMTP is currently disabled for the specified server. |
| Statistics API | 614 | 422 | You are not entitled to use the stats API. Upgrade to the next tier to add it. |
| Statistics | 900 | 422 | A parameter should be a date/time value. |
| Statistics | 1226 | 422 | The message stream for the provided `ID` was not found. |
| Statistics | 1500 | 422 | The `FromDate` field cannot be older than one year ago. |
| Statistics | 1501 | 422 | Parameter `count` should be an integer within the allowed range. |
| Statistics | 1502 | 422 | Parameter `FromDate` must be older than `ToDate`. |
| GDPR API | 1300 | 422 | Empty request, or an invalid offset or count. |
| GDPR | 1301 | 422 | Missing or incorrect data removal request ID. |
| GDPR | 1302 | 422 | You don't have permission to process or review data removal requests through the API. |
| Webhooks API (new) | 1350 | 422 | You cannot create a webhook using an archived `MessageStream`. |
| Webhooks | 1351 | 422 | You cannot create a webhook using an inbound stream. |
| Webhooks | 1352 | 422 | The webhook for the provided `ID` was not found. |
| Webhooks | 1353 | 422 | The webhook trigger is not supported on this message stream. |
| Webhooks | 1354 | 422 | The request must contain a valid `Url` field. |
| Webhooks | 1355 | 422 | A request body must be provided. |
| Webhooks | 1356 | 422 | A request `ID` must not be provided when creating a webhook. |
| Webhooks | 1357 | 422 | You cannot update the `ID` or `MessageStream` fields of a webhook. |
| Webhooks | 1358 | 422 | You must provide a valid `HttpHeader` `Name`. |
| Webhooks | 1359 | 422 | You have reached the maximum number of webhooks for this stream. |
| Webhooks | 1360 | 422 | You cannot update the integration. |
| Webhooks | 1361 | 422 | Invalid value provided for a field. |
| Webhooks | 1362 | 422 | Invalid value provided for `status`. Must be one of: verified, unverified. |
| Webhooks | 1363 | 422 | The `Status` field cannot be provided when creating or updating a webhook. |
| Webhooks | 1364 | 422 | Webhook verification failed; nothing was saved. Fix the endpoint and retry, or send `?verify=false` to save it unverified. |

### Rate limits
- Only HTTP 429 is documented. No numeric limit appears on the overview page or on the three user-guide pages checked. **Treat the limit as unknown**: back off on 429.

### Retention and time windows
- Messages: "Messages expire after your retention period, which is 45 days by default (but retention can be customized from 7 to 365 days)."
- Bounces: "available for your retention period, which is 45 days by default." Bounce dumps: "Postmark doesn't store bounce dumps older than 30 days."
- Stats: "stored permanently and do not expire. All stats use EST timezone. If no from/to date is provided, all time stats are returned." Error `1500`, however, says "The `FromDate` field cannot be older than one year ago." These conflict; not resolved.
- Archived message streams: "deleted 45 days after archiving date. Until this date, it can be restored."
- Webhook statistics: rolling 24-hour window, not filterable.
- Opens: "Postmark API only stores the first open, so TotalCount will always equal "1"" for the per-message opens endpoint.

### Size limits
- `/email`: 10 MB total including attachments. TextBody and HtmlBody up to 5MB each (user guide). Subject max 2000 characters. From max 255. Tag max 1000. Max 50 recipients across To+Cc+Bcc.
- `/email/batch` and `/email/batchWithTemplates`: up to 500 messages, 50 MB payload.
- Suppressions create/delete: max 50 per call.
- Servers: up to 10 message streams each, including defaults; only 1 inbound stream.
- Templates: up to 100 per server.

### Sandbox and test token
- Overview: "You can do this by passing the `POSTMARK_API_TEST` value in the `X-Postmark-Server-Token` header field." It validates the payload and sends test emails that "don't actually get delivered to the recipient."
- Sandbox servers: server `DeliveryType` = `Sandbox` (vs `Live`). The type is set at create time and "cannot be changed after the server has been created." Outbound messages carry `Sandboxed` (boolean). Per the sandbox-mode user guide, sandbox server sends "do count towards your monthly sending volume."

---

## 2. Endpoints NOT in either official spec (21)

| Family | Count |
|---|---|
| Bulk email | 3 |
| Message streams | 6 |
| Suppressions | 3 |
| Webhooks | 7 |
| Data removals | 2 |
| Templates, servers, domains, signatures, stats, messages, bounces, inbound rules, server, email | 0 (all in specs; see Section 3 for drift) |

Layouts have no endpoints of their own. They are templates with `TemplateType: Layout` on the existing `/templates` routes (drift, Section 3). `PUT /templates/push` is already in the account spec.

### 2.1 Bulk email (server token). Account needs approval; otherwise every call returns 422 ErrorCode 14.

**POST /email/bulk**: send one broadcast message to many recipients.
- Body:

| Field | Type | Req | Notes |
|---|---|---|---|
| From | string | yes | confirmed Sender Signature |
| ReplyTo | string | | |
| Subject | string | | |
| HtmlBody | string | | "If no TextBody specified" |
| TextBody | string | | "If no HtmlBody specified" |
| TemplateId | integer | | alternative to inline content |
| TemplateAlias | string | | alternative to TemplateId |
| InlineCss | boolean | | |
| Tag | string | | max 1000 |
| Metadata | object | | message-level metadata takes precedence |
| MessageStream | string | | default: "the outbound broadcast stream" |
| TrackOpens | boolean | | |
| TrackLinks | string | | `None` `HtmlAndText` `HtmlOnly` `TextOnly` |
| Attachments | array | | |
| Headers | array | | message-level headers take precedence |
| Messages | array of Message | yes | |
| Messages[].To | string | | comma-separated, max 50 |
| Messages[].Cc | string | | max 50 |
| Messages[].Bcc | string | | max 50 |
| Messages[].TemplateModel | object | | |
| Messages[].Metadata | object | | |
| Messages[].Headers | array | | |

- Response is a status object: `Id` (string), `Status` (`Accepted`, `Processing`, `Completed`, `Cancelled`), `SubmittedAt`, `TotalMessages` (int), `PercentageCompleted` (float 0–100), `Subject`, `ReleasedCount` (int), `FailedCount` (int).
- Any malformed field rejects the whole request with 422 (`ErrorCode` 11 + `Errors` object). No Id is created.

**GET /email/bulk/{bulk-request-id}**: status of one bulk request.
- Path: `bulk-request-id` (string, UUID).
- Response: the same status object. Null-valued properties are omitted. `TotalMessages`, `PercentageCompleted`, `ReleasedCount`, and `FailedCount` are always present.
- Using a different server's token returns 404 ErrorCode 12.

**GET /email/bulk**: list bulk requests on this server, newest first.
- Query (from curl examples only; no param table on the page): `count` (int, example 20, max not documented), `paginationKey` (string, opaque).
- Response: `Requests` (array of status objects), `PaginationKey` (string; absent on the last page).

### 2.2 Message streams (server token)

Path param `stream_ID` (string). A new stream ID must start with a letter, be at most 30 chars, not start with `pm-`, and not be a reserved ID (errors 1227, 1233, 1237).

Stream object fields: `ID` (string), `ServerID` (integer), `Name`, `Description` (nullable), `MessageStreamType` (`Inbound` `Broadcasts` `Transactional`), `CreatedAt`, `UpdatedAt` (nullable), `ArchivedAt` (nullable), `ExpectedPurgeDate` (nullable), `SubscriptionManagementConfiguration` {`UnsubscribeHandlingType`: `None` `Custom` `Postmark`}.

**GET /message-streams**: list streams.
- Query: `MessageStreamType` (`All` `Inbound` `Transactional` `Broadcasts`, default `All`; the curl example uses lowercase `all`), `IncludeArchivedStreams` (boolean, default `False`).
- Response: `MessageStreams` (array of stream objects), `TotalCount` (integer). No pagination.

**GET /message-streams/{stream_ID}**: returns one stream object.

**PATCH /message-streams/{stream_ID}**: edit a stream.
- Body: `Name` (string), `Description` (string), `SubscriptionManagementConfiguration` (object) → `UnsubscribeHandlingType` (string: `none` `Postmark` `Custom`). Unsubscribe management is required for broadcast streams; `Custom` needs approval.
- Response: stream object.

**POST /message-streams**: create a stream.
- Body: `ID` (string, required), `Name` (string, required), `Description` (string), `MessageStreamType` (string, required; docs list `Broadcasts` `Transasctional` [sic], example uses `Transactional`; inbound cannot be created beyond the single default), `SubscriptionManagementConfiguration` {`UnsubscribeHandlingType`: `none` `Postmark` `Custom`}.
- Response: stream object, without `ExpectedPurgeDate` in the table.

**POST /message-streams/{stream_ID}/archive**: archive a stream; it is purged 45 days later.
- Headers: `Content-Length: 0`.
- Response: `ID`, `ServerID`, `ExpectedPurgeDate`.
- Default transactional and inbound streams cannot be archived (1229).

**POST /message-streams/{stream_ID}/unarchive**: restore an archived stream before its purge date.
- Headers: `Content-Length: 0`.
- Response: stream object with `ArchivedAt` null. Returns 1232 if unarchiving is no longer possible.

### 2.3 Suppressions (server token)

Path param `stream_id` (string, message stream ID; lowercase `id` on this page).

**GET /message-streams/{stream_id}/suppressions/dump**: list suppressed addresses on a stream.
- Query:

| Param | Values |
|---|---|
| SuppressionReason | `HardBounce`, `SpamComplaint`, `ManualSuppression` |
| Origin | `Recipient`, `Customer`, `Admin` (curl uses lowercase `origin=recipient`) |
| todate | date, inclusive, e.g. `2020-02-01` |
| fromdate | date, inclusive |
| EmailAddress | string |

- Response: `Suppressions` array of {`EmailAddress`, `SuppressionReason`, `Origin`, `CreatedAt`}. No TotalCount and no documented pagination. Errors 1400–1403 reference `count`/`offset`, so the server may accept them; **unverified**.

**POST /message-streams/{stream_id}/suppressions**: suppress addresses.
- Body: `Suppressions` (array, required, max 50) of {`EmailAddress` (string)}.
- Response: `Suppressions` array of {`EmailAddress`, `Status` (`Failed`, `Suppressed`), `Message` (null on success)}. Per-item failures come back in a 200 body (codes 1406, 1408).

**POST /message-streams/{stream_id}/suppressions/delete**: remove suppressions. A deletion that uses the POST verb.
- Body: `Suppressions` (array, required, max 50) of {`EmailAddress`}.
- Response: `Suppressions` array of {`EmailAddress`, `Status` (`Failed`, `Deleted`), `Message`}.
- "`SpamComplaint` suppressions cannot be deleted." Deleting a `HardBounce` suppression "is the equivalent of reactivating the associated Bounce."

### 2.4 Webhooks (server token)

Path param `Id` (integer). The docs render the GET and PUT paths as `webhooks/{Id}` without the leading slash; the curl examples use `/webhooks/{Id}`.

Webhook object: `ID` (integer), `Url`, `MessageStream`, `Status` (`verified` | `unverified`), `HttpAuth` {`Username`, `Password`}, `HttpHeaders` [{`Name`, `Value`}], `Triggers` {`Open` {`Enabled`, `PostFirstOpenOnly`}, `Click` {`Enabled`}, `Delivery` {`Enabled`}, `Bounce` {`Enabled`, `IncludeContent`}, `SpamComplaint` {`Enabled`, `IncludeContent`}, `SubscriptionChange` {`Enabled`}}.

**GET /webhooks**: list webhooks.
- Query: `MessageStream` (string). Omitted returns all webhooks on the server. A nonexistent stream returns an error rather than an empty list.
- Response: `Webhooks` array. No TotalCount, no pagination.
- Error 1362 mentions a `status` value (`verified`, `unverified`), which may be a list filter. It is not in the query table; **unverified**.

**GET /webhooks/{Id}**: returns one webhook object.

**POST /webhooks**: create a webhook.
- Body: `Url` (string, required), `MessageStream` (string, default `outbound`), `HttpAuth` (object), `HttpHeaders` (array), `Triggers` (object: `Open`, `Click`, `Delivery`, `Bounce`, `SpamComplaint`, `SubscriptionChange`, shaped as above), `Verify` (boolean, default true).
- `ID` and `Status` must not be sent (1356, 1363).
- Response: webhook object.
- With verification on and any trigger failing: 422 code 1364 and nothing saved. The failure body is `Id`, `Url`, `Success` (bool), `Results` [{`TriggerType`, `Success`, `StatusCode`, `Message`}], `Message`. Error 1364's text says `?verify=false` (query form) while the page documents body `Verify`. Which form the server honors is **unverified**.
- Archived or inbound streams are rejected (1350, 1351).

**PUT /webhooks/{Id}**: edit a webhook.
- Body: `Url`, `HttpAuth`, `HttpHeaders`, `Triggers` (partial: triggers you omit stay unchanged), `Verify` (boolean, default true).
- `ID` and `MessageStream` cannot change (1357).
- Re-verifies on URL change or newly enabled triggers. On failure: 422 code 1364 and the existing webhook stays unchanged.
- Response: webhook object.

**POST /webhooks/{Id}/verify**: test the endpoint for each enabled trigger and set verified or unverified. Postmark sends real HTTP requests to the customer's URL.
- Response: `Id`, `Url`, `Success`, `Results` [{`TriggerType`, `Success`, `StatusCode`, `Message`}], `Message` (e.g. "4/5 triggers verified successfully").
- "A 200 response means the verification check ran, not that it passed."
- The docs curl uses host `https://example.com` (a doc typo).

**DELETE /webhooks/{Id}**: delete a webhook. Response: `ErrorCode`, `Message`.

**GET /webhooks/{Id}/statistics**: delivery stats for one webhook over the last 24h.
- Response: `WebhookId` (int), `ServerId` (int), `MessageStreamId` (string), `Url`, `Statuses` (object keyed by trigger → `verified`/`unverified`), `TimeRange` {`StartTime`, `EndTime`, `Hours`}, `Metrics` {`TotalRequests`, `SuccessCount`, `FailureCount`, `RetryCount`, `SuccessRate` (number), `SlowCount`, `VerySlowCount`, `AverageTerminalResponseTimeMs`, `AverageRetryResponseTimeMs` (int|null)}, `MetricsByTrigger` (object keyed by trigger, same metric fields).
- No query params. "The window is fixed at the last 24 hours in this version and can't be filtered yet."

### 2.5 Data removals (account token). "Available by request only."

**POST /data-removals**: erase a recipient's data (GDPR/CCPA DSR).
- Body: `RequestedBy` (string, email), `RequestedFor` (string, "must be a valid email address"), `NotifyWhenCompleted` (boolean). None are marked required on the page.
- Response: `ID` (integer), `Status` (`Pending`, `Done`).

**GET /data-removals/{id}**: check a removal request.
- Path: `id` (integer).
- Response: `ID`, `Status` (`Pending`, `Done`).

---

## 3. Endpoints in the specs, with drift

### 3.1 Cross-cutting drift
| Item | Spec | Docs |
|---|---|---|
| `MessageStream` on sends | absent | body field on `/email`, `/email/batch`, `/email/withTemplate`, `/email/batchWithTemplates`; default `outbound` |
| `Metadata` on sends | absent | `Metadata` object on all four send endpoints |
| `messagestream` query filter | absent everywhere | on `/bounces`, `/messages/outbound`, `/messages/outbound/opens`, `/messages/outbound/clicks`, and all 12 `/stats/outbound*` (stats: omitted = all streams; messages/bounces/opens/clicks: omitted = `outbound`) |
| `MessageStream` response field | absent | on bounces, outbound messages, outbound details, opens, clicks |
| Server `TrackLinks` enum (account spec `ExtendedServerInfo`, `CreateServerPayload`, `EditServerPayload`) | `None` `HtmlAndTextTracking` `HtmlOnlyTracking` `TextOnlyTracking` | `None` `HtmlAndText` `HtmlOnly` `TextOnly` |
| Server `Color` enum | `purple` `blue` `turqoise` `green` `red` `yellow` `grey` (server spec only; account spec untyped) | `Purple` `Blue` `Turquoise` `Green` `Red` `Yellow` `Grey` `Orange` (examples send lowercase) |
| Server fields | absent | `DeliveryType` (`Live` `Sandbox`, create-only), `IncludeBounceContentInHook` (bool), `EnableSmtpApiErrorHooks` (bool) on `/server`, `/servers`, `/servers/{serverid}` |
| DKIM field name | `DKIMTestValue` (typo) | `DKIMTextValue` (domains, senders, rotatedkim) |
| Date filters on bounces/messages | `format: date` | date/time `YYYY-MM-DDTHH:MM:SS`, Eastern Time |
| Paging caps | `/bounces` count max 500 only | count ≤ 500 and count + offset ≤ 10,000 on bounces, messages, opens |

### 3.2 Per-endpoint drift
| Endpoint | Drift |
|---|---|
| POST /email | Spec lacks `Metadata`, `MessageStream`. Docs mark `From` and `To` required, and `HtmlBody`/`TextBody` required unless the other is given. |
| POST /email/batch | Same as /email. Docs: up to 500 messages, 50 MB; HTTP 200 with per-message `ErrorCode`. |
| POST /email/withTemplate | Spec lacks `Metadata`, `MessageStream`. Spec `required` lists both `TemplateId` and `TemplateAlias`; docs want one or the other. Docs render the path as `/email/withTemplate/` (trailing slash; curl has none). |
| POST /email/batchWithTemplates | Same as withTemplate. Docs: `Messages` required, max 500; per-message: if both `TemplateId` and `TemplateAlias` are sent, `TemplateId` wins. |
| GET /bounces | Spec lacks `messagestream`. Spec `type` enum has `MailFrontier Matador.` where docs have `ChallengeVerification` (code 16384). Spec response `BounceInfoResponse` lacks `RecordType`, `ServerID`, `MessageStream`, `From`; spec types `ID` as string, docs as integer. |
| GET /bounces/{bounceid}, PUT .../activate | Same response-field drift (`ServerID`, `MessageStream`, `From` missing; `ID` type). |
| GET /bounces/{bounceid}/dump | No `operationId` in spec. |
| GET /messages/outbound | Spec lacks `subject`, `messagestream`, `metadata_` (e.g. `metadata_color`, one metadata field at a time). `status` enum: spec `queued` `sent`; docs `queued` or `sent` / `processed` ("same results"). Items lack `MessageStream`, `Metadata`, `Sandboxed`. |
| GET /messages/outbound/{messageid}/details | Spec lacks `MessageStream`, `Metadata`, `Sandboxed`. `Status` values: `Sent`, `Processed`, `Queued`. `MessageEvents[].Type`: `SubscriptionChanged`, `Delivered`, `Transient`, `Opened`, `LinkClicked`, `Bounced`. Spec `Details` lacks `Origin`, `SuppressSending`, `Link`, `ClickLocation`. The docs example shows `Attachments` as an array of filename strings; spec says Attachment objects. |
| GET /messages/inbound | No `messagestream` on this endpoint in the docs either. `status` default is `processed`. The docs response table names the list key `Messages`, but the example (and spec) use `InboundMessages`. Attachment items carry `ContentLength` (not in spec `Attachment`). |
| GET /messages/inbound/{messageid}/details | Attachment `ContentLength` missing from spec. |
| GET /messages/outbound/opens | Spec lacks `messagestream`. Items lack `RecordType`, `MessageStream`. `Platform` values: `WebMail` `Desktop` `Mobile` `Unknown`. |
| GET /messages/outbound/opens/{messageid} | Spec `count` has `default: 1`; docs give none. Items lack `MessageStream`. |
| GET /messages/outbound/clicks | Spec lacks `messagestream`. Items lack `RecordType`, `MessageStream`. |
| GET /messages/outbound/clicks/{messageid} | Items lack `MessageStream`. |
| GET /templates | Spec lacks `TemplateType` (`All` `Standard` `Layout`, default `All`) and `LayoutTemplate` (filter by layout alias). The spec response list key is literally `Templates API` (broken); docs use `Templates`. Items lack `TemplateType`, `LayoutTemplate`. Error 1100 also names an `editorType` query param that is not documented on the page. |
| POST /templates | Spec lacks `TemplateType` (`Standard` or `Layout`, default `Standard`, immutable after create) and `LayoutTemplate` (layout alias; `""` clears it). Spec requires `Name`, `Subject`; docs require `Name`, `HtmlBody` or `TextBody`, and `Subject` only for Standard (a Subject on a Layout errors). Response adds `TemplateType`, `LayoutTemplate`. No `operationId` in spec. |
| GET /templates/{templateIdOrAlias} | Response lacks `TemplateType`, `LayoutTemplate`. Spec field `TemplateID`; docs `TemplateId`. |
| PUT /templates/{templateIdOrAlias} | Spec lacks `LayoutTemplate`. Spec `required: [TemplateId]` is wrong (the ID is in the path). Docs mark `Name`, `Subject`, `HtmlBody`/`TextBody` required. |
| DELETE /templates/{templateIdOrAlias} | Spec response is `TemplateDetailResponse`; docs return `ErrorCode`, `Message`. Deleting a layout that templates still use fails with 1130. |
| POST /templates/validate | Spec lacks `TemplateType` (`Standard` or `Layout`) and `LayoutTemplate`. The docs example wraps `ValidationErrors` inside per-part objects (`HtmlBody`, `TextBody`, `Subject`), matching the spec. |
| PUT /templates/push (account) | Body casing: docs `SourceServerID`, `DestinationServerID` (typed `string` in the table, integers in the example); spec `SourceServerId`, `DestinationServerId`. All three fields, including `PerformChanges`, are required in the docs. Response items add `TemplateType`. `PerformChanges: false` is a dry run. |
| GET /stats/outbound | Spec lacks `messagestream`. Field `SMTPApiErrors` (docs) vs `SMTPAPIErrors` (spec). `BounceRate` and `SpamComplaintsRate` are double in the docs, integer in the spec. |
| GET /stats/outbound/clicks | Spec has `DynamicResponse`; docs define `Clicks`, `Unique`, `Days[]`. |
| GET /stats/outbound/clicks/platforms | Spec dynamic; docs `Desktop`, `Mobile`, `Unknown`, `Days[]`. |
| GET /stats/outbound/clicks/location | Spec dynamic; docs `HTML`, `Text`, `Days[]`. |
| GET /stats/outbound/opens/emailclients, /clicks/browserfamilies | Dynamic keys (client or browser name → count) plus `Days[]`, as in the spec. |
| Other stats (sends, bounces, spam, tracked, opens, opens/platforms) | Only drift is the missing `messagestream`. "Days that didn't produce statistics won't appear in the JSON response." |
| GET/PUT /server | Spec lacks `DeliveryType` (response), `IncludeBounceContentInHook`, `EnableSmtpApiErrorHooks`. Color enum drift (above). |
| GET/POST/PUT /servers, /servers/{serverid} | TrackLinks enum drift and missing fields (above). Docs mark `Name` required on create. In create/edit bodies, `BounceHookUrl`, `OpenHookUrl`, `DeliveryHookUrl`, `ClickHookUrl` are documented as "Use the ... Webhook API instead." `GET /servers` `name` is a substring match. |
| DELETE /servers/{serverid} | Spec response has no schema; docs return `ErrorCode`, `Message`. "This feature is not enabled for all accounts" (error 604). |
| POST /senders, PUT /senders/{signatureid} | Spec lacks `ConfirmationPersonalNote` (max 400 chars) in both body and response. Docs mark `FromEmail` and `Name` required on create and `Name` required on edit. |
| GET /senders/{signatureid} | Response adds `ConfirmationPersonalNote`. |
| POST /senders/{signatureid}/verifyspf | **Deprecated** in docs ("we no longer ask for SPF records"). |
| POST /senders/{signatureid}/requestnewdkim | **Deprecated** in docs ("Please use the new Domains API for updated DKIM methods"). |
| PUT /domains/{domainid}/verifydkim, /verifyreturnpath | Docs path casing is `/verifyDkim`, `/verifyReturnPath`; spec is lowercase. Server-side case sensitivity is **unverified**. |
| POST /domains/{domainid}/verifyspf | **Deprecated** in docs. |
| POST /domains | Docs mark `Name` required. |
| SPF fields (`SPFVerified`, `SPFHost`, `SPFTextValue`) | Still returned, documented as deprecated or no longer necessary. |

### 3.3 Deprecated or removed
- Docs mark these deprecated (still in the specs): `POST /domains/{domainid}/verifyspf`, `POST /senders/{signatureid}/verifyspf`, `POST /senders/{signatureid}/requestnewdkim`.
- Bounce tags (`GET /bounces/tags`) are not on the current Bounce API page and not in either spec. Nothing to carry forward.
- The server-level hook URL fields on `/servers` create/edit point to the Webhooks API instead.
- Every spec operation still appears in the docs.

### 3.4 Doc-internal inconsistencies (do not guess; verify live)
- Inbound search list key: table says `Messages`, example says `InboundMessages`.
- Sender signature response tables type `ReturnPathDomain` as boolean. Examples and the verifyspf table treat it as a string.
- `PUT /server` response table types `DeliveryType` as boolean; example `"Live"`.
- Message streams create enum typo `Transasctional`.
- `UnsubscribeHandlingType` casing appears as both `None` and `none`.
- Error-code hints for params not documented on endpoint pages: `editorType` (templates, 1100), `status` verified/unverified (webhooks, 1362), `?verify=false` (webhooks, 1364), `count`/`offset` (suppressions, 1400–1403), `count` (stats, 1501), `CustomTrackingDomain` (signatures/domains, 521). All **unverified**.

---

## 4. List-response shapes (drives local sync)

| Endpoint | List key | Total key | Pagination |
|---|---|---|---|
| GET /bounces | `Bounces` | `TotalCount` | `count` (req, ≤500) + `offset` (req); count+offset ≤ 10,000 |
| GET /messages/outbound | `Messages` | `TotalCount` | `count` ≤500 + `offset`; ≤ 10,000; window with `fromdate`/`todate` |
| GET /messages/inbound | `InboundMessages` (table says `Messages`) | `TotalCount` | same as outbound |
| GET /messages/outbound/opens | `Opens` | `TotalCount` | `count` ≤500 + `offset`; ≤ 10,000 |
| GET /messages/outbound/opens/{messageid} | `Opens` | `TotalCount` (always 1) | `count` ≤500 + `offset` |
| GET /messages/outbound/clicks | `Clicks` | `TotalCount` | `count` ≤500 + `offset` (10k cap not stated) |
| GET /messages/outbound/clicks/{messageid} | `Clicks` | `TotalCount` | `count` ≤500 + `offset` |
| GET /templates | `Templates` (spec: `Templates API`) | `TotalCount` | `Count` + `Offset` (req; max undocumented; ≤100 templates per server) |
| GET /triggers/inboundrules | `InboundRules` | `TotalCount` | `count` + `offset` (req; ≤500 per error 800) |
| GET /servers | `Servers` | `TotalCount` | `count` + `offset` (req; ≤500 per error 600) |
| GET /senders | `SenderSignatures` | `TotalCount` | `count` ≤500 + `offset` |
| GET /domains | `Domains` | `TotalCount` | `count` ≤500 + `offset` |
| GET /message-streams | `MessageStreams` | `TotalCount` | none (≤10 streams per server) |
| GET /message-streams/{stream_id}/suppressions/dump | `Suppressions` | none | none documented |
| GET /webhooks | `Webhooks` | none | none |
| GET /email/bulk | `Requests` | none | cursor: `count` + `paginationKey` → `PaginationKey` (absent on last page) |
| GET /deliverystats | `Bounces` (per-type counts) | none (`InactiveMails` is a count, not a total) | none |
| GET /stats/outbound/* (11 time series) | `Days` | none (aggregate fields at top level) | none; date-windowed |
| PUT /templates/push (response) | `Templates` | `TotalCount` | n/a |

---

## 5. Mutation and destructive table (43 non-GET operations)

| Method | Path | Token | Class | Note |
|---|---|---|---|---|
| POST | /email | server | send | |
| POST | /email/batch | server | send | |
| POST | /email/withTemplate | server | send | |
| POST | /email/batchWithTemplates | server | send | |
| POST | /email/bulk | server | send | not in spec |
| PUT | /bounces/{bounceid}/activate | server | action | reactivates address |
| PUT | /messages/inbound/{messageid}/bypass | server | action | |
| PUT | /messages/inbound/{messageid}/retry | server | action | |
| POST | /templates | server | create | |
| PUT | /templates/{templateIdOrAlias} | server | update | |
| DELETE | /templates/{templateIdOrAlias} | server | delete | |
| POST | /templates/validate | server | action | no side effects; safe to route as read |
| PUT | /templates/push | account | action | writes to destination server unless `PerformChanges: false` |
| PUT | /server | server | update | |
| POST | /servers | account | create | |
| PUT | /servers/{serverid} | account | update | |
| DELETE | /servers/{serverid} | account | delete | |
| POST | /message-streams | server | create | not in spec |
| PATCH | /message-streams/{stream_ID} | server | update | not in spec; only PATCH in the API |
| POST | /message-streams/{stream_ID}/archive | server | action | purge after 45 days; treat as destructive |
| POST | /message-streams/{stream_ID}/unarchive | server | action | not in spec |
| POST | /message-streams/{stream_id}/suppressions | server | create | not in spec |
| POST | /message-streams/{stream_id}/suppressions/delete | server | delete | not in spec; delete via POST |
| POST | /webhooks | server | create | not in spec; verifies by calling customer URL |
| PUT | /webhooks/{Id} | server | update | not in spec |
| POST | /webhooks/{Id}/verify | server | action | not in spec; hits customer URL |
| DELETE | /webhooks/{Id} | server | delete | not in spec |
| POST | /triggers/inboundrules | server | create | |
| DELETE | /triggers/inboundrules/{triggerid} | server | delete | |
| POST | /domains | account | create | |
| PUT | /domains/{domainid} | account | update | |
| DELETE | /domains/{domainid} | account | delete | |
| PUT | /domains/{domainid}/verifyDkim | account | action | |
| PUT | /domains/{domainid}/verifyReturnPath | account | action | |
| POST | /domains/{domainid}/verifyspf | account | action | deprecated |
| POST | /domains/{domainid}/rotatedkim | account | action | |
| POST | /senders | account | create | sends confirmation email |
| PUT | /senders/{signatureid} | account | update | |
| DELETE | /senders/{signatureid} | account | delete | |
| POST | /senders/{signatureid}/resend | account | send | emails the signature address |
| POST | /senders/{signatureid}/verifyspf | account | action | deprecated |
| POST | /senders/{signatureid}/requestnewdkim | account | action | deprecated |
| POST | /data-removals | account | delete | not in spec; irreversible erasure |

Row count matches the 43 non-GET operations in the docs. `POST /templates/validate` is the only read-shaped POST.

## Addendum: Return-Path CNAME target (Domains API)
- `GET/POST /domains` responses include `ReturnPathDomainCNAMEValue` (string): the CNAME target for the custom Return-Path host. The Domains API docs examples show the value `pm.mtasv.net` (7 mentions on https://postmarkapp.com/developer/api/domains-api). `servers bootstrap` reads the response field and falls back to `pm.mtasv.net` only when it is absent.
