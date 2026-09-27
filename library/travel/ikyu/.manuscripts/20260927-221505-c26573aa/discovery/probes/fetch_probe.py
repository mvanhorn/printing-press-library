#!/usr/bin/env python3
"""Read-only public Ikyu GET probe using the mandatory raw fetch helper."""
import argparse, json, subprocess, time
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlparse

p=argparse.ArgumentParser();p.add_argument('url');p.add_argument('--evidence-dir',required=True);a=p.parse_args()
u=urlparse(a.url)
if u.scheme!='https' or u.netloc!='www.ikyu.com' or u.path.startswith(('/booking','/account','/search')):
    raise SystemExit('Only anonymous public accommodation/destination GETs are allowed')
t=time.monotonic();stamp=datetime.now(timezone.utc).isoformat()
r=subprocess.run(['<home>/.agents/skills/printing-press/references/fetch-docs.sh',a.url],capture_output=True,text=True)
elapsed=round(time.monotonic()-t,3)
fields=dict(line.split('=',1) for line in r.stdout.splitlines() if '=' in line)
if not fields.get('path'):
    import re
    match=re.search(r'HTTP (\d+) .*?\(body: (.*?), final: (.*?)\)',r.stderr)
    if match: fields.update(status=match[1],path=match[2],effective_url=match[3])
body=Path(fields['path']) if fields.get('path') else None
result={'requested_url':a.url,'started_at':stamp,'capture_duration_s':elapsed,'duration_scope':'fetch-docs helper including redirect/cache overhead','exit_code':r.returncode,**fields,'body_bytes':body.stat().st_size if body and body.exists() else None}
if r.returncode and not fields: result['error']='fetch failed; no response metadata'
out=Path(a.evidence_dir);out.mkdir(parents=True,exist_ok=True)
with (out/'capture-metadata.jsonl').open('a') as f: f.write(json.dumps(result,ensure_ascii=False)+'\n')
print(json.dumps(result,ensure_ascii=False))
