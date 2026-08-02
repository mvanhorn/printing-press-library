# Gmail MCP Servers — Research Data (agent 2)

## Landscape
- GongRzhe/Gmail-MCP-Server (1,167 stars, TS) — de-facto standard, unmaintained since ~Aug 2025. 19 tools: send_email, draft_email, read_email, search_emails, modify_email, delete_email, list_email_labels, batch_modify_emails, batch_delete_emails, label CRUD + get_or_create_label, filter CRUD + create_filter_from_template, download_attachment.
- shinzo-labs/gmail-mcp (56 stars, TS) — broadest coverage ~64 tools, 1:1 REST mirror incl. settings/delegates/sendAs/smime/watch.
- jasonsum (73, Py), theposch (21, Py), others minor.
- anthropics/claude-plugins-official: NO Gmail plugin. Hosted claude.ai Gmail connector = read+draft+label ONLY (no send, no attachments).

## Auth conventions (both majors, ecosystem standard)
~/.gmail-mcp/ config dir; gcp-oauth.keys.json (client keys); credentials.json (tokens); localhost:3000 loopback OAuth; access_type=offline; `auth` subcommand. Env: GMAIL_OAUTH_PATH/GMAIL_CREDENTIALS_PATH (GongRzhe); CLIENT_ID/CLIENT_SECRET/REFRESH_TOKEN headless mode (shinzo). Scopes: GongRzhe = gmail.modify + settings.basic; shinzo = modify, compose, send, settings.basic, settings.sharing.

## High-gravity fields (source-verified)
id, threadId, snippet, headers subset: Date, From, To, Subject, Message-ID, In-Reply-To, References (threading trio); payload.parts recursion, body.data, body.attachmentId, body.size, filename, mimeType.

## Content handling patterns
- base64url decode: Buffer.from(data,'base64'); outbound encode base64url (+- /_ strip =)
- Recursive multipart walk branching text/plain vs text/html
- UNIVERSAL GAP: no HTML-to-text conversion anywhere (GongRzhe returns raw HTML w/ note; shinzo omits HTML by default via includeBodyHtml flag)
- Reply threading: extract Message-ID/In-Reply-To/References, Re: prefix, >-quoted original
- RFC2822 hand-built; quoted-printable, 76-char soft wrap (shinzo); Nodemailer for attachment sends (GongRzhe)
- Batch: shinzo native batchDelete/batchModify; GongRzhe client-side chunk 50 + per-item retry
