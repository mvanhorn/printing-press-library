import os,pathlib
w=pathlib.Path(os.environ['CLI_WORK_DIR'])
for name in ['README.md','SKILL.md']:
 p=w/name;s=p.read_text().replace("These capabilities aren't available in any other tool for this API.","These workflows combine supplied local evidence into auditable reports and inactive plans.")
 s=s.replace("HTTP MCP binds loopback; it has no remote-caller authentication and must not be exposed publicly without an authenticated gateway.","HTTP MCP defaults to loopback and requires a caller token in SENDFOX_MCP_HTTP_TOKEN. Non-loopback binds also require TLS. Prefer stdio for agent use; the generated HTTP server still needs upstream header-timeout hardening.")
 s=s.replace('### Known API gaps and verification limits','## Known Gaps')
 if name=='SKILL.md':
  s=s.replace('This is an unpublished local reprint.','This is an unpublished local reprint. Published install links below resolve to the prior release. To use this edition, build its CLI and MCP from the local library directory; see README.md.')
 p.write_text(s)
