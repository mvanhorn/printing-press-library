#!/usr/bin/env python3
import json,sys,subprocess
from pathlib import Path
root=Path(__file__).resolve().parents[1]
state=Path(json.loads((root/'.press-run.json').read_text())['state_file'])
s=json.loads(state.read_text());run=state.parent
if len(sys.argv)==1: print(json.dumps(s,indent=2));sys.exit()
if sys.argv[1]=='path':print(run);sys.exit()
cmd=[s['printing_press_bin'],'phase-receipt',sys.argv[1],'--file',s['phase_receipt_log'],'--run-id',s['run_id']]+sys.argv[2:]
subprocess.run(cmd,check=True,stdout=subprocess.DEVNULL)
