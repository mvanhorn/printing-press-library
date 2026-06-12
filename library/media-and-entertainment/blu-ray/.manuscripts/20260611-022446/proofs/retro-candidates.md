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
