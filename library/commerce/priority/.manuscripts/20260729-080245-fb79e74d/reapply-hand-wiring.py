#!/usr/bin/env python3
"""Re-applies hand wiring and test patches lost on `generate --force` regens."""
W = "~/printing-press/.runstate/priority_cli-2a8cd69b/runs/20260729-080245-fb79e74d/working/priority-pp-cli"

def patch(path, old, new, required=True):
    s = open(path).read()
    if new in s:
        return
    if old not in s:
        if required:
            raise SystemExit(f"ANCHOR MISSING in {path}: {old[:60]!r}")
        return
    open(path, "w").write(s.replace(old, new, 1))

# root.go: hand top-level commands
patch(W + "/internal/cli/root.go",
      "\trootCmd.AddCommand(newNovelShortageCmd(flags))",
      """\trootCmd.AddCommand(newNovelShortageCmd(flags))
\trootCmd.AddCommand(newQueryRawCmd(flags))
\trootCmd.AddCommand(newFilesCmd(flags))
\trootCmd.AddCommand(newTextCmd(flags))
\trootCmd.AddCommand(newAgingCmd(flags))
\trootCmd.AddCommand(newDebtorsCmd(flags))
\trootCmd.AddCommand(newStockCmd(flags))""")

# entity.go: write surface
patch(W + "/internal/cli/entity.go",
      "\tcmd.AddCommand(newEntitySubformCmd(flags))",
      """\tcmd.AddCommand(newEntitySubformCmd(flags))
\tcmd.AddCommand(newEntityCreateCmd(flags))
\tcmd.AddCommand(newEntityUpdateCmd(flags))
\tcmd.AddCommand(newEntitySubformAddCmd(flags))
\tcmd.AddCommand(newEntitySubformUpdateCmd(flags))
\tcmd.AddCommand(newEntitySubformDeleteCmd(flags))""")

# promoted_batch.go: batch load child
patch(W + "/internal/cli/promoted_batch.go",
      "\tcmd.AddCommand(newNovelBatchResumeCmd(flags))",
      "\tcmd.AddCommand(newNovelBatchResumeCmd(flags))\n\tcmd.AddCommand(newNovelBatchLoadCmd(flags))")

# forms.go wiring (usually preserved as implemented scaffold; re-add if regenerated)
patch(W + "/internal/cli/forms.go",
      "\tcmd.AddCommand(newNovelFormsDiffCmd(flags))",
      """\tcmd.AddCommand(newNovelFormsDiffCmd(flags))
\tcmd.AddCommand(newFormsListCmd(flags))
\tcmd.AddCommand(newFormsDescribeCmd(flags))
\tcmd.AddCommand(newFormsRefreshCmd(flags))""", required=False)

# credentials_test.go: two-var Basic pair support (generator test template
# assumes single-token auth; retro candidate filed in build log)
p = W + "/internal/cliutil/credentials_test.go"
s = open(p).read()
if "authHeaderContains" not in s:
    s = s.replace("\nfunc setCredentialValue", '''
// authHeaderContains reports whether the AuthHeader carries the token,
// decoding the base64 payload of Basic headers (two-var Basic pair spec).
func authHeaderContains(header, token string) bool {
	if strings.Contains(header, token) {
		return true
	}
	if rest, ok := strings.CutPrefix(header, "Basic "); ok {
		if decoded, err := base64.StdEncoding.DecodeString(rest); err == nil {
			return strings.Contains(string(decoded), token)
		}
	}
	return false
}

func setCredentialValue''', 1)
    s = s.replace('creds.PriorityApiUsername = token\n}',
                  'creds.PriorityApiUsername = token\n\tcreds.PriorityApiPassword = token + "-pw"\n}')
    s = s.replace('!strings.Contains(got, "data-secret") || strings.Contains(got, "legacy-secret")',
                  '!authHeaderContains(got, "data-secret") || authHeaderContains(got, "legacy-secret")')
    s = s.replace('!strings.Contains(got, "legacy-secret")', '!authHeaderContains(got, "legacy-secret")')
    s = s.replace('!strings.Contains(got, "env-secret")', '!authHeaderContains(got, "env-secret")')
    s = s.replace('\tt.Setenv("PRIORITY_API_USERNAME", "env-secret")',
                  '\tt.Setenv("PRIORITY_API_USERNAME", "env-secret")\n\tt.Setenv("PRIORITY_API_PASSWORD", "env-secret-pw")')
    idx = s.index('legacyCredentialKey() + " = \\"" + token + "\\"\\n")')
    s = s.replace('legacyCredentialKey() + " = \\"" + token + "\\"\\n")',
                  'legacyCredentialKey() + " = \\"" + token + "\\"\\napi_password = \\"" + token + "-pw\\"\\n")')
    if '"encoding/base64"' not in s:
        s = s.replace('import (', 'import (\n\t"encoding/base64"', 1)
    open(p, "w").write(s)

# batch_resume.go: generated surface has no 'batch run'; the raw command is 'batch'
p = W + "/internal/cli/batch_resume.go"
s = open(p).read()
s = s.replace("use 'batch load' (journaled) or 'batch run' (raw) instead", "use 'batch load' (journaled) or 'batch' (raw --requests) instead")
open(p, "w").write(s)
print("hand wiring re-applied")

