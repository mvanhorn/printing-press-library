## Customer model

**Persona 1: the operator, a short-film developer who works through an agent.** the operator develops short films (for example a short film) on one Milanote board tree with sub-boards for Story, Characters, Shot list, Look & Rules, Storyboards and Locked script. He is logged in through Google in Chrome on the Mac mini and talks to Claude over chat from his phone.
- **Today (without this CLI):** The agent drives Milanote by slow phone or browser UI automation (the brief and the "Milanote on iPhone" memory both describe this). For a film he laid out scene headings plus one note per scene by hand. The existing MCP (cotlet) drops every card into the Unsorted panel with no positions.
- **Weekly ritual:** Revises the outline or script in Markdown, then asks the agent to push the new version onto the board as headings, notes and checkboxes. Pulls the script and shot list back out as files for the next writing pass. Ticks off shot-list and prep tasks.
- **Frustration:** Cards land unordered and unevenly spaced. Plain paragraphs get used where native headings, lists and checkboxes belong. Getting the board back out as clean text is manual. In his words: "a keen focus on organization, proper/equal spacing and utilizing the tools available for checkboxes, headings, lists."

**Persona 2: a chat-driven agent, the agent that operates the board.** It runs headless on the mini and is the actual caller of the CLI on the operator's behalf.
- **Today (without this CLI):** It has no API. It screenshots and taps the iPhone or browser, can't see exact positions, and can't confirm that spacing is even without a picture.
- **Weekly ritual:** Turns the operator's chat requests ("put scenes 4-9 on the board", "add these storyboard frames", "what's left on the shot list?") into board edits and read-backs.
- **Frustration:** It needs structured JSON, exact coordinates and a way to check layout by numbers rather than by looking. A bad bulk move costs a slow manual cleanup.

**Persona 3: a designer or art director building moodboards and storyboards.** This is the brief's "designers, filmmakers" user who works mostly with image grids.
- **Today (without this CLI):** Drags dozens of reference images onto the canvas one at a time, then nudges them by eye into rows and adds captions by hand.
- **Weekly ritual:** Collects a folder of frames or references per project, drops them on a board in order, captions or numbers them, and shares the board.
- **Frustration:** Even grids and consistent gutters take a lot of hand work. There is no bulk "folder to grid" option, and numbering frames to match a shot list is tedious.

