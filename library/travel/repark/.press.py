#!/usr/bin/env python3
import json,subprocess,os,sys
from pathlib import Path
s=json.load(open(Path(__file__).with_name('.press-state-pointer').read_text().strip()))
e=os.environ.copy();e['PRINTING_PRESS_HOME']=s['press_home']
args=[s['printing_press_bin'],'phase-receipt',sys.argv[1],'--file',s['phase_receipt_log'],'--run-id',s['run_id']]+sys.argv[2:]
r=subprocess.run(args,env=e,capture_output=True,text=True)
if r.returncode: print(r.stdout+r.stderr);sys.exit(r.returncode)
d=json.loads(r.stdout);a=d.get('receipt',d.get('latest',d)); print(json.dumps(a,ensure_ascii=False))
