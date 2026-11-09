# Full public API reprint (board #637, 2026-10-30)

Contract: the live public document https://conduyt.app/openapi.yaml, byte-identical to the app's `public/openapi.yaml` at main
d03e3c047 (57,493 lines, sha256 8f78f29a209a5e3d952b2def3f2d0729f13d333c4f201621cb47d30e12bde3b4). `upstream/openapi.yaml` is that
document (`make sync-spec`). `spec.yaml` is the same document plus the one extension the drift gate header names and the live document
does not carry: `x-pp-resource: api-search` on GET /search (sha256 f5d3a38962c0e92737fcf66b69f3bc856f5a4e4478654e06e1b90f38651eb3e2).
The x-mcp block is already in the live document. 948 operations (was 834): 116 added, 2 removed (GET /calendar/callback/google and
/microsoft, which the public document no longer lists).

## How it was printed (repeat this for the next reprint)

1. Printing Press 4.28.0, the version this tree was printed with (`go install github.com/mvanhorn/cli-printing-press/v4/cmd/printing-press@v4.28.0`
   into a scratch GOBIN; it needs go >= 1.26.5, the toolchain switches automatically). Newer presses (4.32.1, 4.34.2) emit different
   helpers and would rewrite every file and fight every 4.28-style patch.
2. `printing-press generate --spec <spec> --output <dir> --name conduyt-crm --owner ptaramona --validate=false --max-endpoints-per-resource 200`
   four times: on spec.yaml at 0ee8aa7 (July), at b260625 (September), at HEAD before the reprint ("base") and on the refreshed spec ("fresh").
   Each print was normalised to this repo (module path `conduyt-crm-pp-cli`, copyright "Paul Taramona").
3. Per file: a file equal to one of the three earlier prints carries no hand edit and takes the fresh print; a file only this tree has is
   hand-written and stays; every other file is a 3-way merge (`git merge-file`) of this tree, the closest earlier print and the fresh print,
   with every conflict resolved by hand against the patch notes. go.mod, go.sum and .printing-press.json are not taken from the print.
   `tools-manifest.json` and `internal/mcp/code_orch.go` were merged structurally (per tool / per endpoint entry plus the non-entry hunks).
4. Found on the way: the September reprint added the files of 100+ commands but `root.go` never registered the new top-level groups
   (call-flows, automation-folders, dashboards, pool-filter-sets, ring-groups, support, me, ...), and `code_orch.go` kept the July endpoint
   list, so those operations were unreachable from the CLI and from MCP code orchestration. Both are registered now.

## Curation carried forward (the live document lacks it; a reprint must keep it)

- `deals list`: `--status` (open, won, lost), `--sort` (createdAt, updatedAt, title, value; default createdAt) and `--order` (asc, desc;
  default desc) keep the curated enums and defaults; the public document now types them as plain strings. The board-snapshot tests pin
  the defaults the snapshot sends.
- `tools-manifest.json`: `deals_list.per_page` (max 1000 plus the snapshot sentence), `dialer_patch-settings.wrapupAutoReleaseSeconds`
  (int), and the other hand parameters and descriptions the per-feature notes list.

## Hand-fixes made during the reprint (pinned by tests)

- `contacts list`: the print paged on the `limit` alias (default 50) next to `--per-page`, so `--per-page 100` sent limit=50&per_page=100,
  which the API refuses (both must match). Pages ride per_page as before; `--limit` is sent only when passed
  (`TestContactsListPagesOnPerPageNotLimit`).
- Nested required flags: see the ledger entry nested-required-flags-637 (`TestNestedRequiredFlagsOnlyWhenTheObjectIsSent`).
- Secret path codes on the two new public paths: sensitivePathParams plus masked envelope paths
  (`TestSpecSensitivePathParametersAreListed`, `TestReprintSecretPathCodeEnvelopesHideTheCode`).
- `email send-bulk` replyTo / sendDomainId patterns (`TestEveryBoundedSendFieldCountsUTF16`).
- `deals list --board-snapshot` refuses every flag it does not forward (`TestDealsBoardSnapshotRefusalsSendNothing`).

## Not carried (spec content no note covers and the live document lacks)

The old spec.yaml carried content the live document now writes differently; it is replaced by the live document, not re-added:
the old curated enums and defaults on many list parameters (contacts list sort/order/smart_view, companies list sort/order, tasks
priority/status defaults, page-size defaults), and request-body properties that the API no longer documents (the full removed-flag list with the reason for each is below).

