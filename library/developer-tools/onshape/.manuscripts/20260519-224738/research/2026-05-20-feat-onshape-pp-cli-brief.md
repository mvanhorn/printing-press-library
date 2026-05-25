# Onshape CLI Research Brief

## Sources

- Onshape Developer Documentation: introduces the REST model, DWVME identifiers, versioned `/api/vN` endpoints, and the cloud-native document model.
- Onshape API Keys documentation: documents API-key authentication, Basic auth for local testing, and the request-signature flow using `Date`, `On-Nonce`, and `Authorization: On <AccessKey>:HmacSHA256:<Signature>`.
- Onshape Glassworks API Explorer: official endpoint browser for GET, POST, and DELETE calls with API key, OAuth, or active Onshape session auth.
- PyPI `onshape-rest-api-client` and legacy `onshape-client`: SDK-style clients that expose raw REST coverage but do not provide agent-native CLI workflows.
- Public Onshape MCP listings: demonstrate demand for CAD access through agent protocols, but are narrower than a full CLI + MCP + local-store workflow.

## Research Findings

Onshape is a strong fit for Printing Press because it is cloud-native and API-first compared with traditional file-centered CAD. The core workflow is not merely "call an endpoint"; agents must move through the Onshape document model: document ID, workspace/version/microversion, element ID, then part/assembly/export IDs.

The current generated CLI already covers the first useful CAD workflows: document search, workspace/version discovery, element listing, part listing/get, assembly get/list, translation jobs, exports, sync/search/analytics, profiles, delivery sinks, and MCP mirroring. The important gap found during live testing was authentication: the generated client read access/secret keys but did not sign requests. Onshape API keys require signed request headers for robust automation.

## User Vision

The user is focusing this CLI on CAD assemblies, renderings, and workflows where Onshape models feed Blender and later engineering tools such as CFD/simulation. The CLI should therefore optimize for: discovering the right document, extracting assembly/part structure, finding export targets, keeping responses small for agents, and preserving enough local state that future Blender tooling can consume Onshape IDs without rediscovery.

## Auth

Onshape supports API-key automation for personal/internal tools and OAuth2 for App Store applications. For this CLI, API-key automation is the right immediate path. The implementation accepts both the canonical env vars `ONSHAPE_ACCESS_KEY` / `ONSHAPE_SECRET_KEY` and the user-provided aliases `ONSHAPE_API_ACCESSKEY` / `ONSHAPE_API_SECRETKEY`. Requests are signed with the documented Onshape request-signature headers.

## Recommendation

Proceed with publishing after Phase 5 acceptance. The CLI is not just a mechanical wrapper: it gives agents a repeatable CAD navigation loop and a base for Blender handoff. Follow-up improvements should focus on dedicated export orchestration, BOM/table endpoints, thumbnails, mass-properties, FeatureScript evaluation, and higher-level "prepare-for-Blender" workflows.
