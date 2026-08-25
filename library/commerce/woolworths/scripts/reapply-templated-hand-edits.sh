#!/usr/bin/env bash
# Re-apply the hand-edits that live inside GENERATED (templated) files.
#
# WHY THIS EXISTS
# ---------------
# `cli-printing-press generate --force` preserves whole hand-authored files
# (internal/client/woolworths_warm.go, internal/cli/woolworths_authcheck.go,
# internal/pricehist/, internal/unitprice/, the six novel command files). It does
# NOT reliably preserve edits made *inside* templated files: when the spec
# checksum changes, the fusion guard falls back to novel-only preservation and
# silently drops them.
#
# That happened on 2026-08-24 and regressed a ship blocker: the cold-start warm
# call vanished from client.go and `products search` went from 0.9s to a 50.7s
# HARD FAILURE on any profile with a cold cookie jar. It was only caught because
# live dogfood was re-run; nothing in build, vet or unit tests notices.
#
# Run this after EVERY `generate --force`, then rebuild and re-run live dogfood:
#
#   bash scripts/reapply-templated-hand-edits.sh
#   go build -o ./woolworths-pp-cli.exe ./cmd/woolworths-pp-cli
#
# Every edit is marked `pp:hand-edit` in the source, so `grep -rn pp:hand-edit`
# lists them. The proper fixes are upstream: the generator needs a
# post-construction client hook, should emit an Example on the `feedback` parent,
# and should let a spec declare an envelope-level error field. All filed as retro
# candidates.

set -euo pipefail
cd "$(dirname "$0")/.."

python - <<'PYEOF'
import io, re

TAB = "\t"


def read(p):
    return io.open(p, encoding="utf-8").read()


def write(p, t):
    io.open(p, "w", encoding="utf-8").write(t)


# --------------------------------------------------------------------------
# 1. client.go - warm the Akamai cookie jar at construction.
#    Without this the FIRST POST on a cold jar gets an HTTP/2 INTERNAL_ERROR and
#    a hanging HTTP/1.1 fallback (~50s, exit 5). See woolworths_warm.go.
# --------------------------------------------------------------------------
f = "internal/client/client.go"
t = read(f)
if "WarmSession(context.Background())" in t:
    print("client.go          : warm hook already present")
else:
    m = re.search(r"func New\(cfg \*config\.Config, timeout time\.Duration, rateLimit float64\) \*Client \{", t)
    if not m:
        raise SystemExit("client.go: New() not found - generator shape changed, fix by hand")
    seg = m.end()
    ret = re.search(r"\n(\t)return (\w+)\n\}", t[seg:])
    if not ret:
        raise SystemExit("client.go: return of New() not found - fix by hand")
    indent, var = ret.group(1), ret.group(2)
    ins = (
        "\n"
        + indent + "// pp:hand-edit  Akamai drops unwarmed POST connections (HTTP/2 INTERNAL_ERROR\n"
        + indent + "// then a hanging HTTP/1.1 fallback). One GET of /shop primes the jar; this is\n"
        + indent + "// a no-op once the jar has cookies for the host. See woolworths_warm.go.\n"
        + indent + var + ".WarmSession(context.Background())\n"
    )
    t = t[: seg + ret.start()] + ins + t[seg + ret.start():]
    if '"context"' not in t:
        t = t.replace("import (", 'import (\n\t"context"', 1)
    write(f, t)
    print("client.go          : warm hook RE-APPLIED")

# --------------------------------------------------------------------------
# 2. feedback.go - the generated parent ships no Example, which fails dogfood's
#    help check ("missing Examples section").
# --------------------------------------------------------------------------
f = "internal/cli/feedback.go"
t = read(f)
if "pp:hand-edit" in t:
    print("feedback.go        : Example already present")
else:
    old = (
        TAB * 2 + 'Use:   "feedback [text]",\n'
        + TAB * 2 + 'Short: "Record feedback about this CLI (local by default; upstream opt-in)",'
    )
    if old not in t:
        raise SystemExit("feedback.go: anchor not found - generator shape changed, fix by hand")
    quote, backslash = chr(34), chr(92)
    new = (
        old + "\n"
        + TAB * 2 + "// pp:hand-edit  The generated parent carried no Example, which fails\n"
        + TAB * 2 + '// dogfood\'s help check ("missing Examples section"). Generator gap.\n'
        + TAB * 2 + "Example: strings.Trim(`\n"
        + '  woolworths-pp-cli feedback "swap should rank by unit price by default"\n'
        + "  woolworths-pp-cli feedback list\n"
        + "`, " + quote + backslash + "n" + quote + "),"
    )
    t = t.replace(old, new, 1)
    if '"strings"' not in t:
        t = t.replace("import (", 'import (\n\t"strings"', 1)
    write(f, t)
    print("feedback.go        : Example RE-APPLIED")

# --------------------------------------------------------------------------
# 3. savedlists_{list,get}.go - soft-401 detection.
#    /api/v3/ui/* answers HTTP 200 with {"success":false,"statusCode":401} when
#    the session has expired, so the command exited 0 and looked like an
#    empty-but-successful result - which an agent cannot distinguish from "you
#    have no saved lists". Session cookies last about an hour, so this is the
#    NORMAL end state of every imported session.
#    The check MUST sit before every output branch: `savedlists list` returns
#    early via the human-table path, so a late insertion silently does nothing.
#    See internal/cli/woolworths_authcheck.go.
# --------------------------------------------------------------------------
early = TAB * 3 + "outputData := data\n"
ins = (
    TAB * 3 + "// pp:hand-edit  /api/v3/ui/* reports auth failure inside a HTTP 200\n"
    + TAB * 3 + "// envelope; without this the command exits 0 on an expired session.\n"
    + TAB * 3 + "// Must sit before every output branch. See woolworths_authcheck.go.\n"
    + TAB * 3 + "if authErr := checkWoolworthsSoftAuthFailure(data); authErr != nil {\n"
    + TAB * 4 + "return authErr\n"
    + TAB * 3 + "}\n"
)
for f in ("internal/cli/savedlists_list.go", "internal/cli/savedlists_get.go"):
    short = f.split("/")[-1]
    t = read(f)
    if "checkWoolworthsSoftAuthFailure" in t:
        print("%-18s : soft-401 check already present" % short)
        continue
    if early not in t:
        raise SystemExit("%s: anchor 'outputData := data' not found - generator shape changed, fix by hand" % f)
    write(f, t.replace(early, early + ins, 1))
    print("%-18s : soft-401 check RE-APPLIED" % short)
PYEOF

gofmt -w \
  internal/client/client.go \
  internal/cli/feedback.go \
  internal/cli/savedlists_list.go \
  internal/cli/savedlists_get.go

echo ""
echo "hand-edit markers now in tree:"
grep -rln 'pp:hand-edit' internal/ | sed 's/^/  /'
echo ""
echo "next: go build -o ./woolworths-pp-cli.exe ./cmd/woolworths-pp-cli && re-run live dogfood"
