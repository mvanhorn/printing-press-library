import json,subprocess,sys
s=json.load(open("evidence/run.json"))
r=subprocess.run([s["printing_press_bin"],"phase-receipt",*sys.argv[1:],"--file",s["phase_receipt_log"],"--run-id",s["run_id"]],capture_output=True,text=True)
if r.returncode: print(r.stdout+r.stderr);sys.exit(r.returncode)
v=json.loads(r.stdout);prev=v.get("previous",{});cur=v.get("receipt",{});print(json.dumps({"previous":{k:prev.get(k) for k in ["phase","event","next"]},"receipt":{k:cur.get(k) for k in ["phase","event","next","note"]}}))
