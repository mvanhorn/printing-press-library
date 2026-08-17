# U4 Funnel Capture — TaskRabbit `page.book.*` shapes (read-only)

Captured live from the authenticated Chrome session on 2026-07-03. **No booking placed.**
Walked the Help Moving funnel (template_id `2247`, category_id `6`) Describe → Browse →
Schedule → Confirm, capturing real request/response shapes via a page-context fetch
interceptor + tRPC dehydrated cache + Zod-rejection probing. All PII values redacted;
only field names/types recorded.

## Reachability gate (Phase 1.9): PASS
- REST `/api/v3/*` authenticated GETs return 200 with cookies alone (session + XSRF-TOKEN).
- Account metro: **1053 (SF Bay Area, US)**. `payment_method_types: [card, amazon_pay]`.
- **Card on file present** (masked, ends 1007) — autonomous checkout (R6) is feasible.
- Mode: `standard_http` + cookie auth. No WAF/bot-vendor. Mutations need `X-CSRF-Token` (from `<meta name="csrf-token">`) + cookies.

## tRPC transport
- Base: `/next-api/trpc/<procedure>?batch=1`
- Query (GET): `&input=<urlencoded {"0":{"json":<input>}}>`
- Mutation (POST): body `{"0":{"json":<input>}}`, header `X-CSRF-Token`
- Success envelope: `[{result:{data:{json:<payload>}}}]`
- Error envelope: `[{error:{json:{message:<stringified-zod-issues>, data:{code, httpStatus, zodError}}}}]`
- Empty/partial input → 400 with a stringified Zod issue list (used to pin field names read-only).

## page.book.details  (query) — funnel step 1
- **Input:** `{ jobDraftGuid: object|string, locale: string, taskTemplateId: string }`
- **Output:** `bff{ fieldGroups[]{key,title,fields[]}, jobDraft, routingQuestion{questionText,helpText,options[]}, templateTitle, title }, meta{ category{id,name}, hideVehicles, marketingGroup{id,name}, scopingQuestionsAndOptions... }`
- Help Moving scoping question observed: "Do you need your Tasker to provide a vehicle?" (Yes/No), then Start/End address + task size (Small/Medium/Large) + free-text description.

