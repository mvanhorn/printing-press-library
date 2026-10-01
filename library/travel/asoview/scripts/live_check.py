#!/usr/bin/env python3
"""Live public-source correctness checks; no fixtures are used by this runner."""
import datetime as dt
import json
import os
from pathlib import Path
import subprocess
import tempfile
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
BIN = ROOT / 'asoview-pp-cli'
OUT = ROOT / 'evidence/live'
OUT.mkdir(parents=True, exist_ok=True)
ENV = dict(os.environ, ASOVIEW_CACHE_DIR=str(ROOT / 'evidence/check-cache'))
ORIGIN = 'https://www.asoview.com'
DATE = (dt.datetime.now(dt.timezone(dt.timedelta(hours=9))) + dt.timedelta(days=1)).date().isoformat()
CHECKS = []
OBS = []

def cli(name, *args, expected=0):
    p = subprocess.run([str(BIN), *args, '--agent'], env=ENV, capture_output=True, text=True, timeout=70)
    if p.returncode != expected:
        raise AssertionError(f'{name}: expected exit {expected}, got {p.returncode}: {p.stderr[:800]}')
    if expected:
        assert p.stdout == '', (name, 'error leaked stdout')
        CHECKS.append({'check': name, 'status': 'pass', 'exit': expected})
        return None
    j = json.loads(p.stdout)
    (OUT / f'{name}.json').write_text(p.stdout.replace('to-guest@asoview.com', 'PII_EMAIL_EXAMPLE'))
    return j

def fetch(path, query=None):
    u = ORIGIN + path
    if query:
        u += '?' + urllib.parse.urlencode(query)
    request = urllib.request.Request(u, headers={'User-Agent': 'asoview-live-readonly-check/0.1'})
    with urllib.request.urlopen(request, timeout=25) as response:
        assert response.status == 200
        assert response.url.startswith(ORIGIN + '/')
        body = response.read((4 << 20) + 1)
        assert len(body) <= 4 << 20
    return u, body.decode()

def datasource(path):
    u, s = fetch(path)
    marker = 'var ASOVIEW_DATASOURCE = '
    assert marker in s
    value = json.JSONDecoder().raw_decode(s.split(marker, 1)[1])[0]
    return u, value

def passed(name, **facts):
    CHECKS.append({'check': name, 'status': 'pass', **facts})

# Compare normalized fields against independently fetched public source state.
u, raw = datasource('/item/ticket/ticket0000049223/')
p = cli('ticket-source', 'product', 'ticket0000049223', '--no-cache')['results']
t = raw['ticketType']
assert p['name_ja'] == t['title'] and p['id'] == t['code']
assert p['inclusions'] == t['overview']['description'] and p['conditions'] == t['usingCondition']
assert p['cancellation'] == t['cancellationPolicy']
assert p['validity']['sales_period'] == t['overview']['salesPeriod']
assert p['validity']['unavailable_periods'] == t['unablePeriods']
assert [b['name_ja'] for b in p['options']] == [b['categoryLabel'] for b in t['tickets']]
assert [b['price']['amount'] for b in p['options']] == [b['sellingPrice'] for b in t['tickets']]
assert p['advertised_price']['amount'] == min(b['sellingPrice'] for b in t['tickets'])
assert p['date_party_total'] is None and p['entry']['reserved_slot'] is None
OBS.append({'source': u, 'id': t['code'], 'title': t['title'], 'bands': [{'id': b['id'], 'label': b['categoryLabel'], 'amount': b['sellingPrice']} for b in t['tickets']]})
passed('ticket terms, bands, validity and cancellations match source')

u, raw = datasource('/item/activity/pln3000044589/')
p = cli('activity-source', 'product', 'pln3000044589', '--no-cache')['results']
a = raw['plan']
assert p['name_ja'] == a['title'] and p['inclusions'] == a['priceIncluded']
assert p['cancellation'] == a['cancelPolicies'] and p['age_band']['text'] == a['ageLimit']
assert p['advertised_price']['amount'] == a['sellingPrice']
passed('activity age, inclusion, price and cancellation variants match source')

