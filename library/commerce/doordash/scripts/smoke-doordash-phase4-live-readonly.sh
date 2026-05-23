#!/usr/bin/env bash
set -euo pipefail

# Live read-only DoorDash smoke test.
# Requires an imported DoorDash session at ~/.doordash-mcp/session.json.
# Does not mutate cart, checkout, address, payment, or order state.

cd /home/hermes/projects/doordash-pp-cli/active-wrapper
# PATCH: Paperclip/Hermes runtime may have Node without npm; use checked-in dist when npm is unavailable.
if command -v npm >/dev/null 2>&1; then
  npm run build >/tmp/doordash-phase4-build.log
else
  printf 'npm unavailable; using existing dist/ wrapper after node --check validation\n' >/tmp/doordash-phase4-build.log
fi

export PATH="/home/hermes/.local/bin:/home/hermes/go/bin:$PATH"

python3 - <<'PY'
import json, subprocess, sys
cmds = [
  ('doctor', ['doordash-pp-cli','doctor','--json']),
  ('search_pizza', ['doordash-pp-cli','search','pizza','--json']),
  ('menu_pizza_hut', ['doordash-pp-cli','menu','--store-id','2418408','--json']),
  ('item_options_personal_pan', ['doordash-pp-cli','item-options','--store-id','2418408','--item-id','35292395509','--json']),
  ('convenience_search_cvs_milk', ['doordash-pp-cli','convenience-search','--store-id','23321373','milk','--json']),
  ('orders_recent', ['doordash-pp-cli','orders','recent','--limit','3','--json']),
  ('addresses_list', ['doordash-pp-cli','addresses','list','--json']),
  ('payment_methods_list', ['doordash-pp-cli','payment-methods','list','--json']),
  ('cart_show', ['doordash-pp-cli','cart','show','--json']),
]
summary={}
for name, cmd in cmds:
    p=subprocess.run(['timeout','35s',*cmd], text=True, capture_output=True)
    rec={'ok':p.returncode==0,'exit':p.returncode,'stderr':p.stderr[:200]}
    if p.stdout.strip():
        data=json.loads(p.stdout)
        if name=='doctor': rec.update({'authenticated':data.get('authenticated'), 'csrf_present':data.get('csrf_present')})
        elif name.startswith('search'): rec.update({'count':len(data), 'first_name':data[0].get('name') if data else None})
        elif name.startswith('menu'):
            cats=data.get('categories') or []
            rec.update({'store':data.get('name'), 'category_count':len(cats), 'item_count':sum(len(c.get('items') or []) for c in cats)})
        elif name.startswith('item_options'):
            rec.update({'item':data.get('name'), 'option_group_count':len(data.get('optionGroups') or [])})
        elif name.startswith('convenience'): rec.update({'item_count':len(data), 'first_name':data[0].get('name') if data else None})
        elif name in ('orders_recent','addresses_list','payment_methods_list','cart_show'):
            rec.update({'count':len(data) if isinstance(data,list) else None})
    summary[name]=rec
print(json.dumps(summary, indent=2))
if not all(v.get('ok') for v in summary.values()):
    sys.exit(1)
PY
