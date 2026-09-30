#!/usr/bin/env python3
"""Build the merged Postmark OpenAPI 3.0 spec from the two official Swagger 2.0
specs plus the docs inventory (research/postmark-docs-inventory.md).

Output: postmark-merged.openapi.yaml next to this script.
"""
import copy
import os
import sys

import yaml

HERE = os.path.dirname(os.path.abspath(__file__))
SERVER = yaml.safe_load(open(os.path.join(HERE, "postmark-server.official.yml")))
ACCOUNT = yaml.safe_load(open(os.path.join(HERE, "postmark-account.official.yml")))

TOKEN_HEADERS = {"x-postmark-server-token", "x-postmark-account-token", "accept", "content-type"}
ACCOUNT_PATH_PREFIXES = ("/servers", "/domains", "/senders", "/templates/push", "/data-removals")


def fix_refs(node):
    if isinstance(node, dict):
        out = {}
        for k, v in node.items():
            if k == "$ref" and isinstance(v, str):
                out[k] = v.replace("#/definitions/", "#/components/schemas/")
            elif k == "x-nullable":
                out["nullable"] = v
            else:
                out[k] = fix_refs(v)
        return out
    if isinstance(node, list):
        return [fix_refs(x) for x in node]
    return node


def conv_param(p):
    schema = {}
    for key in ("type", "format", "enum", "default", "maximum", "minimum", "items"):
        if key in p:
            schema[key] = copy.deepcopy(p[key])
    out = {"name": p["name"], "in": p["in"], "required": bool(p.get("required", p["in"] == "path"))}
    if p.get("description"):
        out["description"] = p["description"]
    out["schema"] = fix_refs(schema or {"type": "string"})
    return out


def conv_op(op):
    new = {k: copy.deepcopy(v) for k, v in op.items() if k not in ("parameters", "responses", "consumes", "produces")}
    params, body = [], None
    for p in op.get("parameters", []):
        if p.get("in") == "header" and p["name"].lower() in TOKEN_HEADERS:
            continue
        if p.get("in") == "body":
            body = p
            continue
        params.append(conv_param(p))
    if params:
        new["parameters"] = params
    if body is not None:
        new["requestBody"] = {
            "required": bool(body.get("required", True)),
            "content": {"application/json": {"schema": fix_refs(body["schema"])}},
        }
    responses = {}
    for code, r in (op.get("responses") or {}).items():
        code = str(code)
        if isinstance(r, dict) and "$ref" in r:
            r = SERVER["responses"][int(r["$ref"].rsplit("/", 1)[1])]
        rr = {"description": (r or {}).get("description") or "Response"}
        if r and r.get("schema"):
            rr["content"] = {"application/json": {"schema": fix_refs(r["schema"])}}
        responses[code] = rr
    new["responses"] = responses or {"200": {"description": "OK"}}
    return new


def ok(schema_ref=None, schema=None, desc="OK"):
    body = {"description": desc}
    s = {"$ref": "#/components/schemas/" + schema_ref} if schema_ref else schema
    if s:
        body["content"] = {"application/json": {"schema": s}}
    return {"200": body, "422": {"description": "Postmark API error", "content": {"application/json": {"schema": {"$ref": "#/components/schemas/StandardPostmarkResponse"}}}}}


def qp(name, desc, typ="string", required=False, enum=None, default=None):
    schema = {"type": typ}
    if enum:
        schema["enum"] = enum
    if default is not None:
        schema["default"] = default
    return {"name": name, "in": "query", "required": required, "description": desc, "schema": schema}


def pp(name, desc, typ="string"):
    return {"name": name, "in": "path", "required": True, "description": desc, "schema": {"type": typ}}


def body(schema_ref=None, schema=None, required=True):
    s = {"$ref": "#/components/schemas/" + schema_ref} if schema_ref else schema
    return {"required": required, "content": {"application/json": {"schema": s}}}


paths = {}
schemas = {}
for src in (SERVER, ACCOUNT):
    for name, d in src["definitions"].items():
        schemas[name] = fix_refs(d)
    for p, ops in src["paths"].items():
        item = paths.setdefault(p, {})
        for m, op in ops.items():
            item[m] = conv_op(op)

S = schemas

# ---------------------------------------------------------------- drift fixes
def add_props(name, props):
    S[name].setdefault("properties", {}).update(props)


META = {"type": "object", "additionalProperties": {"type": "string"}, "description": "Custom key/value metadata stored with the message (searchable via metadata_<key>)."}
STREAM = {"type": "string", "description": "Message stream ID to send through. Defaults to the server's transactional stream (outbound)."}
TRACKLINKS = {"type": "string", "enum": ["None", "HtmlAndText", "HtmlOnly", "TextOnly"]}

for n in ("SendEmailRequest", "EmailWithTemplateRequest"):
    add_props(n, {"MessageStream": copy.deepcopy(STREAM), "Metadata": copy.deepcopy(META)})
    if "TrackLinks" in S[n].get("properties", {}):
        S[n]["properties"]["TrackLinks"] = copy.deepcopy(TRACKLINKS)
S["SendEmailRequest"]["required"] = ["From", "To"]
S["EmailWithTemplateRequest"]["required"] = ["From", "To", "TemplateModel"]
S["EmailWithTemplateRequest"]["description"] = "Provide TemplateId or TemplateAlias."

bounce_types = paths["/bounces"]["get"]["parameters"]
for prm in bounce_types:
    if prm["name"] == "type":
        prm["schema"]["enum"] = ["ChallengeVerification" if v == "MailFrontier Matador." else v for v in prm["schema"]["enum"]]
    if prm["name"] in ("todate", "fromdate"):
        prm["schema"].pop("format", None)
        prm["description"] = prm["description"].replace("`2014-02-01`", "`2014-02-01` or `2014-02-01T10:00:00` (Eastern Time)")

MS_DESC_STATS = "Filter by message stream ID. Omit for all streams."
MS_DESC_MSGS = "Filter by message stream ID. Defaults to the transactional stream (outbound)."
for p in ("/bounces", "/messages/outbound", "/messages/outbound/opens", "/messages/outbound/clicks"):
    paths[p]["get"].setdefault("parameters", []).append(qp("messagestream", MS_DESC_MSGS))
for p in list(paths):
    if p.startswith("/stats/outbound"):
        paths[p]["get"].setdefault("parameters", []).append(qp("messagestream", MS_DESC_STATS))
for p in ("/messages/outbound", "/messages/inbound"):
    for prm in paths[p]["get"].get("parameters", []):
        if prm["name"] in ("todate", "fromdate"):
            prm["schema"].pop("format", None)
