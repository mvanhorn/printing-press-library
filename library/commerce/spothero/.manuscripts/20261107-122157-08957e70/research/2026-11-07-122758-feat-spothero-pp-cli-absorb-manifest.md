# SpotHero absorb manifest

Sources: the prior hand-written spothero-pp-cli (PLAN.md and code) and the live site. GitHub and web search found no other SpotHero consumer CLI, MCP or SDK; SpotHero's own API is B2B partner only. Approved by the delegated owner decision in the reprint request.

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Search parking by address or point | prior CLI search | spothero-pp-cli find | Local time zone handling, agent JSON |
| 2 | Facility quote and booking preview | prior CLI book preview | spothero-pp-cli book preview | Never charges, shows cap result |
| 3 | Live booking with gates | prior CLI book place | spothero-pp-cli book place | Exact phrase gate, --max-total, no retry on ambiguous result |
| 4 | Cancellation preview | prior CLI cancel preview | spothero-pp-cli cancel preview | Uses refund-eligibility, token bound to preview |
| 5 | Cancellation | prior CLI cancel | spothero-pp-cli cancel | --yes --confirm token gate |
| 6 | List reservations | prior CLI reservations list | (generated endpoint) reservations list | Sync, select, CSV |
| 7 | Get reservation | prior CLI reservations get | (generated endpoint) reservations get | Select, agent JSON |
| 8 | Refund eligibility | site | (generated endpoint) reservations eligibility | Read only |
| 9 | Saved cards | prior CLI account cards | spothero-pp-cli account cards | Last4 and ids only |
| 10 | Saved vehicles | site | spothero-pp-cli account vehicles | Read only |
| 11 | Chrome session import | prior CLI auth login | spothero-pp-cli auth login | Native decryption, verified against the API |
| 12 | Auth status | prior CLI auth status | spothero-pp-cli auth status | Verified against the API, exit 4 when rejected |
| 13 | Health check | prior CLI doctor | spothero-pp-cli doctor | Live credential probe |
| 14 | Cities and airports | site | (generated endpoint) cities list | Reference data |
| 15 | Local history | new | spothero-pp-cli sync | Offline search |

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Best pick | best | 8 | hand-code | Ranks live inventory by total price and walking distance | Top workflow 1 |
| 2 | Booking preflight | preflight | 8 | hand-code | Read-only pass/fail on session, vehicle, card, overlap and cap | Agent persona, thesis |
| 3 | Next pass | next | 8 | hand-code | Soonest upcoming reservation with pass details | Top workflow 3 |
| 4 | Rebook preview | rebook | 8 | hand-code | Re-quotes a past reservation's garage; never places | Top workflow 5 |
| 5 | Refund view | refundable | 7 | hand-code | Eligibility across upcoming reservations | Top workflow 4 |
| 6 | Commute plan | commute | 7 | hand-code | Quotes a recurring series of windows; never places | Top workflow 5 |
| 7 | Regular garages | regulars | 7 | hand-code | Ranks garages from synced reservations | Local store |

Killed: spend, overlaps (became a preflight check), rate-history, airport-trip (became best --airport), window-shift, calendar, vehicle-check, availability, book-best (would weaken the gates).

Stubs: none. Decision on MCP: search, reservations, and previews only. book place and cancel are hidden from MCP because `confirm` set by a model is not human approval.
