import json, os
DISCOVERY_DIR = r'C:\Users\bazil\printing-press\.runstate\zazu-331861db\runs\20260523-115018\discovery'

all_probes = {}
for fn in ['probe-results-1.json', 'probe-results-2.json', 'probe-results-3.json']:
    p = os.path.join(DISCOVERY_DIR, fn)
    if os.path.exists(p):
        with open(p, encoding='utf-8') as f:
            for k, v in json.load(f).items():
                all_probes[k] = v

sample_lead = None
sample_path = os.path.join(DISCOVERY_DIR, 'sample-lead-full.json')
if os.path.exists(sample_path):
    with open(sample_path, encoding='utf-8') as f:
        sample_lead = json.load(f)

capture = {
    "schema_version": 1,
    "captured_at": "2026-05-23T11:30:00Z",
    "site": "https://www.buildalert.uk",
    "auth_profile": "cookie",
    "session_cookies_required": True,
    "endpoints": [],
    "sample_responses": {},
}

ENDPOINTS = [
    {"method": "GET", "path": "/dapi/user/user",
     "description": "Current user profile + filter preferences (postCode, radius, location, subscription).",
     "params": [], "response_status": 200, "auth_required": True},
    {"method": "GET", "path": "/dapi/user/details",
     "description": "Alias of /dapi/user/user with same payload.",
     "params": [], "response_status": 200, "auth_required": True},
    {"method": "GET", "path": "/dapi/user/dashboard",
     "description": "Dashboard overview: newLeadsCount, credits, totalPlanningApplications, userLeads.",
     "params": [], "response_status": 200, "auth_required": True},
    {"method": "GET", "path": "/dapi/leads/live-leads",
     "description": "Paginated planning-application leads matched to user filters. Embeds quickFilters.",
     "params": [
         {"name": "states", "type": "string", "in": "query", "required": False, "default": "-1",
          "description": "Lead state filter. -1=All (only observed value with effect)."},
         {"name": "page", "type": "integer", "in": "query", "required": False, "default": 1},
         {"name": "itemsPerPage", "type": "integer", "in": "query", "required": False, "default": 50,
          "description": "Page size; 50 in default dashboard usage."},
         {"name": "orderBy", "type": "string", "in": "query", "required": False, "default": "createdDate",
          "description": "Sort field. Observed: createdDate, value."},
         {"name": "force", "type": "string", "in": "query", "required": False, "default": "",
          "description": "Cache-bust marker. Passed empty in the dashboard."},
         {"name": "minValue", "type": "integer", "in": "query", "required": False,
          "description": "Minimum project value (GBP)."},
         {"name": "projectTypes", "type": "string", "in": "query", "required": False,
          "description": "Comma-separated project type IDs. Valid: Extension, Loft_Conversion, Garage_Conversion, Outbuilding, Porch."}
     ],
     "response_status": 200, "auth_required": True},
    {"method": "GET", "path": "/dapi/letter-templates",
     "description": "User's letter templates plus baseLogoUrl.",
     "params": [], "response_status": 200, "auth_required": True},
    {"method": "GET", "path": "/dapi/transactions",
     "description": "User's letter-send transactions in a date window.",
     "params": [
         {"name": "page", "type": "integer", "in": "query", "required": False, "default": 1},
         {"name": "itemsPerPage", "type": "integer", "in": "query", "required": False, "default": 100},
         {"name": "dateFrom", "type": "integer", "in": "query", "required": True,
          "description": "Unix seconds (inclusive)."},
         {"name": "dateTo", "type": "integer", "in": "query", "required": True,
          "description": "Unix seconds (inclusive)."}
     ],
     "response_status": 200, "auth_required": True},
    {"method": "GET", "path": "/dapi/tracking",
     "description": "ROI tracking: letters sent, replies, conversion rate, work won, total return, chart data.",
     "params": [
         {"name": "dateFrom", "type": "integer", "in": "query", "required": True},
         {"name": "dateTo", "type": "integer", "in": "query", "required": True},
         {"name": "page", "type": "integer", "in": "query", "required": False, "default": 1},
         {"name": "itemsPerPage", "type": "integer", "in": "query", "required": False, "default": 50}
     ],
     "response_status": 200, "auth_required": True},
    {"method": "GET", "path": "/dapi/healthcheck/keep-warm",
     "description": "Lightweight liveness probe. Returns {success: true}.",
     "params": [], "response_status": 200, "auth_required": False},
    {"method": "GET", "path": "/dapi/reviews/should-show-modal",
     "description": "Trigger for in-app review prompt. Returns {showModal: bool}. Low CLI value.",
     "params": [], "response_status": 200, "auth_required": True},
]

capture["endpoints"] = ENDPOINTS

if sample_lead and 'data' in sample_lead and sample_lead['data']:
    capture["sample_responses"]["GET /dapi/leads/live-leads"] = {
        "status": 200,
        "content_type": "application/json",
        "body": sample_lead
    }

for url, p in all_probes.items():
    if p.get('status') == 200 and p.get('body'):
        try:
            body_json = json.loads(p['body'])
            capture["sample_responses"].setdefault(url, {
                "status": 200,
                "content_type": p.get('content_type', ''),
                "body": body_json
            })
        except Exception:
            pass

out_capture = os.path.join(DISCOVERY_DIR, 'browser-sniff-capture.json')
with open(out_capture, 'w', encoding='utf-8') as f:
    json.dump(capture, f, indent=2, ensure_ascii=False)
print(f"Wrote browser-sniff-capture.json ({os.path.getsize(out_capture)} bytes)")
print(f"Endpoints: {len(ENDPOINTS)}")
print(f"Sample responses: {len(capture['sample_responses'])}")

traffic = {
    "schema_version": 1,
    "site": "https://www.buildalert.uk",
    "reachability": {
        "mode": "standard_http",
        "confidence": 0.95,
        "rationale": "Plain stdlib HTTP returned a non-error response; no special transport needed."
    },
    "auth": {
        "type": "cookie",
        "cookie_domain": "www.buildalert.uk",
        "session_required": True,
        "login_url": "https://www.buildalert.uk/auth/login"
    },
    "client_pattern": "rest",
    "base_url": "https://www.buildalert.uk",
    "endpoint_clusters": [
        {"name": "leads", "paths": ["/dapi/leads/live-leads"], "observed_auth": ["cookie"]},
        {"name": "user", "paths": ["/dapi/user/user", "/dapi/user/details", "/dapi/user/dashboard"], "observed_auth": ["cookie"]},
        {"name": "letters", "paths": ["/dapi/letter-templates"], "observed_auth": ["cookie"]},
        {"name": "transactions", "paths": ["/dapi/transactions"], "observed_auth": ["cookie"]},
        {"name": "tracking", "paths": ["/dapi/tracking"], "observed_auth": ["cookie"]},
        {"name": "health", "paths": ["/dapi/healthcheck/keep-warm"], "observed_auth": []}
    ]
}
out_ta = os.path.join(DISCOVERY_DIR, 'traffic-analysis.json')
with open(out_ta, 'w', encoding='utf-8') as f:
    json.dump(traffic, f, indent=2)
print(f"Wrote traffic-analysis.json")