paths["/messages/outbound"]["get"]["parameters"].append(qp("subject", "Filter by email subject"))
for prm in paths["/messages/outbound"]["get"]["parameters"]:
    if prm["name"] == "status":
        prm["schema"]["enum"] = ["queued", "sent", "processed"]
for prm in paths["/messages/outbound/opens/{messageid}"]["get"].get("parameters", []):
    if prm["name"] == "count":
        prm["schema"].pop("default", None)

for n in ("OutboundMessageDetail", "OutboundMessageDetailsResponse"):
    add_props(n, {"MessageStream": {"type": "string"}, "Metadata": copy.deepcopy(META), "Sandboxed": {"type": "boolean"}})
add_props("BounceInfoResponse", {"RecordType": {"type": "string"}, "ServerID": {"type": "integer"}, "MessageStream": {"type": "string"}, "From": {"type": "string"}})
S["BounceInfoResponse"]["properties"]["ID"] = {"type": "integer"}
for n in ("ExtendedMessageOpenEventInformation", "ExtendedMessageClickEventInformation"):
    add_props(n, {"RecordType": {"type": "string"}, "MessageStream": {"type": "string"}})

# Templates
lst = S["TemplateListingResponse"]["properties"]
lst["Templates"] = lst.pop("Templates API")
add_props("TemplateRecordResponse", {"TemplateType": {"type": "string", "enum": ["Standard", "Layout"]}, "LayoutTemplate": {"type": "string", "nullable": True}})
add_props("TemplateDetailResponse", {"TemplateType": {"type": "string", "enum": ["Standard", "Layout"]}, "LayoutTemplate": {"type": "string", "nullable": True}})
S["TemplateDetailResponse"]["properties"]["TemplateId"] = S["TemplateDetailResponse"]["properties"].pop("TemplateID")
paths["/templates"]["get"]["parameters"] += [
    qp("TemplateType", "Filter by template type", enum=["All", "Standard", "Layout"], default="All"),
    qp("LayoutTemplate", "Filter templates that use this layout alias"),
]
paths["/templates"]["get"]["x-resource-id"] = "TemplateId"
add_props("CreateTemplateRequest", {"TemplateType": {"type": "string", "enum": ["Standard", "Layout"], "default": "Standard", "description": "Immutable after create."}, "LayoutTemplate": {"type": "string", "description": "Alias of the layout to use. Empty string clears it."}})
S["CreateTemplateRequest"]["required"] = ["Name"]
add_props("EditTemplateRequest", {"LayoutTemplate": {"type": "string", "description": "Alias of the layout to use. Empty string clears it."}})
S["EditTemplateRequest"].pop("required", None)
add_props("TemplateValidationRequest", {"TemplateType": {"type": "string", "enum": ["Standard", "Layout"]}, "LayoutTemplate": {"type": "string"}})
paths["/templates"]["post"]["operationId"] = "createTemplate"
paths["/templates/{templateIdOrAlias}"]["delete"]["responses"] = ok("StandardPostmarkResponse")
# Validation renders content and has no side effects despite the POST verb.
paths["/templates/validate"]["post"]["x-pp-mutation"] = False
push = S["TemplatesPushModel"]["properties"]
push["SourceServerID"] = push.pop("SourceServerId")
push["DestinationServerID"] = push.pop("DestinationServerId")
S["TemplatesPushModel"]["required"] = ["SourceServerID", "DestinationServerID", "PerformChanges"]
paths["/bounces/{bounceid}/dump"]["get"]["operationId"] = "getBounceDump"

# Stats overview
ov = S["OutboundOverviewStatsResponse"]["properties"]
if "SMTPAPIErrors" in ov:
    ov["SMTPApiErrors"] = ov.pop("SMTPAPIErrors")
for k in ("BounceRate", "SpamComplaintsRate"):
    if k in ov:
        ov[k] = {"type": "number"}

# Servers
SERVER_EXTRA = {
    "DeliveryType": {"type": "string", "enum": ["Live", "Sandbox"], "description": "Set at create time only."},
    "IncludeBounceContentInHook": {"type": "boolean"},
    "EnableSmtpApiErrorHooks": {"type": "boolean"},
}
COLOR = {"type": "string", "description": "Server color: Purple, Blue, Turquoise, Green, Red, Yellow, Grey, or Orange."}
for n in ("ExtendedServerInfo", "CreateServerPayload", "EditServerPayload", "ServerConfigurationResponse", "EditServerConfigurationRequest"):
    props = S[n].setdefault("properties", {})
    if "TrackLinks" in props:
        props["TrackLinks"] = copy.deepcopy(TRACKLINKS)
    if "Color" in props:
        props["Color"] = copy.deepcopy(COLOR)
    extra = copy.deepcopy(SERVER_EXTRA)
    if n in ("EditServerPayload", "EditServerConfigurationRequest"):
        extra.pop("DeliveryType")
    props.update(extra)
S["CreateServerPayload"]["required"] = ["Name"]
for prm in paths["/servers"]["get"].get("parameters", []):
    if prm["name"] == "name":
        prm["description"] = "Filter servers whose name contains this text."
paths["/servers/{serverid}"]["delete"]["responses"] = ok("StandardPostmarkResponse")

# Domains and senders
for n in ("SenderSignatureExtendedInformation", "DomainExtendedInformation", "DKIMRotationResponse"):
    props = S[n].get("properties", {})
    if "DKIMTestValue" in props:
        props["DKIMTextValue"] = props.pop("DKIMTestValue")
for n in ("SenderSignatureCreationModel", "SenderSignatureEditingModel", "SenderSignatureExtendedInformation"):
    add_props(n, {"ConfirmationPersonalNote": {"type": "string", "maxLength": 400}})
S["SenderSignatureCreationModel"]["required"] = ["FromEmail", "Name"]
S["DomainCreationModel"]["required"] = ["Name"]
for p, m in (("/senders/{signatureid}/verifyspf", "post"), ("/senders/{signatureid}/requestnewdkim", "post"), ("/domains/{domainid}/verifyspf", "post")):
    paths[p][m]["deprecated"] = True
    paths[p][m]["summary"] = paths[p][m].get("summary", "") + " (deprecated by Postmark)"

# ------------------------------------------------------ doc-only operations
S["BulkRequestStatus"] = {"type": "object", "properties": {
    "Id": {"type": "string"}, "Status": {"type": "string", "enum": ["Accepted", "Processing", "Completed", "Cancelled"]},
    "SubmittedAt": {"type": "string"}, "TotalMessages": {"type": "integer"}, "PercentageCompleted": {"type": "number"},
    "Subject": {"type": "string"}, "ReleasedCount": {"type": "integer"}, "FailedCount": {"type": "integer"}}}