**Persona 4: a writer outlining a script or novel.** This is the brief's "writers, writing outlines" user.
- **Today (without this CLI):** Keeps the outline in a text editor and on a Milanote board at the same time and copies between them by hand.
- **Weekly ritual:** Restructures acts and beats, keeps task lists of rewrites, and exports the outline to share or archive.
- **Frustration:** Milanote's own exporter gives PDF or image, not editable Markdown in reading order. Nested boards make the full outline hard to see in one place.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Long Description | Inline kill/keep check |
|---|---|---|---|---|---|---|---|
| C1 | Markdown to laid-out board | `layout md <file> --board <id> [--cols N --gutter G --update]` | Parses Markdown: `#`/`##` become native heading cards, each section body becomes a rich-text note (native bullet/ordered lists, quote, code), `- [ ]` lines become native TASK_LIST items. Sections go into evenly spaced COLUMN elements on the canvas. `--update` matches existing headings and edits in place instead of duplicating. | the operator, Agent, Writer | (e) (b) (a) | Use this command to push a multi-section Markdown document onto a board as headings, notes and checkbox lists in evenly spaced columns. Do NOT use this command for a single card; use 'notes add' instead. | Pass. Live write over socket.io ELEMENT_CREATE/UPDATE. Columns stack their children natively, so card heights don't need to be estimated. |
| C2 | Even-grid re-space | `layout grid <board> [--cols N --gutter G --from-inbox --dry-run]` | Re-spaces cards already on a board (or pulls them out of Unsorted with `--from-inbox`) into an even grid with equal gutters in one ELEMENT_MOVE_MULTI. Saves the previous positions so `--undo` can restore them. | the operator, Agent, Designer | (e) (b) | Use this command to re-space cards already on a board into an even grid. Do NOT use this command to place one card at an exact x/y; use 'elements move' instead. | Pass. Heights for free text cards are estimated (fixed width per type, line count), so verification is flagged for manual QA. |
| C3 | Align / distribute | `layout align <ids> --left/--top`, `layout distribute <ids> --h/--v` | Aligns selected cards on one edge or spaces them equally. | Agent, Designer | (e) | Overlaps C2 | Sibling of C2: a subset of grid math. |
| C4 | Layout lint | `layout check <board>` | Reports off-grid x/y, uneven column gutters, overlapping boxes, cards left in Unsorted, and paragraph cards that look like they should be headings or lists (lines starting with `#`, `-`, `[ ]`). Read-only. Emits JSON findings with element ids. | Agent, the operator | (e) (c) | Use this command to report spacing problems without changing anything. Do NOT use it to fix them; use 'layout grid' instead. | Pass. Mechanical rules only, no LLM. The overlap rule shares C2's height-estimation caveat, so it is flagged low-confidence. |
| C5 | Position undo | `layout undo <board>` | Restores the positions saved before the last bulk move. | Agent | (e) | Overlaps C2 | Folded into C2 as `--undo`. |
| C6 | Storyboard / moodboard grid | `images grid <dir\|files...> --board <id> [--cols N --gutter G --caption-from filename --number]` | Uploads images in sorted order via S3 presign, places them on an even grid using each file's real pixel dimensions, and optionally adds a numbered or captioned note under each frame. | the operator, Designer | (a) (b) | Use this command to upload a folder or list of images as an evenly spaced, numbered grid with optional captions. Do NOT use this command for one image at a chosen x/y; use 'images add' instead. | Pass. Heights are computed exactly from local image dimensions. |
| C7 | Board tree to Markdown outline | `outline <board> [--depth N --out file.md]` | Walks a board and its sub-boards and renders every card as Markdown in canvas reading order (columns left to right, cards top to bottom). Preserves native headings, lists and `- [x]` checkboxes. Sub-boards become nested sections. | the operator, Writer, Agent | (a) (c) | Use this command to get a board tree as Markdown text in canvas reading order. Do NOT use this command for Milanote's own PDF/image export; use 'boards export' instead. For raw typed element JSON use 'boards get'. | Pass. Distinct from the absorbed `boards export` (Milanote's renderer) and `boards get` (raw JSON). |
| C8 | Open tasks across boards | `todos open [--board <id> --recursive]` | Lists every unchecked TASK_LIST item across the local mirror, with its board path, list title and element id, ready for `todos check`. | the operator, Agent | (c) | none | Pass. Local SQLite, `hintIfUnsynced`/`hintIfStale`. |
| C9 | Inbox placement | `inbox place <board>` | Moves cards from the Unsorted panel onto the canvas. | Agent | (b) | Overlaps C2 | Sibling of C2: covered by `layout grid --from-inbox`. |
| C10 | Pre-production scaffold | `boards scaffold <name> --template film` | Creates a film project tree (Story, Characters, Shot list, Look & Rules, Storyboards, Script sub-boards with headings). | the operator | (a) (b) | n/a | Mostly a hardcoded template. Duplicating a real template board (absorbed `boards duplicate`) or `layout md` on a skeleton file covers it. |
| C11 | Shot list export | `shots <board> --csv` | Parses a shot-list board into shot number, description and done status. | the operator | (a) | n/a | Assumes a board convention the brief doesn't define, so it can't be verified. `outline` plus `todos open` cover the data. |
| C12 | Recent changes | `changes --since 7d` | Lists elements modified since a date from the mirror (`meta.modifiedTime`). | the operator | (c) | n/a | Overlaps framework `sync --since` and `tail`. The brief shows no collaborators, so there is no evidence anyone needs it. |
| C13 | Duplicate cards | `dupes` | Hashes note text across boards to find duplicate cards. | Writer | (c) | n/a | Speculative. No research evidence and no weekly use. |
| C14 | ASCII canvas map | `layout map <board>` | Draws card boxes as a text diagram so a phone or agent can see the layout. | Agent | (e) | n/a | Sibling of C4. Numeric findings are more useful to an agent than a picture, and estimated heights make the drawing unreliable. |
| C15 | Link health | `links check` | Fetches every LINK card's URL and reports dead links. | Designer | (c) | n/a | Kill: external service (arbitrary third-party sites). |

## Survivors and kills

**C1 `layout md`**
1. **Weekly use:** Yes. This is the operator's main loop: revise Markdown, push to the board. It is how the film was built by hand.
2. **Wrapper vs leverage:** Leverage. It combines a Markdown parse, a ProseMirror doc build, column grid math and many ELEMENT_CREATEs in one call, plus idempotent `--update` matching.
3. **Transcendence proof:** Service-specific content pattern (canvas layout computation plus native heading, list and task nodes). cotlet places everything in Unsorted.
4. **Sibling kill:** It absorbs C10 (a scaffold is just a skeleton `.md`).
5. **Buildability:** `hand-code`.
6. **Long description:** References `notes add`, which is absorbed and survives. Valid.

