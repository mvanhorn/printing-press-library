# Retro candidates — blu-ray reprint (v4.24.0)

## R1: novel feature command colliding with a framework command leaves dangling AddCommand wiring
- Symptom: research.json novel_features had command "search"; framework emits internal/cli/search.go (newSearchCmd). Generator warned "novel feature command 'search' maps to existing internal/cli/search.go; leaving existing file unchanged" BUT still emitted `rootCmd.AddCommand(newNovelSearchCmd(flags))` in root.go → undefined: newNovelSearchCmd → govulncheck/build gate FAIL.
- Expected: when a novel-feature command name collides with a framework-emitted command, the generator should either skip the AddCommand wiring for that novel (the framework command already covers it) or rename, not emit a call to a function it deliberately did not generate.
- Workaround: dropped the "search" novel feature from research.json (framework search natively does offline FTS5 in 4.24.0) and regenerated.

## R2: 4.24.0 binary-response envelope broke ported sitemap gzip pipeline (port-adaptation, runtime-only)
- 4.24.0 client wraps non-textual Content-Type responses in a base64 envelope {"_pp_binary":true,"encoding":"base64","data":"..."} (content-type driven; BinaryResponseHeader now only sets Accept and is stripped from the request). Prior 4.8.0 client returned raw bytes for BinaryResponseHeader.
- The ported bluRayGet did []byte(data) + gunzip → "gzip: invalid header" → sync crashed → verify data_pipeline FAIL. Build+vet passed (runtime-only); caught by shipcheck verify, not codex.
- Fix: decodeMaybeBinaryEnvelope() unwraps the envelope before gunzip.
- Machine angle: no EXPORTED helper to decode binaryResponseEnvelope from outside package client; hand/novel code that fetches binary must re-implement the unwrap. Candidate: export a client.DecodeBinaryResponse([]byte)([]byte,contentType,bool) helper.

## R3: 4.24.0 generator stamped manifest.json version "0.0.0" (spec version was "0.1.0")
- Reprint generated manifest.json with version "0.0.0" despite spec.yaml version: "0.1.0". Prior 4.8.0 generation stamped manifest version = press version (4.8.0). Binary version ldflag then reports 0.0.0. Had to hand-set manifest version to 4.24.0 + rebuild. Candidate: generator should stamp manifest version from spec version or press version, not default 0.0.0.

## R4 (polish): verify/dogfood data-pipeline check assumes the GENERIC generated sync shape
- verify's data_pipeline probe hardcodes generic sync flags (--db/--resources/--full) and dogfood reads internal/cli/sync.go. A CLI with a hand-authored domain sync (blu-ray: sync_bluray.go, flags --kind/--max-pages/--wait/--quiet, calling UpsertCatalogRows/UpsertNewsRows) yields a false "sync crashed" / "sync uses generic Upsert only". The sync works (live 88/88). Candidate: the harness should detect/accommodate a domain sync that diverges from the generated interface (e.g. honor a spec/annotation declaring the sync command shape), not assume the generic flags + sync.go path.

## R5 (polish): generated client.APIError.Error() embeds the full raw HTTP response body
- internal/client/client.go APIError.Error() interpolates the entire response body (e.g. an nginx 404 HTML wall) into error strings → noisy, leaks server markup into agent-facing errors. Candidate: Error() should truncate/strip the response body (or cap to N bytes) in generated client.go.