j = cli('discovery-source', 'discover', '--region', 'prf130000', '--category', '192', '--limit', '50', '--no-cache')
rows = j['results']
assert len(rows) >= 4 and all(r['kind'] == 'ticket' and r['location']['prefecture_ja'] == '東京都' for r in rows) and sum(r['category']['name_ja'] == '水族館' for r in rows) >= 5
assert any(r['id'] == 'ticket0000049223' for r in rows)
assert all(r['date_party_total'] is None and r['advertised_price']['basis'] == 'advertised_search_from' for r in rows)
passed('Tokyo aquarium query relevance', returned=len(rows))
negative = cli('no-match', 'discover', '--region', 'prf130000', '--category', '192', '--query', 'zzzx-no-match-4829', '--no-cache')
assert negative['results'] == [] and negative['coverage']['scanned_cards'] >= 4
passed('negative local keyword result is empty with bounded scan coverage')
first = cli('cursor-first', 'discover', '--region', 'prf130000', '--category', '192', '--limit', '2')
assert first['coverage']['next_cursor'] is not None
second = cli('cursor-second', 'discover', '--region', 'prf130000', '--category', '192', '--limit', '2', '--cursor', first['coverage']['next_cursor'])
assert not ({r['id'] for r in first['results']} & {r['id'] for r in second['results']})
passed('same-page continuation neither repeats nor skips source cursor position')
filtered = cli('local-relevance', 'discover', '--region', 'prf130000', '--category', '192', '--query', '葛西')
assert filtered['results'] and all('葛西' in r['name_ja'] for r in filtered['results'])
passed('local Japanese relevance contains requested substring')

j = cli('dated-filter', 'discover', '--region', 'prf130000', '--category', '192', '--date', DATE, '--adults', '2', '--children', '1', '--no-cache')
assert j['coverage']['requested_date'] == DATE and j['coverage']['requested_party'] == {'adults': 2, 'children': 1}
assert j['coverage']['date_stock_confirmed'] is False
passed('native date and party candidates labeled separately from stock confirmation')

u, body = fetch('/stocks/ticket/courses', {'planCode': 'ticket0000049223', 'channelCode': 'asoview', 'date': DATE})
source_slots = json.loads(body)
j = cli('dated-stock-source', 'availability', 'ticket0000049223', '--date', DATE, '--quantity', '2', '--no-cache')
slots = j['results']['slots']
assert len(slots) == len(source_slots) and slots
for s, raw in zip(slots, source_slots):
    assert s['id'] == str(raw['id']) and s['remaining_quantity'] == raw['remainReserveNumber']
    assert s['start_time'] == raw['startTimeLabel'] and s['end_time'] == raw['endTimeLabel']
    assert s['price']['amount'] == int(raw['sellingFee'].replace(',', '')) and s['price']['unit'] == raw['unit']
    assert s['time_meaning'] == 'admission_window_on_selected_date' and s['reserved'] is False
passed('dated stock, price unit and date-only opening window match source', date=DATE)
start = source_slots[0]['startTimeLabel']
if len(start.split(':')[0]) == 1:
    start = '0' + start
u, body = fetch('/item/category-sales-situations/', {'ticketTypeCode': 'ticket0000049223', 'channelCode': 'asoview', 'date': DATE, 'time': start})
source_bands = json.loads(body)['categories']
party = f"{source_bands[0]['basicFeeNumber']}:2,{source_bands[-1]['basicFeeNumber']}:1"
j = cli('dated-bands-source', 'options', 'ticket0000049223', '--date', DATE, '--party', party, '--no-cache')
assert [b['price']['amount'] for b in j['results']['options']] == [b['sellingFee'] for b in source_bands]
assert [b['price']['unit'] for b in j['results']['options']] == [b['unit'] for b in source_bands]
sub = j['results']['party_subtotal']
assert sub['amount'] == source_bands[0]['sellingFee'] * 2 + source_bands[-1]['sellingFee']
assert sub['quote_confirmed'] is False and j['results']['date_party_total'] is None
passed('dated bands and arithmetic subtotal match source without claiming quote')

j = cli('activity-dated-bands', 'options', 'pln3000044589', '--date', DATE, '--no-cache')
u, body = fetch(f'/reservations/plans/pln3000044589/dates/{DATE}/basicfees')
source_bands = json.loads(body)
assert [b['name_ja'] for b in j['results']['options']] == [b['feeLabel'] for b in source_bands]
assert j['results']['product_age_band']['text'] == a['ageLimit']
passed('dated activity bands preserve source label and product age separately')

u, body = fetch('/stocks/courses', {'planCode': 'pln3000044589', 'date': DATE})
activity_slots = json.loads(body)
j = cli('activity-stock-status', 'availability', 'pln3000044589', '--date', DATE, '--no-cache')
assert len(j['results']['slots']) == len(activity_slots)
for normalized, raw in zip(j['results']['slots'], activity_slots):
    assert normalized['id'] == str(raw['id'])
    if raw.get('isClosed'):
        assert normalized['status'] == 'closed'
all_closed = bool(activity_slots) and all(s.get('isClosed') for s in activity_slots)
if all_closed:
    assert j['results']['status'] == 'closed'