S["BulkEmailMessage"] = {"type": "object", "properties": {
    "To": {"type": "string"}, "Cc": {"type": "string"}, "Bcc": {"type": "string"},
    "TemplateModel": {"type": "object"}, "Metadata": copy.deepcopy(META), "Headers": {"$ref": "#/components/schemas/HeaderCollection"}}}
S["BulkEmailRequest"] = {"type": "object", "required": ["From", "Messages"], "properties": {
    "From": {"type": "string"}, "ReplyTo": {"type": "string"}, "Subject": {"type": "string"},
    "HtmlBody": {"type": "string"}, "TextBody": {"type": "string"}, "TemplateId": {"type": "integer"},
    "TemplateAlias": {"type": "string"}, "InlineCss": {"type": "boolean"}, "Tag": {"type": "string"},
    "Metadata": copy.deepcopy(META), "MessageStream": {"type": "string", "description": "Defaults to the broadcast stream."},
    "TrackOpens": {"type": "boolean"}, "TrackLinks": copy.deepcopy(TRACKLINKS),
    "Attachments": {"$ref": "#/components/schemas/AttachmentCollection"}, "Headers": {"$ref": "#/components/schemas/HeaderCollection"},
    "Messages": {"type": "array", "items": {"$ref": "#/components/schemas/BulkEmailMessage"}}}}
paths["/email/bulk"] = {
    "post": {"operationId": "sendBulkEmail", "tags": ["Bulk Email API"], "summary": "Send one message to many recipients (bulk). Requires account approval.",
             "requestBody": body("BulkEmailRequest"), "responses": ok("BulkRequestStatus")},
    "get": {"operationId": "listBulkRequests", "tags": ["Bulk Email API"], "summary": "List bulk requests on this server, newest first",
            "parameters": [qp("count", "Number of requests to return", "integer"), qp("paginationKey", "Opaque cursor from the previous page's PaginationKey")],
            "x-pp-pagination": "none",
            "responses": ok(schema={"type": "object", "properties": {"Requests": {"type": "array", "items": {"$ref": "#/components/schemas/BulkRequestStatus"}}, "PaginationKey": {"type": "string"}}})},
}
paths["/email/bulk/{bulkRequestId}"] = {"get": {"operationId": "getBulkRequestStatus", "tags": ["Bulk Email API"], "summary": "Get the status of one bulk request",
                                                "parameters": [pp("bulkRequestId", "Bulk request ID (UUID)")], "responses": ok("BulkRequestStatus")}}

S["MessageStream"] = {"type": "object", "properties": {
    "ID": {"type": "string"}, "ServerID": {"type": "integer"}, "Name": {"type": "string"}, "Description": {"type": "string", "nullable": True},
    "MessageStreamType": {"type": "string", "enum": ["Inbound", "Broadcasts", "Transactional"]}, "CreatedAt": {"type": "string"},
    "UpdatedAt": {"type": "string", "nullable": True}, "ArchivedAt": {"type": "string", "nullable": True},
    "ExpectedPurgeDate": {"type": "string", "nullable": True},
    "SubscriptionManagementConfiguration": {"type": "object", "properties": {"UnsubscribeHandlingType": {"type": "string"}}}}}
SUBSCRIPTION = {"type": "object", "properties": {"UnsubscribeHandlingType": {"type": "string", "description": "None, Postmark, or Custom (Custom needs approval)."}}}
paths["/message-streams"] = {
    "get": {"operationId": "listMessageStreams", "tags": ["Message Streams API"], "summary": "List message streams on this server",
            "parameters": [qp("MessageStreamType", "Filter by stream type", enum=["All", "Inbound", "Transactional", "Broadcasts"], default="All"),
                           qp("IncludeArchivedStreams", "Include archived streams", "boolean", default=False)],
            "x-pp-pagination": "none", "x-resource-id": "ID",
            "responses": ok(schema={"type": "object", "properties": {"MessageStreams": {"type": "array", "items": {"$ref": "#/components/schemas/MessageStream"}}, "TotalCount": {"type": "integer"}}})},
    "post": {"operationId": "createMessageStream", "tags": ["Message Streams API"], "summary": "Create a message stream",
             "requestBody": body(schema={"type": "object", "required": ["ID", "Name", "MessageStreamType"], "properties": {
                 "ID": {"type": "string", "description": "Starts with a letter, max 30 chars, not starting with pm-."},
                 "Name": {"type": "string"}, "Description": {"type": "string"},
                 "MessageStreamType": {"type": "string", "enum": ["Transactional", "Broadcasts"]},
                 "SubscriptionManagementConfiguration": copy.deepcopy(SUBSCRIPTION)}}),
             "responses": ok("MessageStream")},
}
paths["/message-streams/{streamId}"] = {
    "get": {"operationId": "getMessageStream", "tags": ["Message Streams API"], "summary": "Get one message stream",
            "parameters": [pp("streamId", "Message stream ID")], "responses": ok("MessageStream")},
    "patch": {"operationId": "editMessageStream", "tags": ["Message Streams API"], "summary": "Edit a message stream",
              "parameters": [pp("streamId", "Message stream ID")],
              "requestBody": body(schema={"type": "object", "properties": {"Name": {"type": "string"}, "Description": {"type": "string"},
                                                                             "SubscriptionManagementConfiguration": copy.deepcopy(SUBSCRIPTION)}}),
              "responses": ok("MessageStream")},
}
paths["/message-streams/{streamId}/archive"] = {"post": {"operationId": "archiveMessageStream", "tags": ["Message Streams API"],
    "summary": "Archive a message stream (purged 45 days later unless unarchived)", "parameters": [pp("streamId", "Message stream ID")],
    "responses": ok(schema={"type": "object", "properties": {"ID": {"type": "string"}, "ServerID": {"type": "integer"}, "ExpectedPurgeDate": {"type": "string"}}})}}
paths["/message-streams/{streamId}/unarchive"] = {"post": {"operationId": "unarchiveMessageStream", "tags": ["Message Streams API"],
    "summary": "Restore an archived message stream before its purge date", "parameters": [pp("streamId", "Message stream ID")], "responses": ok("MessageStream")}}

S["Suppression"] = {"type": "object", "properties": {"EmailAddress": {"type": "string"}, "SuppressionReason": {"type": "string"},
                                                     "Origin": {"type": "string"}, "CreatedAt": {"type": "string"}}}
S["SuppressionChangeRequest"] = {"type": "object", "required": ["Suppressions"], "properties": {
    "Suppressions": {"type": "array", "maxItems": 50, "items": {"type": "object", "properties": {"EmailAddress": {"type": "string"}}}}}}
