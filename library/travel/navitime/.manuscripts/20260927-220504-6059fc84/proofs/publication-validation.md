# Publication validation

The canonical publication candidate passed all 13 release checks. Fresh full live dogfood passed 56/56 mandatory checks, with zero failures and 50 framework checks skipped/unverified. Raw publish-time responses were held privately and deleted after extracting the result; the tool-owned acceptance marker is included.

| Check | Result |
|---|---|
| manifest | PASS |
| transcendence | PASS |
| phase5 | PASS |
| go mod tidy | PASS |
| module path | PASS |
| govulncheck | PASS |
| go vet | PASS |
| go build | PASS |
| --help | PASS |
| --version | PASS |
| verify-skill | PASS |
| patches | PASS |
| manuscripts | PASS |

The source was packaged by Printing Press with module `github.com/mvanhorn/printing-press-library/library/travel/navitime`. Public installation instructions replace the historical local-only template exception. The full original archive is preserved locally; this bundle contains selected public evidence.

Public-tree preflight also passed Go tests, vet, build and the repository skill verifier. Pinned govulncheck v1.3.0 passed under the repository Go 1.26.6 toolchain with zero reachable vulnerabilities; six findings in uncalled required modules are outside the reachability gate. Its initial Go 1.27.1 scanner crash is retained in the local diagnostic log, not treated as a clean scan.

Publication review corrections were tested before the final live gate: cached searches repair missing details and keep latest current without renewing response freshness; explicit timeout budgets are honored; mixed place searches request up to ten node candidates; MCP clients cannot relocate the server cache. Regression tests, full Go tests, vet and build pass. A fresh full live gate on the corrected source passed 56/56 mandatory checks, with 50 framework checks skipped/unverified. Runtime MCP discovery confirms seven public tools.
