# Trello CLI Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List boards | EndlessHoper trello_list_boards | (generated endpoint) members boards | offline cache, --json/--select |
| 2 | Get board + lists | trello_get_board | (generated endpoint) boards get | local join, no N calls |
| 3 | Create board | trello_create_board | (generated endpoint) boards create | --dry-run, agent-native |
| 4 | Update board | trello_update_board | (generated endpoint) boards update | --dry-run |
| 5 | Board labels | trello_get_board_labels | (generated endpoint) boards labels | offline |
| 6 | Board members | trello_get_board_members | (generated endpoint) boards members | offline |
| 7 | Create list | trello_create_list | (generated endpoint) lists create | --dry-run |
| 8 | Update list | trello_update_list | (generated endpoint) lists update | --dry-run |
| 9 | Get list cards | trello_get_list_cards | (generated endpoint) lists cards | offline, --select |
| 10 | Archive all cards in list | trello_archive_all_cards | (generated endpoint) lists archive-cards | --dry-run |
| 11 | Move all cards | trello_move_all_cards | (generated endpoint) lists move-cards | --dry-run |
| 12 | Move list | trello_move_list | (generated endpoint) lists move | --dry-run |
| 13 | Get card | trello_get_card | (generated endpoint) cards get | offline, --select |
| 14 | Create card | trello_create_card | (generated endpoint) cards create | --dry-run, agent-native |
| 15 | Update card | trello_update_card | (generated endpoint) cards update | --dry-run |
| 16 | Delete card | trello_delete_card | (generated endpoint) cards delete | --dry-run |
| 17 | Search cards | trello_search_cards | trello-pp-cli search | offline FTS, regex, SQL composable |
| 18 | Copy card | trello_copy_card | (generated endpoint) cards copy | --dry-run |
| 19 | Add comment | trello_add_comment | (generated endpoint) cards comment | --dry-run |
| 20 | Get card comments | trello_get_card_comments | (generated endpoint) cards comments | offline |
| 21 | Add/remove card label | trello_add_card_label | (generated endpoint) cards label | --dry-run |
| 22 | Assign/remove member | trello_assign_member | (generated endpoint) cards member | --dry-run |
| 23 | Attachments add/get/delete | trello_add_attachment | (generated endpoint) cards attachments | --dry-run |
| 24 | Labels create/update/delete | trello_create_label | (generated endpoint) labels create | --dry-run |
| 25 | Checklists CRUD | trello_create_checklist | (generated endpoint) checklists create | --dry-run |
| 26 | Checklist items add/update/delete | trello_add_checklist_item | (generated endpoint) checklists items | --dry-run |
| 27 | Get me / member profile | trello_get_me | (generated endpoint) members get | offline |
| 28 | Member boards/cards | trello_get_member_cards | (generated endpoint) members cards | offline |
| 29 | Search members | trello_search_members | (generated endpoint) search members | offline cache |
| 30 | Webhooks CRUD | spec /webhooks | (generated endpoint) webhooks create | --dry-run |

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Overdue sweep across all boards | overdue | hand-code | Requires local join across cards from every board; Trello has no cross-board overdue query | Use to find every overdue card across all boards in one shot. |
| 2 | Member workload balance | workload | hand-code | Requires aggregating card counts per assignee across boards from local store | Use to see who is overloaded before assigning more work. |
| 3 | Stale-card detection | stale --days 14 | hand-code | Requires dateLastActivity comparison over the full local card set | Use to surface cards untouched for N days. Do NOT use for due dates; use 'overdue'. |
| 4 | Board velocity (cards completed/week) | velocity --weeks 4 | hand-code | Requires historical action snapshots in SQLite no single call provides | Use to track throughput trends over time. |
| 5 | Due-soon agenda | agenda --days 3 | hand-code | Requires time-windowed aggregation across all boards | Use for a personal what-is-due-next view across everything. |
| 6 | What changed since (activity window) | since 2h | hand-code | Requires time-windowed action aggregation across boards | Use for recent changes across all boards. Do NOT use for stale cards; use 'stale'. |
