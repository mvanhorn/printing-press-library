# GoHighLevel API Endpoint Inventory

## Sources
- Local MCP: ~/Documents/ghl-mcp-server-kwcp/ (mastanley13/GoHighLevel-MCP fork) — 19 tool files, 1 API client (6,830 LOC)
- Published docs: https://highlevel.stoplight.io/docs/integrations/ (sidebar resource groups cross-referenced; full Stoplight scrape blocked by JS-rendered sidebar — relied on local MCP as authoritative source since it implements ~250 tools across the full public surface)

## Reachability Mode
mode: standard_http
http_transport: standard
auth_type: api_key (Bearer pit-<uuid>)
header_name: Authorization
base_url: https://services.leadconnectorhq.com
default_version_header: 2021-07-28
conversations_version_header: 2021-04-15

Confirmed from `src/clients/ghl-api-client.ts`:
- `'Version': config.version` (default 2021-07-28) set at line 402
- `'Version': '2021-04-15'` override at line 465 for Conversations endpoints

## Endpoints by Resource

Path placeholders use `{var}` form. `?locationId` indicates the call requires a `locationId` query param (most "list/search/get-all" endpoints require it). All endpoints use `Authorization: Bearer pit-...` plus the version header for their resource.

### Contacts (32 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| POST | /contacts/ | Create contact | bearer | 2021-07-28 |
| GET | /contacts/{contactId} | Get contact by id | bearer | 2021-07-28 |
| PUT | /contacts/{contactId} | Update contact | bearer | 2021-07-28 |
| DELETE | /contacts/{contactId} | Delete contact | bearer | 2021-07-28 |
| POST | /contacts/search | Search contacts (searchAfter cursor; required for 10k+) | bearer | 2021-07-28 |
| GET | /contacts/search/duplicate | Find duplicate contact by email/phone | bearer | 2021-07-28 |
| POST | /contacts/upsert | Upsert by email/phone | bearer | 2021-07-28 |
| POST | /contacts/{contactId}/tags | Add tags to contact | bearer | 2021-07-28 |
| DELETE | /contacts/{contactId}/tags | Remove tags from contact | bearer | 2021-07-28 |
| POST | /contacts/tags/bulk | Bulk add/remove tags across contacts | bearer | 2021-07-28 |
| POST | /contacts/business/bulk | Bulk update business association | bearer | 2021-07-28 |
| GET | /contacts/business/{businessId} | List contacts by business | bearer | 2021-07-28 |
| GET | /contacts/{contactId}/appointments | List appointments for contact | bearer | 2021-07-28 |
| GET | /contacts/{contactId}/tasks | List tasks for contact | bearer | 2021-07-28 |
| POST | /contacts/{contactId}/tasks | Create task for contact | bearer | 2021-07-28 |
| GET | /contacts/{contactId}/tasks/{taskId} | Get specific task | bearer | 2021-07-28 |
| PUT | /contacts/{contactId}/tasks/{taskId} | Update task | bearer | 2021-07-28 |
| DELETE | /contacts/{contactId}/tasks/{taskId} | Delete task | bearer | 2021-07-28 |
| PUT | /contacts/{contactId}/tasks/{taskId}/completed | Mark task complete | bearer | 2021-07-28 |
| GET | /contacts/{contactId}/notes | List notes for contact | bearer | 2021-07-28 |
| POST | /contacts/{contactId}/notes | Create note | bearer | 2021-07-28 |
| GET | /contacts/{contactId}/notes/{noteId} | Get note | bearer | 2021-07-28 |
| PUT | /contacts/{contactId}/notes/{noteId} | Update note | bearer | 2021-07-28 |
| DELETE | /contacts/{contactId}/notes/{noteId} | Delete note | bearer | 2021-07-28 |
| POST | /contacts/{contactId}/followers | Add followers | bearer | 2021-07-28 |
| DELETE | /contacts/{contactId}/followers | Remove followers | bearer | 2021-07-28 |
| POST | /contacts/{contactId}/campaigns/{campaignId} | Add contact to campaign | bearer | 2021-07-28 |
| DELETE | /contacts/{contactId}/campaigns/{campaignId} | Remove contact from campaign | bearer | 2021-07-28 |
| DELETE | /contacts/{contactId}/campaigns | Remove from all campaigns | bearer | 2021-07-28 |
| POST | /contacts/{contactId}/workflow/{workflowId} | Add contact to workflow | bearer | 2021-07-28 |
| DELETE | /contacts/{contactId}/workflow/{workflowId} | Remove contact from workflow | bearer | 2021-07-28 |

