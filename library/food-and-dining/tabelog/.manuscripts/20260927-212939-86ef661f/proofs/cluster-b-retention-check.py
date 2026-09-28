import argparse
import datetime
import json
from pathlib import Path
import sqlite3
import statistics
import subprocess
import time

parser = argparse.ArgumentParser()
parser.add_argument("stage", choices=["before", "after"])
parser.add_argument("binary")
args = parser.parse_args()
proofs = Path(__file__).resolve().parent
home = proofs / "cluster-b-retention-fixture"
home.mkdir(exist_ok=True)
command = [args.binary, "lists", "show", "--agent", "--home", str(home), "--timeout", "10s"]
run = subprocess.run(command, capture_output=True, text=True, timeout=15, check=True)
paths = list(home.rglob("data.db"))
assert len(paths) == 1, paths
db = sqlite3.connect(paths[0])
if args.stage == "before":
    assert db.execute("SELECT COUNT(*) FROM tabelog_snapshots").fetchone()[0] == 0
    now = datetime.datetime.now(datetime.timezone.utc).isoformat()
    snapshots = []
    for i in range(10000):
        venue_id = str(19000000 + i)
        raw = json.dumps({"id": venue_id, "name": "Private performance fixture", "url": "https://tabelog.com/en/tokyo/A1301/A130101/" + venue_id + "/", "fixture_padding": "x" * 128}, separators=(",", ":"))
        snapshots.append((venue_id, raw, now))
    with db:
        db.executemany("INSERT INTO tabelog_snapshots VALUES(?,?,?)", snapshots)
        db.execute("INSERT INTO tabelog_notebooks VALUES(?,?,?)", ("private-performance", now, now))
        db.executemany("INSERT INTO tabelog_memberships VALUES(?,?,?,?,?,?)", [("private-performance", row[0], "Private retained note", i, now, now) for i, row in enumerate(snapshots[:9000])])

query = "SELECT COALESCE(SUM(length(CAST(data AS BLOB))),0) FROM tabelog_snapshots s WHERE NOT EXISTS (SELECT 1 FROM tabelog_memberships m WHERE m.restaurant_id=s.id)"
plan = [row[3] for row in db.execute("EXPLAIN QUERY PLAN " + query)]
times = []
for _ in range(3):
    start = time.perf_counter()
    unsaved_bytes = db.execute(query).fetchone()[0]
    times.append((time.perf_counter() - start) * 1000)
counts = db.execute("SELECT COUNT(*), SUM(EXISTS(SELECT 1 FROM tabelog_memberships m WHERE m.restaurant_id=s.id)) FROM tabelog_snapshots s").fetchone()
saved_bytes = db.execute("SELECT COALESCE(SUM(length(CAST(data AS BLOB))),0) FROM tabelog_snapshots s WHERE EXISTS (SELECT 1 FROM tabelog_memberships m WHERE m.restaurant_id=s.id)").fetchone()[0]
result = {"stage": args.stage, "source_requests": 0, "fixture_home": str(home), "init_command": command, "snapshots": counts[0], "saved_snapshots": counts[1], "unsaved_snapshots": counts[0] - counts[1], "saved_payload_bytes": saved_bytes, "unsaved_payload_bytes": unsaved_bytes, "query_plan": plan, "durations_ms": times, "median_ms": statistics.median(times)}
assert result["snapshots"] == 10000 and result["saved_snapshots"] == 9000 and result["unsaved_snapshots"] == 1000
if args.stage == "after":
    before = json.loads((proofs / "cluster-b-retention-before.json").read_text())
    for key in ("snapshots", "saved_snapshots", "unsaved_snapshots", "saved_payload_bytes", "unsaved_payload_bytes"):
        assert before[key] == result[key], (key, before[key], result[key])
    assert any("SEARCH m" in line and "restaurant_id=?" in line for line in plan), plan
    result["speedup_ratio"] = before["median_ms"] / result["median_ms"]
db.close()
(proofs / ("cluster-b-retention-" + args.stage + ".json")).write_text(json.dumps(result, indent=2) + "\n")
print(json.dumps(result, indent=2))
