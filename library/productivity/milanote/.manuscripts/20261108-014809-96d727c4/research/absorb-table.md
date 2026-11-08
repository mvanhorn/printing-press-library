### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List boards (tree) | cotlet list_boards | milanote-pp-cli boards list | Full tree walk from root, --json/--select, cached in SQLite |
| 2 | Recent boards | cotlet list_recent_boards | milanote-pp-cli boards recent | From /api/users/me/history, agent output |
| 3 | Search boards by title | cotlet search_boards | milanote-pp-cli boards search | Live server search plus offline FTS via `search` |
| 4 | Get board contents | cotlet get_board | milanote-pp-cli boards get | Elements with type, x/y, plain text + raw doc, --select |
| 5 | Create board | cotlet create_board | milanote-pp-cli boards create | Place at x/y on parent canvas, icon/colour, --dry-run |
| 6 | Rename board | cotlet rename_board | milanote-pp-cli boards rename | --dry-run, idempotent |
| 7 | Duplicate board | cotlet duplicate_board | milanote-pp-cli boards duplicate | Use a board as a template |
| 8 | Export board (Milanote exporter) | cotlet export_board | milanote-pp-cli boards export | PDF/PNG via media-service; plus native Markdown export (novel) |
| 9 | Add note | cotlet add_note | milanote-pp-cli notes add | Rich text from Markdown (headings, lists, bold), exact x/y, width |
| 10 | Edit note | cotlet edit_note | milanote-pp-cli notes edit | Replace or append, Markdown input, --dry-run diff |
| 11 | Add link | cotlet add_link | milanote-pp-cli links add | x/y placement |
| 12 | Add to-do list | cotlet add_todo_list | milanote-pp-cli todos add | Native TASK_LIST V2 checkboxes from `- [ ]` lines, x/y |
| 13 | Tick task | cotlet set_task_done | milanote-pp-cli todos check | Match by text or index; uncheck too |
| 14 | Add column | cotlet add_column | milanote-pp-cli columns add | Put existing cards into it, x/y |
| 15 | Move card | cotlet move_card | milanote-pp-cli elements move | Exact x/y or into board/column (cotlet only moves between parents) |
| 16 | Card colour | cotlet set_card_color | milanote-pp-cli elements color | Bulk by id list |
| 17 | Labels list/add | cotlet list_labels/add_label | milanote-pp-cli labels list / labels add | --json |
| 18 | Trash cards | cotlet trash_cards | milanote-pp-cli elements trash | Bulk, --dry-run, restorable |
| 19 | List trash | cotlet list_trash | milanote-pp-cli trash list | --json |
| 20 | Restore card | cotlet restore_card | milanote-pp-cli trash restore | Back to original position |
| 21 | Comments add/edit/delete | cotlet add/edit/delete_comment | milanote-pp-cli comments add / comments edit / comments delete | --json |
| 22 | Sharing get | cotlet get_sharing | milanote-pp-cli share get | --json |
| 23 | Invite / remove editor | cotlet invite_editor/remove_editor | milanote-pp-cli share invite / share remove | --dry-run |
| 24 | Publish / unpublish | cotlet publish_board/unpublish_board | milanote-pp-cli share publish / share unpublish | Prints public URL |
| 25 | Usage / card count | cotlet get_usage | milanote-pp-cli usage | Free-plan 100-card headroom check |
| 26 | Whoami | sniff /api/users/me | milanote-pp-cli whoami | Root board, plan |
| 27 | Add heading | sniff (none of the tools) | milanote-pp-cli headings add | Native transparent heading card, level 1/2, centered |
| 28 | Upload image | sniff presign flow (none of the tools) | milanote-pp-cli images add | Local file → S3 presign → IMAGE element at x/y, width |
| 29 | Element model incl. sketch/document/swatch | serhanceetin Unofficial-API | (behavior in milanote-pp-cli boards get) typed element decoding for all element types | Read-only decoding of every type |
| 30 | Public shared board → JSON | setobox/milanote-api | (behavior in milanote-pp-cli boards get) accepts a board URL or id | Paste a URL from the browser |
| 31 | Auth from browser session | cotlet Chrome CDP cookie import | milanote-pp-cli auth login | `--cookie` / env / cookie-file import, doctor validates |