S["SuppressionChangeResult"] = {"type": "object", "properties": {"Suppressions": {"type": "array", "items": {"type": "object", "properties": {
    "EmailAddress": {"type": "string"}, "Status": {"type": "string"}, "Message": {"type": "string", "nullable": True}}}}}}
paths["/message-streams/{streamId}/suppressions/dump"] = {"get": {"operationId": "listSuppressions", "tags": ["Suppressions API"],
    "summary": "List suppressed addresses on a message stream",
    "parameters": [pp("streamId", "Message stream ID"),
                   qp("SuppressionReason", "Filter by reason", enum=["HardBounce", "SpamComplaint", "ManualSuppression"]),
                   qp("Origin", "Filter by origin", enum=["Recipient", "Customer", "Admin"]),
                   qp("fromdate", "Suppressions created on or after this date (YYYY-MM-DD)"),
                   qp("todate", "Suppressions created on or before this date (YYYY-MM-DD)"),
                   qp("EmailAddress", "Filter by email address")],
    "x-pp-pagination": "none",
    "responses": ok(schema={"type": "object", "properties": {"Suppressions": {"type": "array", "items": {"$ref": "#/components/schemas/Suppression"}}}})}}
paths["/message-streams/{streamId}/suppressions"] = {"post": {"operationId": "createSuppressions", "tags": ["Suppressions API"],
    "summary": "Suppress up to 50 addresses on a message stream", "parameters": [pp("streamId", "Message stream ID")],
    "requestBody": body("SuppressionChangeRequest"), "responses": ok("SuppressionChangeResult")}}
paths["/message-streams/{streamId}/suppressions/delete"] = {"post": {"operationId": "deleteSuppressions", "tags": ["Suppressions API"],
    "summary": "Remove up to 50 suppressions from a message stream (SpamComplaint suppressions cannot be removed)",
    "parameters": [pp("streamId", "Message stream ID")], "requestBody": body("SuppressionChangeRequest"), "responses": ok("SuppressionChangeResult")}}

TRIGGER = lambda extra=None: {"type": "object", "properties": dict({"Enabled": {"type": "boolean"}}, **(extra or {}))}
S["WebhookTriggers"] = {"type": "object", "properties": {
    "Open": TRIGGER({"PostFirstOpenOnly": {"type": "boolean"}}), "Click": TRIGGER(), "Delivery": TRIGGER(),
    "Bounce": TRIGGER({"IncludeContent": {"type": "boolean"}}), "SpamComplaint": TRIGGER({"IncludeContent": {"type": "boolean"}}),
    "SubscriptionChange": TRIGGER()}}
S["Webhook"] = {"type": "object", "properties": {
    "ID": {"type": "integer"}, "Url": {"type": "string"}, "MessageStream": {"type": "string"}, "Status": {"type": "string"},
    "HttpAuth": {"type": "object", "properties": {"Username": {"type": "string"}, "Password": {"type": "string"}}},
    "HttpHeaders": {"type": "array", "items": {"$ref": "#/components/schemas/MessageHeader"}},
    "Triggers": {"$ref": "#/components/schemas/WebhookTriggers"}}}
S["WebhookVerification"] = {"type": "object", "properties": {
    "Id": {"type": "integer"}, "Url": {"type": "string"}, "Success": {"type": "boolean"}, "Message": {"type": "string"},
    "Results": {"type": "array", "items": {"type": "object", "properties": {"TriggerType": {"type": "string"}, "Success": {"type": "boolean"},
                                                                             "StatusCode": {"type": "integer"}, "Message": {"type": "string"}}}}}}
WEBHOOK_WRITE = {"Url": {"type": "string"}, "HttpAuth": {"type": "object", "properties": {"Username": {"type": "string"}, "Password": {"type": "string"}}},
                 "HttpHeaders": {"type": "array", "items": {"$ref": "#/components/schemas/MessageHeader"}},
                 "Triggers": {"$ref": "#/components/schemas/WebhookTriggers"},
                 "Verify": {"type": "boolean", "default": True, "description": "Postmark calls the URL for each enabled trigger before saving."}}
paths["/webhooks"] = {
    "get": {"operationId": "listWebhooks", "tags": ["Webhooks API"], "summary": "List webhooks on this server",
            "parameters": [qp("MessageStream", "Only webhooks for this message stream")], "x-pp-pagination": "none", "x-resource-id": "ID",
            "responses": ok(schema={"type": "object", "properties": {"Webhooks": {"type": "array", "items": {"$ref": "#/components/schemas/Webhook"}}}})},
    "post": {"operationId": "createWebhook", "tags": ["Webhooks API"], "summary": "Create a webhook",
             "requestBody": body(schema={"type": "object", "required": ["Url"], "properties": dict(WEBHOOK_WRITE, MessageStream={"type": "string", "default": "outbound"})}),
             "responses": ok("Webhook")},
}
paths["/webhooks/{webhookId}"] = {
    "get": {"operationId": "getWebhook", "tags": ["Webhooks API"], "summary": "Get one webhook",
            "parameters": [pp("webhookId", "Webhook ID", "integer")], "responses": ok("Webhook")},
    "put": {"operationId": "editWebhook", "tags": ["Webhooks API"], "summary": "Edit a webhook (omitted triggers stay unchanged)",
            "parameters": [pp("webhookId", "Webhook ID", "integer")],
            "requestBody": body(schema={"type": "object", "properties": copy.deepcopy(WEBHOOK_WRITE)}), "responses": ok("Webhook")},
    "delete": {"operationId": "deleteWebhook", "tags": ["Webhooks API"], "summary": "Delete a webhook",
               "parameters": [pp("webhookId", "Webhook ID", "integer")], "responses": ok("StandardPostmarkResponse")},
}
paths["/webhooks/{webhookId}/verify"] = {"post": {"operationId": "verifyWebhook", "tags": ["Webhooks API"],
    "summary": "Send a test request for each enabled trigger and record verified or unverified",
    "parameters": [pp("webhookId", "Webhook ID", "integer")], "responses": ok("WebhookVerification")}}
paths["/webhooks/{webhookId}/statistics"] = {"get": {"operationId": "getWebhookStatistics", "tags": ["Webhooks API"],
    "summary": "Delivery statistics for one webhook over the last 24 hours",
    "parameters": [pp("webhookId", "Webhook ID", "integer")],
    "responses": ok(schema={"type": "object", "properties": {
        "WebhookId": {"type": "integer"}, "ServerId": {"type": "integer"}, "MessageStreamId": {"type": "string"}, "Url": {"type": "string"},
        "Statuses": {"type": "object"}, "TimeRange": {"type": "object"}, "Metrics": {"type": "object"}, "MetricsByTrigger": {"type": "object"}}})}}