## Dropped spec content, full list (operation | kind | name | change); source: the pre-reprint spec.yaml (July print plus the September reprint), no patch note covers it

- GET /contacts | param enum | smart_view | ['needs_follow_up', 'hot_leads', 'new_this_week', 'recently_won', 'untagged', 'missing_email', 'missing_phone']
- GET /contacts | param enum | sort | ['createdAt', 'updatedAt', 'firstName', 'lastName', 'email', 'company', 'leadScore']
- GET /contacts | param default | sort | 'createdAt' -> None
- GET /contacts | param enum | order | ['asc', 'desc']
- GET /contacts | param default | order | 'desc' -> None
- PATCH /contacts/{id} | body property | tags | removed
- POST /contacts/{id}/tags | body property | tagId | removed
- POST /contacts/merge | body property | secondaryId | removed
- POST /contact | body property | phone | removed
- POST /contact | body property | companyId | removed
- POST /contact | body property | jobTitle | removed
- POST /contact | body property | source | removed
- POST /contact | body property | addressLine1 | removed
- POST /contact | body property | addressLine2 | removed
- POST /contact | body property | city | removed
- POST /contact | body property | state | removed
- POST /contact | body property | zip | removed
- POST /contact | body property | country | removed
- POST /contact | body property | assignedTo | removed
- POST /contact | body property | tags | removed
- POST /contact | body property | customFields | removed
- GET /companies | param enum | size | ['1-10', '11-50', '51-200', '201-500', '501-1000', '1001+']
- GET /companies | param enum | sort | ['createdAt', 'updatedAt', 'name']
- GET /companies | param default | sort | 'createdAt' -> None
- GET /companies | param enum | order | ['asc', 'desc']
- GET /companies | param default | order | 'desc' -> None
- GET /deals | param enum | status | ['open', 'won', 'lost']
- GET /deals | param enum | sort | ['createdAt', 'updatedAt', 'title', 'value']
- GET /deals | param default | sort | 'createdAt' -> None
- GET /deals | param enum | order | ['asc', 'desc']
- GET /deals | param default | order | 'desc' -> None
- POST /pipelines/{id}/stages | body property | sortOrder | removed
- POST /tags/merge | body property | sourceId | removed
- POST /tags/merge | body property | targetId | removed
- GET /messages | query/path param | contact_id | removed
- GET /messages | param enum | channel | ['sms', 'email']
- GET /messages | param enum | direction | ['inbound', 'outbound']
- GET /emails | query/path param | page | removed
- GET /emails | query/path param | per_page | removed
- GET /emails/templates | param default | per_page | 50 -> None
- PATCH /emails/templates/{id} | body property | channel | removed
- GET /emails/sequences | param default | per_page | 50 -> None
- POST /emails/sequences/{id}/unenroll | body property | contactIds | removed
- GET /automations | param default | per_page | 50 -> None
- POST /automations | body property | isActive | removed
- POST /automations | body property | reenrollmentRule | removed
- POST /automations | body property | cooldownDays | removed
- POST /automations | body property | executionTimezone | removed
- POST /automations | body property | executionWindowStart | removed
- POST /automations | body property | executionWindowEnd | removed
- POST /automations | body property | stopOnResponse | removed
- GET /automations/{id}/step-logs | query/path param | page | removed
- GET /automations/{id}/step-logs | query/path param | per_page | removed
- POST /automations/templates | body property | templateId | removed
- GET /automation-executions | query/path param | page | removed
- GET /automation-executions | query/path param | per_page | removed
- GET /workflows | param default | per_page | 50 -> None
- POST /workflows/{id}/test | body property | contactId | removed
- GET /drip-campaigns | param default | per_page | 50 -> 25
- POST /drip-campaigns | body property | isActive | removed
- GET /drip-enrollments | query/path param | per_page | removed
- GET /custom-fields | query/path param | entity_type | removed
- PATCH /custom-fields/{id} | body property | entityType | removed
- PATCH /custom-fields/{id} | body property | fieldKey | removed
- PATCH /custom-fields/{id} | body property | fieldType | removed
- POST /calendars/{id}/appointments | body property | calendarId | removed
- GET /calendar/events | param default | per_page | 50 -> 100
- POST /calendar/events | body property | calendarConnectionId | removed
- POST /invoices | body property | status | removed
- POST /invoices | body property | issueDate | removed
- GET /products | param default | per_page | 50 -> None
- GET /files | query/path param | entity_type | removed
- GET /files | query/path param | entity_id | removed
- POST /files | body property | fileName | removed
- POST /files | body property | fileUrl | removed
- POST /files | body property | fileSize | removed
- POST /files | body property | contentType | removed
- PATCH /users/{id} | body property | isActive | removed
- PATCH /settings | body property | name | removed
- PATCH /settings | body property | domain | removed
- PATCH /settings | body property | settings | removed
- POST /settings/sms/test | body property | toNumber | removed
- PUT /settings/twilio | body property | twilioAccountSid | removed
- PUT /settings/twilio | body property | twilioAuthToken | removed
- PUT /settings/twilio | body property | twilioPhoneNumber | removed
- PUT /settings/twilio | body property | twilioTwimlAppSid | removed
- PUT /settings/twilio | body property | twilioApiKey | removed
- PUT /settings/twilio | body property | twilioApiSecret | removed
- POST /billing/checkout | body property | priceId | removed
- GET /webhooks/manage | param default | per_page | 50 -> None
- GET /webhooks/manage/{id}/deliveries | param default | per_page | 50 -> 25
- GET /webhooks/replay | param default | per_page | 50 -> 25
- GET /webhook-logs | query/path param | per_page | removed
- POST /reports/custom | body property | isPinned | removed
- POST /bulk/contacts/tag | body property | ids | removed
- POST /bulk/contacts/tag | body property | addTags | removed
- POST /bulk/contacts/tag | body property | removeTags | removed
- POST /bulk/contacts/update | body property | ids | removed
- POST /bulk/contacts/update | body property | data | removed
- GET /imports | param default | per_page | 50 -> 20
- POST /dialer/call | body property | toNumber | removed
- POST /calls | body property | recordingUrl | removed
- GET /chat/channels/{id}/messages | query/path param | page | removed
- GET /chat/channels/{id}/messages | query/path param | per_page | removed
- POST /chat/channels/{id}/members | body property | userId | removed
- POST /chat/channels/{id}/members | body property | role | removed
- DELETE /chat/channels/{id}/members | body property | userId | removed
- POST /ai/chat | body property | message | removed
- POST /ai/compose-email | body property | prompt | removed
- POST /ai/compose-email | body property | contactId | removed
- POST /ai/improve-email | body property | instruction | removed
- POST /ai/enrich-contact | body property | contactId | removed
- GET /calendar/callback/google | operation | - | removed
- GET /calendar/callback/microsoft | operation | - | removed

