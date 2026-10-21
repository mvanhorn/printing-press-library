#!/usr/bin/env python3
"""Re-apply pinterest-pp-cli hand customizations to templated files after a regen.

Usage: reapply_customizations.py <cli-dir> <baseline-commit> <custom-commit>

- Spec-dependent templated files (sync.go, root.go, helpers.go) get idempotent
  text edits.
- Spec-independent templated files (auth.go, client.go, oauth_token.go) are
  restored from <custom-commit> only when the regenerated file is byte-identical
  to <baseline-commit> (pristine generator output); otherwise this exits 1 so a
  human reconciles the template drift.
"""
import subprocess
import sys

cli, baseline, custom = sys.argv[1:4]


def git(*args):
    return subprocess.run(["git", "-C", cli, *args], check=True, capture_output=True, text=True).stdout


def edit(path, old, new, marker):
    p = f"{cli}/{path}"
    s = open(p).read()
    if marker in s:
        return "kept"
    if s.count(old) != 1:
        sys.exit(f"{path}: anchor not found exactly once: {old[:60]!r}")
    open(p, "w").write(s.replace(old, new))
    return "applied"


results = {}
results["sync.go defaults"] = edit(
    "internal/cli/sync.go",
    '''func defaultSyncResources() []string {
	return []string{
		"resources",
		"resources-delivery-metrics",
	}
}''',
    '''func defaultSyncResources() []string {
	return []string{
		"boards",
		"pins",
	}
}''',
    '"boards",\n\t\t"pins",\n\t}\n}',
)
results["sync.go parentFilter"] = edit(
    "internal/cli/sync.go",
    '''			// If no specific resources, sync top-level resources
			if len(resources) == 0 {
				resources = defaultSyncResources()
			}''',
    '''			// Keep the default sync bounded: dependents follow only the default
			// parents, not the whole API surface. Use --resources for more.
			if len(resources) == 0 {
				resources = defaultSyncResources()
				parentFilter = append(parentFilter, resources...)
			}''',
    "parentFilter = append(parentFilter, resources...)",
)
results["sync.go pagination rename"] = edit(
    "internal/cli/sync.go",
    "func determinePaginationDefaults(resource string) paginationDefaults {",
    "func determineGeneratedPaginationDefaults(resource string) paginationDefaults {",
    "func determineGeneratedPaginationDefaults(",
)
results["root.go ads AddCommand"] = edit(
    "internal/cli/root.go",
    "\taddNovelCommandIfAbsent(rootCmd, newNovelAdsCmd(flags))",
    "\t// Direct AddCommand so dogfood's static collector sees the top-level novel ads group (it ignores addNovelCommandIfAbsent and would match the leaf to ad-accounts ads).\n\trootCmd.AddCommand(newNovelAdsCmd(flags))",
    "rootCmd.AddCommand(newNovelAdsCmd(flags))",
)
results["ads.go AddCommand"] = edit(
    "internal/cli/ads.go",
    "\taddNovelCommandIfAbsent(cmd, newNovelAdsReportCmd(flags))",
    "\tcmd.AddCommand(newNovelAdsReportCmd(flags))",
    "cmd.AddCommand(newNovelAdsReportCmd(flags))",
)
results["helpers.go bookmark --all"] = edit(
    "internal/cli/helpers.go",
    '''	// Generated commands run retainCLIQueryParams first so unset "0"/"false"
	// never reach this loop.''',
    '''	cursorParam, paginationType, nextCursorPath = pinterestBookmarkCursor(cursorParam, paginationType, limitParam, nextCursorPath)
	// Generated commands run retainCLIQueryParams first so unset "0"/"false"
	// never reach this loop.''',
    "pinterestBookmarkCursor(cursorParam",
)

# config_perms_test.go: the generator's synthetic STORED-* token fixture trips the
# publish opaque-credential scan (no suppression path); use an explicit placeholder.
import re as _re
tp = f"{cli}/internal/config/config_perms_test.go"
ts = open(tp).read()
for m in _re.finditer(r'cliutil\.SaveCredentials\(&cliutil\.Credentials\{AccessToken: "(STORED[^"]*)"', ts):
    if len(m.group(1)) >= 32 and m.group(1) != "STORED-PLACEHOLDER-ACCESS-DO-NOT-POST":
        ts = ts.replace(m.group(1), "STORED-PLACEHOLDER-ACCESS-DO-NOT-POST")
        results["config_perms_test.go placeholder"] = "applied"
open(tp, "w").write(ts)

# helpers.go: drop the dead printStampedDryRunOutput if the template re-emitted it.
hp = f"{cli}/internal/cli/helpers.go"
s = open(hp).read()
start = s.find("func printStampedDryRunOutput(")
if start >= 0 and s.count("printStampedDryRunOutput(") == 1:
    doc = s.rfind("\n\n", 0, start) + 1
    depth, i = 0, s.index("{", start)
    while True:
        depth += {"{": 1, "}": -1}.get(s[i], 0)
        if depth == 0:
            break
        i += 1
    open(hp, "w").write(s[:doc] + s[i + 1:].lstrip("\n"))
    subprocess.run(["gofmt", "-w", hp], check=True)
    results["helpers.go dead helper"] = "removed"

def body(text):
    # The copyright header line follows the manifest creator; compare the rest.
    lines = text.split("\n", 1)
    return lines[1] if lines and lines[0].startswith("// Copyright") and len(lines) > 1 else text


for path in ["internal/cli/auth.go", "internal/client/client.go", "internal/cliutil/oauth_token.go"]:
    current = open(f"{cli}/{path}").read()
    pristine = git("show", f"{baseline}:{path}")
    customized = git("show", f"{custom}:{path}")
    header = current.split("\n", 1)[0]
    if body(current) == body(customized):
        results[path] = "kept"
    elif body(current) == body(pristine):
        open(f"{cli}/{path}", "w").write(header + "\n" + body(customized))
        results[path] = "restored"
    else:
        sys.exit(f"{path}: regenerated output drifted from the pristine baseline; reconcile by hand")

for k, v in results.items():
    print(f"{v:9} {k}")