## page.book.recommendations  (query) — funnel step 2 (KILLER endpoint)
- **Input (top-level):** `{ location: object, schedule: { dates: [], dayTimeRanges: [] }, locale, taskTemplateId, jobDraftGuid }`
  - `location` internal shape includes geocoded `{lat, lng, ...address parts}` (from the Describe step's start/end addresses).
- **Output:** `bff{ histogram{...}, recommendations[] }`
  - `histogram`: `currency_code`, `bars[]{attributes{number_of_taskers, minimum_price_cents, formatted_minimum_price}}` (28 bars), `minimum_price_cents`, `median_price_cents`, `maximum_price_cents`, `step_cents`, `expensive_threshold_cents` (+ currency/symbol/formatted variants).
  - `recommendations[]` (56 taskers) item fields (ranking + pricing inputs for U3/U6):
    - Identity: `id`, `user_id`, `slug`, `first_name`, `display_name`, `avatar_url*`, `metro_name`, `locale`, `category_id`, `category_name`
    - Ratings/experience: `rabbit_rating`, `rabbit_average_review`, `rabbit_number_of_reviews`, `rabbit_number_of_message_reviews`, `category_review_count`, `category_family_average_star_rating`, `category_family_review_count`, `category_invoices_count` (tasks completed in category), `category_family_invoices_count`, `job_approved_invoice_count`, `hours_worked`, `experience_level`, `elite`, `reliability_rate`, `response_time`, `most_recent_review`, `reviews_categories`, `identity_label`
    - Flags: `is_favorite`, `past_tasker`, `disabled`, `show_value_badge`, `show_ikea_assembly_badge`, `two_hour_minimum_required_display`, `vehicles`, `vehicles_display`, `special_tools_display`, `spoken_languages_display`
    - Availability: `next_available_at`, `schedule` (per-tasker)
    - **Pricing (the all-in inputs):**
      - `poster_hourly_rate_cents` / `_currency` / `_symbol` / `formatted_poster_hourly_rate`  ← **client-paid base hourly rate** (use this for all-in calc)
      - `poster_fixed_rate_cents` / `formatted_poster_fixed_rate` / `poster_rate_display`
      - `rabbit_hourly_rate_cents` / `formatted_rabbit_hourly_rate` (what the Tasker earns)
      - `discount_saving_hourly_rate_cents` / `discounted_prices`
- **Observed all-in reality:** Browse shows base `poster_hourly_rate` (e.g. Razhap A. $33.33/hr); Confirm charges the fee-folded effective rate ($44.66/hr ≈ +34%). Confirms the all-in transcendence feature: fold service + trust & support fees; surface the confirm-level rate everywhere.

## page.book.schedule  (query) — funnel step 3
- **Input:** `{ categoryId: number, inviteeId: number (selected tasker id), locale: string, location: { lat: number, lng: number }, taskTemplateId: number }`
- **Output:** `bff{ availableDates[]{ date: string, sameday: boolean, slots[]{ durationSeconds: number, offsetSeconds: number, selectLabel: string } }, surgePrice, tasker{ avatarUrl, displayName } }`
- A slot is `(date, offsetSeconds, durationSeconds)`.

## page.book.confirm  (query) — funnel step 4
- Client-fetched (no SSR cache); rendered summary read from DOM.
- Shows: selected Tasker, date/time, 2-hour minimum, start/end addresses, task size, task description, **payment method (card on file, masked)**, promo-code field, "Donate $1" toggle.
- **Cancellation policy text (resolves OQ2 window semantics):** *"we charge a 1 hour deposit that is fully refundable if cancelled at least 24 hours before your appointment. If your task takes more than 1 hour, you'll be charged the remaining balance at $<all-in>/hr once it's done."*
  - **Free cancellation window = ≥24h before the appointment.** Deposit = 1 hour (refundable inside the window).
- Commit button label: **"Confirm and chat"** (this is the checkout/hire commit — NOT clicked).

## Mutations
- **Commit / hire (the checkout):** NOT `page.book.hire` (that path returns `No "mutation"-procedure`). None of `page.book.{create,book,submit,checkout,reserve,request,hireTasker}`, `page.booking.create`, `page.job.create` exist as mutations (all 404 `No mutation-procedure`). The real commit mutation fires **only on the "Confirm and chat" click** and could not be captured read-only.
  - **DEFERRED to the plan's authorized single real `hire`+`cancel` round-trip (U6 acceptance).** That round-trip is where the commit request+response is captured — safely, because `cancelTask` (below) is wired and verified first, and it happens with explicit user go-ahead and the spend cap in place. Do NOT guess the commit mutation name; capture it during that round-trip.
- **Cancel (R7):** **`page.tasks.cancelTask`** (mutation, confirmed — returns 400 Zod on empty input).
  - Input is a **discriminated union on `type`**: `type: 'single' | ...` (e.g. single vs recurring/all). Full member fields (taskId/appointmentId, reason?) to be filled when wiring — the discriminator + procedure name are pinned.

## page.tasks.list  (query) — bookings list (U2 store)
- **Input:** `{ page: number, perPage: number, filters: object, locale: string }`  (`filters` is a required object; `locale` e.g. `en-US`. A bare `{}` filters value tripped an `invalid_value` enum on a filter field — filter sub-keys to be pinned when wiring the sync.)
- **Output (from prior sniff, unchanged):** `bff{ items[]{ details{advanceOrder,promotionCode,...}, taskers[]{review,status,paymentFailed,...}, status, futureAppointments, notification }, page, totalItems, totalPages }`

## Net for U6
Buildable now without guessing: the funnel read chain (details → recommendations → schedule → confirm),
the all-in price inputs (`poster_hourly_rate_cents` + confirm effective rate), ranking inputs, the
schedule slot shape, the cancel mutation (`page.tasks.cancelTask`, discriminated union), the 24h free
cancellation window, and the bookings list. The ONE deferred item is the exact commit mutation
name+shape, captured during the authorized real `hire`+`cancel` acceptance round-trip.

## COMMIT mutation — CAPTURED (2026-07-03, real booking, immediately cancelled)

**`POST /api/v3/jobs/post/hire.json`** (REST, not tRPC) — the "Confirm and chat" checkout.
Needs `X-CSRF-Token` + cookies. Body:

```
{
  "source": "recommendation",
  "job_type": "Template",
  "fixed_rate": false,
  "seconds_between": "0",
  "shown_cancellation_policy": true,
  "task_template_id": 2247,
  "category_id": 6,
  "category_name": "Help Moving",
  "title": "Help Moving",
  "marketing_group_id": 15,            // from details bff meta.marketingGroup.id
  "funnel_id": "<synth uuid_ms>",       // not server-validated (same as recommendations)
  "session_id": "<52-char funnel token>", // client-generated; not a cookie; validation untested
  "recommendation_id": "<bff.recommendation_id from the recommendations response>",
  "invitee_id": <tasker user_id>,       // == rabbit_id
  "rabbit_id": <tasker user_id>,
  "poster_hourly_rate_cents": <tasker poster_hourly_rate_cents>,
  "job_draft_guid": "",
  "form_referrer": "",
  "job_size": "small|medium|large",
  "description": "<task description>",
  "schedule": { "date": "YYYY-MM-DD", "duration_seconds": <slot dur>, "offset_seconds": <time-of-day secs> },
  "address":           { address1, address2, country, formatted_address, lat, lng, locality, metro_id, metro_name, postal_code, region },
  "secondary_location":{ same shape as address — the END address for moving }
}
```

- `recommendation_id`: top-level `bff.recommendation_id` on the recommendations response (e.g. `Organic::MultiDayRecommendationsOp-...`).
- Tasker numeric id = recommendation item `user_id` (id is `profile_<user_id>`).
- **Open dependency for fully-autonomous CLI hire:** the rich `address`/`secondary_location` need geocoding incl. TaskRabbit's internal `metro_id` (Seattle addr = 1057, differs from account metro 1053). recommendations tolerates lat/lng-only; whether the commit does is untested (would require a real booking to confirm).
- Verified cancel: `page.tasks.cancelTask {type:"single", jobId, rabbitId, reason}` (wired into `goat cancel`).
