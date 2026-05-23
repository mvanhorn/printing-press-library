=== venues-now (Helsinki, fast sushi) ===
{
  "count": 3,
  "venues": [
    {
      "estimate_min": 20,
      "slug": "haru-sushi-freda"
    },
    {
      "estimate_min": 20,
      "slug": "konnichiwa"
    },
    {
      "estimate_min": 25,
      "slug": "osaka-sushi-kamppi1"
    }
  ]
}

=== venues-compare ===
{
  "count": 2,
  "venues": [
    {
      "open_status_text": "Open until 20:45",
      "slug": "noodle-story-kamppi"
    },
    {
      "open_status_text": "Open until 00:30",
      "slug": "puttes-bar-pizza"
    }
  ]
}

=== cuisine-bottleneck top 3 ===
{
  "city": "helsinki",
  "count": 3,
  "cuisine_buckets": [
    {
      "tag": "Central Asian",
      "venue_count": 1,
      "open_count": 1,
      "avg_eta_min": 65,
      "min_eta_min": 65,
      "max_eta_min": 65
    },
    {
      "tag": "tapas",
      "venue_count": 1,
      "open_count": 1,
      "avg_eta_min": 50,
      "min_eta_min": 50,
      "max_eta_min": 50
    },
    {
      "tag": "Sashimi",
      "venue_count": 2,
      "open_count": 2,
      "avg_eta_min": 47.5,

=== track ===
{
  "status": "stub",
  "order_id": "5f9132c7b4d5bd0196951924",
  "tracking_url": "https://wolt.com/en/track/5f9132c7b4d5bd0196951924",
  "endpoint_note": "Live JSON tracking endpoint is undocumented. Open the share link in a browser to see status. Help: capture the network call wolt.com makes when loading the tracking page and file an issue with the request shape."
}

=== doctor ===
  OK Config: ok
  OK Auth: not required
  OK Verify Mode: normal operation
  OK API: reachable (HTTP 404 at /)
  config_path: /Users/amit/.config/wolt-pp-cli/config.toml
  base_url: https://restaurant-api.wolt.com
  version: 1.0.0
  INFO Cache: unknown
    db_path: /Users/amit/.local/share/wolt-pp-cli/data.db
    schema_version: 2
    db_bytes: 8130560
    stale_after: 6h0m0s
    hint: sync_state is empty; run 'wolt-pp-cli sync' to hydrate.

=== error path: venues-now missing lat ===
Error: must pass --lat and --lon (or use --profile to set them)
