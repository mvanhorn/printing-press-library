"""Sanitize the three explicitly authorized additional public-source probes."""
import hashlib
import json
from pathlib import Path
import runpy

base = runpy.run_path(str(Path(__file__).with_name("prepare_fixtures.py")))
run, out = base["RUN"], base["OUT"]
Tree, selected, render = base["Tree"], base["selected"], base["render"]
manifest = json.loads((out / "fixture-manifest.json").read_text())
oracle_path = run / "proofs/fixtures/oracle.json"
oracle = json.loads(oracle_path.read_text())
relocation = json.loads((run / "proofs/fixtures/relocation-provenance.json").read_text())
empty = json.loads((run / "proofs/fixtures/empty-source-provenance.json").read_text())
for record in relocation + [dict(empty, label="empty")]:
    raw_path = Path(record["temporary_raw_path"])
    original = raw_path.read_bytes()
    tree = Tree()
    tree.feed(original.decode())
    if record["label"] == "empty":
        wanted = {"c-breadcrumb", "list-condition", "list-sidebar", "navi-rstlst", "c-page-count", "list-rst", "c-pagination", "rstlist-notfound"}
        name = "ginza-empty.html"
    else:
        wanted = {"c-breadcrumb", "rst-status-badge-red", "rdheader-title-data", "rdheader-info-data", "rstinfo-table", "c-alert"}
        name = "miyakawa-" + record["label"] + ".html"
    result = '<!doctype html><html lang="en"><body>\n' + "\n".join(render(n) for n in selected(tree.root, wanted)) + '\n</body></html>\n'
    (out / name).write_text(result)
    manifest.append({"fixture": name, "source_url": record["url"], "http_status": record["http_status"], "captured_at": record["captured_at"], "original_sha256": record["original_sha256"], "sanitized_sha256": hashlib.sha256(result.encode()).hexdigest(), "original_bytes": len(original), "sanitized_bytes": len(result.encode()), "transform": "Keep real public semantic header, status/alert, source criteria/empty markers, JSONLD facts and detail tables. Remove reviewer content, photos, scripts, telemetry and ephemeral attributes/inputs."})
    source_key = "miyakawa_" + record["label"] if record["label"] != "empty" else "ginza_genuine_empty"
    oracle["sources"][source_key] = {"path": "working/tabelog-pp-cli/e2e/testdata/" + name, "url": record["url"], "http_status": record["http_status"], "captured_at": record["captured_at"], "sha256": hashlib.sha256(result.encode()).hexdigest(), "bytes": len(result.encode()), "original_sha256": record["original_sha256"], "original_bytes": len(original), "capture_method": record["transport"], "sanitized": True}
    record.pop("temporary_raw_path", None)
    raw_path.unlink()
    record["sanitized_fixture"] = "working/tabelog-pp-cli/e2e/testdata/" + name

for label, rid, rating, count, station, meters, budget_raw, minimum, maximum in [
    ("old", "1046463", "4.46", 254, "Maruyama Koen Sta.", 133, "JPY 20,000 - JPY 29,999", 20000, 29999),
    ("current", "1073214", "4.48", 266, "Nishi Nijuhatchome Sta.", 270, "JPY 30,000 - JPY 39,999", 30000, 39999),
]:
    oracle["records"].append({"key": "sushi_miyakawa_" + label, "kind": "restaurant_detail", "source": "miyakawa_" + label, "selection_reason": "Real same-name distinct7-digit IDs; historical relocation evidence must survive parsing and saving.", "expected": {"id": rid, "name": "Sushi Miyakawa", "canonical_url": f"https://tabelog.com/en/hokkaido/A0101/A010105/{rid}/", "rating": rating, "displayed_count": {"value": count, "source_label": "reviews", "json_ld_key": "ratingCount"}, "nearest_station": {"label": station, "meters": meters, "distance_reference": "nearest_station_not_user"}, "dinner_budget": {"source_text": budget_raw, "min": minimum, "max": maximum}, "lunch_budget_unknown": True, "status": "Relocated" if label == "old" else None, "source_warnings": ["This is information from before the relocation."] if label == "old" else [], "status_absence_means": "Unknown; do not infer open."}})
oracle["genuine_empty"] = {"source": "ginza_genuine_empty", "expected_count": 0, "page_count_text": "0 results", "empty_subject": 'No restaurants match "Ginza zzztabeloge2ezerocandidates20260927".', "recognition_classes": ["c-page-count", "rstlist-notfound", "rstlist-notfound__subject"], "keyword": "zzztabeloge2ezerocandidates20260927"}
oracle["additional_live_requests"] = {"count": 3, "purpose": "Old/current relocation pair and genuine empty response; one GET each, no retries."}
oracle_path.write_text(json.dumps(oracle, ensure_ascii=False, indent=2) + "\n")
(out / "oracle.json").write_text(json.dumps(oracle, ensure_ascii=False, indent=2) + "\n")
(out / "fixture-manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
(run / "proofs/fixtures/relocation-provenance.json").write_text(json.dumps(relocation, indent=2) + "\n")
empty.pop("temporary_raw_path", None)
empty["sanitized_fixture"] = "working/tabelog-pp-cli/e2e/testdata/ginza-empty.html"
(run / "proofs/fixtures/empty-source-provenance.json").write_text(json.dumps(empty, indent=2) + "\n")
print(json.dumps({"additional_fixtures": 3, "additional_source_requests": 3, "raw_temporary_files_removed": True}))