### Conversations & Messages (16 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /conversations/search | Search conversations (requires locationId) | bearer | 2021-04-15 |
| GET | /conversations/{conversationId} | Get conversation | bearer | 2021-04-15 |
| POST | /conversations/ | Create conversation | bearer | 2021-04-15 |
| PUT | /conversations/{conversationId} | Update conversation | bearer | 2021-04-15 |
| DELETE | /conversations/{conversationId} | Delete conversation | bearer | 2021-04-15 |
| GET | /conversations/{conversationId}/messages | List messages in conversation | bearer | 2021-04-15 |
| GET | /conversations/messages/{id} | Get message by id | bearer | 2021-04-15 |
| POST | /conversations/messages | Send message (sms/email/whatsapp/etc) | bearer | 2021-04-15 |
| GET | /conversations/messages/email/{id} | Get email message | bearer | 2021-04-15 |
| DELETE | /conversations/messages/email/{id}/schedule | Cancel scheduled email | bearer | 2021-04-15 |
| POST | /conversations/messages/inbound | Add inbound message | bearer | 2021-04-15 |
| POST | /conversations/messages/outbound | Add outbound call | bearer | 2021-04-15 |
| DELETE | /conversations/messages/{messageId}/schedule | Cancel scheduled message | bearer | 2021-04-15 |
| POST | /conversations/messages/upload | Upload message attachments | bearer | 2021-04-15 |
| PUT | /conversations/messages/{messageId}/status | Update message status | bearer | 2021-04-15 |
| GET | /conversations/messages/{messageId}/locations/{locationId}/recording | Download recording | bearer | 2021-04-15 |
| GET | /conversations/locations/{locationId}/messages/{messageId}/transcription | Get transcription | bearer | 2021-04-15 |
| GET | /conversations/locations/{locationId}/messages/{messageId}/transcription/download | Download transcription | bearer | 2021-04-15 |
| POST | /conversations/providers/live-chat/typing | Live-chat typing indicator | bearer | 2021-04-15 |

### Calendars (28 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /calendars/groups | List calendar groups | bearer | 2021-07-28 |
| POST | /calendars/groups | Create calendar group | bearer | 2021-07-28 |
| PUT | /calendars/groups/{groupId} | Update calendar group | bearer | 2021-07-28 |
| DELETE | /calendars/groups/{groupId} | Delete calendar group | bearer | 2021-07-28 |
| POST | /calendars/groups/{groupId}/status | Disable/enable calendar group | bearer | 2021-07-28 |
| GET | /calendars/groups/slug/validate | Validate group slug | bearer | 2021-07-28 |
| GET | /calendars/ | List calendars (requires locationId) | bearer | 2021-07-28 |
| POST | /calendars/ | Create calendar | bearer | 2021-07-28 |
| GET | /calendars/{calendarId} | Get calendar | bearer | 2021-07-28 |
| PUT | /calendars/{calendarId} | Update calendar | bearer | 2021-07-28 |
| DELETE | /calendars/{calendarId} | Delete calendar | bearer | 2021-07-28 |
| GET | /calendars/events | List calendar events (locationId required) | bearer | 2021-07-28 |
| GET | /calendars/blocked-slots | List blocked slots | bearer | 2021-07-28 |
| POST | /calendars/blocked-slots | Create blocked slot | bearer | 2021-07-28 |
| GET | /calendars/{calendarId}/free-slots | Get free slots | bearer | 2021-07-28 |
| POST | /calendars/events/appointments | Create appointment | bearer | 2021-07-28 |
| GET | /calendars/events/appointments/{eventId} | Get appointment | bearer | 2021-07-28 |
| PUT | /calendars/events/appointments/{eventId} | Update appointment | bearer | 2021-07-28 |
| DELETE | /calendars/events/appointments/{eventId} | Delete appointment | bearer | 2021-07-28 |
| PUT | /calendars/events/block-slots/{eventId} | Update block slot | bearer | 2021-07-28 |
| GET | /calendars/events/appointments/{appointmentId}/notes | List appointment notes | bearer | 2021-07-28 |
| POST | /calendars/events/appointments/{appointmentId}/notes | Create appointment note | bearer | 2021-07-28 |
| PUT | /calendars/events/appointments/{appointmentId}/notes/{noteId} | Update appointment note | bearer | 2021-07-28 |
| DELETE | /calendars/events/appointments/{appointmentId}/notes/{noteId} | Delete appointment note | bearer | 2021-07-28 |
| GET | /calendars/resources/{resourceType} | List equipment/rooms | bearer | 2021-07-28 |
| POST | /calendars/resources/{resourceType} | Create equipment/room | bearer | 2021-07-28 |
| GET | /calendars/resources/{resourceType}/{resourceId} | Get resource | bearer | 2021-07-28 |
| PUT | /calendars/resources/{resourceType}/{resourceId} | Update resource | bearer | 2021-07-28 |
| DELETE | /calendars/resources/{resourceType}/{resourceId} | Delete resource | bearer | 2021-07-28 |
| GET | /calendars/{calendarId}/notifications | List notifications | bearer | 2021-07-28 |
| POST | /calendars/{calendarId}/notifications | Create notification | bearer | 2021-07-28 |
| GET | /calendars/{calendarId}/notifications/{notificationId} | Get notification | bearer | 2021-07-28 |
| PUT | /calendars/{calendarId}/notifications/{notificationId} | Update notification | bearer | 2021-07-28 |
| DELETE | /calendars/{calendarId}/notifications/{notificationId} | Delete notification | bearer | 2021-07-28 |

