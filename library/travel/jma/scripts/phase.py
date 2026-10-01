import json,subprocess,sys
from pathlib import Path
s=json.loads(Path('evidence/press-state.json').read_text())
for i,v in enumerate(sys.argv):
 if i and sys.argv[i-1]=='--evidence':sys.argv[i]=str(Path(v).resolve())
r=subprocess.run([s['printing_press_bin'],'phase-receipt',sys.argv[1],'--file',s['phase_receipt_log'],'--run-id',s['run_id'],*sys.argv[2:]],capture_output=True,text=True)
if r.returncode:print(r.stdout+r.stderr);sys.exit(r.returncode)
x=json.loads(r.stdout);v=x.get('receipt',x);print(v.get('phase'),v.get('event'),v.get('next',''))