S["DataRemovalStatus"] = {"type": "object", "properties": {"ID": {"type": "integer"}, "Status": {"type": "string", "enum": ["Pending", "Done"]}}}
paths["/data-removals"] = {"post": {"operationId": "createDataRemoval", "tags": ["Data Removals API"],
    "summary": "Request irreversible removal of a recipient's data (enabled on request by Postmark)",
    "requestBody": body(schema={"type": "object", "required": ["RequestedFor"], "properties": {
        "RequestedBy": {"type": "string"}, "RequestedFor": {"type": "string"}, "NotifyWhenCompleted": {"type": "boolean"}}}),
    "responses": ok("DataRemovalStatus")}}
paths["/data-removals/{dataRemovalId}"] = {"get": {"operationId": "getDataRemoval", "tags": ["Data Removals API"],
    "summary": "Check a data removal request", "parameters": [pp("dataRemovalId", "Data removal request ID", "integer")], "responses": ok("DataRemovalStatus")}}

# Undocumented on the docs site but used by the official SDK and MCP.
paths["/stats/outbound/opens/readtimes"] = {"get": {"operationId": "getOutboundOpenReadTimes", "tags": ["Stats API"],
    "summary": "Open counts by read time (seconds spent reading)",
    "parameters": [qp("tag", "Filter by tag"), qp("fromdate", "Start date (YYYY-MM-DD)"), qp("todate", "End date (YYYY-MM-DD)"), qp("messagestream", MS_DESC_STATS)],
    "responses": ok(schema={"type": "object", "properties": {"Days": {"type": "array", "items": {"type": "object"}}}})}}

# ------------------------------------------------------------- resource ids
for p, key in (("/bounces", "ID"), ("/messages/outbound", "MessageID"), ("/messages/inbound", "MessageID"),
               ("/servers", "ID"), ("/senders", "ID"), ("/domains", "ID"), ("/triggers/inboundrules", "ID")):
    paths[p]["get"]["x-resource-id"] = key

# ------------------------------------------------------------ security
for p, item in paths.items():
    for m, op in item.items():
        op.pop("security", None)
        if p.startswith(ACCOUNT_PATH_PREFIXES):
            op["x-postmark-token"] = "account"
        else:
            op["x-postmark-token"] = "server"

doc = {
    "openapi": "3.0.3",
    "info": {
        "title": "Postmark",
        "version": "2026-09-30",
        "description": (
            "Postmark transactional and broadcast email API. Server-scoped endpoints authenticate with the "
            "X-Postmark-Server-Token header; account-scoped endpoints (servers, domains, sender signatures, "
            "template push, data removals) use X-Postmark-Account-Token."
        ),
        "x-display-name": "Postmark",
        "x-website": "https://postmarkapp.com",
    },
    "servers": [{"url": "https://api.postmarkapp.com"}],
    "security": [{"serverToken": [], "accountToken": []}],
    "x-learn": {
        "ticker_patterns": ["^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"],
        "entity_lookup_seeds": {
            "bounce_type": [
                {"canonical": "HardBounce", "aliases": ["hard bounce", "hard bounces", "permanent bounce"]},
                {"canonical": "SoftBounce", "aliases": ["soft bounce", "soft bounces"]},
                {"canonical": "Transient", "aliases": ["deferred", "temporary failure"]},
                {"canonical": "SpamComplaint", "aliases": ["spam complaint", "complaint", "marked as spam"]},
                {"canonical": "Blocked", "aliases": ["blocked bounce"]},
                {"canonical": "DnsError", "aliases": ["dns error"]},
                {"canonical": "AutoResponder", "aliases": ["auto responder", "out of office"]},
            ],
            "suppression_reason": [
                {"canonical": "HardBounce", "aliases": ["hard bounce suppression"]},
                {"canonical": "SpamComplaint", "aliases": ["spam complaint suppression"]},
                {"canonical": "ManualSuppression", "aliases": ["manual suppression", "manually suppressed"]},
            ],
            "message_stream_type": [
                {"canonical": "Transactional", "aliases": ["transactional stream", "outbound stream"]},
                {"canonical": "Broadcasts", "aliases": ["broadcast stream", "broadcast", "broadcasts stream"]},
                {"canonical": "Inbound", "aliases": ["inbound stream"]},
            ],
            "message_event": [
                {"canonical": "Delivered", "aliases": ["delivery", "delivered event"]},
                {"canonical": "Opened", "aliases": ["open", "opens"]},
                {"canonical": "LinkClicked", "aliases": ["click", "clicks", "link click"]},
                {"canonical": "Bounced", "aliases": ["bounce event"]},
                {"canonical": "SubscriptionChanged", "aliases": ["unsubscribe", "subscription change"]},
            ],
            "template_type": [
                {"canonical": "Standard", "aliases": ["standard template"]},
                {"canonical": "Layout", "aliases": ["layout", "layouts", "layout template"]},
            ],
        },
    },
    "paths": dict(sorted(paths.items())),
    "components": {
        "schemas": dict(sorted(S.items())),
        "securitySchemes": {
            "serverToken": {
                "type": "apiKey", "in": "header", "name": "X-Postmark-Server-Token",
                "x-auth-vars": [{"name": "POSTMARK_SERVER_TOKEN", "kind": "per_call", "required": False, "sensitive": True,
                                 "description": "Server API token for one Postmark server. Set this OR POSTMARK_ACCOUNT_TOKEN with --server <name>."}],
                "x-auth-key-url": "https://account.postmarkapp.com/servers",
            },
            "accountToken": {
                "type": "apiKey", "in": "header", "name": "X-Postmark-Account-Token",
                "x-auth-vars": [{"name": "POSTMARK_ACCOUNT_TOKEN", "kind": "per_call", "required": False, "sensitive": True,
                                 "description": "Account API token. Required for servers, domains, senders, template push, and data removals; also lets --server <name> look up server tokens."}],
                "x-auth-key-url": "https://account.postmarkapp.com/api_tokens",
            },
        },
    },
}

out = os.path.join(HERE, "postmark-merged.openapi.yaml")
with open(out, "w") as fh:
    yaml.safe_dump(doc, fh, sort_keys=False, width=120, allow_unicode=True)
ops = sum(1 for item in doc["paths"].values() for m in item if m in ("get", "post", "put", "patch", "delete"))
print(f"wrote {out}: {len(doc['paths'])} paths, {ops} operations, {len(S)} schemas")