### Opportunities & Pipelines (11 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /opportunities/search | Search opportunities (locationId required) | bearer | 2021-07-28 |
| GET | /opportunities/pipelines | List pipelines (locationId required) | bearer | 2021-07-28 |
| GET | /opportunities/{id} | Get opportunity | bearer | 2021-07-28 |
| POST | /opportunities/ | Create opportunity | bearer | 2021-07-28 |
| PUT | /opportunities/{id} | Update opportunity | bearer | 2021-07-28 |
| PUT | /opportunities/{id}/status | Update opportunity status only | bearer | 2021-07-28 |
| POST | /opportunities/upsert | Upsert opportunity | bearer | 2021-07-28 |
| DELETE | /opportunities/{id} | Delete opportunity | bearer | 2021-07-28 |
| POST | /opportunities/{id}/followers | Add followers | bearer | 2021-07-28 |
| DELETE | /opportunities/{id}/followers | Remove followers | bearer | 2021-07-28 |

### Locations / Sub-Accounts (12 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /locations/search | Search locations (agency token) | bearer | 2021-07-28 |
| GET | /locations/{locationId} | Get location detail | bearer | 2021-07-28 |
| POST | /locations/ | Create location | bearer | 2021-07-28 |
| PUT | /locations/{locationId} | Update location | bearer | 2021-07-28 |
| DELETE | /locations/{locationId} | Delete location | bearer | 2021-07-28 |
| GET | /locations/{locationId}/timezones | List timezones for location | bearer | 2021-07-28 |
| POST | /locations/{locationId}/tasks/search | Search tasks within location | bearer | 2021-07-28 |
| GET | /locations/{locationId}/templates | List email/sms templates | bearer | 2021-07-28 |
| DELETE | /locations/{locationId}/templates/{id} | Delete template | bearer | 2021-07-28 |

### Location Tags (5 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /locations/{locationId}/tags | List location tags | bearer | 2021-07-28 |
| POST | /locations/{locationId}/tags | Create tag | bearer | 2021-07-28 |
| GET | /locations/{locationId}/tags/{tagId} | Get tag | bearer | 2021-07-28 |
| PUT | /locations/{locationId}/tags/{tagId} | Update tag | bearer | 2021-07-28 |
| DELETE | /locations/{locationId}/tags/{tagId} | Delete tag | bearer | 2021-07-28 |

