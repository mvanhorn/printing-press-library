# Morgen CLI — Research Brief

## API Identity
Morgen (morgen.so) is a calendar + task aggregator over Google Calendar, Microsoft 365, iCloud, Fastmail/CalDAV, and Todoist. Public REST API at https://api.morgen.so/v3, auth via `Authorization: ApiKey <key>`.

## Users
- **The multi-calendar operator** — runs work + personal + shared calendars across providers; today juggles several apps/tabs and can never see one unified day.
- **The agent-assisted planner** — an AI agent preparing a daily brief that needs events and tasks together in one machine-readable call.

## Top Workflows
- Morning "what's my day" review across every connected calendar and task list.
- Quick capture of tasks/events from chat.
- Rescheduling and triage of due/overdue tasks.

## Table Stakes
Calendars list, events CRUD, tasks CRUD, tags, integrations discovery — all covered by endpoint mirrors.

## Transcendence
`agenda` — joins events (fanned out across all connected calendars) with tasks due that day into one chronological timeline. No single API endpoint provides this; it is a genuine cross-source synthesis.
