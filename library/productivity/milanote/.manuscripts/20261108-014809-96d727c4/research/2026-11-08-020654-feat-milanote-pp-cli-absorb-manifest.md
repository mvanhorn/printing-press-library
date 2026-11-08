# Milanote CLI Absorb Manifest

Sources: cotletdev/milanote-api-mcp-server (30 tools, Python), serhanceetin/Milanote-Unofficial-API, setobox/milanote-api, authenticated browser sniff (2026-11-08).

## Absorb Manifest

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

### Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona served | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|---|
| 1 | Markdown to laid-out board with native headings, lists and checkboxes | `layout md <file> --board <id> [--cols N --gutter G --update]` | 10/10 (Fit 3, Pain 3, Feas 2, Research 2) | the operator, Agent, Writer | hand-code | This parses the Markdown file locally and uses socket.io ELEMENT_CREATE (heading CARD, CARD with tiptap bulletList/orderedList/quote/codeBlock, TASK_LIST, COLUMN) at computed x/y positions, plus ELEMENT_UPDATE for `--update`, to build an evenly spaced column layout with no external dependencies. `// pp:data-source live` | User Vision ("proper/equal spacing … checkboxes, headings, lists"). Brief Top Workflow #2 (the the film scene headings + notes done by hand). Table Stakes: cotlet has no headings or x/y placement and everything lands in Unsorted. | Use this command to push a multi-section Markdown document onto a board as headings, notes and checkbox lists in evenly spaced columns. Do NOT use this command for a single card; use 'notes add' instead. |
| 2 | Even-grid re-space with undo | `layout grid <board> [--cols N --gutter G --from-inbox --dry-run --undo]` | 9/10 (Fit 3, Pain 3, Feas 1, Research 2) | the operator, Agent, Designer | hand-code | This uses `/api/boards?ids=` element positions (location.position, section CANVAS/INBOX) to compute an equal-gutter grid and applies it in one socket.io ELEMENT_MOVE_MULTI, saving the prior positions in the local store for `--undo`, with no external dependencies. Free-text card heights are estimated from a fixed per-type width and line count. `// pp:data-source live` | User Vision (auto-arrange, align, distribute, even grid/gutters). Table Stakes: cotlet's move_card is single-card and has no layout. Brief Product Thesis: "every existing tool … ignores layout". | Use this command to re-space cards already on a board into an even grid. Do NOT use this command to place one card at an exact x/y; use 'elements move' instead. |
| 3 | Storyboard / moodboard image grid | `images grid <dir\|files...> --board <id> [--cols N --gutter G --caption-from filename --number]` | 8/10 (Fit 3, Pain 2, Feas 2, Research 1) | the operator, Designer | hand-code | This uses POST `/api/elements/attachments/presign` for each S3 upload and socket.io ELEMENT_CREATE (IMAGE, plus a caption CARD) at grid positions computed from each local file's pixel dimensions, with no external dependencies. `// pp:data-source live` | Brief Users (the operator's Storyboards board; designers' moodboards). Table Stakes: cotlet has no image upload; image upload at x/y came only from the sniff. | Use this command to upload a folder or list of images as an evenly spaced, numbered grid with optional captions. Do NOT use this command for one image at a chosen x/y; use 'images add' instead. |
| 4 | Board tree to Markdown outline in reading order | `outline <board> [--depth N --out file.md]` | 10/10 (Fit 3, Pain 3, Feas 2, Research 2) | the operator, Writer, Agent | hand-code | This walks the board tree from the local SQLite mirror (or live `/api/boards?ids=`), orders elements by column membership and canvas x/y, and converts tiptap/ProseMirror content to Markdown with headings, lists and `- [x]` items, with no external dependencies. It calls `hintIfUnsynced`/`hintIfStale`. `// pp:data-source auto` | Brief Top Workflow #1 ("pull the a short film script and shot list into files"). Build Priority 4 (`board export --md`). cotlet's export_board only drives Milanote's own PDF/image exporter. | Use this command to get a board tree as Markdown text in canvas reading order. Do NOT use this command for Milanote's own PDF/image export; use 'boards export' instead. For raw typed element JSON use 'boards get'. |
| 5 | Layout lint | `layout check <board>` | 6/10 (Fit 2, Pain 2, Feas 1, Research 1) | Agent, the operator | hand-code | This reads element positions, sections and text content from the local mirror (or live `/api/boards?ids=`) and applies mechanical rules: off-grid coordinates, unequal column gutters, bounding-box overlap, cards in INBOX, and paragraphs starting with `#`, `-` or `[ ]`. It emits JSON findings with element ids, with no external dependencies. It calls `hintIfUnsynced`/`hintIfStale`. `// pp:data-source auto` | User Vision (equal spacing; use native headings and lists instead of plain paragraphs). The chat agent can't confirm layout visually (brief User Vision: agent-driven instead of browser automation). | Use this command to report spacing problems without changing anything. Do NOT use it to fix them; use 'layout grid' instead. |
| 6 | Open tasks across all boards | `todos open [--board <id> --recursive]` | 7/10 (Fit 2, Pain 2, Feas 2, Research 1) | the operator, Agent | hand-code | This queries TASK_LIST items with an unchecked state across all boards in the local SQLite mirror and joins them to board titles for a path breadcrumb. It returns element ids usable by `todos check`, with no external dependencies. It calls `hintIfUnsynced`/`hintIfStale` and uses the drain-first rows pattern. `// pp:data-source local` | Brief Users (shot list and prep for short-film development). User Vision ("utilizing the tools available for checkboxes"). | none |