### Custom Fields v1 (location-scoped) (6 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /locations/{locationId}/customFields | List custom fields | bearer | 2021-07-28 |
| POST | /locations/{locationId}/customFields | Create custom field | bearer | 2021-07-28 |
| GET | /locations/{locationId}/customFields/{id} | Get custom field | bearer | 2021-07-28 |
| PUT | /locations/{locationId}/customFields/{id} | Update custom field | bearer | 2021-07-28 |
| DELETE | /locations/{locationId}/customFields/{id} | Delete custom field | bearer | 2021-07-28 |
| POST | /locations/{locationId}/customFields/upload | Upload custom field file | bearer | 2021-07-28 |

### Custom Fields v2 (object-keyed) (7 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /custom-fields/{id} | Get custom field v2 by id | bearer | 2021-07-28 |
| POST | /custom-fields/ | Create custom field v2 | bearer | 2021-07-28 |
| PUT | /custom-fields/{id} | Update custom field v2 | bearer | 2021-07-28 |
| DELETE | /custom-fields/{id} | Delete custom field v2 | bearer | 2021-07-28 |
| GET | /custom-fields/object-key/{objectKey} | List custom fields by object key | bearer | 2021-07-28 |
| POST | /custom-fields/folder | Create custom field folder | bearer | 2021-07-28 |
| PUT | /custom-fields/folder/{id} | Update custom field folder | bearer | 2021-07-28 |
| DELETE | /custom-fields/folder/{id} | Delete custom field folder | bearer | 2021-07-28 |

### Custom Values (5 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /locations/{locationId}/customValues | List custom values | bearer | 2021-07-28 |
| POST | /locations/{locationId}/customValues | Create custom value | bearer | 2021-07-28 |
| GET | /locations/{locationId}/customValues/{id} | Get custom value | bearer | 2021-07-28 |
| PUT | /locations/{locationId}/customValues/{id} | Update custom value | bearer | 2021-07-28 |
| DELETE | /locations/{locationId}/customValues/{id} | Delete custom value | bearer | 2021-07-28 |

### Custom Objects (8 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /objects/ | List object schemas (locationId required) | bearer | 2021-07-28 |
| POST | /objects/ | Create object schema | bearer | 2021-07-28 |
| GET | /objects/{key} | Get object schema | bearer | 2021-07-28 |
| PUT | /objects/{key} | Update object schema | bearer | 2021-07-28 |
| POST | /objects/{schemaKey}/records | Create object record | bearer | 2021-07-28 |
| GET | /objects/{schemaKey}/records/{id} | Get object record | bearer | 2021-07-28 |
| PUT | /objects/{schemaKey}/records/{id} | Update object record | bearer | 2021-07-28 |
| DELETE | /objects/{schemaKey}/records/{id} | Delete object record | bearer | 2021-07-28 |
| POST | /objects/{schemaKey}/records/search | Search object records | bearer | 2021-07-28 |

### Associations & Relations (9 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /associations/ | List associations (locationId required) | bearer | 2021-07-28 |
| POST | /associations/ | Create association | bearer | 2021-07-28 |
| GET | /associations/{associationId} | Get association by id | bearer | 2021-07-28 |
| PUT | /associations/{associationId} | Update association | bearer | 2021-07-28 |
| DELETE | /associations/{associationId} | Delete association | bearer | 2021-07-28 |
| GET | /associations/key/{keyName} | Get association by key name | bearer | 2021-07-28 |
| GET | /associations/objectKey/{objectKey} | Get associations by object key | bearer | 2021-07-28 |
| POST | /associations/relations | Create relation between records | bearer | 2021-07-28 |
| GET | /associations/relations/{recordId} | Get relations for record | bearer | 2021-07-28 |
| DELETE | /associations/relations/{relationId} | Delete relation | bearer | 2021-07-28 |

### Blogs (7 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /blogs/site/all | List blog sites for location | bearer | 2021-07-28 |
| GET | /blogs/posts/all | List blog posts for a site | bearer | 2021-07-28 |
| POST | /blogs/posts | Create blog post | bearer | 2021-07-28 |
| PUT | /blogs/posts/{postId} | Update blog post | bearer | 2021-07-28 |
| GET | /blogs/posts/url-slug-exists | Check URL slug availability | bearer | 2021-07-28 |
| GET | /blogs/authors | List blog authors (locationId required) | bearer | 2021-07-28 |
| GET | /blogs/categories | List blog categories (locationId required) | bearer | 2021-07-28 |

