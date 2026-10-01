#!/usr/bin/env python3
"""Measure real CLI stdout, requests, latency and per-process peak RSS on macOS."""
import datetime as dt
import json
import os
from pathlib import Path
import re
import subprocess
import time

ROOT = Path(__file__).resolve().parents[1]
DATE = (dt.datetime.now(dt.timezone(dt.timedelta(hours=9))) + dt.timedelta(days=1)).date().isoformat()
cases = {'discovery': ['discover', '--region', 'prf130000', '--category', '192', '--limit', '5'], 'product': ['product', 'ticket0000049223'], 'availability': ['availability', 'ticket0000049223', '--date', DATE], 'dated_options': ['options', 'ticket0000049223', '--date', DATE], 'comparison': ['compare', 'ticket0000049223', 'ticket0000012233']}
rows = []
for name, args in cases.items():
    env = dict(os.environ, ASOVIEW_CACHE_DIR=str(ROOT / 'evidence/benchmark-cache' / name))
    prime = subprocess.run([str(ROOT / 'asoview-pp-cli'), *args, '--refresh', '--agent'], env=env, capture_output=True, timeout=70)
    assert prime.returncode == 0, prime.stderr[:600]
    for mode, extra in [('uncached', ['--no-cache']), ('cached', [])]:
        started = time.monotonic()
        p = subprocess.run(['/usr/bin/time', '-l', str(ROOT / 'asoview-pp-cli'), *args, '--agent', *extra], env=env, capture_output=True, timeout=70)
        elapsed = round((time.monotonic() - started) * 1000, 3)
        assert p.returncode == 0, p.stderr[:600]
        j = json.loads(p.stdout)
        match = re.search(rb'(\d+)\s+maximum resident set size', p.stderr)
        assert match, 'time -l did not supply peak RSS'
        rows.append({'command': name, 'mode': mode, 'output_bytes': len(p.stdout), 'network_requests': j['meta']['metrics']['network_requests'], 'cache_hits': j['meta']['metrics']['cache_hits'], 'wall_latency_ms': elapsed, 'cli_latency_ms': j['meta']['metrics']['elapsed_ms'], 'peak_memory_bytes': int(match[1])})
report = {'measured_at': dt.datetime.now(dt.timezone.utc).isoformat(), 'source': 'live anonymous Asoview CLI calls; cached outputs use explicitly primed fresh responses', 'peak_memory_method': 'per-process /usr/bin/time -l maximum resident set size bytes (macOS)', 'rows': rows}
(ROOT / 'evidence/benchmark.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps({'status': 'pass', 'measurements': len(rows), 'max_output_bytes': max(r['output_bytes'] for r in rows), 'max_peak_memory_bytes': max(r['peak_memory_bytes'] for r in rows)}))
