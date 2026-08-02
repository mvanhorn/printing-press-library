# Gmail API SDK Wrappers & Automation Patterns — Research Data (agent 3)

## SDK Wrappers
npm: googleapis (official ~35M/wk), @googleapis/gmail (standalone official), gmail-api-parse-message (MIME walk + base64url decode -> {textPlain, textHtml, headers, attachments[]}), node-gmail-api (search+fetch w/ batching, unmaintained), gmail-node-mailer (send-focused), gmail-tester (CI inbox polling).
PyPI: google-api-python-client (official), simplegmail, ezgmail, yagmail (SMTP send-only, keyring).

simplegmail full surface: send_message(to,sender,subject,msg_html,msg_plain,cc,bcc,attachments,signature), get_messages(query), get_unread_messages, get_unread_inbox, get_starred_messages, get_sent_messages, get_trash_messages, get_important_messages, get_drafts, list_labels, create_label, delete_label. Message: mark_as_read/unread, star/unstar, mark_as_important/not_important, mark_as_spam/not_spam, trash/untrash, move_to_inbox, archive, add/remove/modify_labels. Attachment: download(), save(). query.construct_query kwargs->Gmail query compiler (sender, recipient, subject, unread, starred, newer_than=(2,"day"), older_than, labels nested OR-of-ANDs, attachment=True, exclude_* variants).

ezgmail full surface: init, send(recipient,subject,body,attachments,cc,bcc,mimeSubtype), search(query,maxResults)->threads, recent, unread, summary(threads), markAsRead/Unread(threads), addLabel/removeLabel(threads); GmailThread .messages .snippet trash; GmailMessage sender/recipient/subject/body/timestamp/attachments, downloadAttachment, downloadAllAttachments.

## API surface
All resources incl. users.watch/stop (Pub/Sub push), messages (list/get/send/insert/import/delete/trash/untrash/modify/batchDelete/batchModify), attachments.get, threads, labels CRUD, drafts (create/get/list/update/delete/send), history.list, settings (imap/pop/vacation/language/autoForwarding, filters CRUD, forwardingAddresses, sendAs+smimeInfo, delegates, cse).

## Quota (developers.google.com/workspace/gmail/api/reference/quota)
1.2M units/min/project; 6,000 units/min/user/project. Per-method: messages.list=5, get=20, send=100, modify=5, insert=25, import=25, delete=10, trash=20, batchDelete/batchModify=50, attachments.get=20, threads.list=10, threads.get=40, threads.modify=10, history.list=2, drafts.create=10, drafts.send=100, drafts.get=20, drafts.list=5, labels.get/list=1, labels CUD=5, getProfile=1, watch=100, stop=50. 429 -> exponential backoff mandatory.

## Batch API
POST https://gmail.googleapis.com/batch/gmail/v1 multipart/mixed; max 100/batch, Google recommends <=50 for Gmail. Only way to fetch N bodies without N round-trips.

## Incremental sync
Full sync once -> store historyId; then history.list(startHistoryId, historyTypes=[messageAdded,messageDeleted,labelAdded,labelRemoved]) paged; 404 = historyId expired (~1 week) -> full resync. users.watch -> Pub/Sub, renew every 7 days.

## Search syntax
from: to: cc: bcc: subject: label: in:(inbox/sent/spam/trash/anywhere) is:(unread/read/starred/important/snoozed) has:(attachment/drive/document/youtube/userlabels/nouserlabels) filename: category:(primary/social/promotions/updates/forums) newer_than: older_than: after: before: larger: smaller: deliveredto: list: rfc822msgid: "phrase" OR {} -neg AROUND n. Spam/trash excluded unless includeSpamTrash=true.

## format= on messages.get
minimal (ids+snippet), metadata (+metadataHeaders — right for listings), full (parsed MIME tree, base64url body.data, attachmentId refs), raw (RFC2822 base64url — for .eml export). Same quota cost; tradeoff = bandwidth/parsing.

## Automation patterns
- Inbox-zero auto-archive: in:inbox older_than:30d -is:starred -> batchModify remove INBOX
- Unsubscribe automation: List-Unsubscribe header (metadata), list: operator, group by List-Id
- Sender frequency analysis: metadata batch gets -> top-senders reports -> bulk filters.create
- Storage cleanup: larger:10M older_than:1y, category:promotions older_than:6m -> trash/batchDelete (batchDelete skips trash — dangerous); sizeEstimate for reclaimed bytes
- Attachment harvesting: has:attachment filename:pdf -> attachments.get -> base64url (-_ alphabet) -> save under sender/date dirs
- Email-to-task: history.list poll -> classify -> external API -> "processed" label as idempotency marker
- Digest generation: cron + newer_than:1d category:primary metadata fetches, group by sender/thread
- Send-later / mail-merge: drafts.create then drafts.send on schedule; 100 units/send (~60 sends/min cap); reply threading needs threadId + In-Reply-To/References in raw MIME
- CI email verification: poll messages.list with query+timeout, assert

## Gotchas
base64url (-_ alphabet) everywhere; messages.list returns only ids; MIME parts nest arbitrarily (text/plain under multipart/alternative under multipart/mixed); userId='me'; scope hierarchy readonly<modify<mail.google.com; send/labels/metadata/settings.basic/.sharing separable; restricted scopes trigger OAuth verification/CASA for published apps.
