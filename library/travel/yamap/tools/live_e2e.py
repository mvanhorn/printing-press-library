#!/usr/bin/env python3
"""Bounded anonymous live correctness and performance proof; never uses fixtures."""
import datetime
import json
import os
from pathlib import Path
import re
import subprocess
import time
import tempfile
import urllib.request

PROJECT = Path(__file__).resolve().parents[1]
PROOFS = Path(os.environ.get('YAMAP_E2E_OUTPUT', tempfile.mkdtemp(prefix='yamap-live-e2e-')))
BIN = Path(os.environ.get('YAMAP_E2E_BINARY', str(PROJECT / 'yamap-pp-cli')))
ENV = os.environ.copy()
ENV.update(YAMAP_HOME=str(PROOFS / 'live-runtime'), YAMAP_NO_LEARN='1')
PROOFS.mkdir(parents=True, exist_ok=True)
records = []

def invoke(label, args, expected=0):
    started = time.monotonic()
    p = subprocess.run(['/usr/bin/time', '-l', str(BIN), *args, '--agent', '--diagnostics'], env=ENV, capture_output=True, timeout=45)
    elapsed = time.monotonic() - started
    stderr = p.stderr.decode()
    match = re.search(r'(\d+)\s+maximum resident set size', stderr)
    row = {'case': label, 'args': args, 'exit': p.returncode, 'stdout_bytes': len(p.stdout), 'latency_ms': round(elapsed * 1000, 2), 'peak_memory_bytes': int(match.group(1)) if match else None}
    assert p.returncode == expected, (label, row, stderr[:2000])
    if expected == 0:
        data = json.loads(p.stdout)
        assert data['meta']['official_closure_coverage'] == 'unknown'
        assert data['meta']['safety_status'] is None
        row.update(request_count=data['meta']['request_count'], cache_hits=data['meta']['cache_hits'], upstream_bytes=data['meta']['response_bytes'], returned=data['meta']['returned'])
        (PROOFS / f'live-{label}.json').write_bytes(p.stdout)
    else:
        assert not p.stdout, (label, 'failure emitted result rows')
        data = None
        row['stderr'] = stderr.splitlines()[0:3]
    records.append(row)
    return data

def independent_source(data, field):
    source_url = data['meta']['fetches'][0]['url']
    request = urllib.request.Request(source_url, headers={'Accept-Language': 'ja', 'User-Agent': 'Mozilla/5.0 (compatible; YAMAP-CLI-live-verification/0.1)'})
    time.sleep(0.5)
    with urllib.request.urlopen(request, timeout=20) as response:
        assert response.status == 200
        raw = json.load(response)
    return raw[field]

for kind, field, namefield in [('mountains', 'mountains', 'name'), ('routes', 'model_courses', 'name'), ('reports', 'activities', 'title'), ('maps', 'maps', 'name')]:
    data = invoke(kind + '-search', [kind, 'search', '高尾山', '--limit', '3', '--refresh'])
    source = independent_source(data, field)
    assert len(data['results']) == min(3, len(source))
    for actual, upstream in zip(data['results'], source):
        assert actual['id'] == str(upstream['id'])
        assert actual[namefield] == upstream[namefield]
        if kind != 'reports':
            assert '高尾山' in actual[namefield], (kind, actual)
        else:
            text = upstream['title'] + upstream.get('description', '') + json.dumps(upstream.get('map'), ensure_ascii=False)
            assert '高尾山' in text, actual
            assert actual['metrics']['distance_m'] == upstream.get('distance')
    assert data['meta']['request_count'] == 1

