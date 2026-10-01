#!/usr/bin/env python3
from pathlib import Path
import json,shutil,hashlib
root=Path(__file__).resolve().parents[1];r=Path(json.loads((root/'.press-run.json').read_text())['state_file']).parent
# Preserve endpoint/schema provenance, omitting bulky personal photo/report data.
summary=[]
for p in sorted((r/'discovery').glob('*.raw')):
 summary.append({'artifact':p.name,'bytes':p.stat().st_size,'sha256':hashlib.sha256(p.read_bytes()).hexdigest(),'scope':'public first-party source research; raw SSR report streams omitted from archive'})
(r/'discovery/source-capture-index.json').write_text(json.dumps(summary,indent=2)+'\n')
# Schema arrays/scripts are recoverable from public source; keep the provenance index.
for p in list((r/'discovery').glob('*.raw'))+list((r/'discovery').glob('*.decoded.json')):p.unlink()
har=r/'discovery/browser-sniff-capture.har'
if har.exists():
 a=json.loads(har.read_text())
 for x in a['log']['entries']:x['response'].get('content',{}).pop('text',None)
 har.write_text(json.dumps(a,ensure_ascii=False,indent=2)+'\n')
# Final report and compact proof files are the durable consumer-facing evidence.
for name in ['final-report.md','independent-review.md','polish.md','paths.json','shipcheck.log','go-test.log','go-vet.log','gosec.json','tools-audit.txt','pii-audit.json','dogfood-live.json','review-projection-regression.json','review-format-regression.json']:
 p=root/'evidence'/name
 if p.exists():shutil.copy2(p,r/'proofs'/name)
for name in ['live','performance','edges']:
 if (root/'evidence'/name).exists():shutil.copytree(root/'evidence'/name,r/'proofs'/name,dirs_exist_ok=True)
