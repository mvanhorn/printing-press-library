# WriteStack & Substack-Growth-Tool Research

Evidence-driven catalog of every WriteStack feature, every adjacent growth tool, and every repeated tactic in successful Substack writers' playbooks. Drives the absorb manifest for an agent-operated Substack growth CLI.

Source extraction: WriteStack feature copy was pulled directly from its Next.js client bundles (`/_next/static/chunks/*.js`) at writestack.io, since the SPA shell renders empty HTML. Pricing constants extracted from JS object literals. Adjacent tools and growth playbook tactics pulled from indexed third-party content.

---

## 1. WriteStack feature inventory

WriteStack canonical positioning: "Schedule a month of Substack Notes in minutes. Learn what converts with deep analytics and use a personalized AI to come up with Notes." (meta description, writestack.io homepage).

Tagline of headline section: "Ready to stop doing Substack the hard way?" (pricing-section header constant).

### 1.1 Marquee features (verbatim from JS bundle)

| # | Feature (badge) | What it does (extracted copy) | Sub-features | Persona | Agent-automatable? |
|---|---|---|---|---|---|
| 1 | **Smart Scheduling** | "Post when your audience is actually online... Batch-schedule a week or a month of Notes in one sitting. WriteStack analyzes when your audience engages most and suggests optimal posting times — so every Note gets the reach it deserves." | Bulk Scheduling; Personalized Time Suggestions; Posting Habit Heatmap | Solo, Ghostwriter, Network | YES — pure automation |
| 2 | **AI Note Generator** | "Generate Notes that sound like you — not like AI... WriteStack learns from your published content, your writing style, and your personal prompt to generate Notes that actually sound like you wrote them. Tighten a draft, rewrite an article as a Note, brainstorm variations, or go from a rough idea to a polished Note in seconds." | Tone-Matched Generation; Rewrites & Variations; Generate From Articles & Files | Solo, Ghostwriter | YES — AI-assisted; output review optional |
| 3 | **Advanced Statistics** | "See which Notes actually bring subscribers... Substack buries your Note stats — you'd have to open each one individually to see what happened. WriteStack pulls it all into one dashboard. See which Notes convert free readers into subscribers, which formats get the most engagement, and how your growth connects to your posting habits." | Conversion Tracking; Growth Correlation Graphs; Format & Timing Breakdown | Solo, Ghostwriter, Network | YES — read-only |
| 4 | **Activity Center** | "Handle all your Substack notifications in 15 minutes... A streamlined notification feed... Reply with keyboard shortcuts, auto-dismiss on reply, and blow through your notifications without the friction. What used to take 1-2 hours now takes 10-20 minutes." | Keyboard Shortcuts; Auto-Like on Reply; Auto-Dismiss on Reply; Streamlined Feed | Solo, Ghostwriter | PARTIAL — agent can list and draft replies; sending replies is human-in-loop by default |
| 5 | **Follow Page** | "Build a personal feed of the creators who matter most... Stop refreshing Substack's noisy feed... See their latest Notes, reply directly, and build real relationships." Tagline: "The creators who grow fastest on Substack aren't the best writers — they're the best community members." | Curated Feed; Direct Engagement; No Algorithm | Solo, Network | YES — read; engagement actions are human-in-loop |
| 6 | **AI Chat Mode** | "An AI that knows your Substack inside and out... has context on your Notes, posts, analytics, and writing style. It's not a generic chatbot — it can pull up your data, spot patterns in what's working, and help you plan content based on real performance." | Context-Aware Responses; Built-In Web Research; Analytics-Driven Suggestions | Solo, Ghostwriter | YES — read + AI |
| 7 | **Advanced Notes Search ("Inspiration")** | "Study what's working across all of Substack... Search across millions of Substack Notes. Filter by reactions, comments, reposts, date range, or keywords." Listed as "Notes Inspirations" in pricing tooltips: "Get personalized Notes inspirations based on millions of top-performing Notes." | Search Across Millions of Notes; Text Search; Performance-Focused Filters; Per-niche relevance | Solo, Ghostwriter, Network | YES — read-only |
| 8 | **MCP / Claude Integration** | "Run your Substack from Claude. Connect WriteStack to Claude once. Then just talk... WriteStack exposes your entire notes workflow as tools Claude can call. Say 'write and schedule 10 notes in my voice' — Claude drafts them and schedules them directly to WriteStack." | Draft and schedule in bulk; Manage your notes by talking; Automate your entire notes workflow | Solo, Ghostwriter | YES — this IS the agent surface |

