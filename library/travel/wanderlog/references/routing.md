# Routing Reference

Route body generation, Wanderlog route optimization, and applying order back to an itinerary.

## Commands

- `plan route day-body`: Build the optimization request body from places in one day.
- `plan route optimize`: Call Wanderlog route optimization from an explicit JSON body or a day-derived body.
- `plan block move`: Apply optimized ordering after review.

## Workflow

1. `plan outline --target-key KEY --day N --agent`.
2. `plan route day-body --target-key KEY --day N --agent`.
3. Review places, coordinates, and travel mode.
4. `plan route optimize` only if the body looks correct.
5. Convert optimized order into explicit `plan block move` calls. Do not blindly reorder.

Route optimization sequences stops; it does not replace itinerary judgment. Preserve reservations, meal timing, opening hours, transit constraints, and collaborator preferences.
