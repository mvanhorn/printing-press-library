import os,pathlib,re,json,subprocess
r=pathlib.Path(os.environ['API_RUN_DIR']);w=pathlib.Path(os.environ['CLI_WORK_DIR']);text=(r/'research/absorb-manifest.md').read_text();paths=[]
for line in text.splitlines():
 if not line.startswith('|') or '(generated endpoint)' in line:continue
 cells=[x.strip().strip('`') for x in line.split('|')]
 for value in cells:
  if value.startswith('sendfox-pp-cli '):paths.append(value[len('sendfox-pp-cli '):].split(' --')[0]);break
  if value.startswith('workflow ') and len(value.split())==2:paths.append(value);break
results=[]
for path in sorted(set(paths)):
 p=subprocess.run([str(r/'scripts/safe-run.sh'),str(w/'sendfox-pp-cli'),*path.split(),'--help'],capture_output=True,text=True)
 usage=re.search(r'Usage:\s*\n\s*([^\n]+)',p.stdout);actual=usage[1] if usage else ''
 results.append({'command':path,'passed':p.returncode==0 and actual.startswith('sendfox-pp-cli '+path+' ') and '[command]' not in actual,'usage':actual})
(r/'proofs/manifest-gate.json').write_text(json.dumps(results,indent=2)+'\n');print(json.dumps(results,indent=2));assert all(x['passed'] for x in results)