**C2 `layout grid`**
1. **Weekly use:** Yes. Every board the operator touches by hand drifts. The agent tidies after edits and empties Unsorted.
2. **Wrapper vs leverage:** Leverage. It reads all positions, computes an equal-gutter grid, sends one ELEMENT_MOVE_MULTI, and saves undo state.
3. **Transcendence proof:** Layout computation, a content pattern specific to this service. No existing tool does it.
4. **Sibling kill:** It absorbs C3 (align/distribute), C5 (undo) and C9 (inbox placement).
5. **Buildability:** `hand-code`.
6. **Long description:** References `elements move`, which is absorbed and survives. Valid.

**C4 `layout check`**
1. **Weekly use:** Yes. The agent runs it after every `layout md`, `layout grid` or `images grid` to confirm the result without a screenshot.
2. **Wrapper vs leverage:** Leverage. It applies geometric and content rules across every element. No endpoint provides this.
3. **Transcendence proof:** Agent-shaped output (JSON findings with ids), computed from the mirror or a live board.
4. **Sibling kill:** It absorbs C14 (ASCII map).
5. **Buildability:** `hand-code`.
6. **Long description:** References `layout grid`, which survives. Valid.

**C6 `images grid`**
1. **Weekly use:** Yes for Storyboards and Look boards during pre-production. Designers do this for every moodboard.
2. **Wrapper vs leverage:** Leverage. It batches presign uploads, IMAGE creates, dimension-aware grid math and caption notes.
3. **Transcendence proof:** Layout computation using exact local image dimensions.
4. **Sibling kill:** None killed. It stays separate from `images add`, which places one image at a chosen x/y.
5. **Buildability:** `hand-code`.
6. **Long description:** References `images add`, which is absorbed and survives. Valid.

**C7 `outline`**
1. **Weekly use:** Yes. Brief Top Workflow #1 is pulling the the film script and shot list into files for each writing pass.
2. **Wrapper vs leverage:** Leverage. It walks the tree recursively, sorts cards into reading order by position and column, and converts ProseMirror to Markdown with checkboxes.
3. **Transcendence proof:** Agent-shaped output plus a reading-order computation from canvas positions. Milanote's exporter only produces PDF or image.
4. **Sibling kill:** It absorbs C11 (shot list export).
5. **Buildability:** `hand-code`.
6. **Long description:** References `boards export` and `boards get`, both absorbed and surviving. Valid.

**C8 `todos open`**
1. **Weekly use:** Yes. "What's left on the shot list or prep?" is a recurring chat question.
2. **Wrapper vs leverage:** Leverage. It runs a local SQLite query across every board. The API has no cross-board task query.
3. **Transcendence proof:** Local SQLite cross-board query that returns ids ready for `todos check`.
4. **Sibling kill:** It absorbs C12 (recent changes) as a lower-value cross-board read.
5. **Buildability:** `hand-code`.
6. **Long description:** none.

### Survivors

