package pipeline

import (
    "encoding/json"
    "fmt"
    "os"
    "testing"
    presspipeline "github.com/mvanhorn/cli-printing-press/v4/internal/pipeline"
)

func TestTabiwaReviewerCurrentAcceptanceFingerprint(t *testing.T) {
    root := "<source-project>"
    proof := "<press-workspace>/.runstate/tabiwa-cli-dcb60d1f/runs/20261004-191703-ed1b3f92/proofs"
    snapshot, err := presspipeline.CaptureSourceFingerprint(root)
    if err != nil { t.Fatal(err) }
    raw, err := os.ReadFile(proof + "/phase5-acceptance.json")
    if err != nil { t.Fatal(err) }
    var marker struct {
        Fingerprint string `json:"source_fingerprint"`
        Files map[string]string `json:"source_files"`
    }
    if err := json.Unmarshal(raw, &marker); err != nil { t.Fatal(err) }
    mismatches := []string{}
    for path, expected := range marker.Files {
        if snapshot.Files[path] != expected { mismatches = append(mismatches, path) }
    }
    if snapshot.Digest != marker.Fingerprint || len(snapshot.Files) != len(marker.Files) || len(mismatches) != 0 {
        t.Fatalf("current fingerprint=%s marker=%s files=%d/%d mismatches=%v", snapshot.Digest, marker.Fingerprint, len(snapshot.Files), len(marker.Files), mismatches)
    }
    evidence := map[string]any{
        "method": "CaptureSourceFingerprint from the cached github.com/mvanhorn/cli-printing-press/v4 v4.33.0 matching installed binary build-info, imported through an isolated temporary child module and local replacement; no cached source/marker/harness file changed",
        "module_path": "tabiwa-pp-cli",
        "module_placeholder": "printing.press/generated-cli/tabiwa/tabiwa-pp-cli",
        "file_byte_normalization": "go.mod module identity and Go AST import declarations only; import entries/attached comments canonicalized and sorted; all bytes outside import declarations retained",
        "map_digest_method": "sorted relative path + NUL + per-file canonical-byte SHA256 + newline",
        "current_fingerprint": snapshot.Digest,
        "marker_fingerprint": marker.Fingerprint,
        "file_count": len(snapshot.Files),
        "current_canonical_file_hashes": snapshot.Files,
        "per_file_mismatches": mismatches,
        "marker_mutated": false,
        "harness_mutated": false,
    }
    data, err := json.MarshalIndent(evidence, "", "  ")
    if err != nil { t.Fatal(err) }
    if err := os.WriteFile(proof+"/reviewer-round4-acceptance-correspondence.json", append(data, '\n'), 0600); err != nil { t.Fatal(err) }
    fmt.Printf("Reproduced current canonical fingerprint %s and all %d marker file digests\n", snapshot.Digest, len(snapshot.Files))
}
