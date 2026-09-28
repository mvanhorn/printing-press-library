# Phase19 manual MCP judgment

The stdio tools/list catalog was inspected, not just the static four-endpoint spec manifest. It contains 14 user-facing tools:12 domain CLI mirrors and2 lifted recipes. Every tool and its required/optional input fields was reviewed. No generic SQL, raw-store sync/search, auth, delivery, browser, or background-service tool is exposed. The tool descriptions are intentionally concise; parameters carry concrete names rather than repetitive prose.

| Runtime tool | Judgment |
|---|---|
| `areas` | Typed geography lookup; kinds/selector fields explain prefecture/area/station intent. Read-only source semantics with incidental derived cache writes; no user membership/notes mutation. |
| `compare_saved_candidates_offline` | Useful lifted comparison recipe with required list selector. Description is accurate. Its conservative default read-write/destructive/open-world hints were assigned to the builder for explicit read-only/non-destructive/closed-world alignment. |
| `cuisines` | Verified English cuisine taxonomy lookup. The description and typed query fields are specific; no mutation of user notebooks. |
| `find` | Source-ranked restaurant discovery; explicit area/cuisine/meal/budget/limit are exposed. Derived snapshot/cache persistence supports source reads; it is not a user-note write. |
| `lists_add` | Local membership/note write from already fetched snapshots; required list/id, note string. Local-write/non-read-only correctly describes user state mutation. |
| `lists_alternatives` | Offline factual matching within the saved set, not live/global recommendations. Required anchor and exact area/category/budget fields support the approved constraints. |
| `lists_audit` | Offline evidence-state and age check, not an availability checker or automatic refresh. The declared requirements/max-age fields are explicit. |
| `lists_compare` | Offline saved evidence comparison in saved order with separate budgets, unknowns, notes and source provenance. Read-only and non-destructive. |
| `lists_note` | Local personal-note update only; required list/id/note and no source fetch. Structured values do not become shell flags. |
| `lists_refresh` | Remote detail GET plus atomic local snapshot replacement, preserving notes and failed valid snapshots. Correctly non-read-only/non-destructive; discovered false closed-world hint is a real mismatch, assigned to the builder for open-world=true. |
| `lists_remove` | Local membership removal retains source facts. Correctly non-read-only/local; current actual-tree and wire regression owns this annotation fix. |
| `lists_show` | Offline notebook names or saved candidates with notes, source status and freshness. Optional list correctly allows notebook discovery. |
| `refresh_one_saved_candidate` | Useful bounded one-ID refresh recipe; description states fetch/current facts, failure retention and notes. Assigned explicit non-read-only/non-destructive/open-world hints; runtime handler/argv safety remains unchanged. |
| `show` | Canonical URL or fetched ID detail inspection; actual non-dry validation enforces the target, while a bare dry-run is a truthful unresolved plan. Source facts and age are preserved. |

The public read-only annotation means no external/user-managed state mutation; incidental derived source cache/snapshot writes are intentional read support. MCP writes to notes/membership/source snapshots are explicitly classified. The open-world refresh mismatch is fixed separately and must have a direct final wire assertion before acceptance. Conservatively true open-world defaults on other offline read mirrors do not claim source access or permit side effects; they are a future template precision improvement, not a false closed-world claim.

## Automated finding

The only tools-audit finding is the generated conditional client-profile `list` Short (platform_client.go:517). This leaf is not registered in the no-auth CLI because registeredPlatformSource is nil; it is absent from the actual 14-tool catalog. Its concise Short accurately describes the dormant management operation. Accepted with this evidence in .printing-press-tools-polish.json; scanner exclusion for unregistered conditional leaves is a template/tooling follow-up. The after audit reports0 pending,1 accepted, no incomplete block.

Evidence: polish-runtime-mcp-catalog.json; phase18-remove-classification-proof.json; builder final phase19 metadata/wire proof (to be linked in the completion report).

## Final metadata correction verified

The custom hook after runtime registration preserves the existing refresh schema and handler while setting openWorldHint=true. Explicit recipe options now classify offline comparison as read-only/non-destructive/closed-world and one-record refresh as non-read-only/non-destructive/open-world. A fresh protocol-only stdio tools/list capture asserts those exact values plus unchanged closed-world add/note/remove; it invokes no tool and performs no source GET. The catalog remains 14tools. See polish-runtime-mcp-catalog-final.json and the builder durable patch/focused 19-event proof. The previously identified false closed-world hint is resolved, not accepted as a gap.
