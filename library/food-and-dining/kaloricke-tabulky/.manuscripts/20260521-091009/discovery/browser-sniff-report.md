# Discovery Report: kaloricketabulky.cz

## Method
Authenticated discovery via login + curl probing + JS bundle endpoint mining.
NOT via browser-use (deterministic curl/JS-bundle path was more efficient given
the API's clean envelope format and consistent URL convention).

## Login Flow
- **Endpoint:** `POST /login/create?format=json`
- **Body:** `{"email": "<email>", "password": "<md5(plain_password)>"}`
- **Hash:** Client-side MD5 hex of the plaintext password before sending.
- **Response:** Envelope `{"requestId": null, "code": 0, "message": null, "data": "user/diary"}`
- **Set-Cookie:** `JSESSIONID=<id>`, `<session-cookie-name>=<token>`, locale cookie
- **Subsequent requests:** Send both session cookies via `Cookie:` header.

## Envelope Format (all JSON endpoints)
```json
{"requestId": <string|null>, "code": <int>, "message": <string|null>, "data": <T>}
```
`code: 0` = success. Non-zero codes have human-readable `message`.

Error variants:
- `{"message": "...", "internalCode": "USER_NOT_FOUND", "springMessages": {...}}` — application-level error
- Plain Tomcat HTML 404/500 pages — endpoint shape wrong

## Discovered Endpoints

### Public (no auth)
| Method | Path | Returns |
|---|---|---|
| GET | `/autocomplete/foodstuff?query=<text>&page=<n>` | Array of food matches |
| GET | `/autocomplete/activity?query=<text>&page=<n>` | Array of activity matches |
| GET | `/autocomplete/meal?query=<text>&page=<n>` | Array of recipe matches |
| GET | `/autocomplete/foodstuff-activity-meal?query=<text>&page=<n>` | Combined global search |
| GET | `/autocomplete/user-meal/item?query=<text>` | Custom meal item search |
| GET | `/home/foodstuff/count?format=json` | Total foods (244719+) |
| GET | `/home/diary/count?format=json` | Total diary entries (556594+) |
| GET | `/home/user/count?format=json` | Total registered users (7125480+) |
| GET | `/home/feedback?format=json` | Feedback list |
| GET | `/foodstuff/suggest?title=<text>` | Title-based food suggest |
| GET | `/potraviny/<slug>` | HTML; nutrition in `<script type="application/ld+json">` (Energie, Bílkoviny, Tuky, Sacharidy, Vláknina, Vápník, fats, sugars) |
| GET | `/recepty/<slug>` | HTML recipe detail |
| GET | `/aktivita/<slug>` | HTML activity detail |
| GET | `/sitemap.xml`, `/sitemap/foodstuff/<n>` | Crawlable indexes |

### Authenticated reads
| Method | Path | Returns |
|---|---|---|
| GET | `/user/diary/<DD.MM.YYYY>/get?format=json` | Diary day: `times[]` (meal slots) -> `foodstuff[]`, `notes[]` |
| GET | `/statistic/summary/<DD.MM.YYYY>/get?format=json` | Daily summary: energy, energy target, drink, drink target, energyUnit, **monthWeight[] (weight history)** |
| GET | `/user/diary/filled-out?format=json` | Which days have entries |
| GET | `/user/streak?format=json&date=<DD.MM.YYYY>` | Gamification streak |
| GET | `/user/messages/inapp?format=json` | In-app messages |
| GET | `/site/messages?format=json` | Site-wide messages |
| GET | `/user/settings/common/activity?format=json` | Activity defaults |
| GET | `/user/settings/favorite/foodstuff?format=json` | Favorite foods |
| GET | `/user/settings/favorite/activity?format=json` | Favorite activities |
| GET | `/user/settings/meal/list?format=json&limit=100` | User's saved meals |
| GET | `/user/settings/share/item/list/forme?format=json` | Shares received |
| GET | `/statistic/analysis/achievements/get?format=json&type=<type>` | Achievements list |
| GET | `/statistic/analysis/tips/<id>` | Tip detail |
| GET | `/session/keepalive?format=json` | Session refresh |
| GET | `/nutritionist/client/check?format=json` | Nutritionist module (trainer view) |

### Authenticated writes (form posts; param shape needs Phase 2 capture or manual confirmation)
| Method | Path | Purpose |
|---|---|---|
| POST | `/login/create?format=json` | Login |
| POST | `/user/weight/add?format=json` | Record weight `{weight, date}` |
| POST | `/user/diary/foodstuff/...` (delete/, time/edit, unit/edit) | Modify diary food entries |
| POST | `/user/diary/activity/...` | Modify diary activity entries |
| POST | `/user/diary/note/add?format=json` | Add diary note |
| POST | `/user/diary/copy?format=json` | Copy one diary day to another |
| POST | `/user/diary-time/copy?format=json` | Copy a meal slot |
| POST | `/user/foodstuff/add?format=json` | Create user custom foodstuff |
| POST | `/user/meal/create?format=json` | Create custom meal |
| POST | `/user/activity/add?format=json` | Log activity |

### Export
| Method | Path | Returns |
|---|---|---|
| GET | `/user/export/pdf/<date>` | PDF export of diary |
| GET | `/user/export/xls/<date>` | XLS export of diary |

## Replayability
**Yes.** Every endpoint uses standard HTTP + session cookies. No live JS execution
needed, no signed-timestamp headers, no CSRF token in JS state. Once a session
cookie is captured (or login is performed via MD5 password), all calls replay.

The CLI's runtime transport should be:
- For login: direct HTTPS POST with MD5-hashed password (matches the public web form)
- For all other endpoints: HTTPS GET/POST with `Cookie:` header
- For HTML pages (`/potraviny/...`): HTTPS GET, parse `<script type="application/ld+json">`
- No browser sidecar required

## Auth Strategy for Generated CLI
- `auth login --email <email>` — prompts for password (read from stdin), MD5-hashes, posts to `/login/create`, stores cookies to `~/.config/kaloricke-tabulky/session`
- `auth login --chrome` — extracts session cookies from a logged-in Chrome profile (cookie name: `<session-cookie-name>`, `JSESSIONID`), stores them
- Refresh on 401: re-call login if email+password are stored, otherwise prompt
