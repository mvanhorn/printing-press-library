import os,re,sys,json,time,html,hashlib,subprocess,datetime
from curl_cffi import requests
RD=os.environ["RD"]; SD=os.environ["SD"]
CF=open(f"{SD}/cf_clearance.txt").read().strip()
UA=open(f"{SD}/ua.txt").read().strip()
MINTED=int(open(f"{SD}/minted_at.txt").read().strip())
DEADLINE=MINTED+27*60          # 27-min safety margin inside the measured 30-min TTL
AJ="https://www.cdcpakistan.com/wp-admin/admin-ajax.php"
s=requests.Session(impersonate="chrome124")
s.headers.update({"User-Agent":UA,"Content-Type":"application/x-www-form-urlencoded; charset=UTF-8"})
s.cookies.set("cf_clearance",CF,domain=".cdcpakistan.com")
CATS=["miscellaneous","list-of-securities","publications","notices","circulars",
      "annual-reports","quarterly-accounts","disciplinary-registers","guidelines",
      "procedures","newsletter","forms","designated-time-schedule",
      "tariff-fee-structure","sustainability-report"]
YEARS=list(range(2026,2006,-1))
ITEM=re.compile(r'(?is)<div class="download_list">.*?<h4>(.*?)</h4>.*?<div class="meta">(.*?)</div>.*?href="([^"]+)"')
CHAL=re.compile(r'Just a moment|cf-mitigated|cf_chl_opt',re.I)
items={}; buckets=[]; delay=1.0; nreq=0
try:
    _prev=json.load(open(f"{RD}/discovery/corpus-sweep.json"))
    for _b in _prev.get("buckets",[]): buckets.append(_b)
    for _i in _prev.get("items",[]): items[_i["url"]]=_i
    print(f"RESUME: {len(buckets)} buckets, {len(items)} files carried forward",flush=True)
except Exception as _e:
    print(f"no checkpoint to resume ({_e})",flush=True)
_settled=set()
for _b in buckets:
    if _b.get("note") in ("ok","empty-terminator","REPEAT-BLOCK-terminator"):
        _settled.add((_b["category"],_b["year"]))
OUT=f"{RD}/discovery/corpus-sweep.json"
def save():
    json.dump({"minted_at":MINTED,"requests":nreq,"buckets":buckets,
               "items":list(items.values())},open(OUT,"w"),indent=1)
def post(term,year,paged):
    global nreq
    nreq+=1
    r=s.post(AJ,data={"action":"update_posts_by_year","year":str(year),"paged":str(paged),
        "cpt":"downloads","taxonomy":"downloads_category","term":term},timeout=45)
    return r
for cat in CATS:
    for y in YEARS:
        if time.time()>DEADLINE:
            buckets.append({"category":cat,"year":y,"note":"DEADLINE-STOP-clearance-expiring"})
            print(f"!! DEADLINE reached at {nreq} requests -- stopping cleanly, checkpointed",flush=True)
            save(); sys.exit(3)
        if (cat,y) in _settled:
            continue
        seen=set(); npg=0; nit=0
        for pg in range(1,40):
            try: r=post(cat,y,pg)
            except Exception as e:
                buckets.append({"category":cat,"year":y,"paged":pg,"status":None,"note":f"exception:{type(e).__name__}"})
                break
            time.sleep(delay)
            if r.status_code==429:
                print("   429 -> backing off 30s",flush=True); time.sleep(30); delay=min(delay*2,10); continue
            if r.status_code!=200 or CHAL.search(r.text[:2000]):
                buckets.append({"category":cat,"year":y,"paged":pg,"status":r.status_code,
                                "note":"TRANSPORT-ERROR-challenge"})
                print(f"!! CHALLENGE/403 at {cat}/{y}/p{pg} status={r.status_code} -- clearance dead, ABORTING (not recording as empty)",flush=True)
                save(); sys.exit(4)
            found=ITEM.findall(r.text)
            h=hashlib.sha256("|".join(u for _,_,u in found).encode()).hexdigest()[:16]
            if not found:
                buckets.append({"category":cat,"year":y,"paged":pg,"status":200,"items":0,"note":"empty-terminator"}); break
            if h in seen:
                buckets.append({"category":cat,"year":y,"paged":pg,"status":200,"items":len(found),
                                "block_hash":h,"note":"REPEAT-BLOCK-terminator"}); break
            seen.add(h); npg+=1; nit+=len(found)
            buckets.append({"category":cat,"year":y,"paged":pg,"status":200,"items":len(found),"block_hash":h,"note":"ok"})
            for t,meta,href in found:
                title=html.unescape(re.sub(r'<[^>]+>','',t)).strip()
                m=re.search(r'/assets/uploads/(\d{4})/(\d{2})/',href)
                items[href]={"title":title,"meta_day_month":html.unescape(meta).strip(),"url":href,
                    "upload_year":m.group(1) if m else None,"upload_month":m.group(2) if m else None,
                    "category":cat,"year_param":y}
        if nit: print(f"  {cat:26s} {y}  pages={npg} items={nit} total={len(items)}",flush=True)
    save()
save()
print(f"\nSWEEP COMPLETE requests={nreq} distinct_files={len(items)} elapsed={int(time.time()-MINTED)}s",flush=True)