# store.go: uppercase field-name probe in LookupFieldValue (Priority uses
# ALL-UPPERCASE OData field names; generator only probes snake/camel/Pascal —
# retro candidate).
p = W + "/internal/store/store.go"
s = open(p).read()
if "probe the uppercase form" not in s:
    anchor = "\tif parts[0] != \"\" {\n\t\tpascal := strings.ToUpper(parts[0][:1]) + parts[0][1:] + strings.Join(parts[1:], \"\")\n\t\tif v, ok := obj[pascal]; ok {\n\t\t\treturn sqliteFieldValue(v)\n\t\t}\n\t}\n\treturn nil\n}"
    replacement = anchor.replace("\treturn nil\n}", "\t// Priority (and other OData ERPs) use ALL-UPPERCASE field names (IVNUM,\n\t// CUSTNAME); probe the uppercase form of the column key last.\n\tif v, ok := obj[strings.ToUpper(snakeKey)]; ok {\n\t\treturn sqliteFieldValue(v)\n\t}\n\treturn nil\n}")
    if anchor not in s:
        raise SystemExit("store.go LookupFieldValue anchor missing")
    open(p, "w").write(s.replace(anchor, replacement, 1))
print("store.go uppercase probe ok")

# README: Claude Desktop MCP env block must include BOTH Basic-pair vars
# (template emits only the first env var — retro candidate), and exit-code
# tables should include 6 (partial failure).
p = W + "/README.md"
s = open(p).read()
if '"PRIORITY_API_USERNAME"' in s and '"PRIORITY_API_PASSWORD"' not in s:
    s = s.replace('"PRIORITY_API_USERNAME": "your-key-here"',
                  '"PRIORITY_API_USERNAME": "your-username",\n        "PRIORITY_API_PASSWORD": "your-password"')
    s = s.replace("Fill in `PRIORITY_API_USERNAME` when Claude Desktop prompts you",
                  "Fill in `PRIORITY_API_USERNAME` and `PRIORITY_API_PASSWORD` when Claude Desktop prompts you")
for doc in ("/README.md", "/SKILL.md"):
    q = W + doc
    t = open(q).read()
    if "| 5 |" in t and "| 6 |" not in t and "Partial failure" not in t:
        t = t.replace("| 7 |", "| 6 | Partial failure — some batch/aggregate operations failed |\n| 7 |", 1)
        open(q, "w").write(t)
open(p, "w").write(s)
print("README/SKILL doc patches ok")

# meta metadata: resolveRead drops the binary header on the store-strategy
# path, misclassifying EDMX XML as an HTML auth error (retro candidate).
# Fetch directly and answer --json with an honest envelope.
p = W + "/internal/cli/meta_metadata.go"
s = open(p).read()
anchor = 'data, prov, err := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "auto", "meta", false, path, params, headerOverrides, "", cmd.ErrOrStderr())'
if anchor in s:
    s = s.replace(anchor, '''prov := DataProvenance{Source: "live"}
			_ = params
			data, err := c.GetWithHeadersNoCache(cmd.Context(), path, nil, headerOverrides)''', 1)
anchor2 = '''			if flags.asJSON || flags.csv || flags.compact || flags.plain || flags.selectFields != "" {
				return fmt.Errorf("binary response cannot be rendered as structured output; redirect stdout or use --deliver file:<path>")
			}'''
if anchor2 in s:
    s = s.replace(anchor2, '''			if flags.asJSON || flags.csv || flags.compact || flags.plain || flags.selectFields != "" {
				envelope, mErr := json.Marshal(map[string]any{
					"content_type": "application/xml",
					"bytes":        len(data),
					"note":         "EDMX is binary XML; redirect stdout without --json for the raw document, or use 'forms describe <FORM>' for structured schema",
				})
				if mErr != nil {
					return mErr
				}
				_, err = cmd.OutOrStdout().Write(append(envelope, '\\n'))
				return err
			}''', 1)
if "fmt." not in s.replace('_ = fmt.Sprintf', ''):
    import re as _re
    if not _re.search(r'\bfmt\.', s):
        s = s.replace('\t"fmt"\n', '')
open(p, "w").write(s)
print("meta_metadata patch ok")

# workflow archive: curtail under live-dogfood (30s budget; retro candidate —
# generated workflow commands do not honor PRINTING_PRESS_DOGFOOD).
p = W + "/internal/cli/channel_workflow.go"
s = open(p).read()
if "cliutil.IsDogfoodEnv()" not in s:
    anchor = '''			resources := []string{"customers", "invoices", "orders", "parts", "porders", "suppliers", "warehouses"}
			totalSynced := 0'''
    assert anchor in s, "channel_workflow anchor missing"
    s = s.replace(anchor, '''			resources := []string{"customers", "invoices", "orders", "parts", "porders", "suppliers", "warehouses"}
			maxPages := 100
			if cliutil.IsDogfoodEnv() {
				// Live-dogfood budget is 30s per command; one page per resource
				// proves the pipeline without a full archive.
				maxPages = 1
			}
			totalSynced := 0''', 1)
    s = s.replace('res := syncResource(cmd.Context(), c, s, resource, "", full, 100, false, false, nil, syncEventWriter)',
                  'res := syncResource(cmd.Context(), c, s, resource, "", full, maxPages, false, false, nil, syncEventWriter)')
    if '"priority-pp-cli/internal/cliutil"' not in s:
        s = s.replace('\t"github.com/spf13/cobra"\n\t"priority-pp-cli/internal/store"',
                      '\t"github.com/spf13/cobra"\n\t"priority-pp-cli/internal/cliutil"\n\t"priority-pp-cli/internal/store"', 1)
open(p, "w").write(s)
print("channel_workflow patch ok")
