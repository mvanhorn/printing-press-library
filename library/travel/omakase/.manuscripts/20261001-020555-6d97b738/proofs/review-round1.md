Independent fresh-context reviewer gpt-6.1-sol/xhigh, sole review agent, no edits.

Five P2 findings: raw generated HTML/CSRF response cache; uncapped generated HTML response; included-tax/service wording parsed unknown; Japanese seasonal 上下 price variability not recognized; comparison dropped course notes/private-room fee. One P3: README timeout 30 vs runtime 60.

Fixes: preserved metadata client guard applies to generated CLI and MCP, disables raw caching, forces first-party HTTPS GET/observed public paths, identity response encoding and 2 MiB body cap; financial variants parsed and included-service flag added; observed Japanese price movement recognized; compare retains course_notes; timeout documentation corrected. Consequential regressions test each case. Same reviewer is verifying final source/live behaviors.