### Workflows (1 endpoint)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /workflows/ | List workflows (locationId required) | bearer | 2021-07-28 |

(Note: workflow membership writes use `POST/DELETE /contacts/{contactId}/workflow/{workflowId}` — see Contacts section.)

### Surveys (2 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /surveys/ | List surveys (locationId required) | bearer | 2021-07-28 |
| GET | /locations/{locationId}/surveys/submissions | List survey submissions | bearer | 2021-07-28 |

### Email Templates & Campaigns (7 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /emails/schedule | List email campaigns/scheduled emails | bearer | 2021-07-28 |
| GET | /emails/builder | List email templates (includes `?include=html`) | bearer | 2021-07-28 |
| POST | /emails/builder | Create email template | bearer | 2021-07-28 |
| POST | /emails/builder/data | Update template HTML in place (round-trip) | bearer | 2021-07-28 |
| DELETE | /emails/builder/{locationId}/{templateId} | Delete email template | bearer | 2021-07-28 |
| POST | /email/verify | Verify email address (ISV) | bearer | 2021-07-28 |
| POST | /conversations/messages | Send transactional email via conversation (see Conversations) | bearer | 2021-04-15 |

### Media Library (3 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /medias/files | List media files | bearer | 2021-07-28 |
| POST | /medias/upload-file | Upload media file (multipart) | bearer | 2021-07-28 |
| DELETE | /medias/{id} | Delete media file | bearer | 2021-07-28 |

### Social Media Posting (15 endpoints)
All require `locationId` in path, not query.
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| POST | /social-media-posting/{locationId}/posts/list | Search/list social posts | bearer | 2021-07-28 |
| POST | /social-media-posting/{locationId}/posts | Create social post | bearer | 2021-07-28 |
| GET | /social-media-posting/{locationId}/posts/{postId} | Get social post | bearer | 2021-07-28 |
| PUT | /social-media-posting/{locationId}/posts/{postId} | Update social post | bearer | 2021-07-28 |
| DELETE | /social-media-posting/{locationId}/posts/{postId} | Delete social post | bearer | 2021-07-28 |
| POST | /social-media-posting/{locationId}/posts/bulk-delete | Bulk delete social posts | bearer | 2021-07-28 |
| GET | /social-media-posting/{locationId}/accounts | List connected social accounts | bearer | 2021-07-28 |
| DELETE | /social-media-posting/{locationId}/accounts/{accountId} | Disconnect social account | bearer | 2021-07-28 |
| POST | /social-media-posting/{locationId}/csv | Upload CSV of posts | bearer | 2021-07-28 |
| GET | /social-media-posting/{locationId}/csv | List CSV uploads | bearer | 2021-07-28 |
| POST | /social-media-posting/{locationId}/set-accounts | Set accounts for CSV upload | bearer | 2021-07-28 |
| GET | /social-media-posting/{locationId}/csv/{csvId} | Get CSV upload status | bearer | 2021-07-28 |
| DELETE | /social-media-posting/{locationId}/csv/{csvId} | Delete CSV upload | bearer | 2021-07-28 |
| PATCH | /social-media-posting/{locationId}/csv/{csvId} | Edit CSV upload | bearer | 2021-07-28 |
| DELETE | /social-media-posting/{locationId}/csv/{csvId}/post/{postId} | Delete post within CSV upload | bearer | 2021-07-28 |
| GET | /social-media-posting/{locationId}/oauth/{platform}/start | Start OAuth for platform (google/facebook/instagram/linkedin/twitter/tiktok) | bearer | 2021-07-28 |

### Payments — Orders, Transactions, Subscriptions (11 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /payments/orders | List orders | bearer | 2021-07-28 |
| GET | /payments/orders/{orderId} | Get order | bearer | 2021-07-28 |
| POST | /payments/orders/{orderId}/fulfillments | Create order fulfillment | bearer | 2021-07-28 |
| GET | /payments/orders/{orderId}/fulfillments | List fulfillments | bearer | 2021-07-28 |
| GET | /payments/transactions | List transactions | bearer | 2021-07-28 |
| GET | /payments/transactions/{transactionId} | Get transaction | bearer | 2021-07-28 |
| GET | /payments/subscriptions | List subscriptions | bearer | 2021-07-28 |
| GET | /payments/subscriptions/{subscriptionId} | Get subscription | bearer | 2021-07-28 |

