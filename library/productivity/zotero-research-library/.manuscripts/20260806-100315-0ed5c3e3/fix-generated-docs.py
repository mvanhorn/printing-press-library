#!/usr/bin/env python3
"""Re-apply doc fixes after every regen: drop 'Covered paths' bullets that do
not resolve via --help (generator emits sync-resource names as commands)."""
import re, subprocess, sys, os
zw = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'working/zotero-research-library-pp-cli')
binp = os.path.join(zw, 'zotero-research-library-pp-cli')
def resolves(args):
    r = subprocess.run([binp] + args.split() + ['--help'], capture_output=True)
    return r.returncode == 0
for doc in ('SKILL.md', 'README.md', 'AGENTS.md'):
    p = os.path.join(zw, doc)
    if not os.path.exists(p): continue
    lines = open(p).read().split('\n')
    out, dropped = [], 0
    for ln in lines:
        m = re.match(r'^- `zotero-research-library-pp-cli ([a-z][a-z -]*)`\s*$', ln)
        if m and not resolves(m.group(1)):
            dropped += 1
            continue
        out.append(ln)
    if dropped:
        open(p, 'w').write('\n'.join(out))
        print(f'{doc}: dropped {dropped} phantom path bullets')
