## Customer model
The requesting agent planner repeatedly searches Japan accommodation, shortlists high-quality stays, and must explain which exact room and plan justify the cost. Today it must cross-check search banners, room descriptions and plan drawers. Its concrete frustrations are conditional prices, incomplete room evidence and false equivalence.
The same user's room-focused planning branch prioritizes space, bedding, views and private baths. It must distinguish room amenities from a shared onsen before recommending a ryokan.

The root Astra performed this planning directly because the user's explicit topology correction reserves research synthesis, architecture and planning to the root and restricts workers to concrete implementation/test work. This overrides the skill's generic brainstorm-subagent packaging, while retaining its candidate/cut audit. No additional orchestration child was spawned.

## Candidates (pre-cut)
| Candidate | Source | Decision |
|---|---|---|
| Offer equivalence and differences | User correctness brief; real plan cancellation fields | keep |
| Bounded exact-room/plan alternative dates | User purpose; verified dated amounts | keep |
| Room fit with unknown evidence | User bath distinctions; room attributes 16/18 | keep as rooms filter behavior |
| Conditional price explanation | Official points/coupon rules; server amount fields | keep as offer output behavior |
| Search coverage and shortlist readiness | SSR previews vs totalCount; lazy detail requirement | keep as search/rooms output behavior |
| Universal best-hotel score | Requires subjective weights and unsupported claims | kill |
| Automated bargain alerts | Background polling beyond focused request | kill |
| Cross-OTA/direct booking comparison | User integrates Ikyu only | kill |
| Book/cancel or coupon acquisition | Read-only boundary | kill |
| Account stage price optimizer | No authenticated eligibility evidence | kill |
| Review sentiment summaries | LLM dependency and unnecessary review text | kill |
| Unlimited calendar scan | Unbounded request load | kill |

## Survivors and kills
### Survivors
| Feature | Command | Score | Buildability | Proof |
|---|---|---|---|---|
| Offer equivalence | stay compare | 10/10 | hand-code | Joins verified RoomPlanDetailAlt/Amount results by exact stay, occupancy, room, meals and cancellation; differences and unknowns prevent false savings claims. |
| Date alternatives | stay dates | 9/10 | hand-code | Replays one exact source property/room/plan for at most seven explicit check-in dates; outputs freshness and condition differences without treating different dates as equivalent. |
| Room fit | stay rooms | 9/10 | hand-code | Filters source-backed room size, bedding and bath attributes with unknowns retained; no property-to-room inheritance. |
| Price explanation | stay offer | 10/10 | hand-code | Keeps source integer amounts, points scenarios, coupon effects and eligibility separate; final checkout payable stays null when unverified. |
| Shortlist readiness | stay search | 8/10 | hand-code | Combines returned property summaries with source pagination/coverage and missing-detail markers; no hidden per-property fan-out. |

These are useful agent workflows and output refinements, not claims of industry-first functionality. They fit the user's single-source product rather than expanding it into a travel suite.

### Killed candidates
Universal score loses to source review categories; alerts lose to explicit date checks; cross-OTA comparison loses to Ikyu offer equivalence; booking/coupons lose to canonical handoff; member optimization loses to explicit unknown eligibility; sentiment loses to source category scores; unlimited scans lose to seven-date alternatives.
