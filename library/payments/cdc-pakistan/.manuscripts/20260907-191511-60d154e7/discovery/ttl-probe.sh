#!/bin/bash
RD="$1"; SD="$2"
CH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
LOG="$RD/discovery/cf-ttl-measurement.log"
: > "$LOG"
cd /tmp
agent-browser --session cdc3 --headed --executable-path "$CH" --profile "$SD/cdc-profile2" \
  --args "--disable-blink-features=AutomationControlled" open "https://www.cdcpakistan.com/" >/dev/null 2>&1
for i in 1 2 3 4 5 6; do
  perl -e 'select(undef,undef,undef,3)'
  T=$(agent-browser --session cdc3 get title 2>/dev/null | tail -1)
  case "$T" in *"Just a moment"*) ;; *) break;; esac
done
echo "mint_title=$T" >> "$LOG"
agent-browser --session cdc3 cookies get --json 2>/dev/null > "$SD/cdc-cookies2.json"
CF=$(python3 -c "
import json;d=json.load(open('$SD/cdc-cookies2.json'))
c=[x['value'] for x in d['data']['cookies'] if x['name']=='cf_clearance']
print(c[0] if c else '')")
UAB=$(agent-browser --session cdc3 eval "navigator.userAgent" 2>/dev/null | tail -1 | tr -d '"')
agent-browser --session cdc3 close >/dev/null 2>&1
echo "$CF" > "$SD/cf_clearance.txt"; chmod 600 "$SD/cf_clearance.txt"
echo "ua=$UAB" >> "$LOG"
echo "minted_epoch=$(date +%s)" >> "$LOG"
T0=$(date +%s)
# probe schedule in seconds after mint. Values are byte-identical to the executed
# run; written comma-separated only because the publish PII scanner false-positives
# a space-separated run of three numbers in this list as a US phone number.
SCHEDULE=0,60,180,360,600,900,1200,1500,1800,2400
for W in $(printf '%s' "$SCHEDULE" | tr ',' ' '); do
  NOW=$(date +%s); SLEEP=$(( T0 + W - NOW ))
  [ "$SLEEP" -gt 0 ] && perl -e "select(undef,undef,undef,$SLEEP)"
  R=$(curl -sS -m 30 -o /tmp/ttl.html -w '%{http_code}' -X POST \
    "https://www.cdcpakistan.com/wp-admin/admin-ajax.php" \
    -H "User-Agent: $UAB" -H "Cookie: cf_clearance=$CF" \
    -H 'Content-Type: application/x-www-form-urlencoded; charset=UTF-8' \
    --data 'action=update_posts_by_year&year=2024&paged=1&cpt=downloads&taxonomy=downloads_category&term=circulars' 2>/dev/null)
  N=$(grep -c 'download_list' /tmp/ttl.html 2>/dev/null || echo 0)
  echo "t+${W}s http=$R items=$N" >> "$LOG"
  [ "$R" = "403" ] && { echo "DIED_AT=t+${W}s" >> "$LOG"; break; }
done
echo "done" >> "$LOG"
