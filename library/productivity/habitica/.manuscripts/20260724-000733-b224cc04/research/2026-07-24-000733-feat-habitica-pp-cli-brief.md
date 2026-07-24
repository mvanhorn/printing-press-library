# Habitica CLI Brief

## API Identity

- Domain: Habitica is a gamified personal productivity service where real-world habits, dailies, to-dos, and custom rewards affect an RPG character.
- Users: (1) a ritual-driven solo player who starts the day by clearing dailies and choosing a small set of to-dos; (2) a fitness-focused player who records workouts immediately and wants completion to be low-friction; (3) a budget-conscious player who uses custom rewards and must decide whether current gold can fund a weekend reward without undermining planned goals; (4) a power user who manages long chore lists with checklists and tags.
- Data profile: authenticated task list, task checklists/tags/order, character stats and gold, inventory/shop data, custom/in-app rewards, and optional historical local snapshots.

## Reachability Risk

- Low. `GET https://habitica.com/api/v3/status` returned `200` on 2026-07-24 with `{"success":true,"data":{"status":"up"}}`; it advertised `x-ratelimit-limit: 30` and `x-ratelimit-remaining: 25`.
- Auth/platform constraint: unauthenticated `GET /api/v3/tasks/user` returned `400 Missing x-client headers`. Official server source requires `x-api-user` and `x-api-key` for authenticated routes, and can enforce `x-client`; the generated CLI must send a distinct `x-client` identifier and treat both credentials as sensitive.
- No active upstream reachability-pattern issue was found in a GitHub search for `403`, `blocked`, `deprecated`, and `rate limit`.

## Top Workflows

1. Turn a morning list of chores into a small, realistic set of Habitica to-dos, including checklists, due dates, priority, and tags.
2. See what is due today, then score a workout habit/daily the moment it is completed.
3. Compare current gold with a custom/in-app reward before committing to a weekend treat; purchase only after an explicit confirmation.
4. Maintain tasks, checklists, tags, and ordering without leaving scripts or an agent session.
5. Inspect character stats, inventory, shop candidates, pets/mounts, and notifications as part of a regular game-management routine.

## Table Stakes

- Hopla: authenticated CLI; create a to-do with due date and a file-backed checklist; add/buy/feed/hatch/cast; user stats/inventory; shell completion and option defaults.
- Habitica Python CLI: status; list habits/dailies/to-dos; score habits up/down; mark dailies and to-dos done/undo; add to-dos; open the task page.
- iBreaker Habitica MCP: profile/stats/inventory; CRUD and scoring for all task types; full checklist lifecycle; tags; pets/mounts; shop/rewards/purchases; notifications; class skills.
- Official API: task CRUD/score/move/checklist/tag endpoints; user profile/stats, buy list, in-app rewards, shop/inventory and purchase routes.

## Data Layer

- Primary entities: task (including `type`, `completed`, `checklist`, tags, due date), tag, user stats (especially `gp`), in-app reward, buy-list item, inventory snapshot.
- Sync cursor: timestamped snapshots of tasks and user/reward data; keep a short, explicit manual sync rather than surprise background refresh because scoring and purchases are account-affecting.
- FTS/search: task text, notes, checklist text, and tag labels for fast workout/chore resolution.

## Product Thesis

- Name: Habitica CLI
- Why it should exist: Existing clients wrap task CRUD or game management; this CLI puts an agent-safe, preview-first daily loop on top of the official API—turn chores into quests, record a workout, and evaluate a real-world reward budget without accidental scoring or spending.

## Build Priorities

1. Correct two-header authentication, platform header, adaptive rate limiting, and explicit mutation confirmation/dry-run semantics.
2. Complete task, checklist, tag, user stats, rewards/shop, inventory, pet/mount, notification, and skill surfaces from official API/MCP evidence.
3. Agent-friendly daily briefing, chore-to-quest preview/apply, workout check-off, and reward-affordability workflows backed by live API calls or synced SQLite data.
