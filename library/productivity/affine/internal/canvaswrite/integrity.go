package canvaswrite

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/yjs"
)

type DocIntegrityOptions struct {
	WorkspaceID  string
	DocID        string
	SnapshotFile string
	Timestamp    string
}

type DocIntegrityIssue struct {
	Code    string `json:"code"`
	ID      string `json:"id,omitempty"`
	Parent  string `json:"parent,omitempty"`
	Child   string `json:"child,omitempty"`
	Flavour string `json:"flavour,omitempty"`
	Text    string `json:"text,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

type DocIntegrityResult struct {
	DocID      string              `json:"doc_id,omitempty"`
	Timestamp  string              `json:"timestamp,omitempty"`
	OK         bool                `json:"ok"`
	BlockCount int                 `json:"block_count"`
	IssueCount int                 `json:"issue_count"`
	Summary    map[string]int      `json:"summary"`
	Issues     []DocIntegrityIssue `json:"issues,omitempty"`
}

type DocRepairOptions struct {
	WorkspaceID string
	DocID       string
	Fix         string
	BackupDir   string
	Apply       bool
}

type DocRepairResult struct {
	DocID     string              `json:"doc_id"`
	Fix       string              `json:"fix"`
	DryRun    bool                `json:"dry_run"`
	BackupDir string              `json:"backup_dir,omitempty"`
	FixedIDs  []string            `json:"fixed_ids"`
	Before    DocIntegrityResult  `json:"before"`
	After     *DocIntegrityResult `json:"after,omitempty"`
}

func CheckDocIntegrity(cfg *config.Config, opts DocIntegrityOptions) (DocIntegrityResult, error) {
	if opts.WorkspaceID == "" {
		return DocIntegrityResult{}, fmt.Errorf("--workspace is required")
	}
	if opts.DocID == "" && opts.SnapshotFile == "" {
		return DocIntegrityResult{}, fmt.Errorf("--doc is required")
	}
	engine, err := yjs.NewEngine()
	if err != nil {
		return DocIntegrityResult{}, err
	}
	blocks, err := auditBlocks(cfg, engine, DocAuditOptions{
		WorkspaceID:  opts.WorkspaceID,
		DocID:        opts.DocID,
		SnapshotFile: opts.SnapshotFile,
		Timestamp:    opts.Timestamp,
	})
	if err != nil {
		return DocIntegrityResult{}, err
	}
	result := CheckBlocksIntegrity(opts.DocID, blocks)
	result.Timestamp = opts.Timestamp
	return result, nil
}

func CheckBlocksIntegrity(docID string, blocks map[string]map[string]any) DocIntegrityResult {
	result := DocIntegrityResult{
		DocID:      docID,
		BlockCount: len(blocks),
		Summary:    map[string]int{},
	}
	ids := sortedBlockIDs(blocks)
	parentByChild := map[string]string{}
	roots := pageRootIDs(blocks, ids)

	for _, id := range ids {
		block := blocks[id]
		if got := stringField(block, "sys:id"); got == "" {
			result.addIssue(DocIntegrityIssue{Code: "missing_id", ID: id, Flavour: stringField(block, "sys:flavour")})
		} else if got != id {
			result.addIssue(DocIntegrityIssue{Code: "id_mismatch", ID: id, Detail: got, Flavour: stringField(block, "sys:flavour")})
		}
		flavour := stringField(block, "sys:flavour")
		if flavour == "" {
			result.addIssue(DocIntegrityIssue{Code: "missing_flavour", ID: id})
		}
		children, ok := block["sys:children"].([]any)
		if _, exists := block["sys:children"]; !exists {
			result.addIssue(DocIntegrityIssue{Code: "missing_children", ID: id, Flavour: flavour, Text: truncateRunes(stringField(block, "prop:text"), 120)})
			continue
		}
		if !ok {
			result.addIssue(DocIntegrityIssue{Code: "invalid_children", ID: id, Flavour: flavour})
			continue
		}
		seen := map[string]bool{}
		for _, rawChild := range children {
			childID, ok := rawChild.(string)
			if !ok || childID == "" {
				result.addIssue(DocIntegrityIssue{Code: "invalid_child_ref", ID: id, Flavour: flavour, Detail: fmt.Sprintf("%v", rawChild)})
				continue
			}
			if seen[childID] {
				result.addIssue(DocIntegrityIssue{Code: "duplicate_child_ref", Parent: id, Child: childID, Flavour: flavour})
			}
			seen[childID] = true
			if blocks[childID] == nil {
				result.addIssue(DocIntegrityIssue{Code: "missing_child_ref", Parent: id, Child: childID, Flavour: flavour})
				continue
			}
			if previousParent := parentByChild[childID]; previousParent != "" && previousParent != id {
				result.addIssue(DocIntegrityIssue{Code: "multiple_parents", Parent: id, Child: childID, Detail: previousParent})
				continue
			}
			parentByChild[childID] = id
		}
	}

	if len(roots) != 1 {
		result.addIssue(DocIntegrityIssue{Code: "root_count", Detail: strconv.Itoa(len(roots))})
	}
	for _, id := range unreachableBlockIDs(blocks, roots) {
		result.addIssue(DocIntegrityIssue{Code: "unreachable_block", ID: id, Flavour: stringField(blocks[id], "sys:flavour")})
	}
	result.IssueCount = len(result.Issues)
	result.OK = result.IssueCount == 0
	return result
}

func EnsureBlocksIntegrity(docID string, blocks map[string]map[string]any, stage string) error {
	result := CheckBlocksIntegrity(docID, blocks)
	if result.OK {
		return nil
	}
	return fmt.Errorf("canvas doc integrity failed at %s: %s", stage, integritySummary(result))
}

func RepairDoc(cfg *config.Config, opts DocRepairOptions) (DocRepairResult, error) {
	if opts.WorkspaceID == "" {
		return DocRepairResult{}, fmt.Errorf("--workspace is required")
	}
	if opts.DocID == "" {
		return DocRepairResult{}, fmt.Errorf("--doc is required")
	}
	if opts.Fix == "" {
		opts.Fix = "missing-children"
	}
	if opts.Fix != "missing-children" {
		return DocRepairResult{}, fmt.Errorf("unsupported --fix %q; supported: missing-children", opts.Fix)
	}

	client, err := connect(cfg, opts.WorkspaceID)
	if err != nil {
		return DocRepairResult{}, err
	}
	defer client.Close()
	engine, err := yjs.NewEngine()
	if err != nil {
		return DocRepairResult{}, err
	}
	loaded, err := client.LoadDoc(opts.WorkspaceID, opts.DocID)
	if err != nil {
		return DocRepairResult{}, err
	}
	if loaded.Missing == "" {
		return DocRepairResult{}, fmt.Errorf("document returned empty snapshot")
	}
	doc, err := engine.ApplyBase64Update(loaded.Missing)
	if err != nil {
		return DocRepairResult{}, err
	}
	blocks, err := engine.ReadBlocks(doc)
	if err != nil {
		return DocRepairResult{}, err
	}
	before := CheckBlocksIntegrity(opts.DocID, blocks)
	fixedIDs := missingChildrenIDs(before)
	result := DocRepairResult{
		DocID:    opts.DocID,
		Fix:      opts.Fix,
		DryRun:   !opts.Apply,
		FixedIDs: fixedIDs,
		Before:   before,
	}
	if len(fixedIDs) == 0 {
		after := before
		result.After = &after
		return result, nil
	}
	if !opts.Apply {
		return result, nil
	}
	if opts.BackupDir == "" {
		opts.BackupDir = filepath.Join(os.TempDir(), "affine-repair-"+time.Now().Format("20060102-150405"))
	}
	result.BackupDir = opts.BackupDir
	if err := writeRepairBackup(opts.BackupDir, "before", loaded.Missing); err != nil {
		return DocRepairResult{}, err
	}
	stateVector, err := engine.SaveStateVector(doc)
	if err != nil {
		return DocRepairResult{}, err
	}
	rawIDs, err := json.Marshal(fixedIDs)
	if err != nil {
		return DocRepairResult{}, err
	}
	raw, err := engine.RunScript(fmt.Sprintf(`
		(function() {
			var doc = globalThis._docs[%d];
			var blocks = doc.getMap("blocks");
			var ids = %s;
			var fixed = [];
			for (var i = 0; i < ids.length; i++) {
				var id = ids[i];
				var block = blocks.get(id);
				if (block instanceof Y.Map && !(block.get("sys:children") instanceof Y.Array)) {
					block.set("sys:children", new Y.Array());
					fixed.push(id);
				}
			}
			return JSON.stringify({fixed_ids: fixed});
		})()
	`, doc, string(rawIDs)))
	if err != nil {
		return DocRepairResult{}, err
	}
	var patch struct {
		FixedIDs []string `json:"fixed_ids"`
	}
	if err := json.Unmarshal([]byte(raw), &patch); err != nil {
		return DocRepairResult{}, fmt.Errorf("parse repair result: %w", err)
	}
	result.FixedIDs = patch.FixedIDs
	blocks, err = engine.ReadBlocks(doc)
	if err != nil {
		return DocRepairResult{}, err
	}
	if err := EnsureBlocksIntegrity(opts.DocID, blocks, "after local repair"); err != nil {
		return DocRepairResult{}, err
	}
	delta, err := engine.EncodeDelta(doc, stateVector)
	if err != nil {
		return DocRepairResult{}, err
	}
	if err := writeRepairBackup(opts.BackupDir, "delta", delta); err != nil {
		return DocRepairResult{}, err
	}
	if err := client.PushDocUpdate(opts.WorkspaceID, opts.DocID, delta); err != nil {
		return DocRepairResult{}, err
	}
	reloaded, err := client.LoadDoc(opts.WorkspaceID, opts.DocID)
	if err != nil {
		return DocRepairResult{}, err
	}
	reloadedDoc, err := engine.ApplyBase64Update(reloaded.Missing)
	if err != nil {
		return DocRepairResult{}, err
	}
	reloadedBlocks, err := engine.ReadBlocks(reloadedDoc)
	if err != nil {
		return DocRepairResult{}, err
	}
	after := CheckBlocksIntegrity(opts.DocID, reloadedBlocks)
	if !after.OK {
		return DocRepairResult{}, fmt.Errorf("canvas doc integrity failed after pushed repair: %s", integritySummary(after))
	}
	result.After = &after
	return result, nil
}

func ensureEngineDocIntegrity(engine *yjs.Engine, doc int, docID, stage string) error {
	blocks, err := engine.ReadBlocks(doc)
	if err != nil {
		return err
	}
	return EnsureBlocksIntegrity(docID, blocks, stage)
}

func (r *DocIntegrityResult) addIssue(issue DocIntegrityIssue) {
	r.Issues = append(r.Issues, issue)
	r.Summary[issue.Code]++
}

func sortedBlockIDs(blocks map[string]map[string]any) []string {
	ids := make([]string, 0, len(blocks))
	for id := range blocks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func pageRootIDs(blocks map[string]map[string]any, ids []string) []string {
	var roots []string
	for _, id := range ids {
		if stringField(blocks[id], "sys:flavour") == "affine:page" {
			roots = append(roots, id)
		}
	}
	return roots
}

func unreachableBlockIDs(blocks map[string]map[string]any, roots []string) []string {
	seen := map[string]bool{}
	var walk func(string)
	walk = func(id string) {
		if id == "" || seen[id] || blocks[id] == nil {
			return
		}
		seen[id] = true
		children, _ := blocks[id]["sys:children"].([]any)
		for _, rawChild := range children {
			childID, _ := rawChild.(string)
			walk(childID)
		}
	}
	for _, root := range roots {
		walk(root)
	}
	var out []string
	for _, id := range sortedBlockIDs(blocks) {
		if !seen[id] {
			out = append(out, id)
		}
	}
	return out
}

func missingChildrenIDs(result DocIntegrityResult) []string {
	ids := []string{}
	for _, issue := range result.Issues {
		if issue.Code == "missing_children" {
			ids = append(ids, issue.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func integritySummary(result DocIntegrityResult) string {
	if len(result.Summary) == 0 {
		return "unknown integrity failure"
	}
	parts := make([]string, 0, len(result.Summary))
	for code, count := range result.Summary {
		parts = append(parts, fmt.Sprintf("%s=%d", code, count))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func writeRepairBackup(dir, name, b64 string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, name+".bin"), raw, 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".b64"), []byte(b64), 0o600)
}
