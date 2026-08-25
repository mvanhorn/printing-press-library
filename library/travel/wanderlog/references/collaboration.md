# Collaboration Reference

Comments, votes, collaborators, invites, and share keys. Canonical target: `--target-key KEY` or `--plan-url URL`. Never a numeric `tripPlan.id` and never `comments list <planId>`.

## Commands

```bash
wanderlog-pp-cli plan votes --target-key KEY --agent
wanderlog-pp-cli plan comments list --target-key KEY --agent
wanderlog-pp-cli plan comments add --target-key KEY --text "..." --dry-run --agent
wanderlog-pp-cli plan comments add --target-key KEY --parent-id ID --text "..." --dry-run --agent
wanderlog-pp-cli plan comments edit --target-key KEY --comment-id ID --text "..." --dry-run --agent
wanderlog-pp-cli plan comments delete --target-key KEY --comment-id ID --dry-run --agent
wanderlog-pp-cli plan comments vote --target-key KEY --comment-id ID --direction up --agent
wanderlog-pp-cli plan collaborators --target-key KEY --agent
wanderlog-pp-cli plan collaborators invites --target-key KEY --agent
wanderlog-pp-cli plan collaborators invite --target-key KEY --email example@example.com --message "..." --dry-run --agent
wanderlog-pp-cli plan collaborators add --target-key KEY --user-id ID --dry-run --agent
wanderlog-pp-cli plan collaborators remove --target-key KEY --user-id ID --dry-run --agent
wanderlog-pp-cli plan collaborators share-key --target-key KEY --permissions view --dry-run --agent
```

- Invite: repeatable `--email` and `--user-id`, or `--invitees-json`.
- Place picks live on blocks as `upvotedBy` — read them with `plan votes`, not `plan comments list`.
- Comment vote `--direction`: `up`, `down`, `none`.
- share-key `--permissions`: `view`, `edit`, or `suggest`; `--permissions-json` overrides.

## Safety

- Keep invites on dry-run until recipients and message are explicit.
- Do not remove a collaborator unless the user names the user id and the plan.
- Comments/collaboration may use REST and are not always in the ShareDB undo journal. Re-read after apply.

## Workflow

1. `plan collaborators` and `plan collaborators invites`.
2. `plan comments list --target-key KEY` before editing a collaborative plan.
3. Itinerary/budget edits via named ShareDB commands.
4. Optional short comment explaining applied edits.
