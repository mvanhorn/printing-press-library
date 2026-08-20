#!/usr/bin/env python3
"""Enrich the RideWithGPS OpenAPI spec for printing-press generation.

Fixes discovered during Phase 2:
1. .json path bug: users/current and auth_tokens lack the required .json extension.
2. Auth: model the two required headers (x-rwgps-api-key + x-rwgps-auth-token) as
   two apiKey securitySchemes in ONE uniform AND requirement on every op, so the
   parser's sibling-header collection (which requires a consistent sibling set
   across all winner requirements) wires BOTH headers. createAuthToken does NOT
   get a single-key override (that inconsistency dropped the auth_token header).
3. Naming: .json path segments leak into resource names (routes.json -> routes-json,
   split from routes/{id}.json -> routes). x-pp-resource forces clean grouping.
4. Display name: x-display-name = "Ride with GPS" (slug derived to "Ride Gps").
"""
import sys, yaml

SRC, DST = sys.argv[1], sys.argv[2]
with open(SRC) as f:
    spec = yaml.safe_load(f)

AUTH_PARAM_REFS = {"#/components/parameters/api_key", "#/components/parameters/auth_token"}
HTTP_METHODS = {"get", "post", "put", "patch", "delete", "head", "options"}

# tag -> clean resource slug for x-pp-resource (Sync intentionally omitted:
# framework `sync` owns that surface; the raw getSyncInfo endpoint stays secondary)
TAG_RESOURCE = {
    "Routes": "routes",
    "Trips": "trips",
    "Events": "events",
    "Collections": "collections",
    "Points of Interest": "points_of_interest",
    "Club Members": "members",
    "Users": "users",
    "Authentication Tokens": "auth_tokens",
}

# --- 1. Fix .json path bug (rename path keys) ---
renames = {
    "/api/v1/users/current": "/api/v1/users/current.json",
    "/api/v1/auth_tokens": "/api/v1/auth_tokens.json",
}
spec["paths"] = {renames.get(p, p): item for p, item in spec.get("paths", {}).items()}
paths = spec["paths"]

# --- 2/3. Walk operations: strip auth param refs, drop per-op security, set x-pp-resource ---
stripped_params = stripped_security = tagged = 0
for path, item in paths.items():
    for method, op in list(item.items()):
        if method.lower() not in HTTP_METHODS or not isinstance(op, dict):
            continue
        # strip auth param refs
        if isinstance(op.get("parameters"), list):
            before = len(op["parameters"])
            op["parameters"] = [
                prm for prm in op["parameters"]
                if not (isinstance(prm, dict) and prm.get("$ref") in AUTH_PARAM_REFS)
            ]
            stripped_params += before - len(op["parameters"])
            if not op["parameters"]:
                del op["parameters"]
        # drop ALL per-op security (incl. dangling basic_auth AND any single-key
        # override) so every op inherits the uniform global AND requirement
        if "security" in op:
            del op["security"]
            stripped_security += 1
        # x-pp-resource from tag
        for t in op.get("tags", []):
            if t in TAG_RESOURCE:
                op["x-pp-resource"] = TAG_RESOURCE[t]
                tagged += 1
                break

# --- 4. securitySchemes + uniform global security (one AND object) ---
comps = spec.setdefault("components", {})
schemes = comps.setdefault("securitySchemes", {})
schemes["rwgpsApiKey"] = {
    "type": "apiKey", "in": "header", "name": "x-rwgps-api-key",
    "description": "Ride with GPS client API key (create an API client under your account's developers tab).",
    "x-auth-env-vars": ["RIDEWITHGPS_API_KEY"],
}
schemes["rwgpsAuthToken"] = {
    "type": "apiKey", "in": "header", "name": "x-rwgps-auth-token",
    "description": "Per-user auth token (run `auth login`, or POST /api/v1/auth_tokens.json with email+password).",
    "x-auth-env-vars": ["RIDEWITHGPS_AUTH_TOKEN"],
}
spec["security"] = [{"rwgpsApiKey": [], "rwgpsAuthToken": []}]  # both = AND

# --- info display name ---
spec.setdefault("info", {})["x-display-name"] = "Ride with GPS"

# --- drop now-unused auth parameters ---
for k in ("api_key", "auth_token"):
    comps.get("parameters", {}).pop(k, None)

with open(DST, "w") as f:
    yaml.safe_dump(spec, f, sort_keys=False, default_flow_style=False, allow_unicode=True, width=120)

ops = sum(1 for it in paths.values() for m in it if m.lower() in HTTP_METHODS)
print(f"paths: {len(paths)} | operations: {ops}")
print(f"stripped auth param refs: {stripped_params}")
print(f"stripped per-op security blocks: {stripped_security}")
print(f"x-pp-resource tagged operations: {tagged}")
print("uniform global security: rwgpsApiKey + rwgpsAuthToken (AND) on every op")
print("x-display-name: Ride with GPS")