| # | Feature | Command | Score | Persona served | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|---|
| 1 | Markdown to laid-out board with native headings, lists and checkboxes | `layout md <file> --board <id> [--cols N --gutter G --update]` | 10/10 (Fit 3, Pain 3, Feas 2, Research 2) | the operator, Agent, Writer | hand-code | This parses the Markdown file locally and uses socket.io ELEMENT_CREATE (heading CARD, CARD with tiptap bulletList/orderedList/quote/codeBlock, TASK_LIST, COLUMN) at computed x/y positions, plus ELEMENT_UPDATE for `--update`, to build an evenly spaced column layout with no external dependencies. `// pp:data-source live` | User Vision ("proper/equal spacing … checkboxes, headings, lists"). Brief Top Workflow #2 (the the film scene headings + notes done by hand). Table Stakes: cotlet has no headings or x/y placement and everything lands in Unsorted. | Use this command to push a multi-section Markdown document onto a board as headings, notes and checkbox lists in evenly spaced columns. Do NOT use this command for a single card; use 'notes add' instead. |
| 2 | Even-grid re-space with undo | `layout grid <board> [--cols N --gutter G --from-inbox --dry-run --undo]` | 9/10 (Fit 3, Pain 3, Feas 1, Research 2) | the operator, Agent, Designer | hand-code | This uses `/api/boards?ids=` element positions (location.position, section CANVAS/INBOX) to compute an equal-gutter grid and applies it in one socket.io ELEMENT_MOVE_MULTI, saving the prior positions in the local store for `--undo`, with no external dependencies. Free-text card heights are estimated from a fixed per-type width and line count. `// pp:data-source live` | User Vision (auto-arrange, align, distribute, even grid/gutters). Table Stakes: cotlet's move_card is single-card and has no layout. Brief Product Thesis: "every existing tool … ignores layout". | Use this command to re-space cards already on a board into an even grid. Do NOT use this command to place one card at an exact x/y; use 'elements move' instead. |
| 3 | Storyboard / moodboard image grid | `images grid <dir\|files...> --board <id> [--cols N --gutter G --caption-from filename --number]` | 8/10 (Fit 3, Pain 2, Feas 2, Research 1) | the operator, Designer | hand-code | This uses POST `/api/elements/attachments/presign` for each S3 upload and socket.io ELEMENT_CREATE (IMAGE, plus a caption CARD) at grid positions computed from each local file's pixel dimensions, with no external dependencies. `// pp:data-source live` | Brief Users (the operator's Storyboards board; designers' moodboards). Table Stakes: cotlet has no image upload; image upload at x/y came only from the sniff. | Use this command to upload a folder or list of images as an evenly spaced, numbered grid with optional captions. Do NOT use this command for one image at a chosen x/y; use 'images add' instead. |
| 4 | Board tree to Markdown outline in reading order | `outline <board> [--depth N --out file.md]` | 10/10 (Fit 3, Pain 3, Feas 2, Research 2) | the operator, Writer, Agent | hand-code | This walks the board tree from the local SQLite mirror (or live `/api/boards?ids=`), orders elements by column membership and canvas x/y, and converts tiptap/ProseMirror content to Markdown with headings, lists and `- [x]` items, with no external dependencies. It calls `hintIfUnsynced`/`hintIfStale`. `// pp:data-source auto` | Brief Top Workflow #1 ("pull the a short film script and shot list into files"). Build Priority 4 (`board export --md`). cotlet's export_board only drives Milanote's own PDF/image exporter. | Use this command to get a board tree as Markdown text in canvas reading order. Do NOT use this command for Milanote's own PDF/image export; use 'boards export' instead. For raw typed element JSON use 'boards get'. |
| 5 | Layout lint | `layout check <board>` | 6/10 (Fit 2, Pain 2, Feas 1, Research 1) | Agent, the operator | hand-code | This reads element positions, sections and text content from the local mirror (or live `/api/boards?ids=`) and applies mechanical rules: off-grid coordinates, unequal column gutters, bounding-box overlap, cards in INBOX, and paragraphs starting with `#`, `-` or `[ ]`. It emits JSON findings with element ids, with no external dependencies. It calls `hintIfUnsynced`/`hintIfStale`. `// pp:data-source auto` | User Vision (equal spacing; use native headings and lists instead of plain paragraphs). The chat agent can't confirm layout visually (brief User Vision: agent-driven instead of browser automation). | Use this command to report spacing problems without changing anything. Do NOT use it to fix them; use 'layout grid' instead. |
| 6 | Open tasks across all boards | `todos open [--board <id> --recursive]` | 7/10 (Fit 2, Pain 2, Feas 2, Research 1) | the operator, Agent | hand-code | This queries TASK_LIST items with an unchecked state across all boards in the local SQLite mirror and joins them to board titles for a path breadcrumb. It returns element ids usable by `todos check`, with no external dependencies. It calls `hintIfUnsynced`/`hintIfStale` and uses the drain-first rows pattern. `// pp:data-source local` | Brief Users (shot list and prep for short-film development). User Vision ("utilizing the tools available for checkboxes"). | none |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Align / distribute (`layout align`, `layout distribute`) | A subset of the grid math. Two extra commands would split a single layout entry point. | `layout grid` |
| Position undo (`layout undo`) | Only meaningful after a bulk move, so it is folded in as `layout grid --undo`. | `layout grid` |
| Inbox placement (`inbox place`) | The same operation as re-spacing with Unsorted as the source, so it is folded in as `layout grid --from-inbox`. | `layout grid` |
| Pre-production scaffold (`boards scaffold`) | A hardcoded template. Covered by absorbed `boards duplicate` of a real template board, or by `layout md` on a skeleton file. | `layout md` |
| Shot list export (`shots --csv`) | Depends on an undefined board convention, so it can't be verified. The data is already available as Markdown and task rows. | `outline` |
| Recent changes (`changes --since`) | Overlaps framework `sync --since` and `tail`. No evidence of collaborators who need a change feed. | `todos open` |
| Duplicate cards (`dupes`) | Speculative. No research evidence and no weekly use. | `layout check` |
| ASCII canvas map (`layout map`) | Estimated heights make the drawing unreliable. Numeric JSON findings serve the agent better. | `layout check` |
| Link health (`links check`) | External service: fetches arbitrary third-party URLs that the spec doesn't cover. | `outline` |