# ------------------------------------------------------------ command naming
# (path, method) -> (operationId, x-pp-resource or None). operationId words that
# repeat the resource are stripped by the generator, so these read as leaf names.
NAMES = {
    ("/email", "post"): ("send", "email"),
    ("/email/withTemplate", "post"): ("sendWithTemplate", "email"),
    ("/email/batch", "post"): ("sendBatch", "email"),
    ("/email/batchWithTemplates", "post"): ("sendBatchWithTemplates", "email"),
    ("/email/bulk", "post"): ("sendBulk", "email"),
    ("/email/bulk", "get"): ("bulkList", "email"),
    ("/email/bulk/{bulkRequestId}", "get"): ("bulkStatus", "email"),
    ("/bounces", "get"): ("list", "bounces"),
    ("/bounces/{bounceid}", "get"): ("get", "bounces"),
    ("/bounces/{bounceid}/dump", "get"): ("dump", "bounces"),
    ("/bounces/{bounceid}/activate", "put"): ("activate", "bounces"),
    ("/deliverystats", "get"): ("deliveryStats", "bounces"),
    ("/messages/outbound", "get"): ("list", "messages"),
    ("/messages/outbound/{messageid}/details", "get"): ("get", "messages"),
    ("/messages/outbound/{messageid}/dump", "get"): ("dump", "messages"),
    ("/messages/inbound", "get"): ("list", "inbound"),
    ("/messages/inbound/{messageid}/details", "get"): ("get", "inbound"),
    ("/messages/inbound/{messageid}/bypass", "put"): ("bypass", "inbound"),
    ("/messages/inbound/{messageid}/retry", "put"): ("retry", "inbound"),
    ("/messages/outbound/opens", "get"): ("list", "opens"),
    ("/messages/outbound/opens/{messageid}", "get"): ("get", "opens"),
    ("/messages/outbound/clicks", "get"): ("list", "clicks"),
    ("/messages/outbound/clicks/{messageid}", "get"): ("get", "clicks"),
    ("/templates", "get"): ("list", "templates"),
    ("/templates", "post"): ("create", "templates"),
    ("/templates/{templateIdOrAlias}", "get"): ("get", "templates"),
    ("/templates/{templateIdOrAlias}", "put"): ("update", "templates"),
    ("/templates/{templateIdOrAlias}", "delete"): ("delete", "templates"),
    ("/templates/validate", "post"): ("validate", "templates"),
    ("/templates/push", "put"): ("pushBetweenServers", "templates"),
    ("/stats/outbound", "get"): ("overview", "stats"),
    ("/stats/outbound/sends", "get"): ("sends", "stats"),
    ("/stats/outbound/bounces", "get"): ("bounces", "stats"),
    ("/stats/outbound/spam", "get"): ("spam", "stats"),
    ("/stats/outbound/tracked", "get"): ("tracked", "stats"),
    ("/stats/outbound/opens", "get"): ("opens", "stats"),
    ("/stats/outbound/opens/platforms", "get"): ("openPlatforms", "stats"),
    ("/stats/outbound/opens/emailclients", "get"): ("openClients", "stats"),
    ("/stats/outbound/opens/readtimes", "get"): ("openReadTimes", "stats"),
    ("/stats/outbound/clicks", "get"): ("clicks", "stats"),
    ("/stats/outbound/clicks/browserfamilies", "get"): ("clickBrowsers", "stats"),
    ("/stats/outbound/clicks/platforms", "get"): ("clickPlatforms", "stats"),
    ("/stats/outbound/clicks/location", "get"): ("clickLocations", "stats"),
    ("/server", "get"): ("get", "server"),
    ("/server", "put"): ("update", "server"),
    ("/servers", "get"): ("list", "servers"),
    ("/servers", "post"): ("create", "servers"),
    ("/servers/{serverid}", "get"): ("get", "servers"),
    ("/servers/{serverid}", "put"): ("update", "servers"),
    ("/servers/{serverid}", "delete"): ("delete", "servers"),
    ("/senders", "get"): ("list", "senders"),
    ("/senders", "post"): ("create", "senders"),
    ("/senders/{signatureid}", "get"): ("get", "senders"),
    ("/senders/{signatureid}", "put"): ("update", "senders"),
    ("/senders/{signatureid}", "delete"): ("delete", "senders"),
    ("/senders/{signatureid}/resend", "post"): ("resendConfirmation", "senders"),
    ("/senders/{signatureid}/verifyspf", "post"): ("verifySpf", "senders"),
    ("/senders/{signatureid}/requestnewdkim", "post"): ("requestNewDkim", "senders"),
    ("/domains", "get"): ("list", "domains"),
    ("/domains", "post"): ("create", "domains"),
    ("/domains/{domainid}", "get"): ("get", "domains"),
    ("/domains/{domainid}", "put"): ("update", "domains"),
    ("/domains/{domainid}", "delete"): ("delete", "domains"),
    ("/domains/{domainid}/verifydkim", "put"): ("verifyDkim", "domains"),
    ("/domains/{domainid}/verifyreturnpath", "put"): ("verifyReturnPath", "domains"),
    ("/domains/{domainid}/verifyspf", "post"): ("verifySpf", "domains"),
    ("/domains/{domainid}/rotatedkim", "post"): ("rotateDkim", "domains"),
    ("/message-streams", "get"): ("list", "streams"),
    ("/message-streams", "post"): ("create", "streams"),
    ("/message-streams/{streamId}", "get"): ("get", "streams"),
    ("/message-streams/{streamId}", "patch"): ("update", "streams"),
    ("/message-streams/{streamId}/archive", "post"): ("archive", "streams"),
    ("/message-streams/{streamId}/unarchive", "post"): ("unarchive", "streams"),
    ("/message-streams/{streamId}/suppressions/dump", "get"): ("list", "suppressions"),
    ("/message-streams/{streamId}/suppressions", "post"): ("create", "suppressions"),
    ("/message-streams/{streamId}/suppressions/delete", "post"): ("delete", "suppressions"),
    ("/webhooks", "get"): ("list", "webhooks"),
    ("/webhooks", "post"): ("create", "webhooks"),
    ("/webhooks/{webhookId}", "get"): ("get", "webhooks"),
    ("/webhooks/{webhookId}", "put"): ("update", "webhooks"),
    ("/webhooks/{webhookId}", "delete"): ("delete", "webhooks"),
    ("/webhooks/{webhookId}/verify", "post"): ("verify", "webhooks"),
    ("/webhooks/{webhookId}/statistics", "get"): ("statistics", "webhooks"),
    ("/triggers/inboundrules", "get"): ("list", "inbound-rules"),
    ("/triggers/inboundrules", "post"): ("create", "inbound-rules"),
    ("/triggers/inboundrules/{triggerid}", "delete"): ("delete", "inbound-rules"),
    ("/data-removals", "post"): ("create", "data-removals"),
    ("/data-removals/{dataRemovalId}", "get"): ("get", "data-removals"),
}
missing = []
for (p, m), (op_id, res) in NAMES.items():
    op = doc["paths"].get(p, {}).get(m)
    if op is None:
        missing.append(f"{m.upper()} {p}")
        continue
    op["operationId"] = op_id
    if res:
        op["x-pp-resource"] = res