## Command surface change (agent-context tree, before -> after)

1003 -> 1186 commands: 185 added, 2 removed (`calendar google-callback`, `calendar microsoft-callback`: the public document no longer lists the OAuth callbacks). On commands that exist on both sides: 2556 flags added, 106 removed, 7 changed type (`--per-page` int -> string on automations list, emails list-sequences, emails list-templates, products list, webhooks list, workflows list; notifications list `--is-read` bool -> string), 18 changed default (page-size defaults now the documented ones; contacts list and companies list no longer default `--sort createdAt --order desc`; tasks create/update no longer default `--priority medium --status todo`, which every `tasks update` used to send). No alias was added. Round 2 (reprint-637-freeform-json-flags.md) restored every "generator artifact" flag below (16 flags on 9 commands), so 90 removals remain, all upstream body or parameter changes. After round 2 the totals are 2554 flags added (--cf-key is gone), 90 removed, 8 type changes (plus `contacts list --tag` string -> repeatable stringArray), 18 default changes. Removed flags ("generator artifact" = the property is still documented but the print now expands an object with declared sub-properties into sub-flags, or falls back to `--body-json` for an anyOf body):

- activities create-activity --metadata: generator artifact: body property metadata still documented ({"type": "object", "additionalProperties": {}, "properties": {"cancelledBy": {"description": "Read by the handler from t)
- ai chat --message: upstream: body property dropped (new body keys: context,messages,section)
- ai compose-email --contact-id: upstream: body property dropped (new body keys: companyName,contactCompany,contactEmail,contactName,contactTitle,dealStage,dealValue,previousMessages,purpose,senderName,senderTitle,tone)
- ai compose-email --prompt: upstream: body property dropped (new body keys: companyName,contactCompany,contactEmail,contactName,contactTitle,dealStage,dealValue,previousMessages,purpose,senderName,senderTitle,tone)
- ai enrich-contact --contact-id: upstream: body property dropped (new body keys: company,email,firstName,jobTitle,lastName,phone)
- ai improve-email --instruction: upstream: body property dropped (new body keys: body,instructions,subject)
- automation-executions list --page: upstream: query param no longer documented on this operation
- automation-executions list --per-page: upstream: query param no longer documented on this operation
- automations create --cooldown-days: upstream: body property dropped (new body keys: actions,description,folderId,graph,graphVersion,kind,n8nWebhookUrl,n8nWorkflowId,name,nodes,schedule,scheduleTimezone,startNodeId,trigger,triggerCondi)
- automations create --execution-timezone: upstream: body property dropped (new body keys: actions,description,folderId,graph,graphVersion,kind,n8nWebhookUrl,n8nWorkflowId,name,nodes,schedule,scheduleTimezone,startNodeId,trigger,triggerCondi)
- automations create --execution-window-end: upstream: body property dropped (new body keys: actions,description,folderId,graph,graphVersion,kind,n8nWebhookUrl,n8nWorkflowId,name,nodes,schedule,scheduleTimezone,startNodeId,trigger,triggerCondi)
- automations create --execution-window-start: upstream: body property dropped (new body keys: actions,description,folderId,graph,graphVersion,kind,n8nWebhookUrl,n8nWorkflowId,name,nodes,schedule,scheduleTimezone,startNodeId,trigger,triggerCondi)
- automations create --is-active: upstream: body property dropped (new body keys: actions,description,folderId,graph,graphVersion,kind,n8nWebhookUrl,n8nWorkflowId,name,nodes,schedule,scheduleTimezone,startNodeId,trigger,triggerCondi)
- automations create --reenrollment-rule: upstream: body property dropped (new body keys: actions,description,folderId,graph,graphVersion,kind,n8nWebhookUrl,n8nWorkflowId,name,nodes,schedule,scheduleTimezone,startNodeId,trigger,triggerCondi)
- automations create --stop-on-response: upstream: body property dropped (new body keys: actions,description,folderId,graph,graphVersion,kind,n8nWebhookUrl,n8nWorkflowId,name,nodes,schedule,scheduleTimezone,startNodeId,trigger,triggerCondi)
- automations create-from-template --template-id: upstream: body property dropped (new body keys: )
- automations step-logs get-automation --page: upstream: query param no longer documented on this operation
- automations step-logs get-automation --per-page: upstream: query param no longer documented on this operation
- automations update --trigger-conditions: generator artifact: body property triggerConditions still documented ({"type": "object", "properties": {"requireSignature": {"description": "Read by the handler from the JSON body (the route)
- billing create-checkout-session --price-id: upstream: body property dropped (new body keys: interval,planId)
- bulk edit-contacts --updates: generator artifact: body property updates still documented ({"type": "object", "description": "The fields to set on every listed contact. A non-empty `country` may be changed; it c)
- bulk edit-deals --updates: generator artifact: body property updates still documented ({"type": "object", "properties": {"assignedTo": {"description": "Read by the handler from the JSON body (the route decla)
- bulk tag-contacts --add-tags: upstream: body property dropped (new body keys: contactIds,tagId)
- bulk tag-contacts --ids: upstream: body property dropped (new body keys: contactIds,tagId)
- bulk tag-contacts --remove-tags: upstream: body property dropped (new body keys: contactIds,tagId)
- bulk update-contacts --data: upstream: body property dropped (new body keys: updates)
- bulk update-contacts --ids: upstream: body property dropped (new body keys: updates)
- calendar create-event --calendar-connection-id: upstream: body property dropped (new body keys: allDay,attendees,connectionId,description,endAt,location,startAt,title)
- calendars appointments create-calendar --calendar-id: upstream: body property dropped (new body keys: assignedTo,contactId,description,endTime,location,metadata,startTime,status,title)
- calls create-record --recording-url: upstream: body property dropped (new body keys: contactId,direction,disposition,duration,fromNumber,notes,source,status,toNumber)
- chat add-member --role: upstream: body property dropped (new body keys: userIds)
- chat add-member --user-id: upstream: body property dropped (new body keys: userIds)
- chat list-messages --page: upstream: query param no longer documented on this operation
- chat list-messages --per-page: upstream: query param no longer documented on this operation
- chat remove-member --stdin: upstream: request body removed or no longer JSON (content types now [])
- contact --address-line1: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contact --address-line2: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contact --assigned-to: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contact --city: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contact --company-id: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contact --country: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contact --custom-fields: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contact --job-title: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contact --phone: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contact --source: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contact --state: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contact --tags: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contact --zip: upstream: body property dropped (new body keys: company,elapsedMs,email,firstName,intent,lastName,message,teamSize,website)
- contacts create --custom-fields: generator artifact: body property customFields still documented ({"type": "object", "additionalProperties": {}, "properties": {"linkedin": {"description": "Read by the handler from the )
- contacts import --file: upstream: body property dropped (new body keys: )
- contacts merge --secondary-id: upstream: body property dropped (new body keys: confirmImpact,dryRun,expectedCounts,expectedGuardCounts,expectedIdentifiers,partial,primaryId,secondaryIds)
- contacts tags add-contact --tag-id: upstream: body property dropped (new body keys: tagIds)
- contacts update --tags: upstream: body property dropped (new body keys: addressLine1,addressLine2,assignedTo,avatarUrl,city,company,companyId,country,customFields,customFieldsPutIfEmpty,doNotContact,doNotContactReason,emai)
- custom-fields update --entity-type: upstream: body property dropped (new body keys: confirmRequired,isRequired,label,options,section,sortOrder)
- custom-fields update --field-key: upstream: body property dropped (new body keys: confirmRequired,isRequired,label,options,section,sortOrder)
- custom-fields update --field-type: upstream: body property dropped (new body keys: confirmRequired,isRequired,label,options,section,sortOrder)
- dialer initiate-call --to-number: upstream: body property dropped (new body keys: autoDialer,callerIdStrategy,contactId,dncOverride,to,vendorBridgeNonce,vendorBridgeable)
- drip-campaigns create --is-active: upstream: body property dropped (new body keys: baseUrl,batchSize,name,provider,sendDays,sendEndHour,sendStartHour,smsProviderId,stepDelays,tagPrefix,timezone,totalSteps,urlSegment)
- drip-enrollments list --per-page: upstream: query param no longer documented on this operation
- emails create-template --body: generator artifact: body property body still documented ({"type": "string", "description": "Optional; defaults to the HTML's visible text."})
- emails create-template --body-html: generator artifact: body property bodyHtml still documented ({"type": "string", "description": "Optional; only its builder markers (blocks, metadata) are read and kept."})
- emails create-template --category: generator artifact: body property category still documented ({"type": "string"})
- emails create-template --channel: generator artifact: body property channel still documented ({"type": "string", "enum": ["email"]})
- emails create-template --is-active: generator artifact: body property isActive still documented ({"type": "boolean", "default": true})
- emails create-template --name: generator artifact: body property name still documented ({"type": "string"})
- emails create-template --subject: generator artifact: body property subject still documented ({"type": "string"})
- emails list --all: upstream: query param no longer documented on this operation
- emails list --page: upstream: query param no longer documented on this operation
- emails list --per-page: upstream: query param no longer documented on this operation
- emails unenroll-from-sequence --contact-ids: upstream: body property dropped (new body keys: contactId)
- emails update-template --channel: upstream: body property dropped (new body keys: body,bodyHtml,bodyMode,category,html,isActive,name,provider,subject)
- files create-record --content-type: upstream: body property dropped (new body keys: entityId,entityType,uploadId)
- files create-record --file-name: upstream: body property dropped (new body keys: entityId,entityType,uploadId)
- files create-record --file-size: upstream: body property dropped (new body keys: entityId,entityType,uploadId)
- files create-record --file-url: upstream: body property dropped (new body keys: entityId,entityType,uploadId)
- forms create --settings: generator artifact: body property settings still documented ({"type": "object", "properties": {"consent": {"type": "object", "properties": {"linkUrl": {"description": "Read by the h)
- imports create --body-json: upstream: the POST /imports body is no longer an anyOf, so the print emits typed flags (--object-type required) instead of the --body-json fallback
- integrations connect --config: generator artifact: body property config still documented ({"type": "object", "properties": {"channel": {"description": "Read by the handler from the JSON body (the route declares)
- integrations connect --credentials: generator artifact: body property credentials still documented ({"type": "object", "properties": {"apiKey": {"description": "Read by the handler from the JSON body (the route declares )
- invoices create --issue-date: upstream: body property dropped (new body keys: companyId,contactId,currency,dealId,dueDate,invoiceNumber,items,notes,taxRate,terms)
- invoices create --status: upstream: body property dropped (new body keys: companyId,contactId,currency,dealId,dueDate,invoiceNumber,items,notes,taxRate,terms)
- pipelines stages create-pipeline --sort-order: upstream: body property dropped (new body keys: color,isLost,isSystem,isWon,name)
- reports create-custom --is-pinned: upstream: body property dropped (new body keys: aggregate,chartType,columns,description,entity,filters,groupBy,groupBy2,isPrivate,isShared,measureField,name,pivot,population,sortBy,sortDir,timeGrain)
- scoring-rules create --condition: generator artifact: body property condition still documented ({"type": "object", "description": "Rule condition: { field, operator, value }", "properties": {"field": {"description": )
- settings test-sms --to-number: upstream: body property dropped (new body keys: to)
- settings update --domain: upstream: body property dropped (new body keys: agentStatuses,aiDisposition,appointmentOutcomeStages,autoEnrichment,bookingCreatesOwnCard,bookingMessageGuard,callDispositions,callingWindow,companyNa)
- settings update --name: upstream: body property dropped (new body keys: agentStatuses,aiDisposition,appointmentOutcomeStages,autoEnrichment,bookingCreatesOwnCard,bookingMessageGuard,callDispositions,callingWindow,companyNa)
- settings update --settings: upstream: body property dropped (new body keys: agentStatuses,aiDisposition,appointmentOutcomeStages,autoEnrichment,bookingCreatesOwnCard,bookingMessageGuard,callDispositions,callingWindow,companyNa)
- settings update-twilio --twilio-account-sid: upstream: body property dropped (new body keys: accountSid,apiKey,apiSecret,authToken,lineTypeVerificationEnabled,phoneNumber,skipLandlinesOnSms,twimlAppSid,voicemailGreeting)
- settings update-twilio --twilio-api-key: upstream: body property dropped (new body keys: accountSid,apiKey,apiSecret,authToken,lineTypeVerificationEnabled,phoneNumber,skipLandlinesOnSms,twimlAppSid,voicemailGreeting)
- settings update-twilio --twilio-api-secret: upstream: body property dropped (new body keys: accountSid,apiKey,apiSecret,authToken,lineTypeVerificationEnabled,phoneNumber,skipLandlinesOnSms,twimlAppSid,voicemailGreeting)
- settings update-twilio --twilio-auth-token: upstream: body property dropped (new body keys: accountSid,apiKey,apiSecret,authToken,lineTypeVerificationEnabled,phoneNumber,skipLandlinesOnSms,twimlAppSid,voicemailGreeting)
- settings update-twilio --twilio-phone-number: upstream: body property dropped (new body keys: accountSid,apiKey,apiSecret,authToken,lineTypeVerificationEnabled,phoneNumber,skipLandlinesOnSms,twimlAppSid,voicemailGreeting)
- settings update-twilio --twilio-twiml-app-sid: upstream: body property dropped (new body keys: accountSid,apiKey,apiSecret,authToken,lineTypeVerificationEnabled,phoneNumber,skipLandlinesOnSms,twimlAppSid,voicemailGreeting)
- tags merge --source-id: upstream: body property dropped (new body keys: sourceTagId,targetTagId)
- tags merge --target-id: upstream: body property dropped (new body keys: sourceTagId,targetTagId)
- users avatar post-users-id --stdin: upstream: request body removed or no longer JSON (content types now ['multipart/form-data'])
- users update --is-active: upstream: body property dropped (new body keys: avatarUrl,avatar_url,custom1,custom2,custom3,custom4,custom_1,custom_2,custom_3,custom_4,did,email,emailSenderLocal,emailSenderName,email_sender_local)
- webhook-logs --per-page: upstream: query param no longer documented on this operation
- webhooks sms-inbound --stdin: upstream: request body removed or no longer JSON (content types now ['application/x-www-form-urlencoded', 'multipart/form-data'])
- webhooks sms-status --stdin: upstream: request body removed or no longer JSON (content types now ['application/x-www-form-urlencoded', 'multipart/form-data'])
- webhooks voice-inbound --stdin: upstream: request body removed or no longer JSON (content types now ['application/x-www-form-urlencoded', 'multipart/form-data'])
- webhooks voice-recording --stdin: upstream: request body removed or no longer JSON (content types now ['application/x-www-form-urlencoded', 'multipart/form-data'])
- webhooks voice-status --stdin: upstream: request body removed or no longer JSON (content types now ['application/x-www-form-urlencoded', 'multipart/form-data'])
- webhooks voice-voicemail --stdin: upstream: request body removed or no longer JSON (content types now ['application/x-www-form-urlencoded', 'multipart/form-data'])
- workflows test workflow --contact-id: upstream: body property dropped (new body keys: sampleData)


## Round 4 (Codex reprint637-r1): reachability and duplicate commands

- `users invite`: the press names both POST /users/invite and the invite sub-resource group (cancel, resend) `users invite` and kept
  only the group, so the endpoint was never reachable from the CLI (true since the July print). The endpoint command keeps the press's
  own name and carries the two children: `users invite --email <e> --role admin|member [--first-name --last-name --phone --extension
  --permissions | --stdin]` runs POST /users/invite; `users invite post-users-id-cancel <id>` / `post-users-id-resend <id>` are
  unchanged; bare `users invite` prints help (listing both children). internal/cli/users_invite.go, `// PATCH(reprint-637-users-invite)`.
- Duplicate AddCommand removed: `contacts duplicate-proposals` (contacts.go; the second registration came from the reprint merge) and
  `users invite` (users.go).
- Duplicate alias removed: the generated `email post-opt-out-list` carried the alias `create`, which `email send` has had since before
  the reprint; cobra resolved `email create` to the new command. The generated alias is dropped.
- Pinned by `internal/cli/command_tree_test.go`: `TestEverySpecOperationHasExactlyOneCommand` (943/943 non-OPTIONS operations, each
  through exactly one command annotation; 942/943 without the users invite fix) and `TestSiblingCommandNamesAreUnique` (names and
  aliases unique at every level of the tree).

## Refresh 2026-11-08 (reprint637 second pass): app public/openapi.yaml at 86cb86329

Contract: the app's `public/openapi.yaml` at batch17 tip 86cb86329 (62,755 lines, sha256
f4ac1e3b2c4519f9d2a01d2f51d035f6ab856ec20739fa8d63511b64cd2e5cac) is `upstream/openapi.yaml`; `spec.yaml` is the same document plus
`x-pp-resource: api-search` on GET /search (sha256 9e6eb974c2d72cfe15c8869deede425768cabd7a44efde2e295e8eaeba6ef824). 974 operations
(was 948): 26 added (account exports, automation distribution, Grok Bot connect/webhook and agent MCP/knowledge routes, chat thread
follow/files/thread/linked-record, contact phone reachability, dialer status-log timeline, imports restore, email opt-out-list check,
activities assignment reasons, settings ai-local-files), none removed.

Printed with Printing Press 4.28.0 twice (`--validate=false --max-endpoints-per-resource 200`): "base" on spec.yaml at fabf31d and "fresh"
on the refreshed spec, both normalised (module path, copyright). A file the base print equals takes the fresh print (33), a file only
the fresh print has is added (32), every other file the fresh print changed is a `git merge-file` of tree, base and fresh (9 clean,
3 conflicts), and 2 files the press renamed are removed (`promoted_account-exports.go`, now the `account-exports` group with get/post;
`contacts_dnc_get-contacts-id.go`, now `contacts_dnc_get-contact-verdicts.go`: the upstream operationId changed, so the command `contacts dnc get-contacts-id` is now `contacts dnc get-contact-verdicts`, same GET /contacts/{id}/dnc; and the bare `account-exports` list command is now `account-exports get`; no alias was added, as in the first reprint). Conflicts: README.md and SKILL.md keep
the curated tree text; `calls link call-to-contact` re-emitted the body dryRun as a local `--dry-run` (re-applied as `--server-dry-run`,
the new `--replace-slot` kept). Every `PATCH(<id>)` marker count under internal/ and cmd/ is identical before and after (12 ids).
Tooling and logs: Cerebro `domains/conduyt/projects/session-tools-2026-10-08/briefs/reprint637-logs/` (merge.sh, merge-report.txt).

## Post-print step (2026-11-08, Codex reprint637-r3)

After the 3-way merge, run `perl .printing-press-patches/required-on-changed.pl internal/cli/*.go` from the repo root (ledger entry
reprint-637-required-on-changed): every required scalar body flag is serialized on Changed(). TestEveryRequiredBodyFlagSerializesItsZeroValue
fails until it has run.