for kind, ident, key in [('mountains', '108', 'mountain'), ('routes', '1771', 'model_course'), ('reports', '51497803', 'activity'), ('maps', '77', 'map')]:
    data = invoke(kind + '-detail', [kind, 'get', ident, '--refresh'])
    source = independent_source(data, key)
    assert data['results'][0]['id'] == str(source['id'])
    if kind == 'reports':
        metrics = data['results'][0]['metrics']
        assert metrics['source'] == 'activity_whole_section'
        for output, wire in [('elapsed_seconds', 'total_time'), ('active_seconds', 'active_time'), ('rest_seconds', 'rest_time'), ('distance_m', 'distance')]:
            assert metrics[output] == source['activity_whole_section'][wire]
        assert metrics['moving_seconds'] is None
        assert data['results'][0]['observation_text'] == source['description'][:1600]
    if kind == 'routes':
        assert data['results'][0]['route_kind'] == 'planned_model_course'
        assert data['results'][0]['metrics']['standard_time_seconds'] == source['course_time']
        assert data['results'][0]['trail_open'] is None
    if kind == 'maps':
        assert data['results'][0]['bounds_lon_lat'] == source['bound']

for name in ['routes', 'reports']:
    data = invoke('mountain-' + name, ['mountains', name, '108', '--limit', '3', '--refresh'])
    assert data['results'] and data['meta']['match_basis'].startswith('source_mountain_relationship')
    source = independent_source(data, 'model_courses' if name == 'routes' else 'activities')
    assert [r['id'] for r in data['results']] == [str(r['id']) for r in source[:3]]

recent = invoke('recent', ['reports', 'recent', '--mountain-id', '108', '--days', '30', '--limit', '3', '--refresh'])
assert recent['meta']['scanned_records'] <= 20
for item in recent['results']:
    assert item['started_at'] >= recent['meta']['since']
    assert item['recency_basis'] == 'activity_start_date'
    assert item['is_planned'] is not True
assert recent['meta']['partial_coverage'] is True

observation = invoke('observations', ['reports', 'observations', '51497803'])
assert observation['results'][0]['evidence_kind'] == 'contributor_observation'
assert observation['results'][0]['trail_open'] is None
coverage = invoke('coverage', ['maps', 'coverage', '77'])
assert coverage['results'][0]['offline_navigation_included'] is False
comparison = invoke('compare', ['routes', 'compare', '1771', '51497803'])
assert comparison['results'][0]['route_equivalence_verified'] is False
assert comparison['meta']['request_count'] <= 2
inventory = invoke('inventory', ['inventory', 'status'])
assert inventory['results'][0]['complete_inventory'] is False
assert inventory['meta']['request_count'] == 0

invoke('unknown-id', ['reports', 'get', '999999999999', '--no-cache'], 3)
invoke('bad-limit', ['reports', 'search', '高尾山', '--limit', '0'], 2)
invoke('wrong-provider', ['reports', 'get', 'https://example.org/activities/1'], 2)
invoke('offline-miss', ['reports', 'get', '123456789012345', '--data-source', 'local'], 3)
empty = invoke('no-match', ['mountains', 'search', '存在しない山XYZ987654321', '--limit', '2', '--refresh'])
assert empty['results'] == []
page = invoke('pagination', ['mountains', 'search', '高尾山', '--page', '2', '--limit', '3', '--refresh'])
assert page['meta']['source_pagination']['current_page'] == 2

uncached = invoke('benchmark-uncached', ['reports', 'search', '高尾山', '--limit', '5', '--refresh'])
cached = invoke('benchmark-cached', ['reports', 'search', '高尾山', '--limit', '5'])
assert uncached['results'] == cached['results']
assert uncached['meta']['request_count'] == 1 and cached['meta']['request_count'] == 0 and cached['meta']['cache_hits'] == 1
projected = invoke('benchmark-projected', ['reports', 'search', '高尾山', '--limit', '5', '--select', 'id,title,url'])
assert all(set(r) == {'id', 'title', 'url'} for r in projected['results'])
offline = invoke('benchmark-offline', ['reports', 'search', '高尾山', '--limit', '5', '--data-source', 'local'])
assert offline['meta']['request_count'] == 0
report = {'verified_at': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'data_kind': 'live first-party reads, no fixtures', 'passed': len(records), 'cases': records}
(PROOFS / 'live-e2e.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({'passed': len(records), 'benchmarks': [r for r in records if r['case'].startswith('benchmark')]}, ensure_ascii=False))