passed('activity closed slot and headline status match live source', all_closed=all_closed, headline=j['results']['status'])

with tempfile.TemporaryDirectory(prefix='asoview-cache-failure-') as temporary:
    blocked = Path(temporary) / 'not-a-directory'
    blocked.write_text('synthetic cache storage failure')
    original_cache = ENV['ASOVIEW_CACHE_DIR']
    try:
        ENV['ASOVIEW_CACHE_DIR'] = str(blocked)
        j = cli('cache-write-failure-live', 'product', 'ticket0000049223', '--refresh')
    finally:
        ENV['ASOVIEW_CACHE_DIR'] = original_cache
    assert j['results']['id'] == 'ticket0000049223' and j['meta']['source'] == 'live'
    assert j['meta']['metrics']['cache_write_failures'] == 1
    assert j['meta']['sources'] and all(s['cached'] is False for s in j['meta']['sources'])
passed('successful live product survives a synthetic optional cache storage failure')

j = cli('timed-stock', 'availability', 'ticket0000012832', '--date', DATE, '--no-cache')
assert j['results']['slots'] and all(s['time_meaning'] in ('reserved_entry_window', 'source_time_window_unclassified') for s in j['results']['slots'])
assert all(s['status'] != 'party_outside_limits' for s in j['results']['slots'] if s['maximum_quantity'] == 0)
j = cli('timed-bands-selection', 'options', 'ticket0000012832', '--date', DATE, '--no-cache')
if len(j['results']['slots']) > 1:
    assert j['results']['coverage'] == 'select_slot_to_fetch_dated_bands' and j['results']['options'] == []
else:
    assert j['results']['coverage'] == 'date_specific_bands_not_checkout_quote' and j['results']['options']
passed('explicit timed schedules distinguished; legacy ambiguous windows remain unclassified; one slot auto-selects')
j = cli('general-validity', 'availability', 'ticket0000034693', '--date', DATE, '--no-cache')
assert j['results']['entry']['selection'] == 'no_reserved_slot_in_product' and j['results']['slots'] == []
assert j['results']['coverage']['public_dated_stock'] is False
passed('general admission validity never fabricated as reserved inventory')

j = cli('comparison', 'compare', 'ticket0000049223', 'ticket0000012233', '--no-cache')
assert len(j['results']) == 2 and all(x['cancellation'] is not None for x in j['results'])
assert j['comparison']['date_party_total'] is None
passed('comparison preserves both source cancellation sets and missing party total')
j = cli('projection', 'discover', '--region', 'prf130000', '--category', '192', '--select', 'meta,results.id,results.name_ja,results.booking_url')
assert all(set(r) == {'id', 'name_ja', 'booking_url'} for r in j['results'])
passed('dotted field projection retains only requested result fields')
j = cli('inventory-refresh', 'inventory', '--kind', 'categories', '--query', '水族館', '--refresh-inventory')
assert j['inventory']['basis'] == 'explicit_live_inventory_refresh' and j['meta']['metrics']['network_requests'] == 1
j = cli('inventory-reuse', 'inventory', '--kind', 'categories', '--query', '水族館')
assert j['inventory']['basis'] == 'locally_refreshed_first_party_inventory' and j['meta']['metrics']['network_requests'] == 0
passed('taxonomy refresh is explicit, persisted and reused without network')
j = cli('handoff', 'handoff', 'ticket0000049223')
assert j['results']['booking_url'] == ORIGIN + '/item/ticket/ticket0000049223/' if 'results' in j else j['booking_url'] == ORIGIN + '/item/ticket/ticket0000049223/'
passed('canonical handoff derives source URL without transaction')

cli('bad-date', 'availability', 'ticket0000049223', '--date', '2026-02-30', expected=2)
cli('bad-limit', 'discover', '--limit', '1000', expected=2)
cli('bad-id', 'product', 'not-a-source-id', expected=2)
cli('wrong-host', 'product', 'https://evil.example/item/ticket/ticket0000049223/', expected=2)
cli('bad-party', 'options', 'ticket0000049223', '--party', '2411665:2', expected=2)
report = {'status': 'pass', 'checked_at': dt.datetime.now(dt.timezone.utc).isoformat(), 'requested_date': DATE, 'fixture_use': False, 'sample_redaction': 'Stored samples replace the published provider email with PII_EMAIL_EXAMPLE; runtime comparisons use unredacted live data.', 'checks': CHECKS, 'source_observations': OBS}
(ROOT / 'evidence/live-source-checks.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({'status': 'pass', 'checks': len(CHECKS), 'date': DATE, 'evidence': 'evidence/live-source-checks.json'}))