### 1.2 Sub-features observed in code surface (Prisma schema strings + UI fragments)

Each is a verifiable capability surfaced by the WriteStack client; included even when not promoted on landing copy.

| Feature | Evidence | Persona | Agent-automatable? |
|---|---|---|---|
| **Auto-restack on publish** ("Reshare to increase visibility") | UI string: `key: "autoRetweet", title: "Auto restack", description: "Reshare to increase visibility", tooltip: "Automatically restacks your note after it's published to give it a second wave of visibility with your audience."` (rendered with `opacity-50` — gated behind a plan/flag) | Solo | YES — pure automation |
| **Auto-like own note on schedule fire** | DB field `shouldLike` on scheduled note; `likedAt`/`likeFailedReason` audit fields; user setting `shouldLikeSelfNotes` on `UserMetadata` | Solo, Ghostwriter | YES — pure automation |
| **Streak tracking + reminder** | `fetchingStreak`, `setStreak`, user setting `shouldSendStreakReminder`, `sendStreakReminderAt` | Solo | YES — local automation; the *reminder* is email push, not an agent job |
| **Missed-schedule recovery** | User settings `shouldSendMailOnMissedSchedule`, `shouldSendAfterMissedSchedule` (post-publish-after-missing-window control) | Solo, Ghostwriter | YES |
| **Best-time-to-publish recommender** | Statistics store: `bestTimeToPublish`; UI prompt: "Click create suggested schedule and I'll build a posting plan for you based on the best times to post Notes." | Solo, Ghostwriter | YES — read + recommendation |
| **Top engagers leaderboard** | Statistics store: `topEngagers` (per user) | Solo, Ghostwriter | YES — read-only |
| **Note categories / tags** | DB tables: `NoteCategory`, `NoteCategoryAssignment`; per-schedule `categoryIdsToPostWith` | Solo, Ghostwriter | YES |
| **Note templates with goals** | `NoteTemplateGoal` enum: `paidSubscribers`, `freeSubscribers`, `restacks`, `likes`, `comments`, `clicks`, `balanced`. `NoteTemplateDifficulty`: `beginner`, `intermediate`, `advanced` | Solo | YES |
| **Reply / restack / quote-restack drafts** | DB table: `replyJsonBody`, `restackJsonBody`, `restackBody`, `repliedAt`, `restackedAt` keyed to source `substackNoteId` | Solo, Ghostwriter | YES — drafting; sending is human-in-loop default |
| **Comment drafting on a target post** | DB fields: `commentBody`, `commentBodyJson`, `commentPublicationId`, `commentPostId`, `commentUserId`, plus `startOffset`/`endOffset`/`text`/`isAutoSelection` for inline-anchored comments | Solo | YES drafting; human-in-loop send |
| **Activity feed types** | `ActivityType` enum: `feed`, `comment`, `restack`, `restackWithComment`, `newFreeSubscriber`, `newPaidSubscriber` | Solo, Ghostwriter | YES — read |
| **Cross-post via Buffer channels** | DB fields: `bufferChannelIdsToPostTo`, `bufferChannelIds`, `bufferChannelIdsSent` | Solo | YES — pure automation |
| **Article/Idea outline generator** | DB table: `Idea` with `topic`, `title`, `subtitle`, `outline`, `inspiration`, `body`, `bodyHistory`, `modelUsedForIdeas`, `modelUsedForOutline`. Testimonial: "outline generator is pure genius" | Solo | YES — AI-assisted |
| **Note generation from articles/files** | "Pick up to 10 articles and generate Notes based on them. Perfect for promoting your Posts." (Indiepreneur review) | Solo | YES |
| **Search with sort options** | `Relevance / Date / Most Liked / Most Commented / Most Restacked` | All | YES — read |
| **Onboarding tour / product tour** | PostHog product-tour analytics events present | All (UX, not agent) | NO — human flow |
| **Heatmap visualization** (`/heatmap`) | "See your notes activity and streaks visualized in a GitHub-style heatmap." Public free-tier tool | Solo | YES — read |
| **Super Fans page** (`/fans`) | "Find out who are your top readers." Public free-tier tool | Solo, Ghostwriter | YES — read |
| **Article Teaser Generator** (`/note-generator/post`) | "Generate a teaser for your article." Public free-tier tool | Solo | YES — AI-assisted |
| **Profile Scorer** (`profile-score.writestack.io`) | "Score your profile based on your writing style." External tool | Solo | YES — AI-assisted read |
| **Ghostwriter / Multi-Client mode** | DB tables: `Ghostwriter`, `GhostwriterAccess`, `GhostwriterToken`. Subscription field `isGhostwriter`. API endpoints: `POST /api/ghost-writer/create-invitation`, `DELETE /api/ghost-writer/clients`. Redux store: `clientList`, `selectedClientId`, `clientNotes`, `clientArticles`, `clientNoteCategories`, `clientSchedules`, `seats`, `seatCount`, `isPrimaryComputerForGhostwriter` | Ghostwriter, Network | YES |
| **Seats / per-client subscription billing** | `seats` Redux state, `seatCount`, ghostwriter `subscriptionId` | Ghostwriter | NO — billing is human |
| **Coupon / incentive system** | `firstTimerCouponCode`, `couponIdApplied`, `animateIncentive` | All | NO — promo only |
| **i-am-a / usually-post-about onboarding** | `iAmA`, `usuallyPostAbout` user metadata fields | Solo | YES — config |
| **Notes prompt versioning** | `notesPromptVersion` user metadata; persisted globally and per-publication | Solo | NO — internal |
| **Email-unsubscribers ledger** | `EmailUnsubscribers` table | Solo | YES — read |
| **Inspiration "fan-in" via cursor pagination** | Redux: `inspirationNotes`, `inspirationNotesCursor`, `hasMoreInspirationNotes`; per-note `isFromInspiration` flag tracks inspiration-derived drafts | Solo | YES — read |

