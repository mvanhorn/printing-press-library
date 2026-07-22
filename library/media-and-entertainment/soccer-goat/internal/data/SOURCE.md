# Bundled potential dataset

`potential-sofifa-2025.csv.gz` — one row per player, columns `ea_id,name,overall,potential`.

## Provenance
- Source: Kaggle `aniss7/fifa-player-data-from-sofifa-2025-06-03` (`player-data-full-2025-june.csv`), a June 2025 sofifa snapshot (FC25 era).
- Chosen over the older stefanoleone992 FC24 set because it is ~1.5 years newer with current ratings, while still carrying a real career-mode `potential` column (EA's own drop-api and Ultimate-Team-derived FC25/FC26 datasets omit potential).
- Join key: `ea_id` is the dataset `player_id`, which equals the EA drop-api player id exactly (verified: Mbappé 231747, Bellingham 252371, Schjelderup 260952 → potential 84). Normalized-name is the fallback.
- 18,166 players (deduped by id, keeping the highest potential row).

## Build procedure (reproducible)
1. Download: `aniss7/fifa-player-data-from-sofifa-2025-06-03` (kaggle CLI, or the Bearer API with a KGAT_ token).
2. From `player-data-full-2025-june.csv`, keep `player_id`, `full_name` (falls back to `name`), `overall_rating`, `potential`; drop rows with no potential; dedupe by `player_id` keeping the max potential.
3. Emit `ea_id,name,overall,potential`; `gzip -9`.

## Refresh
Re-run when a newer sofifa snapshot with a `potential` column is published. Potential for established players is season-stable, so staleness is low-risk; `potentialSource`/`captured_at` surface the vintage to users.
