import os,re,sys,json,time,html,hashlib
from curl_cffi import requests
CF=open(os.environ["SDF"]).read().strip()
RD=os.environ["RD"]
UA='Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36'
AJ="https://www.cdcpakistan.com/wp-admin/admin-ajax.php"
s=requests.Session(impersonate="chrome124")
s.headers.update({"User-Agent":UA,"Content-Type":"application/x-www-form-urlencoded; charset=UTF-8"})
s.cookies.set("cf_clearance",CF,domain=".cdcpakistan.com")
CATS=["miscellaneous","list-of-securities","publications","circulars","notices",
      "annual-reports","quarterly-accounts","disciplinary-registers","guidelines",
      "procedures","newsletter","forms","designated-time-schedule",
      "tariff-fee-structure","sustainability-report"]
YEARS=list(range(2026,2006,-1))
ITEM=re.compile(r'(?is)<div class="download_list">.*?<h4>(.*?)</h4>.*?<div class="meta">(.*?)</div>.*?href="([^"]+)"')
delay=1.0; ok=0
items={}; buckets=[]
def post(term,year,paged):
    global delay, ok
    for attempt in range(4):
        try:
            r=s.post(AJ,data={"action":"update_posts_by_year","year":str(year),"paged":str(paged),
                "cpt":"downloads","taxonomy":"downloads_category","term":term},timeout=45)
        except Exception as e:
            time.sleep(5*(attempt+1)); continue
        if r.status_code==429:
            delay=min(delay*2,20); print(f"    429 -> delay {delay}s",flush=True); time.sleep(30); continue
        if r.status_code!=200:
            return None,r.status_code
        ok+=1
        if ok%5==0: delay=max(delay*0.8,0.3)
        return r.text,200
    return None,-1
for cat in CATS:
    for y in YEARS:
        seen_hashes=set(); n_pages=0; n_items=0
        for pg in range(1,40):
            body,code=post(cat,y,pg)
            time.sleep(delay)
            if body is None:
                buckets.append({"category":cat,"year":y,"paged":pg,"status":code,"items":0,"note":"fetch-failed"})
                break
            found=ITEM.findall(body)
            h=hashlib.sha256("|".join(u for _,_,u in found).encode()).hexdigest()[:16]
            if not found:
                buckets.append({"category":cat,"year":y,"paged":pg,"status":200,"items":0,"note":"empty-terminator"})
                break
            if h in seen_hashes:
                buckets.append({"category":cat,"year":y,"paged":pg,"status":200,"items":len(found),
                                "block_hash":h,"note":"REPEAT-BLOCK-terminator"})
                break
            seen_hashes.add(h); n_pages+=1; n_items+=len(found)
            buckets.append({"category":cat,"year":y,"paged":pg,"status":200,"items":len(found),"block_hash":h,"note":"ok"})
            for t,meta,href in found:
                title=html.unescape(re.sub(r'<[^>]+>','',t)).strip()
                m=re.search(r'/assets/uploads/(\d{4})/(\d{2})/',href)
                items[href]={"title":title,"meta_day_month":html.unescape(meta).strip(),
                    "url":href,"upload_year":m.group(1) if m else None,
                    "upload_month":m.group(2) if m else None,
                    "seen_in_category":cat,"seen_under_year_param":y}
        print(f"  {cat:26s} {y}  pages={n_pages} items={n_items} distinct_total={len(items)}",flush=True)
json.dump({"buckets":buckets,"items":list(items.values())},open(f"{RD}/discovery/corpus-sweep.json","w"),indent=1)
print(f"\nDONE distinct_files={len(items)} buckets={len(buckets)} effective_delay={delay:.2f}s",flush=True)
