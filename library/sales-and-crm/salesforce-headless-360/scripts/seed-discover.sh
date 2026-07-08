#!/usr/bin/env bash
# Discover required fields and restricted picklist values on the seed targets.
# Run this before seed-minimal.apex if your org has custom validation rules.
#
# Usage: ORG=<sf-alias> bash scripts/seed-discover.sh
#
# Output: a per-sobject table of createable non-nullable fields that have no
# default value, plus the picklist values for Account.Industry and
# Opportunity.StageName (the two picklists most commonly customized).

set -euo pipefail

ORG=${ORG:?ORG env var required (sf CLI alias)}

command -v sf >/dev/null || { echo "ERROR: sf CLI not installed"; exit 10; }
command -v jq >/dev/null || { echo "ERROR: jq not installed"; exit 10; }

sf org display --target-org "$ORG" --json >/dev/null 2>&1 || {
  echo "ERROR: sf alias $ORG not authenticated. Run: sf org login web --alias $ORG"
  exit 4
}

echo "Discovering required fields in org $ORG..."
echo ""

for sobject in Account Contact Opportunity Case Task Event; do
  echo "=== $sobject required + createable + not-defaulted fields ==="
  sf sobject describe --sobject "$sobject" --target-org "$ORG" --json 2>/dev/null \
    | jq -r '.result.fields[]
        | select(.nillable == false and .defaultedOnCreate == false and .createable == true)
        | "  \(.name) (\(.type)) — \(.label)"'
  echo ""
done

echo "=== Account.Industry allowed values ==="
sf sobject describe --sobject Account --target-org "$ORG" --json 2>/dev/null \
  | jq -r '.result.fields[] | select(.name == "Industry") | .picklistValues[] | select(.active == true) | "  \(.value)"'
echo ""

echo "=== Opportunity.StageName allowed values ==="
sf sobject describe --sobject Opportunity --target-org "$ORG" --json 2>/dev/null \
  | jq -r '.result.fields[] | select(.name == "StageName") | .picklistValues[] | select(.active == true) | "  \(.value)"'
echo ""

echo "Edit scripts/seed-minimal.apex if any required field above is not yet populated,"
echo "or if 'Technology/Software' / 'Prospecting' are not valid picklist values for your org."
echo ""
echo "Then run: sf apex run --file scripts/seed-minimal.apex --target-org $ORG"
