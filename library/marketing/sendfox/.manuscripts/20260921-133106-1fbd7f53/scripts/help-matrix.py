import os,pathlib,json,subprocess
r=pathlib.Path(os.environ['API_RUN_DIR']);w=pathlib.Path(os.environ['CLI_WORK_DIR']);proof=r/'proofs'
d=json.loads((proof/'agent-context.json').read_text());rows=[]
def walk(nodes,prefix=[]):
 for n in nodes:
  argv=prefix+[n['name']]
  p=subprocess.run([str(r/'scripts/safe-run.sh'),str(w/'sendfox-pp-cli'),*argv,'--help'],cwd=w,text=True,capture_output=True,timeout=20)
  rows.append({'command':' '.join(argv),'exit':p.returncode,'has_usage':'Usage:' in p.stdout,'leaf':not n.get('subcommands'),'endpoint':n.get('annotations',{}).get('pp:endpoint')})
  walk(n.get('subcommands',[]),argv)
walk(d['commands']); result={'commands':len(rows),'leaves':sum(x['leaf'] for x in rows),'endpoint_commands':sum(bool(x['endpoint']) for x in rows),'help_passed':sum(x['exit']==0 and x['has_usage'] for x in rows),'rows':rows}
(proof/'help-matrix.json').write_text(json.dumps(result,indent=2)+'\n');print({k:v for k,v in result.items() if k!='rows'});assert result['help_passed']==len(rows)