### Payments — Coupons (5 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /payments/coupon/list | List coupons | bearer | 2021-07-28 |
| GET | /payments/coupon | Get coupon | bearer | 2021-07-28 |
| POST | /payments/coupon | Create coupon | bearer | 2021-07-28 |
| PUT | /payments/coupon | Update coupon | bearer | 2021-07-28 |
| DELETE | /payments/coupon | Delete coupon | bearer | 2021-07-28 |

### Payments — Integrations & Custom Providers (7 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| POST | /payments/integrations/provider/whitelabel | Create whitelabel integration | bearer | 2021-07-28 |
| GET | /payments/integrations/provider/whitelabel | List whitelabel integrations | bearer | 2021-07-28 |
| POST | /payments/custom-provider/provider | Create custom provider integration | bearer | 2021-07-28 |
| DELETE | /payments/custom-provider/provider | Delete custom provider integration | bearer | 2021-07-28 |
| GET | /payments/custom-provider/connect | Get custom provider config | bearer | 2021-07-28 |
| POST | /payments/custom-provider/connect | Create custom provider config | bearer | 2021-07-28 |
| POST | /payments/custom-provider/disconnect | Disconnect custom provider config | bearer | 2021-07-28 |

### Invoices (Templates, Schedules, Items) (30 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| POST | /invoices/ | Create invoice | bearer | 2021-07-28 |
| GET | /invoices/ | List invoices | bearer | 2021-07-28 |
| GET | /invoices/{invoiceId} | Get invoice | bearer | 2021-07-28 |
| PUT | /invoices/{invoiceId} | Update invoice | bearer | 2021-07-28 |
| DELETE | /invoices/{invoiceId} | Delete invoice | bearer | 2021-07-28 |
| PATCH | /invoices/{invoiceId}/late-fees-configuration | Update invoice late-fee config | bearer | 2021-07-28 |
| POST | /invoices/{invoiceId}/void | Void invoice | bearer | 2021-07-28 |
| POST | /invoices/{invoiceId}/send | Send invoice | bearer | 2021-07-28 |
| POST | /invoices/{invoiceId}/record-payment | Record payment on invoice | bearer | 2021-07-28 |
| PATCH | /invoices/stats/last-visited-at | Mark stats last visited | bearer | 2021-07-28 |
| POST | /invoices/text2pay | Create text-to-pay invoice | bearer | 2021-07-28 |
| GET | /invoices/generate-invoice-number | Generate next invoice number | bearer | 2021-07-28 |
| POST | /invoices/template | Create invoice template | bearer | 2021-07-28 |
| GET | /invoices/template | List invoice templates | bearer | 2021-07-28 |
| GET | /invoices/template/{templateId} | Get invoice template | bearer | 2021-07-28 |
| PUT | /invoices/template/{templateId} | Update invoice template | bearer | 2021-07-28 |
| DELETE | /invoices/template/{templateId} | Delete invoice template | bearer | 2021-07-28 |
| PATCH | /invoices/template/{templateId}/late-fees-configuration | Update template late-fees config | bearer | 2021-07-28 |
| PATCH | /invoices/template/{templateId}/payment-methods-configuration | Update template payment methods | bearer | 2021-07-28 |
| POST | /invoices/schedule | Create invoice schedule (recurring) | bearer | 2021-07-28 |
| GET | /invoices/schedule | List invoice schedules | bearer | 2021-07-28 |
| GET | /invoices/schedule/{scheduleId} | Get invoice schedule | bearer | 2021-07-28 |
| PUT | /invoices/schedule/{scheduleId} | Update invoice schedule | bearer | 2021-07-28 |
| DELETE | /invoices/schedule/{scheduleId} | Delete invoice schedule | bearer | 2021-07-28 |
| POST | /invoices/schedule/{scheduleId}/updateAndSchedule | Update + activate schedule | bearer | 2021-07-28 |
| POST | /invoices/schedule/{scheduleId}/schedule | Activate schedule | bearer | 2021-07-28 |
| POST | /invoices/schedule/{scheduleId}/auto-payment | Configure auto-payment on schedule | bearer | 2021-07-28 |
| POST | /invoices/schedule/{scheduleId}/cancel | Cancel schedule | bearer | 2021-07-28 |

