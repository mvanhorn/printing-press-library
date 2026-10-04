package reviewer
import("encoding/json";"fmt";"net/http";"net/http/httptest";"os";"path/filepath";"strings";"testing";"time";"github.com/mvanhorn/cli-printing-press/v4/internal/pipeline")
func TestTabiwaReviewerSupportedLocalWriteOptIn(t *testing.T){
 dir:=t.TempDir()
 if err:=pipeline.WriteCLIManifest(dir,pipeline.CLIManifest{SchemaVersion:1,APIName:"fixture",CLIName:"fixture-pp-cli",RunID:"tabiwa-reviewer-local-write-opt-in",AuthType:"none"});err!=nil{t.Fatal(err)}
 methods:=[]string{}
 source:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){methods=append(methods,r.Method);fmt.Fprint(w,`{"response":[{"id":"J0001900","name":"ポイント券"}]}`)}));defer source.Close()
 logPath:=filepath.Join(t.TempDir(),"invocations.jsonl");t.Setenv("PP_REVIEW_LOG",logPath);t.Setenv("PP_REVIEW_SOURCE",source.URL)
 sentinelHome:=t.TempDir();sentinel:=filepath.Join(sentinelHome,"sentinel");os.WriteFile(sentinel,[]byte("unchanged"),0600);t.Setenv("FIXTURE_HOME",sentinelHome)
 script:=`#!/usr/bin/env python3
import json,os,pathlib,sys,urllib.request
args=sys.argv[1:]
if args and args[0]=='agent-context':
 a={'mcp:read-only':'false','mcp:local-write':'true','pp:happy-args':'--save'}
 if os.getenv('PP_REVIEW_LIVE_HINT')=='true':a['pp:live-happy-path']='true'
 print(json.dumps({'commands':[{'name':'save','annotations':a}]}));sys.exit(0)
if '--help' in args:
 print('''Save selected local evidence using a public GET.

Usage:
  fixture-pp-cli save [flags]

Examples:
  fixture-pp-cli save --save --dry-run

Flags:
      --save       Persist selected evidence

Global Flags:
      --dry-run    Show request without sending
      --json       Output as JSON
''');sys.exit(0)
if not args or args[0]!='save':sys.exit(2)
if '--dry-run' in args:
 print('{"dry_run":true,"action":"preview"}');sys.exit(0)
with urllib.request.urlopen(os.environ['PP_REVIEW_SOURCE']) as response:data=json.load(response)
cache=pathlib.Path(os.environ['XDG_CACHE_HOME'])/'fixture-pp-cli'/'selected.json'
cache.parent.mkdir(parents=True,exist_ok=True);cache.write_text(json.dumps(data))
record={'args':args,'cwd':os.getcwd(),'storage_file':str(cache),'storage_exists_at_execution':cache.exists(),'relocation_env_present':bool(os.getenv('FIXTURE_HOME')),'provider_method':'GET'}
with open(os.environ['PP_REVIEW_LOG'],'a') as log:log.write(json.dumps(record)+'\n')
print(json.dumps({'saved':True,'products':data['response'],'storage_file':str(cache)}))
`
 if err:=os.WriteFile(filepath.Join(dir,"fixture-pp-cli"),[]byte(script),0755);err!=nil{t.Fatal(err)}
 cases:=[]struct{name string;hint,allow bool;dry bool}{{"annotation_alone",true,false,true},{"operator_flag_alone",false,true,true},{"both_keys",true,true,false}}
 proof:=map[string]any{"cached_sdk":"<go-module-cache>/github.com/mvanhorn/cli-printing-press/v4@v4.33.0","cases":[]map[string]any{}}
 for _,c:=range cases{
 t.Setenv("PP_REVIEW_LIVE_HINT",fmt.Sprint(c.hint))
 report,err:=pipeline.RunLiveDogfood(pipeline.LiveDogfoodOptions{CLIDir:dir,BinaryName:"fixture-pp-cli",Level:"full",Timeout:5*time.Second,AllowDestructive:c.allow});if err!=nil{t.Fatal(err)}
 found:=false
 for _,r:=range report.Tests{if r.Command=="save" && string(r.Kind)=="happy_path"{found=true;dry:=false;for _,a:=range r.Args{if a=="--dry-run"{dry=true}}
 if dry!=c.dry||string(r.Status)!="pass"{t.Fatalf("%s argv=%v status=%s reason=%s",c.name,r.Args,r.Status,r.Reason)}
 proof["cases"]=append(proof["cases"].([]map[string]any),map[string]any{"name":c.name,"annotation":c.hint,"allow_destructive":c.allow,"actual_argv":r.Args,"status":r.Status,"output_sample":r.OutputSample})}}
 if !found{b,_:=json.Marshal(report.Tests);t.Fatalf("happy not found %s",b)}}
 raw,err:=os.ReadFile(logPath);if err!=nil{t.Fatal(err)};lines:=strings.Split(strings.TrimSpace(string(raw)),"\n");if len(lines)!=1{t.Fatalf("real saves=%d",len(lines))}
 var record map[string]any;if err:=json.Unmarshal([]byte(lines[0]),&record);err!=nil{t.Fatal(err)};storage:=record["storage_file"].(string)
 if strings.HasPrefix(storage,sentinelHome+string(os.PathSeparator))||record["relocation_env_present"]==true||record["cwd"]==dir||record["storage_exists_at_execution"]!=true{t.Fatalf("isolation %s",lines[0])}
 if _,err:=os.Stat(storage);!os.IsNotExist(err){t.Fatal("scope cleanup failed")};unchanged,err:=os.ReadFile(sentinel);if err!=nil||string(unchanged)!="unchanged"{t.Fatal("operator sentinel changed")}
 if len(methods)!=1||methods[0]!="GET"{t.Fatalf("methods=%v",methods)}
 proof["actual_execution"]=record;proof["provider_methods"]=methods;proof["isolation_cleanup_pass"]=true;proof["operator_sentinel_unchanged"]=true
 b,_:=json.MarshalIndent(proof,"","  ");if err:=os.WriteFile("<temporary>/tabiwa-local-write-runner-recheck.json",append(b,'\n'),0600);err!=nil{t.Fatal(err)}
}
