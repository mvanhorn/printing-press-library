# /tags (internal API)

Confirmed: internal API host (`api.lunchmoney.app`) uses `/v2/tags` — **same shape as the
documented public API** at `api.lunchmoney.dev/v2/tags`. No new CLI work needed; the
existing `tags create/update/delete` commands in the public spec cover this.

## CREATE
```
POST https://api.lunchmoney.app/v2/tags
→ 201, tag object (id, name, description, archived, archived_at, background_color, text_color)
```

## UPDATE
```
PUT https://api.lunchmoney.app/v2/tags/{tag_id}
→ 200 (or 400 if archived_at is set with archived=false — same validation as public)
```

## DELETE
```
DELETE https://api.lunchmoney.app/v2/tags/{tag_id}
→ 204 No Content
```

Skip implementing — already covered by public-API generated commands.