### 1.3 Pricing tiers (verbatim)

Three tiers, both monthly and yearly billing. Numeric limits extracted from constants `o = {hobbyist:50, standard:150, premium:99999}` (AI credits/month), `l = {hobbyist:20, standard:100, premium:99999}` (max queued notes), `i = {hobbyist:10, standard:20, premium:80}` (max followed creators):

| Tier | Pitch | Monthly | Yearly (per mo) | AI Credits/mo | Notes Queued | Followed Creators | Annual Savings |
|---|---|---|---|---|---|---|---|
| **Hobbyist** | "For creators building their posting rhythm and wanting to stay consistent without stress." | $23.99 | $19.99 | 50 | 20 | 10 | $47.98 |
| **Standard** ★ popular | "For creators who want to grow faster, post smarter, and level up using real insights." | $32.99 | $24.99 | 150 | 100 | 20 | $83.98 |
| **Enterprise** (constant name `premium`) | "For high-growth publications needing unlimited output, elite tools, and pro-level workflows." | $99.99 | $79.99 | Unlimited (99999) | Unlimited | 80 | $239.88 |

All plans include: World-class customer support; "free trial; cancel anytime."

Per-tier feature gates from tooltip strings:
- "Use AI credits to generate personalized Notes, based on your publication, past articles or files you upload."
- "Plan your Notes ahead so you stay consistent even on busy days. Up to N Notes can be queued at a time. Once a scheduled Note is published, you can queue another one."
- "Follow up to N creators to get their latest Notes and interact with them."
- "Notes Inspirations" — all tiers
- "Advanced Notes statistics" — Standard+
- "Activity Center" — Standard+
- "Claude MCP" — Standard+ (Premium only per separator pattern)
- "Advanced search through millions of Notes" — Premium only
- "World-class customer support" — all

(Free trial banner: "every plan includes a free trial." Free tools without account: Heatmap, Super Fans, Article Teaser Generator, Profile Scorer.)

### 1.4 What WriteStack does NOT have (notable absences)

Confirmed by examining the codebase:

- No native cross-post to LinkedIn / Twitter / X / Mastodon (Buffer integration is the only outbound channel; Narrareach competes on this).
- No referral/affiliate widget for the writer's own newsletter (SparkLoop / Beehiiv Boosts territory).
- No paid recommendation marketplace (Beehiiv Boosts, SparkLoop Upscribe).
- No DM / direct-message tooling.
- No scheduled posts (long-form articles) — only Notes.
- No team comments / draft review workflow per-Note (ghostwriter mode is multi-tenant, not multi-reviewer).
- No a/b test / variant headline picker for the published Note.
- No paid-subscriber re-engagement campaigns or behavioral triggers (Beehiiv strength).

---

## 2. Adjacent Substack-growth tool ecosystem

