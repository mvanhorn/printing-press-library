-- Keyword Planner portfolio compatibility recipe (DuckDB).
--
-- This is a read-only consumer recipe. Run DuckDB from the directory containing
-- snapshots.db (by default ~/.local/share/keyword-planner), or replace the path
-- below with your portfolio's absolute path. Tilde expansion is not supported
-- by the SQLite ATTACH path. The attached SQLite file is never modified.
ATTACH 'snapshots.db' AS keyword_planner
    (TYPE sqlite, READ_ONLY);

-- Evidence view: complete, eligible monthly rows with all provenance fields.
-- The compatibility view below narrows this to the verified US/English,
-- single-country mapping. Combined or unresolved targeting remains available
-- here for inspection and is never relabelled as a country.
CREATE OR REPLACE VIEW keyword_monthly_evidence AS
WITH account_currency AS (
    SELECT snapshot_id, currency_code, source,
           ROW_NUMBER() OVER (
               PARTITION BY snapshot_id
               ORDER BY recorded_at DESC, id DESC
           ) AS rn
    FROM keyword_planner.account_metadata
), metric_coverage AS (
    SELECT mv.snapshot_id,
           mv.receipt_id,
           mv.metric_id,
           COUNT(DISTINCT mv.month) AS returned_month_count,
           DATE_DIFF(
               'month',
               CAST(s.requested_start || '-01' AS DATE),
               CAST(s.requested_end || '-01' AS DATE)
           ) + 1 AS requested_month_count
    FROM keyword_planner.monthly_volumes AS mv
    JOIN keyword_planner.snapshots AS s ON s.id = mv.snapshot_id
    WHERE mv.eligibility = 'eligible'
    GROUP BY mv.snapshot_id, mv.receipt_id, mv.metric_id,
             s.requested_start, s.requested_end
)
SELECT
    km.returned_text AS keyword,
    CASE
        WHEN s.geo_target_constants = '["geoTargetConstants/2840"]' THEN 'US'
        ELSE NULL
    END AS geo,
    CASE
        WHEN s.language = 'languageConstants/1000' THEN 'en'
        ELSE NULL
    END AS language_code,
    CAST(mv.month AS DATE) AS month,
    CAST(mv.monthly_searches AS BIGINT) AS monthly_searches,
    CAST(km.low_bid_micros AS BIGINT) AS low_bid_micros,
    CAST(km.high_bid_micros AS BIGINT) AS high_bid_micros,
    km.competition AS competition,
    CAST(s.fetched_at AS TIMESTAMP) AS fetched_at,

    mv.snapshot_id,
    mv.receipt_id,
    mv.metric_id,
    km.endpoint,
    km.submitted_text AS submitted_keyword,
    km.submitted_index,
    km.variant_group,
    km.linkage_status,
    s.geo_set_id,
    s.geo_target_constants AS geo_targets_json,
    s.language AS language_resource,
    s.network,
    COALESCE(ac.currency_code, s.currency_code) AS currency,
    COALESCE(ac.source, s.currency_source) AS currency_source,
    CAST(s.requested_start || '-01' AS DATE) AS requested_start,
    CAST(s.requested_end || '-01' AS DATE) AS requested_end,
    CAST(km.average_monthly_searches AS BIGINT) AS average_monthly_searches,
    CAST(km.average_cpc_micros AS BIGINT) AS average_cpc_micros,
    CAST(km.competition_index AS BIGINT) AS competition_index,
    mv.raw_month,
    mv.raw_year,
    mv.value_state,
    mv.eligibility,
    mv.flags AS flags_json,
    CASE
        WHEN instr(lower(mv.flags), 'ambiguous_zero_unspecified') > 0
            THEN 'PLANNER-BLIND'
        ELSE NULL
    END AS analyst_marker,
    mc.returned_month_count,
    mc.requested_month_count,
    CASE
        WHEN mc.returned_month_count < mc.requested_month_count
            THEN 'PLANNER-SHORT-WINDOW'
        ELSE NULL
    END AS short_window_marker,
    s.status,
    s.complete,
    CAST(s.fetched_at AS TIMESTAMP) AS fetched_timestamp
FROM keyword_planner.monthly_volumes AS mv
JOIN keyword_planner.keyword_metrics AS km ON km.id = mv.metric_id
JOIN keyword_planner.snapshots AS s ON s.id = mv.snapshot_id
LEFT JOIN account_currency AS ac
    ON ac.snapshot_id = s.id AND ac.rn = 1
LEFT JOIN metric_coverage AS mc
    ON mc.snapshot_id = mv.snapshot_id
   AND mc.receipt_id = mv.receipt_id
   AND mc.metric_id = mv.metric_id
WHERE mv.eligibility = 'eligible'
  AND s.complete = 1
  AND s.status = 'complete';

-- Warehouse compatibility view. Only the verified single US geography and
-- English mapping enter this relation. Unknown mappings and combined geo
-- selections remain in keyword_monthly_evidence with NULL labels.
CREATE OR REPLACE VIEW keyword_monthly AS
SELECT *
FROM keyword_monthly_evidence
WHERE geo = 'US'
  AND language_code = 'en';

-- Safe descriptive inputs: nullable, ambiguous and negative values are
-- excluded explicitly. A short returned window stays usable and retains its
-- marker in the evidence view.
CREATE OR REPLACE VIEW keyword_monthly_safe AS
SELECT *
FROM keyword_monthly
WHERE monthly_searches IS NOT NULL
  AND monthly_searches >= 0
  AND instr(lower(flags_json), 'ambiguous_zero_unspecified') = 0;

-- Example read-only aggregate:
-- SELECT keyword, month, monthly_searches
-- FROM keyword_monthly_safe
-- ORDER BY fetched_at DESC, keyword, month;