unnamed = [f"{m.upper()} {p}" for p, item in doc["paths"].items() for m in item if (p, m) not in NAMES]
if missing or unnamed:
    sys.exit(f"naming table mismatch: missing={missing} unnamed={unnamed}")
with open(out, "w") as fh:
    yaml.safe_dump(doc, fh, sort_keys=False, width=120, allow_unicode=True)
print(f"applied {len(NAMES)} command names")

# ------------------------------------------------------ sync shaping
# Postmark requires offset on every paged list, including the first page.
for p, item in doc["paths"].items():
    for m, op in item.items():
        for prm in op.get("parameters", []):
            if prm.get("in") == "query" and prm["name"].lower() == "offset":
                prm["schema"]["default"] = 0
            if prm.get("in") == "query" and prm["name"].lower() == "count" and p != "/email/bulk":
                prm["schema"].setdefault("default", 100)
# Stats responses are typed aggregates without IDs, so default sync skips them.
DAYS = {"type": "array", "description": "Per-day rows (Days without statistics are omitted)."}
TYPED_STATS = {
    "/stats/outbound/clicks": {"Clicks": {"type": "integer"}, "Unique": {"type": "integer"}, "Days": dict(DAYS)},
    "/stats/outbound/clicks/platforms": {"Desktop": {"type": "integer"}, "Mobile": {"type": "integer"}, "Unknown": {"type": "integer"}, "Days": dict(DAYS)},
    "/stats/outbound/clicks/location": {"HTML": {"type": "integer"}, "Text": {"type": "integer"}, "Days": dict(DAYS)},
    "/stats/outbound/clicks/browserfamilies": {"Days": dict(DAYS)},
    "/stats/outbound/opens/readtimes": {"Days": dict(DAYS)},
}
for p, props in TYPED_STATS.items():
    doc["paths"][p]["get"]["responses"]["200"]["content"] = {"application/json": {"schema": {"type": "object", "additionalProperties": True, "properties": props}}}
# Bulk list needs account approval (ErrorCode 14) and pages by an opaque cursor;
# print the whole object so PaginationKey is visible and keep it out of default sync.
bulk = doc["paths"]["/email/bulk"]["get"]["responses"]["200"]["content"]["application/json"]["schema"]["properties"]
bulk["Requests"] = {"type": "array", "description": "Bulk request status objects, newest first."}

# ------------------------------------------- single-object response arrays
# The generator treats any object response with exactly one typed array
# property as a list envelope and prints only that array. For resource objects
# (GET /server returns ApiTokens beside 20 scalar fields) that hides the object
# and prints the server token. Arrays without an items schema are not treated
# as list paths, so drop items from arrays inside non-envelope responses.
ENVELOPE_SIBLINGS = {"TotalCount", "PaginationKey"}


def _resolve(schema):
    if isinstance(schema, dict) and "$ref" in schema:
        return doc["components"]["schemas"][schema["$ref"].rsplit("/", 1)[1]]
    return schema


stripped = []
for p, item in doc["paths"].items():
    for m, op in item.items():
        content = (op.get("responses", {}).get("200", {}).get("content") or {}).get("application/json")
        if not content:
            continue
        schema = _resolve(content["schema"])
        props = (schema or {}).get("properties") or {}
        arrays = [k for k, v in props.items() if isinstance(_resolve(v), dict) and _resolve(v).get("type") == "array" and "items" in _resolve(v)]
        if len(arrays) != 1:
            continue
        others = set(props) - set(arrays)
        if others <= ENVELOPE_SIBLINGS:
            continue
        target = _resolve(props[arrays[0]])
        target.pop("items", None)
        target.setdefault("description", "List of values (items untyped in this spec).")
        stripped.append(f"{m.upper()} {p}:{arrays[0]}")
with open(out, "w") as fh:
    yaml.safe_dump(doc, fh, sort_keys=False, width=120, allow_unicode=True)
print("untyped single-object arrays:", ", ".join(stripped))

# ------------------------------------------------ resource ids on path items
# x-resource-id is read from the path item, not the operation.
RESOURCE_IDS = {
    "/bounces": "ID", "/messages/outbound": "MessageID", "/messages/inbound": "MessageID",
    "/messages/outbound/opens": "MessageID+ReceivedAt+Recipient",
    "/messages/outbound/clicks": "MessageID+ReceivedAt+Recipient",
    "/templates": "TemplateId", "/servers": "ID", "/senders": "ID", "/domains": "ID",
    "/triggers/inboundrules": "ID", "/message-streams": "ID", "/webhooks": "ID",
}
for p, item in doc["paths"].items():
    for m, op in item.items():
        if isinstance(op, dict):
            op.pop("x-resource-id", None)
for p, key in RESOURCE_IDS.items():
    doc["paths"][p]["x-resource-id"] = key
with open(out, "w") as fh:
    yaml.safe_dump(doc, fh, sort_keys=False, width=120, allow_unicode=True)
print("resource ids on path items:", len(RESOURCE_IDS))

# ------------------------------------------------ examples and descriptions
# Fake but realistic values so generated help and verify fixtures have inputs.
PATH_EXAMPLES = {
    "serverid": 1234567, "signatureid": 3456789, "domainid": 2345678, "bounceid": 692560173,
    "messageid": "0ac29aee-e1cd-480d-b08d-4f48548ff48d", "templateIdOrAlias": "password-reset",
    "streamId": "broadcast", "webhookId": 1234, "triggerid": 5678, "dataRemovalId": 91011,
    "bulkRequestId": "f24af63c-533d-4b7a-ad65-4a7b3202d3a7",
}
FIELD_DESCRIPTIONS = {
    "Attachments": "JSON array of attachments: [{\"Name\":\"invoice.pdf\",\"Content\":\"<base64>\",\"ContentType\":\"application/pdf\"}]",
    "Headers": "JSON array of custom headers: [{\"Name\":\"X-Order-ID\",\"Value\":\"1234\"}]",
    "TrackLinks": "Link tracking: None, HtmlAndText, HtmlOnly, or TextOnly",
    "TrackOpens": "Record opens for this message",
    "TemplateModel": "JSON object of template variables, e.g. {\"name\":\"Jane\",\"action_url\":\"https://...\"}",
    "TestRenderModel": "JSON object of sample variables used to render the template",
    "InlineCss": "Inline the template's CSS into HTML elements before sending",
    "Messages": "JSON array of message objects (up to 500)",
    "Metadata": "JSON object of string key/values stored with the message, e.g. {\"order_id\":\"1234\"}",
    "HttpAuth": "JSON object with Username and Password for webhook basic auth",
    "HttpHeaders": "JSON array of headers Postmark sends with each webhook call",
    "Triggers": "JSON object of webhook triggers, e.g. {\"Bounce\":{\"Enabled\":true},\"Delivery\":{\"Enabled\":true}}",
    "SubscriptionManagementConfiguration": "JSON object, e.g. {\"UnsubscribeHandlingType\":\"Postmark\"}",
    "Suppressions": "JSON array of {\"EmailAddress\":\"...\"} objects (max 50)",
    "Rule": "Email address or domain to block for inbound mail",
}
for p, item in doc["paths"].items():
    for m, op in item.items():
        if not isinstance(op, dict):
            continue
        for prm in op.get("parameters", []):
            if prm.get("in") == "path" and prm["name"] in PATH_EXAMPLES:
                # Integer path examples are dropped by the generator; path
                # segments are text on the wire, so type IDs as strings.
                value = str(PATH_EXAMPLES[prm["name"]])
                prm["schema"] = {"type": "string", "example": value}
                prm["example"] = value