| Tool | One-line | Unique action it automates | Substack API or adjacent? | Notes |
|---|---|---|---|---|
| **Substack Notes (native)** | Substack's TikTok-style short-post feed inside the app. | First-party; everything else here wraps it. | Native | Drives 30–62% of growth for many of the 10K-subscriber writers in the playbook section. Substack does NOT offer a public API for it. |
| **Substack Recommendations (native)** | Cross-promotion: writers list other Substacks under "I recommend" and reciprocally. | First-party; on opt-in to your newsletter, reader is shown your recommendations and asked to subscribe to the others (and vice versa). | Native | One swap regularly delivers 50–100 new subscribers in a day. |
| **Substack Boosts (native paid recs)** | Boosts paid program — a publisher pays per acquired subscriber from another Substack's recommendation slot. | Paid acquisition via recommendations. | Native | Within Substack's recommendations system; rarely emphasized vs Beehiiv's named "Boosts." |
| **WriteStack** | Note scheduling, AI Note generation, analytics, Activity Center, Follow Page, MCP. | Schedules Notes (Substack has no native scheduler); AI-drafts in writer's voice. | Adjacent (uses session cookie / browser-bridge to write Notes) | Reference baseline. Chrome-extension variant required local computer to run; web-app variant uses cloud worker. |
| **StackSweller** | Cloud-based Substack Notes scheduler with stats. | Scheduling without keeping a laptop running ("schedule then close laptop"). | Adjacent (cloud-side cookie holder) | Differentiator: $25/mo unlimited; advanced "which Note brought followers/subscribers/comments/likes" stats. |
| **StackBuddy** | Substack Notes scheduler. | Scheduling only — "does one thing cleanly." | Adjacent | WriteStack's own comparison page positions it as a single-purpose competitor. |
| **Narrareach** | Multi-channel publishing desk: Substack + LinkedIn + Medium + X. | Schedule once, cross-post everywhere; analytics that compare you against top writers in your niche; LinkedIn/X/Mastodon native cross-posting (which WriteStack lacks). | Adjacent (each network's own API/cookie) | Free tier covers most individual creators. Headline: "The Distribution Engine for Substack, LinkedIn, X & Medium." |
| **Substack Pro Studio** (Finn Tropy, beta) | Custom schedules, deep Notes analytics, bulk Notes uploader, prompt-driven Note creation. | Bulk uploader differentiator. | Adjacent | Less automated than WriteStack; "for users who don't mind a manual experience." |
| **SparkLoop / Upscribe** | Newsletter recommendation widget on opt-in; partner-newsletter swap network; paid recommendations marketplace. | Pay-per-sub acquisition + reciprocal-rec swap. | Adjacent (works alongside any ESP, including Substack — though Substack's first-party recs reduce reliance). | $2-$20 per successful recommendation; ~10-20% higher engagement than organic. |
| **Beehiiv Boosts** | Paid-recommendation marketplace inside Beehiiv platform. | Cost-per-sub auction marketplace; verified, fraud-screened conversions. | Adjacent — only for newsletters on Beehiiv. | Stats from beehiiv: avg $1.63/sub, +137% monthly growth utilizing Boosts. Substack publishers cannot directly use Beehiiv Boosts. |
| **Beehiiv (platform)** | Substack alternative built around growth/automation/segmentation/API. | Multi-step welcome sequences; behavioral triggers; segmentation; referral program. | Competitor platform | Informs feature wishlist, not directly callable for Substack writers. |
| **substack-api (PyPI / NHagar)** | Unofficial Python wrapper around Substack's reverse-engineered endpoints. | Programmatic read of posts, comments, Notes, profiles, followees, recommendations. | Adjacent (reverse-engineered) | Endpoint examples: `https://<sub>.substack.com/api/v1/notes`; auth via `connect.sid` session cookie; "could break since not published." Multiple wrappers exist (Python `substack-api`, TypeScript `substack-api`, Go libraries). |
| **n8n-Substack workflows (slys.dev)** | DIY automation flows that publish Notes via the reverse-engineered API. | Trigger Notes from Notion/Airtable/RSS without UI. | Adjacent | Demonstrates the cookie-auth pattern is stable enough for consumer automation. |
| **arthurcolle/substack-mcp, dkships/substack-publisher-mcp, ty13r/substack-mcp-plus** | Open-source Substack MCP servers for Claude/agents. | Programmatic posts/notes/profile/get_drafts via MCP. | Adjacent | Closest existing prior art for an agent-Substack surface; informs target tool list. |
| **Reletter** | Newsletter discovery/search engine by topic, title, author, issues. | Find newsletters in your niche to swap recs with. | Adjacent (read-only directory) | Useful as a niche-discovery data source; not an action surface. |
| **Buffer** | Generic social-post scheduler (Substack via Buffer is indirect). | Scheduling to Twitter/LinkedIn/IG. | Adjacent | WriteStack uses Buffer channel IDs as its outbound social bridge. |
| **Hootsuite** | Enterprise social-media manager. | Multi-account management for agencies. | No native Substack | Not optimized for Substack. |
| **Customers.ai / Mailmodo** | Newsletter automation / interactive email. | Behavioral automation around the email newsletter (not Notes). | Adjacent (email layer) | Limited overlap. |
| **Google Workspace + Notion + Airtable + Sheets** | DIY content calendars. | Manual planning. | Not API-bound. | Common pre-WriteStack baseline. |

---

## 3. Substack growth playbook — repeated tactics with sources

Tactics extracted from multiple high-signal sources where the same instruction recurs. Each is concrete enough to encode as a CLI command. Frequency = number of independent sources advocating it (out of the 11 indexed creator playbooks).

### 3.1 Posting cadence

1. **Post 3–5 Notes per day**, mixing formats (educational, motivational, personal, contrarian). Frequency: 4 sources. (`thrivewithcarrie`, `thecreatorplaybook`, `escapethecubicle/0-to-10k`, `escapethecubicle/0-to-1k`)
2. **Space Notes across the day; never dump 5 in 10 min** — each Note gets its own algorithm window. Source: `thrivewithcarrie`. Suggested time blocks: 6–7 AM (personal/reflection), 8–9 AM (tactical), 11 AM–12 PM (contrarian/conversation), 2–3 PM (behind-the-scenes), 7–8 PM (vulnerability).
3. **Best windows**: weekday 7–10 AM (open rates), 5–7 PM (browse), 11 AM–1 PM (engagement), 7–9 PM (comments + subscribers). Sources: `narrareach`, `thrivewithcarrie`.
4. **Mornings (6–9 AM in audience TZ) → reach; mid-day → engagement; evening → comments + subs.** Source: `thrivewithcarrie`.
5. **Batch-write 21 Notes for the week in 30 minutes**, then schedule. Source: `thrivewithcarrie 30-day plan, week 3`.
6. **Long-form post cadence**: 1–2 posts/week on a consistent day (e.g. Tuesday/Thursday). Sources: `growth-beginners-2026`, `kirsten-learnings`.

### 3.2 Engagement (what "engage" actually means)

7. **Leave 5–10 thoughtful comments per day** on other creators' Notes. Frequency: 3 sources.
8. **Comments must be substantive** — never "Great post!"/"🔥🔥🔥". Use one of three patterns: (a) shared experience ("Interesting point about X — I've seen the same thing work for me"); (b) clarifying question ("Curious, do you think this also applies to coaching offers?"); (c) one-line reframe ("Consistency > virality — love that reminder"). Source: `thecreatorplaybook`.
9. **Restack 3–5 great Notes from others daily.** Source: `thrivewithcarrie`.
10. **Restack with comment** is a 2x leverage: combine someone's Note with your own takeaway. Three named patterns from `growth-21-restack-ideas` and `escapethecubicle`:
    - Endorsement: "More people should be reading this. If you care about X, follow this writer."
    - Bridge: "This explains the problem. I tackled the solution in my latest post."
    - Comment-then-restack: only restack Notes where you've left a comment first — doubles surface area.
11. **Only restack writers with overlapping audiences**, since the algorithm cross-shows your content to their network. Source: `escapethecubicle/3-promotion-ways`.
12. **Reply to every comment on your own Notes** — replies create thread depth, which the algorithm reads as conversation-quality. Source: `thrivewithcarrie`.
13. **Auto-restack your own Note after publish** to give it a second visibility wave. Source: WriteStack `autoRetweet` UI string.
14. **Auto-like your scheduled Notes** at fire time (mild self-engagement seed). Source: WriteStack `shouldLike` schema.
15. **Engage with 3-4 new writers immediately after posting your Note** (restack + comment on theirs); compounds reciprocity. Source: `growth-creatorplaybook`.

### 3.3 Discovery / network building

16. **Follow 30+ writers in your niche** week-1 baseline. Source: `thrivewithcarrie 30-day plan`.
17. **Reach out to 5 writers/week for recommendation swaps.** Source: `thrivewithcarrie`.
18. **Form a 4-5 person cross-promo "circle" in adjacent niches**, monthly meeting, each promotes one member's newsletter every month. Source: `growth-recommendations-tactics` web search synthesis.
19. **Personalize cross-promo outreach** with the specific post you enjoyed + your transparent metrics. Source: same.
20. **Find niche writers via Reletter, Substack discover, or category pages** before pitching. Source: `reletter-recommendations-guide`.
21. **Set up Substack Recommendations** to auto-show partner newsletters at signup time. Source: `substack-recommendations-feature`.

### 3.4 Note format / hooks

22. **3-sentence formula** for a viral Note: relatable observation → personal insight → encouraging perspective. Source: `escapethecubicle/0-to-10k`.
23. **Reframe pattern**: identity reader recognizes → unexpected reframe of what it really is → defiant earned conclusion. Source: `growth-17-viral-notes`.
24. **Curiosity-gap colon hook**: "X people don't realize this:" — forces profile click. Source: `growth-17-viral-notes`.
25. **Optimal length**: 50–120 words, 1–3 short sentences for "wisdom" Notes. Source: `growth-17-viral-notes`.
26. **Short hook (first line)** roughly doubles avg subscribers gained for 64–255-word Notes. Source: `growth-17-viral-notes`.
27. **Convert long posts → punchy Notes** as your primary post-promo tactic ("pick up to 10 articles, generate Notes from each"). Source: WriteStack feature copy + `indiepreneur-writestack-2026`.
28. **Note format mix**: educational, motivational, personal, contrarian, conversation-starter, generosity, behind-the-scenes, vulnerability, quick-fix. Source: `thrivewithcarrie`.

### 3.5 Algorithm-aware behavior

29. **Comments > restacks > likes** in algorithm weight. A Note with 20 comments outreaches one with 200 likes + 2 comments. Source: `thrivewithcarrie`.
30. **Three-tier distribution**: Tier 1 your subscribers → Tier 2 their subscribers (if engagement signals trip) → Tier 3 public discovery feed. Source: `thecreatorplaybook`.
31. **Algorithm builds slowly then compounds**: months 1–3 minimal, 4–6 noticeable, 7–12 exponential at same content quality. Source: `thrivewithcarrie`.
32. **Don't dump scheduled Notes batches simultaneously** — algorithm de-prioritizes burst posts.

### 3.6 Conversion / monetization

33. **Tease paid content in 1–2 Notes/week**. Source: `thrivewithcarrie 30-day plan, week 4`.
34. **Track which Notes drove subs** via subscriber-list "joined-via-Note" metadata; double down on top 2 formats. Source: `thrivewithcarrie`.
35. **Set up a free lead magnet/checklist** — "free checklist brought 500 subscribers in 30 days." Source: `ditchthetemplates`.

### 3.7 Multi-platform amplification (cross-posting)

36. **Cross-post Notes/articles to LinkedIn and X**; LinkedIn = "the story of why you wrote it" not the article itself. Source: `growth-47-ideas/27`.
37. **Re-share own Substack post on Twitter multiple times over the next week.** Source: `narrareach-schedule-guide`.
38. **Run a podcast-guest growth loop** with templated outreach; ~30% reply rate. Source: `growth-47-ideas/11`.

---

## 4. Persona–feature heatmap

Heat: ▓▓▓ heavy use, ▓▓ frequent, ▓ occasional, · rarely.

| WriteStack-equivalent feature | Solo writer | Ghostwriter / agency | Cross-promo network |
|---|:-:|:-:|:-:|
| Schedule Notes (single account) | ▓▓▓ | ▓▓▓ | ▓▓ |
| Bulk-schedule monthly batch | ▓▓▓ | ▓▓▓ | ▓▓ |
| AI Note generation (voice-matched) | ▓▓▓ | ▓▓▓ | ▓ |
| Generate Notes from own articles | ▓▓▓ | ▓▓ | · |
| Best-time-to-publish recommendation | ▓▓▓ | ▓▓ | ▓ |
| Posting heatmap / streak | ▓▓ | ▓▓ | · |
| Auto-restack on publish | ▓▓ | ▓▓ | ▓ |
| Auto-like own note at fire-time | ▓▓ | ▓▓ | · |
| Note conversion analytics | ▓▓▓ | ▓▓▓ | ▓▓ |
| Top-engagers / Super Fans report | ▓▓ | ▓▓ | ▓▓ |
| Inspiration search (millions of Notes) | ▓▓▓ | ▓▓▓ | ▓ |
| Activity Center (notif triage) | ▓▓▓ | ▓▓▓ | ▓ |
| Auto-like-on-reply / kbd shortcuts | ▓▓ | ▓▓▓ | · |
| Curated Follow Page | ▓▓ | ▓▓ | ▓▓▓ |
| Restack-with-comment drafting | ▓▓▓ | ▓▓ | ▓▓▓ |
| Comment drafting on others' posts | ▓▓ | ▓▓ | ▓▓▓ |
| Engagement reciprocity tracking | ▓ | ▓▓ | ▓▓▓ |
| Multi-client switching | · | ▓▓▓ | ▓▓ |
| Per-client AI prompt + voice | · | ▓▓▓ | ▓ |
| Per-client schedule isolation | · | ▓▓▓ | ▓ |
| Seat / billing management | · | ▓▓▓ | ▓ |
| Recommendation-swap finder | ▓▓ | ▓▓ | ▓▓▓ |
| Outreach-message drafter for swaps | ▓▓ | ▓▓ | ▓▓▓ |
| Cross-post to LinkedIn / X / Threads | ▓▓ | ▓▓ | ▓ |
| Set Substack Recommendations list | ▓▓ | ▓▓ | ▓▓▓ |
| Lookalike-subscriber export | ▓ | ▓▓ | ▓▓ |
| Idea/outline generator (long-form) | ▓▓ | ▓▓▓ | · |
| MCP / agent surface | ▓▓ | ▓▓▓ | ▓▓ |

---

## 5. Top 12 agent-automatable growth actions (must-ship as commands)

These are the "agent does the whole thing" surfaces. Each maps to a clean Cobra-style verb that an MCP host can call with no human checkpoint, because the action is either purely-mechanical or fully-delegable to AI with an unambiguous output contract.

| # | Command (proposed) | What it actually does (concrete) | Data dependency |
|---|---|---|---|
| 1 | `notes schedule --batch <file>` | Reads N Note bodies + scheduled-times, posts them via Substack's Notes endpoint at fire time. Spaces them ≥30 min apart by default to avoid burst-penalty. | local `notes/` queue + auth cookie |
| 2 | `notes generate --from-article <post-id> --count <n> --voice <profile>` | Pulls the long-form post body, runs voice-matched generation, writes N drafts to the queue. | publication post list + voice corpus |
| 3 | `notes generate --from-inspiration --niche <slug> --count <n>` | Searches Top-N performing Notes in niche, extracts format/hook patterns, rewrites in the writer's voice. | inspiration corpus (`/notes/search`) + voice corpus |
| 4 | `notes auto-restack-self --enable` | Sets per-Note `shouldRestack=true` so each scheduled Note self-restacks ~12h after publish. | none |
| 5 | `notes auto-like-self --enable` | Sets per-Note `shouldLike=true` so each scheduled Note self-likes at fire-time. | none |
| 6 | `analytics best-times --window <days>` | Returns 5 best [day-of-week × hour] pairs based on actual conversion of the user's last N Notes. | own Notes stats endpoint |
| 7 | `analytics top-engagers --limit <n>` | Lists the top N readers by like+comment+restack count over a window. | activity feed endpoint |
| 8 | `analytics conversion --by format` / `--by time` / `--by hook` | Per-format/time/hook subscribers-gained breakdown. | Notes stats + sub-list join |
| 9 | `inspiration search --niche <s> --sort restacks --since 7d` | Substack-wide search across Notes; returns ranked rows. | `/notes/search` endpoint |
| 10 | `follow-list import --from <handle>...` / `follow-list curate --niche` | Builds a curated follow list of N creators in a niche by category leaderboard + algorithm-overlap heuristic. | Substack `/category/public` + followees endpoints |
| 11 | `recs sync --partners <handle>...` | Sets the writer's Substack Recommendations list (cross-promo) to a target set. | Substack recommendations endpoint |
| 12 | `streak status` / `streak remind --at <local-time>` | Reports current posting streak; schedules a daily local-time reminder if today's queue is empty. | own schedule + Notes feed |

Each of these meets the agent-native bar: deterministic input → bounded I/O → no human-judgment-required output that would warrant `--dry-run`-default semantics.

---

## 6. Top 5 agent-assistable but human-in-loop actions (expose with `--dry-run` default)

These commands MUST default to print-and-wait. The agent can draft and propose, but a human approves before the side-effect lands, because the action is high-trust (touches another writer's relationship), reputation-shaping, or reaches a paying customer.

| # | Command (proposed) | What it drafts | Why human-in-loop default |
|---|---|---|---|
| 1 | `engage comment --on <note-url> --tone <thoughtful|question|reframe>` | Drafts a 1–2 sentence substantive comment on someone else's Note. | Public reputation; "Great post!"-style failure mode is permanent. Prints the draft; posting requires `--send` flag. |
| 2 | `engage restack-with-comment --on <note-url> --pattern <endorsement|bridge|comment-first>` | Produces a quote-restack body using one of the three documented patterns. | Tags your audience to theirs; bad take harms both. Drafts; `--send` to publish. |
| 3 | `recs outreach --to <handle> --based-on <post-slug>` | Drafts a personalized recommendation-swap DM/email referencing a specific post + your transparent metrics. | Cold outreach to a peer creator; sloppy template damages relationship. Drafts; `--send` posts to Substack DM (or copies to clipboard). |
| 4 | `replies draft --batch` | Reads new comments and DMs on the writer's own Notes/posts, drafts reply candidates per item with auto-dismiss-on-reply enabled. | Replies to paying subscribers and engaged readers — wrong-tone reply is costly. Outputs `replies.draft.json`; `replies send-approved` commits per-item. |
| 5 | `notes publish --note <draft-id>` (manual) | Publishes a single hand-written Note immediately. | Even with author oversight, a typo/factual error in a published Note is harder to undo than a draft. Prompts confirmation unless `--yes`. |

Implementation pattern (matches AGENTS.md side-effect rule): each of these short-circuits to `--dry-run` when `PRINTING_PRESS_VERIFY=1` is set, and requires an explicit `--send` / `--yes` to actually act.

---

## Source bibliography

WriteStack:
- writestack.io homepage + JS bundles `_next/static/chunks/*.js` (feature objects, pricing constants, schema strings).
- writestack.io/pricing (rendered shell + bundle constants).
- writestack.io/blogs/writestack-vs-stackbuddy (positioning vs single-purpose schedulers).
- writestack.io/note-generator/post, /heatmap, /fans (free-tier tools).
- academy.writestack.io.
- letters.byburk.net "What I Love About WriteStack", "My Experience With WriteStack Premium".
- theindiepreneur.substack.com "Best Tool For Substack Creators In 2026", "WriteStack Tech Stack and Costs Breakdown".
- wonderingaboutai.substack.com "I tested two AI-enabled tools for cracking the code on Substack Notes".
- natashatynes.substack.com "How Orel Zilberman Built WriteStack" (referenced via search; not deeply indexed).
- postmake.io/tools/writestack.
- chrome-stats.com WriteStack (403; metadata only).

Adjacent tools:
- stacksweller.com/stacksweller-vs-writestack, stacksweller.com/blog.
- narrareach.com, narrareach.com/blog/how-to-schedule-substack-notes-complete-guide (and variant `-3`).
- beehiiv.com/features/boosts, beehiiv.com/comparisons/substack, beehiiv.com/comparisons/sparkloop.
- sparkloop.app, sparkloop.app/upscribe.
- on.substack.com/p/recommendations, on.substack.com/p/notes-collaboration-growth-guide.
- reletter.com/blog/substack-recommendations.
- iam.slys.dev "How I reverse-engineered Substack API", "Substack automation with n8n".
- github.com/NHagar/substack_api.

Growth playbook:
- escapethecubicle.substack.com (5 pieces): "47 Hidden Growth Tactics", "31 Fresh Ideas", "How I'd grow from Zero to 1,000", "How I Grew from Zero to 10,000", "I Just Analyzed My 10,000 Subscribers", "I Tracked Every New Subscriber for 365 Days", "The 3 Best Ways to Promote".
- thrivewithcarrie.substack.com "Substack Notes Strategy 2026 (60% of My Subscriber Growth)".
- thecreatorplaybook.substack.com "How I Gain 10–30 New Subscribers Every Day Using Substack Notes".
- writebuildscale.substack.com "Substack Notes Added 10,000+ Subscribers", "These 17 Substack Notes Went Viral", "Ultimate Substack Checklist", "Substack for Beginners 2026".
- pubstacksuccess.substack.com "How to Use Substack Notes to Get More Subscribers".
- howwegrowtoday.substack.com "7 Substack Promotion Strategies Nobody Talks About".
- professoroffduty.substack.com "How I grew my Substack from 0 to 10,000 subscribers in 12 months".
- kirstenpowers.substack.com "What I Learned Growing from Zero to 10K Subscribers".
- thewritinglonggame.substack.com "25 Viral Substack Note Templates", "I Analyzed 80+ Viral Substack Notes".
- thewritingedge.substack.com "I Analyzed 19,471 Viral Substack Notes".
- substack.com/resources (first-party resource hub).

Open-source Substack-MCP prior art (informs target tool list):
- github.com/arthurcolle/substack-mcp.
- github.com/dkships/substack-publisher-mcp.
- github.com/ty13r/substack-mcp-plus.