Stubs: none.

### User-added at Phase Gate 1.5 (the operator, 2026-11-08 10:16: "yes add all please")
| # | Feature | Command | Score | Persona served | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|---|
| 7 | Two-way Markdown ↔ board sync | `mirror <file.md> --board <id> [--watch --once --direction push\|pull\|both]` | 9/10 | the operator, Writer | hand-code | Keeps a pairing record (element ids + content hashes per section) in the local store; diffs file vs board since last run and applies `layout md --update` style writes or `outline` style rewrites; conflicts reported, never silently overwritten. `// pp:data-source live` | User idea; Top Workflows #1+#2 combined | Use this command to keep one Markdown file and one board in step over time. Do NOT use it for a one-off push; use 'layout md' instead, or 'outline' for a one-off read. |
| 8 | Board snapshots + diff | `snapshot save <board>` / `snapshot diff <board> [--from <snap> --to <snap>\|live]` / `snapshot list` | 7/10 | the operator, Agent | hand-code | Stores full element JSON per snapshot in SQLite; diff reports added/removed/moved/edited/ticked elements with ids. `// pp:data-source local` (diff vs live is auto) | User idea; script revision workflow | none |
| 9 | Board templates | `template save <board> --out kit.yaml` / `template apply kit.yaml --parent <id> --name <title>` | 7/10 | the operator, Designer | hand-code | Serialises board structure (headings, columns, notes, task lists, sub-boards, relative positions) to YAML with content optional; apply recreates it with fresh ids. `// pp:data-source live` | User idea (short-film kit) | Use this command to stamp out a reusable board structure from a file. Do NOT use it to copy one existing board as-is; use 'boards duplicate' instead. |
| 10 | CSV → card grid | `layout csv <file.csv> --board <id> [--title-col --cols N --gutter G]` | 6/10 | the operator, Designer | hand-code | Each row becomes a rich-text note (title as heading line, other columns as bold-label lines), numbered and placed on an even grid. `// pp:data-source live` | User idea (shot list spreadsheet) | Use this command for tabular rows. Do NOT use it for prose documents; use 'layout md' instead. |
| 11 | One-command login from Chrome | `chrome-login [--port 9222]` | 6/10 | All public users | hand-code | Connects to a running Chrome over the DevTools protocol, calls Storage.getCookies, keeps only the Milanote session cookie, stores it 0600 in the config dir. `// pp:data-source computed` | User idea; cotlet uses the same CDP approach | none |
