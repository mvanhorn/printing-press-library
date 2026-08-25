# Human-Goat CLI Brief

One binary, `goat`, that dispatches real-world tasks to real humans across two backends and
tracks them to completion: **TaskRabbit** for in-person local labor and **Magic** for remote
errands. Headline capability: autonomous soup-to-nuts hire on TaskRabbit against the card on
file, made safe by a verified cancel + spend cap.

## API Identity
- Two human-labor networks behind one common task model.
- **TaskRabbit** (primary): IKEA-owned local-services marketplace. Cookie auth (session + XSRF-TOKEN
  from logged-in Chrome; reCAPTCHA on login so never headless). REST `/api/v3/*` reads + tRPC
  `/next-api/trpc/*` BFF funnel. Metro-scoped (account metro 1053, SF Bay Area). Card on file present.
- **Magic** (secondary): remote human-assistant API (getmagic.com). `x-api-key` REST:
  POST /request, GET /request/{id}, POST /conversation. Status flows PENDING->ONGOING->COMPLETED;
  answer arrives in the conversation array, not `result`.

## Users (concrete)
- **Matt, the operator (or an agent acting as Matt):** wants to say "go be a mover on this date with
  good reviews" and have it done end to end — the agent manages the human. Runs `hire`, `track`,
  `cancel`, `dispatch` from the terminal or Claude Code. Today he taps through the TaskRabbit app one
  hidden-fee Tasker at a time, or click-drives the Magic console with no terminal surface.
- **The comparison shopper:** before booking a mover/mounter/cleaner, wants the honest all-in price
  and review quality of every available Tasker across a date window, side by side. Today the app
  shows one Tasker at a time and hides fees until checkout.
- **The delegator running errands:** needs a phone call made, a business's hours confirmed, an online
  booking placed, or data entered — remote work a human assistant can do. Today he click-drives the
  Magic console and polls for the answer.
- **The repeat hirer:** found a Tasker who worked out; wants to re-book them for a new date without
  re-searching, and to watch for a near-term opening.

## Top Workflows (named rituals)
1. **Autonomous hire (F1):** `goat hire movers --on saturday --min-rating 4.9 --max-total 200` ->
   search -> rank by all-in price x review quality -> spend-cap check -> checkout on card on file ->
   print booking id + total. No prompt.
2. **Undo (F2):** `goat cancel <booking-id>` -> cancel mutation -> re-read status -> confirm cancelled
   + report free-window/fee. The safety valve that makes hands-off checkout tolerable.
3. **Comparison shop:** `goat best "help moving" --on sat --min-rating 4.9` / `goat compare` -> honest
   all-in ranked Taskers the app hides.
4. **Remote errand (F3):** `goat call 5209076052 "when does the jewelry store open"` -> Magic request
   -> `goat track <id>` until terminal -> answer read from conversation.
5. **Cross-source dispatch:** `goat dispatch "<task>"` routes to Magic (remote-doable) or TaskRabbit
   (in-person) by task shape, with `--via` override.
6. **Spend analytics:** `goat spend --by source|category|tasker|month` -> SQL over local
   booking/invoice/Magic-task history with true all-in effective $/hr.
7. **Rebook / watch:** re-hire a favorite; poll for a near-term qualifying opening.

## Reachability Risk
- LOW. TaskRabbit: standard_http + cookie; REST reads 200 with cookies alone; no WAF; reCAPTCHA only
  on login (handled by cookie import). Magic: plain REST + x-api-key (working key on file).

## Data Layer
- Common `Task {id, source, status, title, created_at, ...}`. Entities: tasker, category, metro,
  booking, appointment, invoice, magic_request. FTS over taskers/categories; SQL over
  bookings+invoices+magic tasks for `track`/`spend` and ranking.

## Killer differentiators (only the CLI can do)
- **Autonomous checkout** on the card on file with no pre-charge prompt — gated by post-hoc verified
  cancel + spend cap (empirically: free cancel >=24h before appointment; 1hr refundable deposit).
- **Honest all-in price** everywhere: empirically $33.33 base -> $44.66 charged (~+34%), CA/MA
  service-fee-only. The app hides this.
- **One surface over two human networks** with task-shape routing.
- Verified cancel (re-reads status), availability watch, first-class rebook, cross-source spend.

## U4 empirical findings (live capture)
- Funnel: details -> recommendations (56 taskers, `poster_hourly_rate_cents` + rating/tasks/elite/
  availability) -> schedule (slots) -> confirm (all-in total, card on file). Cancel mutation =
  `page.tasks.cancelTask` (discriminated union on `type: single`). Commit mutation captured during
  the authorized real hire+cancel round-trip.

## User Vision (from plan)
Soup-to-nuts autonomous hire: "go be a mover on this date with good reviews" — the agent searches,
filters by reviews, books end to end against the card on file, and manages the human. Autonomous
checkout is gated NOT by a pre-charge confirm but by a reliable post-hoc verified cancel + a spend
cap. Cross-source verbs (dispatch/track/spend) let the user ignore which backend does the work.
Deferred: multi-Tasker crew, recurring planner, sending TR messages, editing payment method.

## Product Thesis
- Name: `goat` (binary human-goat-pp-cli; display Human-Goat).
- Why it exists: say what you want done to a human and have it done — hire in person via TaskRabbit
  with honest pricing and autonomous, undoable checkout, or dispatch remote errands to Magic — from
  one agent-native binary.
