#!/usr/bin/env bash
# Orchestrate seed discovery and execution, then print env exports for live-verify.
#
# Usage: ORG=<sf-alias> bash scripts/seed-run.sh
#
# This script:
#   1. Runs seed-discover.sh to show required fields and picklist values.
#   2. Pauses for you to edit scripts/seed-minimal.apex if customization is needed.
#   3. Runs seed-minimal.apex.
#   4. Queries the seeded Account + Opportunity IDs.
#   5. Prints ready-to-paste export lines for ORG, ACME_ID, OPP_ID.

set -euo pipefail

ORG=${ORG:?ORG env var required (sf CLI alias)}
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
cd "$SCRIPT_DIR/.."

bash "$SCRIPT_DIR/seed-discover.sh"

echo ""
echo "-------------------------------------------------------------------"
echo "Edit scripts/seed-minimal.apex if the defaults don't match your org."
echo "Hit Enter when ready to seed (or Ctrl-C to abort)."
read -r

echo ""
echo "Running seed-minimal.apex against $ORG..."
sf apex run --file "$SCRIPT_DIR/seed-minimal.apex" --target-org "$ORG" 2>&1 \
  | tee /tmp/sf360-seed-output.log

# The Apex debug log is on stderr; grep the saved run for our marker lines
ACME_ID=$(grep -oE '>>> ACME_ID=[0-9A-Za-z]+' /tmp/sf360-seed-output.log | head -1 | cut -d= -f2 || true)
OPP_ID=$(grep -oE '>>> OPP_ID=[0-9A-Za-z]+' /tmp/sf360-seed-output.log | head -1 | cut -d= -f2 || true)

# Fallback: query by Account Name if debug log grep missed
if [ -z "$ACME_ID" ]; then
  ACME_ID=$(sf data query --target-org "$ORG" --query \
    "SELECT Id FROM Account WHERE Name = 'Acme Corp SF360 Test' ORDER BY CreatedDate DESC LIMIT 1" \
    --json 2>/dev/null | jq -r '.result.records[0].Id // empty')
fi
if [ -z "$OPP_ID" ] && [ -n "$ACME_ID" ]; then
  OPP_ID=$(sf data query --target-org "$ORG" --query \
    "SELECT Id FROM Opportunity WHERE AccountId = '$ACME_ID' ORDER BY CreatedDate ASC LIMIT 1" \
    --json 2>/dev/null | jq -r '.result.records[0].Id // empty')
fi

if [ -z "$ACME_ID" ] || [ -z "$OPP_ID" ]; then
  echo "ERROR: seed did not produce usable ACME_ID or OPP_ID. Inspect /tmp/sf360-seed-output.log."
  exit 5
fi

# Pick a restricted-profile user for FLS checks (any non-admin active user)
RESTRICTED_USER=$(sf data query --target-org "$ORG" --query \
  "SELECT Id FROM User WHERE IsActive = true AND Profile.Name != 'System Administrator' LIMIT 1" \
  --json 2>/dev/null | jq -r '.result.records[0].Id // empty')

cat <<EOF

-------------------------------------------------------------------
Seed complete. Paste these into your shell:

export ORG=$ORG
export ACME_ID=$ACME_ID
export OPP_ID=$OPP_ID
export OPP_STAGE='<a forward stage in your Opp picklist, e.g. Qualification>'
export RESTRICTED_USER=$RESTRICTED_USER
export RESTRICTED_WRITE_USER=\$RESTRICTED_USER

Then: bash scripts/live-verify.sh
-------------------------------------------------------------------
EOF