### Estimates (10 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| POST | /invoices/estimate | Create estimate | bearer | 2021-07-28 |
| PUT | /invoices/estimate/{estimateId} | Update estimate | bearer | 2021-07-28 |
| DELETE | /invoices/estimate/{estimateId} | Delete estimate | bearer | 2021-07-28 |
| GET | /invoices/estimate/list | List estimates | bearer | 2021-07-28 |
| GET | /invoices/estimate/number/generate | Generate estimate number | bearer | 2021-07-28 |
| POST | /invoices/estimate/{estimateId}/send | Send estimate | bearer | 2021-07-28 |
| POST | /invoices/estimate/{estimateId}/invoice | Convert estimate to invoice | bearer | 2021-07-28 |
| PATCH | /invoices/estimate/stats/last-visited-at | Mark estimate stats visited | bearer | 2021-07-28 |
| GET | /invoices/estimate/template | List estimate templates | bearer | 2021-07-28 |
| POST | /invoices/estimate/template | Create estimate template | bearer | 2021-07-28 |
| PUT | /invoices/estimate/template/{templateId} | Update estimate template | bearer | 2021-07-28 |
| DELETE | /invoices/estimate/template/{templateId} | Delete estimate template | bearer | 2021-07-28 |
| GET | /invoices/estimate/template/preview | Preview estimate template | bearer | 2021-07-28 |

### Products & Pricing (18 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| POST | /products/ | Create product | bearer | 2021-07-28 |
| GET | /products/ | List products | bearer | 2021-07-28 |
| GET | /products/{productId} | Get product | bearer | 2021-07-28 |
| PUT | /products/{productId} | Update product | bearer | 2021-07-28 |
| DELETE | /products/{productId} | Delete product | bearer | 2021-07-28 |
| POST | /products/bulk-update | Bulk update products | bearer | 2021-07-28 |
| POST | /products/{productId}/price | Create price | bearer | 2021-07-28 |
| GET | /products/{productId}/price | List prices | bearer | 2021-07-28 |
| GET | /products/{productId}/price/{priceId} | Get price | bearer | 2021-07-28 |
| PUT | /products/{productId}/price/{priceId} | Update price | bearer | 2021-07-28 |
| DELETE | /products/{productId}/price/{priceId} | Delete price | bearer | 2021-07-28 |
| GET | /products/inventory | List inventory | bearer | 2021-07-28 |
| POST | /products/inventory | Update inventory | bearer | 2021-07-28 |
| GET | /products/store/{storeId}/stats | Product store stats | bearer | 2021-07-28 |
| POST | /products/store/{storeId} | Update store association | bearer | 2021-07-28 |
| POST | /products/collections | Create collection | bearer | 2021-07-28 |
| PUT | /products/collections/{collectionId} | Update collection | bearer | 2021-07-28 |
| GET | /products/collections/{collectionId} | Get collection | bearer | 2021-07-28 |
| GET | /products/collections | List collections | bearer | 2021-07-28 |
| DELETE | /products/collections/{collectionId} | Delete collection | bearer | 2021-07-28 |

### Product Reviews (5 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| GET | /products/reviews | List reviews | bearer | 2021-07-28 |
| GET | /products/reviews/count | Count reviews | bearer | 2021-07-28 |
| PUT | /products/reviews/{reviewId} | Update review | bearer | 2021-07-28 |
| DELETE | /products/reviews/{reviewId} | Delete review | bearer | 2021-07-28 |
| POST | /products/reviews/bulk-update | Bulk update reviews | bearer | 2021-07-28 |

