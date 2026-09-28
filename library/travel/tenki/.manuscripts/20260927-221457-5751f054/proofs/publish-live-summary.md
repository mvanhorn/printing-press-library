Publish-time live gate

Full live dogfood reran against the publication source after the command-layout and HTTP-timeout changes. 58 executed checks passed, 0 failed; 47 non-applicable or guarded probes skipped. All product leaf happy paths and JSON checks passed. The binary wrote the fresh source-bound phase5-acceptance.json; embedded and archive copies match. Raw transcript was held in a private temporary directory and deleted after evaluation.

## After review corrections

The full live gate reran on 2026-09-28 Asia/Tokyo against the repaired source: 58 executed checks passed, zero failed, 47 inapplicable/guarded probes skipped. The tool refreshed the source-bound marker with fingerprint `701d0845247cc80c06b69e290bf439b35500a22ffde0e0d6a26a5e54b89ff3ff`. No pass fields were edited manually.

## After selector correction on the replacement PR

Full live gate: 58 passed, zero failed, 47 guarded/inapplicable skips; completed 2026-09-28T02:20:32Z. Source fingerprint: `ff5e0b70fbd336253baf00ca44efe27914fe5f2375a9e07cc5c820cf87e8e343`. All thirteen canonical publication checks passed.

## After blank-selector regression restoration

Full live gate: 58 passed, zero failed, 47 guarded/inapplicable skips; completed 2026-09-28T02:30:12.059532+00:00. Source fingerprint: `fc13a062c7b4774da4f0eba2e31e2582a8c1605d50ea1da33ded2573251411fa`. All thirteen canonical publication checks passed.
