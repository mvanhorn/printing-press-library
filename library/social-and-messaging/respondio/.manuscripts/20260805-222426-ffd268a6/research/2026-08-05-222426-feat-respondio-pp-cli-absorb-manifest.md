# Respond.io Absorb Manifest

## Absorbed (match or beat everything that exists)

Best source: respond-io/typescript-sdk (official).

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Get contact by identifier | respond-io SDK contact.get | respondio-pp-cli contact get | Works offline after sync, --json |
| 2 | Create contact | respond-io SDK contact.create | respondio-pp-cli contact create | Scriptable, --dry-run |
| 3 | Update contact | respond-io SDK contact.update | respondio-pp-cli contact update | Scriptable, --dry-run |
| 4 | Delete contact | respond-io SDK contact.delete | respondio-pp-cli contact delete | Typed exit codes |
| 5 | Upsert contact | respond-io SDK contact.createOrUpdate | respondio-pp-cli contact upsert | Idempotent upsert |
| 6 | List contacts with filter | respond-io SDK contact.list | respondio-pp-cli contact list | Offline FTS fallback, --json |
| 7 | Merge contacts | respond-io SDK contact.merge | respondio-pp-cli contact merge | Typed exit codes |
| 8 | Add tags to contact | respond-io SDK contact.addTags | respondio-pp-cli contact add-tags | Scriptable |
| 9 | Remove tags from contact | respond-io SDK contact.deleteTags | respondio-pp-cli contact remove-tags | Scriptable |
| 10 | List contact channels | respond-io SDK contact.listChannels | respondio-pp-cli contact list-channels | Offline, --json |
| 11 | Update contact lifecycle | respond-io SDK contact.updateLifecycle | respondio-pp-cli contact update-lifecycle | Scriptable |
| 12 | Send message | respond-io SDK messaging.send | respondio-pp-cli message send | Agent-native, --dry-run |
| 13 | Get message by id | respond-io SDK messaging.get | respondio-pp-cli message get | Offline, --json |
| 14 | List messages for contact | respond-io SDK messaging.list | respondio-pp-cli message list | Offline, --json |
| 15 | Assign/unassign conversation | respond-io SDK conversations.assign | respondio-pp-cli conversation assign | Scriptable |
| 16 | Open/close conversation | respond-io SDK conversations.updateStatus | respondio-pp-cli conversation update-status | Scriptable |
| 17 | Create comment | respond-io SDK comments.create | respondio-pp-cli comment create | Agent-native |
| 18 | List workspace users | respond-io SDK space.listUsers | respondio-pp-cli space list-users | Offline, syncable |
| 19 | Get user by id | respond-io SDK space.getUser | respondio-pp-cli space get-user | Offline |
| 20 | CRUD custom fields | respond-io SDK space.customField | respondio-pp-cli space create/list/get-custom-field | Offline, syncable |
| 21 | List closing notes | respond-io SDK space.listClosingNotes | respondio-pp-cli space list-closing-notes | Offline |
| 22 | List workspace channels | respond-io SDK space.listChannels | respondio-pp-cli space list-channels | Offline, syncable |
| 23 | List WhatsApp templates | respond-io SDK space.listTemplates | respondio-pp-cli space list-templates | Offline |
| 24 | Workspace tag CRUD | respond-io SDK space.tag | respondio-pp-cli space create/update/delete-tag | Scriptable |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Inbox workload overview | overview | hand-code | Requires local join across contacts + assignees + conversation status; no single API call provides it | Summary of open conversations, unassigned contacts, per-agent distribution, and recent activity from the local mirror |
| 2 | Channel mix report | report channel-mix | hand-code | Requires aggregating channel sources across all synced contacts/channels in SQLite | Distribution of contact channels by source (whatsapp, instagram, email...) |
| 3 | Workload by agent | report workload | hand-code | Requires correlating space users with assigned contacts in the local store | Message/handling volume per workspace user from synced assignments |
| 4 | Tag cohort segments | contact by-tag | hand-code | Lets agents find a whole tag cohort (e.g. VIP, unpaid) offline | List contacts that carry a given tag |
| 5 | Custom-field gaps | contact field-gaps | hand-code | Queries which contacts are missing a custom field value (e.g. no orderId) | Find contacts missing a custom field or matching a value filter |
| 6 | Idle/unassigned follow-up | contact idle | hand-code | Time-windowed scan over the local mirror for follow-up routing | Contacts with no assignee and no recent conversation activity worth working |
| 7 | Offline contact search | contact search | hand-code | SQLite FTS5 over synced contact data | Free-text search across synced contacts without hitting the API |