### Store — Shipping Zones, Rates, Carriers, Settings (15 endpoints)
| Method | Path | Description | Auth | Version |
|--------|------|-------------|------|---------|
| POST | /store/shipping-zone | Create shipping zone | bearer | 2021-07-28 |
| GET | /store/shipping-zone | List shipping zones | bearer | 2021-07-28 |
| GET | /store/shipping-zone/{shippingZoneId} | Get shipping zone | bearer | 2021-07-28 |
| PUT | /store/shipping-zone/{shippingZoneId} | Update shipping zone | bearer | 2021-07-28 |
| DELETE | /store/shipping-zone/{shippingZoneId} | Delete shipping zone | bearer | 2021-07-28 |
| POST | /store/shipping-zone/shipping-rates | Get available shipping rates (estimate) | bearer | 2021-07-28 |
| POST | /store/shipping-zone/{shippingZoneId}/shipping-rate | Create shipping rate in zone | bearer | 2021-07-28 |
| GET | /store/shipping-zone/{shippingZoneId}/shipping-rate | List shipping rates | bearer | 2021-07-28 |
| GET | /store/shipping-zone/{shippingZoneId}/shipping-rate/{shippingRateId} | Get shipping rate | bearer | 2021-07-28 |
| PUT | /store/shipping-zone/{shippingZoneId}/shipping-rate/{shippingRateId} | Update shipping rate | bearer | 2021-07-28 |
| DELETE | /store/shipping-zone/{shippingZoneId}/shipping-rate/{shippingRateId} | Delete shipping rate | bearer | 2021-07-28 |
| POST | /store/shipping-carrier | Create shipping carrier | bearer | 2021-07-28 |
| GET | /store/shipping-carrier | List shipping carriers | bearer | 2021-07-28 |
| GET | /store/shipping-carrier/{shippingCarrierId} | Get shipping carrier | bearer | 2021-07-28 |
| PUT | /store/shipping-carrier/{shippingCarrierId} | Update shipping carrier | bearer | 2021-07-28 |
| DELETE | /store/shipping-carrier/{shippingCarrierId} | Delete shipping carrier | bearer | 2021-07-28 |
| POST | /store/store-setting | Create store setting | bearer | 2021-07-28 |
| GET | /store/store-setting | Get store setting | bearer | 2021-07-28 |

## Endpoints Documented in Stoplight But Not in Local MCP

The published GHL docs (https://highlevel.stoplight.io/docs/integrations/) include additional resource groups the mastanley13 MCP fork has not yet implemented. These should be considered for the CLI's roadmap but treated as second-tier (less battle-tested):

- **Users** — `GET/POST/PUT/DELETE /users/`, `/users/{userId}`, `/users/search`, `/users/{userId}/permissions`
- **Forms** — `GET /forms/`, `GET /forms/submissions`, `POST /forms/upload-custom-files`, `POST /forms/upload-file`
- **Funnels** — `GET /funnels/funnel/list`, `GET /funnels/page`, `GET /funnels/page/count`, `POST /funnels/lookup/redirect`
- **Companies** — `GET /companies/{companyId}` (agency-level)
- **Businesses** — `GET/POST/PUT/DELETE /businesses/`, `/businesses/{businessId}`
- **Memberships / Courses & Communities** — `GET /courses/`, `POST /courses/courses-exporter/public/import`, `GET /courses/courses-exporter/public/export`
- **Snapshots** — `GET /snapshots/`, `POST /snapshots/share/link`, `POST /snapshots/share/{shareId}`
- **OAuth** — `POST /oauth/token`, `GET /oauth/installedLocations`, `POST /oauth/locationToken`
- **SaaS** — `GET /saas-api/public-api/locations`, `PUT /saas-api/public-api/update-saas-subscription/{locationId}`
- **Triggers / Phone System** — present on Stoplight under Phone System; sparse public surface

For these, the recommendation is to drive CLI verbs from the local MCP (well-covered) and stub Users/Forms/Funnels as Phase 2.

## Summary Stats
- Local MCP endpoint definitions extracted: 265 lines, 205 unique URL templates
- Resource groups represented: 23
- Methods: GET / POST / PUT / DELETE / PATCH
- Auth: all `Authorization: Bearer pit-<uuid>`
- Version headers: `2021-07-28` (default) + `2021-04-15` (Conversations only)
- `locationId` requirement: ~70% of endpoints require it as either query param or path segment