for name, schema in doc["components"]["schemas"].items():
    for field, prop in (schema.get("properties") or {}).items():
        if field in FIELD_DESCRIPTIONS and isinstance(prop, dict) and "$ref" not in prop:
            desc = (prop.get("description") or "").strip()
            if len(desc.split()) < 5:
                prop["description"] = FIELD_DESCRIPTIONS[field]
        elif field in FIELD_DESCRIPTIONS and isinstance(prop, dict) and "$ref" in prop:
            target = copy.deepcopy(doc["components"]["schemas"][prop["$ref"].rsplit("/", 1)[1]])
            target["description"] = FIELD_DESCRIPTIONS[field]
            schema["properties"][field] = target
with open(out, "w") as fh:
    yaml.safe_dump(doc, fh, sort_keys=False, width=120, allow_unicode=True)
print("path examples + field descriptions applied")

# Required body-field examples so create/send commands get usable help examples.
BODY_EXAMPLES = {
    "SendEmailRequest": {"From": "sender@example.com", "To": "jane@example.com", "Subject": "Your receipt", "TextBody": "Thanks for your order."},
    "EmailWithTemplateRequest": {"From": "sender@example.com", "To": "jane@example.com", "TemplateAlias": "welcome", "TemplateModel": {"name": "Jane"}},
    "BulkEmailRequest": {"From": "news@example.com", "Subject": "Product update", "TextBody": "What's new this month."},
    "CreateTemplateRequest": {"Name": "Password reset", "Alias": "password-reset", "Subject": "Reset your password", "HtmlBody": "<p>Hi {{name}}</p>"},
    "CreateServerPayload": {"Name": "Staging"},
    "DomainCreationModel": {"Name": "mail.example.com"},
    "SenderSignatureCreationModel": {"FromEmail": "support@example.com", "Name": "Support"},
    "TemplatesPushModel": {"SourceServerID": 1234567, "DestinationServerID": 7654321, "PerformChanges": False},
}
for name, fields in BODY_EXAMPLES.items():
    props = doc["components"]["schemas"][name].get("properties", {})
    for field, value in fields.items():
        if field in props and isinstance(props[field], dict):
            props[field]["example"] = value
INLINE_BODY_EXAMPLES = {
    ("/message-streams", "post"): {"ID": "broadcast", "Name": "Broadcasts", "MessageStreamType": "Broadcasts"},
    ("/webhooks", "post"): {"Url": "https://example.com/postmark/webhook"},
    ("/data-removals", "post"): {"RequestedFor": "jane@example.com", "RequestedBy": "privacy@example.com"},
}
for (p, m), fields in INLINE_BODY_EXAMPLES.items():
    props = doc["paths"][p][m]["requestBody"]["content"]["application/json"]["schema"]["properties"]
    for field, value in fields.items():
        props[field]["example"] = value
sup = doc["components"]["schemas"]["SuppressionChangeRequest"]["properties"]["Suppressions"]
sup["example"] = [{"EmailAddress": "jane@example.com"}]
with open(out, "w") as fh:
    yaml.safe_dump(doc, fh, sort_keys=False, width=120, allow_unicode=True)
print("body examples applied")

for p, verb in (("/message-streams/{streamId}/suppressions", "create"), ("/message-streams/{streamId}/suppressions/delete", "delete")):
    doc["paths"][p]["post"]["x-pp-example"] = (
        f"postmark-pp-cli suppressions {verb} outbound --suppressions '[{{\"EmailAddress\":\"jane@example.com\"}}]'"
    )
with open(out, "w") as fh:
    yaml.safe_dump(doc, fh, sort_keys=False, width=120, allow_unicode=True)
print("suppressions examples applied")

# ------------------------------------------------ live dogfood fixtures
# Explicit positional happy-args (synthesized from the fake path examples above)
# stop the live runner from resolving a real ID through the list companion.
# Flag-only happy-args keep the help examples and let the runner pick real IDs.
for p, item in doc["paths"].items():
    get = item.get("get")
    if isinstance(get, dict) and "{" in p:
        get["x-happy-args"] = "--json"
# Features that need Postmark to enable them on the account.
for p in ("/email/bulk", "/email/bulk/{bulkRequestId}"):
    for m, op in doc["paths"][p].items():
        op["x-live-dogfood-requires-tier"] = "bulk-approved"
for p in ("/data-removals", "/data-removals/{dataRemovalId}"):
    for m, op in doc["paths"][p].items():
        op["x-live-dogfood-requires-tier"] = "data-removals-enabled"
tv = doc["components"]["schemas"]["TemplateValidationRequest"]["properties"]
tv["Subject"]["example"] = "Hi {{name}}"
tv["HtmlBody"]["example"] = "<p>Hello {{name}}</p>"
tv["TextBody"]["example"] = "Hello {{name}}"
with open(out, "w") as fh:
    yaml.safe_dump(doc, fh, sort_keys=False, width=120, allow_unicode=True)
print("live dogfood fixtures applied")

doc["paths"]["/templates/validate"]["post"]["x-pp-example"] = (
    "postmark-pp-cli templates validate --subject 'Hi {{name}}' --html-body '<p>Hello {{name}}</p>' --test-render-model '{\"name\":\"Jane\"}'"
)
with open(out, "w") as fh:
    yaml.safe_dump(doc, fh, sort_keys=False, width=120, allow_unicode=True)
print("validate example applied")
