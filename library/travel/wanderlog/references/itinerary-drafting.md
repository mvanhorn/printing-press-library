# Itinerary Drafting Reference

Turn trip assumptions, flight anchors, transport, costs, restaurants, and candidate places into a day-by-day draft.

## Workflow

1. Verify hard anchors: arrival/departure times, trip dates, lodging bases, immovable bookings. Label unverified schedules as assumptions.
2. `plan outline --target-key KEY --agent`. Distinguish place-list sections from day-plan sections.
3. Add confirmed bookings with `plan reservation add` (flight, lodging, rental-car, restaurant, train, bus, ferry, cruise, attachment). Multi-night lodging: `reservations-attachments.md` (`--span-nights`).
4. Build the candidate pool before filling days. Food: `places autocomplete` with location bias; inspect before add. Booked restaurants: `--kind restaurant`.
5. Decide transport before placing days. Let the destination's geography drive walk vs transit vs rental — do not reuse a previous trip's mode mix.
6. Apply in small batches: reservation anchors, headings, day notes, candidate/food places, then day-specific places. Dry-run each batch.
7. Verify with `plan outline`, `plan reservation list`, and `plan inspect --check=unformatted,lodging-coverage` (always the `=` form). Undo immediately if a batch lands in the wrong section.

## Day Notes

Preserve reasoning collaborators need: flight buffers, car pickup/dropoff, weather fallbacks, estimated cost bands, food intent, intentionally flexible days.

## Food Candidates

`places autocomplete` with bias; inspect results. Apostrophes break naive quoting — pass the query as a JSON arg or via a script.

Store place id, display name, area, food type, and why it belongs. High-confidence food goes on a master candidate list first; only strong day picks go into day sections.

## Costs

Do not present vendor prices unless verified from a current source. If search is blocked, label bands as estimates in notes or `plan budget expense add`.

## Apply Order

```bash
wanderlog-pp-cli plan outline --target-key TARGET --agent
wanderlog-pp-cli plan reservation add --target-key TARGET --day 1 --kind flight --airline XX --flight-number 123 --start-date YYYY-MM-DD --departure-airport AAA --arrival-airport BBB --dry-run --agent
wanderlog-pp-cli plan section set-field --target-key TARGET --day 1 --field heading --value "Arrive / settle in" --dry-run --agent
wanderlog-pp-cli plan note add --target-key TARGET --day 1 --text "Arrival and transport assumptions..." --dry-run --agent
wanderlog-pp-cli plan place add --target-key TARGET --day 1 --place-id PLACE_ID --text "Why this stop fits" --dry-run --agent
```

`--apply` only after the user approves the target and batch.
