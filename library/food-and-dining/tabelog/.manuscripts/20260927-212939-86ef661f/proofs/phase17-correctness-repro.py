from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import hashlib
import json
import os
import subprocess
import tempfile
import threading

run = Path(__file__).resolve().parents[1]
work = run / 'working/tabelog-pp-cli'
binary = work / 'tabelog-pp-cli'
fixture = work / 'e2e/testdata'
sushi = (fixture / 'sushi-detail.html').read_bytes()
ranked = (fixture / 'tokyo-ranked.html').read_text()
wrong_sort = ranked.replace('navi-rstlst__tab--rank is-active', 'navi-rstlst__tab--rank').replace('navi-rstlst__tab navi-rstlst__tab--trend"', 'navi-rstlst__tab navi-rstlst__tab--trend is-active"').replace('SrtT=rt', 'SrtT=inbound_most_reserved').encode()
class Replay(BaseHTTPRequestHandler):
    def do_GET(self):
        body = wrong_sort if self.path.startswith('/en/tokyo/rstLst/') else sushi
        self.send_response(200)
        self.send_header('Content-Type', 'text/html; charset=utf-8')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def log_message(self, *args):
        pass
server = ThreadingHTTPServer(('127.0.0.1', 0), Replay)
thread = threading.Thread(target=server.serve_forever, daemon=True)
thread.start()
env = dict(os.environ, TABELOG_TEST_MODE='1', TABELOG_TEST_BASE_URL=f'http://127.0.0.1:{server.server_port}', HTTP_PROXY='', HTTPS_PROXY='', ALL_PROXY='', NO_PROXY='127.0.0.1,localhost')
report = {'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(), 'scope': 'Controlled loopback replay; real captured source detail/listing fixtures; deliberate wrong-identity and active-sort mutations; no Internet request; disposable home only.'}
try:
    with tempfile.TemporaryDirectory(prefix='tabelog-correctness-review-') as temp:
        home = str(Path(temp).resolve())
        def invoke(name, args):
            result = subprocess.run([str(binary), '--home', home, *args], cwd=work, env=env, capture_output=True, text=True, timeout=25)
            try: payload = json.loads(result.stdout)
            except ValueError: payload = None
            report[name] = {'args': args, 'exit_code': result.returncode, 'stderr': result.stderr, 'payload': payload}
        invoke('wrong_detail_identity', ['show', 'https://tabelog.com/en/tokyo/A1301/A130101/13005012/', '--data-source', 'live', '--agent', '--select', 'items.id,items.name,items.url,items.area'])
        invoke('default_facilities', ['show', 'https://tabelog.com/en/tokyo/A1301/A130103/13294162/', '--data-source', 'live', '--agent'])
        invoke('projected_facilities', ['show', 'https://tabelog.com/en/tokyo/A1301/A130103/13294162/', '--data-source', 'local', '--agent', '--select', 'items.id,items.facilities'])
        invoke('wrong_active_sort', ['find', '--area', 'tokyo', '--data-source', 'live', '--limit', '1', '--agent'])
        invoke('unknown_flag_machine_error', ['--agent', 'find', '--not-a-real-flag'])
finally:
    server.shutdown()
    server.server_close()
proof = run / 'proofs/phase17-correctness-repro.json'
proof.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
summary = {}
for name, item in report.items():
    if not isinstance(item, dict) or 'exit_code' not in item: continue
    p = item['payload'] or {}
    summary[name] = {'exit_code': item['exit_code'], 'items': p.get('items'), 'source_sort': p.get('meta',{}).get('source_sort'), 'stderr': item['stderr']}
print(json.dumps(summary, ensure_ascii=False))
