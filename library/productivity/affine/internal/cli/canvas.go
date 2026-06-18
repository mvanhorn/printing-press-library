package cli

// pp:data-source auto
// pp:client-call

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/canvaswrite"
	"github.com/mvanhorn/printing-press-library/library/productivity/affine/internal/config"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

type canvasBuildSpec struct {
	Frame       string              `json:"frame"`
	Orientation string              `json:"orientation"`
	Hub         string              `json:"hub"`
	Levels      [][]string          `json:"levels"`
	Connections []canvasConnection  `json:"connections,omitempty"`
	Card        canvasCardSize      `json:"card,omitempty"`
	Spacing     canvasLayoutSpacing `json:"spacing,omitempty"`
	Origin      canvasPoint         `json:"origin,omitempty"`
}

type canvasCardSize struct {
	W float64 `json:"w,omitempty"`
	H float64 `json:"h,omitempty"`
}

type canvasLayoutSpacing struct {
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
}

type canvasPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type canvasNode struct {
	ID    string  `json:"id"`
	Text  string  `json:"text"`
	Role  string  `json:"role,omitempty"`
	Level int     `json:"level"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
}

type canvasConnection struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type,omitempty"`
}

type canvasPlan struct {
	Frame       string             `json:"frame"`
	Orientation string             `json:"orientation"`
	Nodes       []canvasNode       `json:"nodes"`
	Connections []canvasConnection `json:"connections"`
	Warnings    []string           `json:"warnings,omitempty"`
}

type canvasModel struct {
	Nodes       []map[string]any `json:"nodes"`
	Connections []map[string]any `json:"connections"`
	Warnings    []string         `json:"warnings,omitempty"`
}

type canvasValidationReport struct {
	OK         bool                    `json:"ok"`
	IssueCount int                     `json:"issue_count"`
	Issues     []canvasValidationIssue `json:"issues,omitempty"`
}

type canvasValidationIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	NodeID   string `json:"node_id,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
}

func newCanvasCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "canvas",
		Short: "Plan and inspect AFFiNE canvas layouts",
		Long:  "Plan and inspect AFFiNE canvas layouts. Vertical trees are the default; horizontal layouts are available with --orientation horizontal.",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCanvasPlanCmd(flags))
	cmd.AddCommand(newCanvasApplyCmd(flags))
	cmd.AddCommand(newCanvasModelCmd(flags))
	cmd.AddCommand(newCanvasValidateCmd(flags))
	cmd.AddCommand(newCanvasExampleCmd())
	cmd.AddCommand(newCanvasCardCmd(flags))
	cmd.AddCommand(newCanvasDocCmd(flags))
	cmd.AddCommand(newCanvasBlockCmd(flags))
	return cmd
}

func newCanvasBlockCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.BlockInspectOptions
	cmd := &cobra.Command{
		Use:     "block",
		Short:   "Inspect one AFFiNE block without modifying it",
		Example: "  affine-pp-cli canvas block --workspace <workspace-id> --doc <doc-id> --block <block-id> --json",
		Annotations: map[string]string{
			"mcp:read-only":    "true",
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.InspectBlock(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.BlockID, "block", "", "Block ID")
	return cmd
}
func newCanvasDocCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doc",
		Short: "Inspect a canvas document",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCanvasDocAuditCmd(flags))
	cmd.AddCommand(newCanvasDocIntegrityCmd(flags))
	cmd.AddCommand(newCanvasDocRepairCmd(flags))
	return cmd
}

func newCanvasDocIntegrityCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.DocIntegrityOptions
	cmd := &cobra.Command{
		Use:   "integrity",
		Short: "Check structural integrity of one AFFiNE canvas document",
		Example: "  affine-pp-cli canvas doc integrity --workspace <workspace-id> --doc <doc-id> --json\n" +
			"  affine-pp-cli canvas doc integrity --snapshot-file doc.bin --json",
		Annotations: map[string]string{
			"mcp:read-only":    "true",
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.CheckDocIntegrity(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.SnapshotFile, "snapshot-file", "", "Local AFFiNE Y.js snapshot file to check")
	cmd.Flags().StringVar(&opts.Timestamp, "timestamp", "", "AFFiNE history timestamp to check")
	return cmd
}

func newCanvasDocRepairCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.DocRepairOptions
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Repair a narrow set of safe AFFiNE canvas document integrity issues",
		Example: "  affine-pp-cli canvas doc repair --workspace <workspace-id> --doc <doc-id> --dry-run --json\n" +
			"  affine-pp-cli canvas doc repair --workspace <workspace-id> --doc <doc-id> --apply --backup-dir ./backups --json",
		Annotations: map[string]string{
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				opts.Apply = false
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.RepairDoc(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.Fix, "fix", "missing-children", "Integrity issue to fix")
	cmd.Flags().StringVar(&opts.BackupDir, "backup-dir", "", "Directory for before/delta backups when applying")
	cmd.Flags().BoolVar(&opts.Apply, "apply", false, "Push the repair update to AFFiNE")
	return cmd
}

func newCanvasDocAuditCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.DocAuditOptions
	var keywords string
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit one AFFiNE canvas document without modifying it",
		Example: "  affine-pp-cli canvas doc audit --workspace <workspace-id> --doc <doc-id> --keywords trabalho,comunidade --json\n" +
			"  affine-pp-cli canvas doc audit --snapshot-file doc.bin --json",
		Annotations: map[string]string{
			"mcp:read-only":    "true",
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			opts.Keywords = splitCanvasKeywords(keywords)
			result, err := canvaswrite.AuditDoc(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.SnapshotFile, "snapshot-file", "", "Local AFFiNE Y.js snapshot file to audit")
	cmd.Flags().StringVar(&opts.Timestamp, "timestamp", "", "AFFiNE history timestamp to audit")
	cmd.Flags().StringVar(&keywords, "keywords", "", "Comma-separated keywords to count and sample")
	cmd.Flags().IntVar(&opts.TextLimit, "text-limit", 240, "Maximum text length for each match sample")
	return cmd
}

func newCanvasCardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "card",
		Short: "Patch individual canvas cards",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCanvasCardSetImageCmd(flags))
	cmd.AddCommand(newCanvasCardInspectCmd(flags))
	return cmd
}

func newCanvasCardInspectCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.CardInspectOptions
	cmd := &cobra.Command{
		Use:     "inspect",
		Short:   "Inspect one canvas card's child blocks",
		Example: "  affine-pp-cli canvas card inspect --workspace <workspace-id> --doc <doc-id> --card <card-id> --json",
		Annotations: map[string]string{
			"mcp:read-only":    "true",
			"pp:requires-tier": "affine-workspace-fixture",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.InspectCard(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.CardID, "card", "", "Canvas note/card block ID")
	return cmd
}

func newCanvasCardSetImageCmd(flags *rootFlags) *cobra.Command {
	var opts canvaswrite.CardImageOptions
	cmd := &cobra.Command{
		Use:     "set-image",
		Short:   "Set a card image/logo without replacing the whole card",
		Example: "  affine-pp-cli canvas card set-image --workspace <workspace-id> --doc <doc-id> --card <card-id> --source-id <blob-id> --alt LightRAG --dry-run --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"doc_id":  opts.DocID,
					"card_id": opts.CardID,
					"source":  opts.SourceID,
				})
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			result, err := canvaswrite.SetCardImage(cfg, opts)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&opts.WorkspaceID, "workspace", "", "AFFiNE workspace ID")
	cmd.Flags().StringVar(&opts.DocID, "doc", "", "AFFiNE document ID")
	cmd.Flags().StringVar(&opts.CardID, "card", "", "Canvas note/card block ID")
	cmd.Flags().StringVar(&opts.SourceID, "source-id", "", "Existing AFFiNE blob source ID")
	cmd.Flags().StringVar(&opts.Alt, "alt", "", "Fallback alt text to remove, e.g. LightRAG")
	cmd.Flags().IntVar(&opts.Width, "width", 120, "Image width")
	cmd.Flags().IntVar(&opts.Height, "height", 90, "Image height")
	return cmd
}

func newCanvasPlanCmd(flags *rootFlags) *cobra.Command {
	var specPath string
	var orientation string
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Create a deterministic card layout plan from a JSON spec",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: "  affine-pp-cli canvas example > comunidade.json\n" +
			"  affine-pp-cli canvas plan --json\n" +
			"  affine-pp-cli canvas plan --spec comunidade.json --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, err := readCanvasBuildSpecOrExample(specPath, cmd.InOrStdin())
			if err != nil {
				return err
			}
			if orientation != "" {
				spec.Orientation = orientation
			}
			plan := buildCanvasPlan(spec)
			return writeJSON(cmd.OutOrStdout(), plan)
		},
	}
	cmd.Flags().StringVar(&specPath, "spec", "", "JSON layout spec path; reads stdin when empty")
	cmd.Flags().StringVar(&orientation, "orientation", "", "Layout orientation: vertical or horizontal")
	return cmd
}

func newCanvasApplyCmd(flags *rootFlags) *cobra.Command {
	var planPath string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Print canvas operations for a generated plan",
		Long:  "Print canvas operations for a generated plan. Live Y.js writes are intentionally not enabled yet; use --dry-run.",
		Example: "  affine-pp-cli canvas apply --dry-run --json\n" +
			"  affine-pp-cli canvas example | affine-pp-cli canvas plan | affine-pp-cli canvas apply --dry-run --json\n" +
			"  affine-pp-cli canvas apply --plan plan.json --dry-run --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !flags.dryRun {
				return fmt.Errorf("canvas apply currently requires --dry-run")
			}
			plan, err := readCanvasPlanOrExample(planPath, cmd.InOrStdin())
			if err != nil {
				return err
			}
			ops := map[string]any{
				"dry_run": true,
				"frame":   plan.Frame,
				"operations": map[string]any{
					"upsert_nodes":         plan.Nodes,
					"upsert_connections":   plan.Connections,
					"live_write_supported": false,
				},
			}
			return writeJSON(cmd.OutOrStdout(), ops)
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "JSON plan path; reads stdin when empty")
	return cmd
}

func newCanvasModelCmd(flags *rootFlags) *cobra.Command {
	var inputPath string
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Normalize inspect-canvas JSON into nodes and connections",
		Example: "  affine-pp-cli canvas model --json\n" +
			"  affine-pp-cli canvas model --input inspect-canvas.json --json\n" +
			"  cat inspect-canvas.json | affine-pp-cli canvas model --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			model, err := readCanvasModelOrExample(inputPath, cmd.InOrStdin())
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), model)
		},
	}
	cmd.Flags().StringVar(&inputPath, "input", "", "inspect-canvas JSON path; reads stdin when empty")
	return cmd
}

func newCanvasValidateCmd(flags *rootFlags) *cobra.Command {
	var planPath string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a canvas plan before applying it",
		Example: "  affine-pp-cli canvas validate --json\n" +
			"  affine-pp-cli canvas example | affine-pp-cli canvas plan | affine-pp-cli canvas validate --json\n" +
			"  affine-pp-cli canvas validate --plan plan.json --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := readCanvasPlanOrExample(planPath, cmd.InOrStdin())
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), validateCanvasPlan(plan))
		},
	}
	cmd.Flags().StringVar(&planPath, "plan", "", "JSON plan path; reads stdin when empty")
	return cmd
}

func newCanvasExampleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "example",
		Short: "Print a vertical tree JSON spec",
		Example: "  affine-pp-cli canvas example --json\n" +
			"  affine-pp-cli canvas example > comunidade.json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return writeJSON(cmd.OutOrStdout(), exampleCanvasBuildSpec())
		},
	}
}

func readCanvasBuildSpecOrExample(path string, stdin io.Reader) (canvasBuildSpec, error) {
	if path != "" {
		return readCanvasBuildSpec(path, stdin)
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return canvasBuildSpec{}, fmt.Errorf("reading stdin: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return exampleCanvasBuildSpec(), nil
	}
	var spec canvasBuildSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return spec, fmt.Errorf("parsing canvas spec JSON: %w", err)
	}
	return spec, nil
}

func exampleCanvasBuildSpec() canvasBuildSpec {
	return canvasBuildSpec{
		Frame:       "COMUNIDADE",
		Orientation: "vertical",
		Hub:         "Carta Clube",
		Levels: [][]string{
			{"Como Trabalhamos", "Identidade Visual", "Carta Clube - Principios", "Carta Clube - Qualidades"},
			{"Carta Clube"},
			{"Coding", "Comunidade", "Criacao", "Execucao"},
		},
		Connections: []canvasConnection{
			{From: "Carta Clube - Principios", To: "Carta Clube"},
			{From: "Carta Clube", To: "Coding"},
			{From: "Carta Clube", To: "Comunidade"},
			{From: "Carta Clube", To: "Criacao"},
			{From: "Carta Clube", To: "Execucao"},
		},
	}
}

func readCanvasBuildSpec(path string, stdin io.Reader) (canvasBuildSpec, error) {
	var spec canvasBuildSpec
	data, err := readAllOrFile(path, stdin)
	if err != nil {
		return spec, err
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		return spec, fmt.Errorf("parsing canvas spec JSON: %w", err)
	}
	return spec, nil
}

func readCanvasPlan(path string, stdin io.Reader) (canvasPlan, error) {
	var plan canvasPlan
	data, err := readAllOrFile(path, stdin)
	if err != nil {
		return plan, err
	}
	if err := json.Unmarshal(data, &plan); err != nil {
		return plan, fmt.Errorf("parsing canvas plan JSON: %w", err)
	}
	return plan, nil
}

func readCanvasPlanOrExample(path string, stdin io.Reader) (canvasPlan, error) {
	if path != "" {
		return readCanvasPlan(path, stdin)
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return canvasPlan{}, fmt.Errorf("reading stdin: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return buildCanvasPlan(exampleCanvasBuildSpec()), nil
	}
	var plan canvasPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return plan, fmt.Errorf("parsing canvas plan JSON: %w", err)
	}
	return plan, nil
}

func readCanvasModel(path string, stdin io.Reader) (canvasModel, error) {
	data, err := readAllOrFile(path, stdin)
	if err != nil {
		return canvasModel{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return canvasModel{}, fmt.Errorf("parsing inspect-canvas JSON: %w", err)
	}
	var model canvasModel
	walkCanvasValue(raw, &model)
	return model, nil
}

func readCanvasModelOrExample(path string, stdin io.Reader) (canvasModel, error) {
	if path != "" {
		return readCanvasModel(path, stdin)
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return canvasModel{}, fmt.Errorf("reading stdin: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return exampleCanvasModel(), nil
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return canvasModel{}, fmt.Errorf("parsing inspect-canvas JSON: %w", err)
	}
	var model canvasModel
	walkCanvasValue(raw, &model)
	return model, nil
}

func exampleCanvasModel() canvasModel {
	return canvasModel{
		Nodes: []map[string]any{
			{"id": "carta-clube", "text": "Carta Clube", "xywh": "[0,0,360,220]"},
			{"id": "comunidade", "text": "Comunidade", "xywh": "[0,360,360,220]"},
		},
		Connections: []map[string]any{
			{"id": "carta-clube-comunidade", "source": "carta-clube", "target": "comunidade", "type": "connector"},
		},
	}
}

func walkCanvasValue(v any, model *canvasModel) {
	switch x := v.(type) {
	case map[string]any:
		if connection, ok := normalizeCanvasConnection(x); ok {
			model.Connections = append(model.Connections, connection)
		} else if node, ok := normalizeCanvasNode(x); ok {
			model.Nodes = append(model.Nodes, node)
		}
		for key, child := range x {
			if key == "props" {
				continue
			}
			walkCanvasValue(child, model)
		}
	case []any:
		for _, child := range x {
			walkCanvasValue(child, model)
		}
	}
}

func normalizeCanvasNode(src map[string]any) (map[string]any, bool) {
	text := firstCanvasString(src, "text", "title")
	xywh := firstCanvasString(src, "xywh", "prop:xywh")
	if props, _ := src["props"].(map[string]any); props != nil {
		if text == "" {
			text = firstCanvasString(props, "text", "title")
		}
		if xywh == "" {
			xywh = firstCanvasString(props, "xywh", "prop:xywh")
		}
	}
	if text == "" || xywh == "" {
		return nil, false
	}
	out := compactCanvasMap(src, "id", "flavour", "type", "text", "title", "surface_id")
	out["text"] = text
	out["xywh"] = xywh
	return out, true
}

func normalizeCanvasConnection(src map[string]any) (map[string]any, bool) {
	source := firstCanvasString(src, "source", "from")
	target := firstCanvasString(src, "target", "to")
	props, _ := src["props"].(map[string]any)
	if props != nil {
		if source == "" {
			source = firstCanvasString(props, "source", "from")
		}
		if target == "" {
			target = firstCanvasString(props, "target", "to")
		}
	}
	if source == "" || target == "" {
		return nil, false
	}
	out := compactCanvasMap(src, "id", "type", "flavour", "text", "surface_id", "xywh", "prop:xywh")
	if props != nil {
		if _, ok := out["type"]; !ok {
			copyCanvasField(out, props, "type")
		}
		if _, ok := out["xywh"]; !ok {
			copyCanvasField(out, props, "xywh")
		}
	}
	out["source"] = source
	out["target"] = target
	return out, true
}

func firstCanvasString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := canvasStringValue(m[key]); value != "" {
			return value
		}
	}
	return ""
}

func canvasStringValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		return firstCanvasString(x, "id", "blockId", "elementId")
	default:
		if x == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func copyCanvasField(dst map[string]any, src map[string]any, key string) {
	if value, ok := src[key]; ok && value != nil && value != "" {
		dst[key] = value
	}
}

func compactCanvasMap(src map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := src[key]; ok && value != nil && value != "" {
			out[key] = value
		}
	}
	return out
}

func buildCanvasPlan(spec canvasBuildSpec) canvasPlan {
	spec = normalizeCanvasSpec(spec)
	var nodes []canvasNode
	seen := map[string]bool{}
	for level, names := range spec.Levels {
		for idx, name := range names {
			node := canvasNode{
				ID:    slugCanvasID(name),
				Text:  name,
				Role:  canvasRole(name, spec.Hub),
				Level: level,
				W:     spec.Card.W,
				H:     spec.Card.H,
			}
			node.X, node.Y = canvasPosition(spec, level, idx, len(names))
			if seen[node.ID] {
				node.ID = fmt.Sprintf("%s-%d-%d", node.ID, level, idx)
			}
			seen[node.ID] = true
			nodes = append(nodes, node)
		}
	}
	connections := spec.Connections
	if len(connections) == 0 && spec.Hub != "" {
		connections = inferredHubConnections(spec)
	}
	return canvasPlan{
		Frame:       spec.Frame,
		Orientation: spec.Orientation,
		Nodes:       nodes,
		Connections: connections,
		Warnings:    canvasPlanWarnings(spec),
	}
}

func normalizeCanvasSpec(spec canvasBuildSpec) canvasBuildSpec {
	if spec.Orientation == "" {
		spec.Orientation = "vertical"
	}
	spec.Orientation = strings.ToLower(spec.Orientation)
	if spec.Card.W == 0 {
		spec.Card.W = 360
	}
	if spec.Card.H == 0 {
		spec.Card.H = 220
	}
	if spec.Spacing.X == 0 {
		spec.Spacing.X = 80
	}
	if spec.Spacing.Y == 0 {
		spec.Spacing.Y = 140
	}
	return spec
}

func canvasPosition(spec canvasBuildSpec, level, idx, count int) (float64, float64) {
	span := float64(count-1) * (spec.Card.W + spec.Spacing.X)
	if spec.Orientation == "horizontal" {
		return spec.Origin.X + float64(level)*(spec.Card.W+spec.Spacing.X), spec.Origin.Y + float64(idx)*(spec.Card.H+spec.Spacing.Y) - span/2
	}
	return spec.Origin.X + float64(idx)*(spec.Card.W+spec.Spacing.X) - span/2, spec.Origin.Y + float64(level)*(spec.Card.H+spec.Spacing.Y)
}

func inferredHubConnections(spec canvasBuildSpec) []canvasConnection {
	var out []canvasConnection
	for _, level := range spec.Levels {
		for _, name := range level {
			if name != spec.Hub {
				out = append(out, canvasConnection{From: spec.Hub, To: name, Type: "tree"})
			}
		}
	}
	return out
}

func canvasPlanWarnings(spec canvasBuildSpec) []string {
	var warnings []string
	if spec.Orientation != "vertical" && spec.Orientation != "horizontal" {
		warnings = append(warnings, "unknown orientation; use vertical or horizontal")
	}
	if spec.Hub == "" {
		warnings = append(warnings, "hub is empty; connections must be explicit")
	}
	if len(spec.Levels) == 0 {
		warnings = append(warnings, "levels is empty")
	}
	return warnings
}

func validateCanvasPlan(plan canvasPlan) canvasValidationReport {
	var issues []canvasValidationIssue
	if plan.Frame == "" {
		issues = append(issues, canvasValidationIssue{
			Severity: "warning",
			Code:     "missing_frame",
			Message:  "plan frame is empty",
		})
	}
	if plan.Orientation != "vertical" && plan.Orientation != "horizontal" {
		issues = append(issues, canvasValidationIssue{
			Severity: "error",
			Code:     "unknown_orientation",
			Message:  "orientation must be vertical or horizontal",
		})
	}
	if len(plan.Nodes) == 0 {
		issues = append(issues, canvasValidationIssue{
			Severity: "error",
			Code:     "empty_nodes",
			Message:  "plan has no nodes",
		})
	}

	known := map[string]bool{}
	for _, node := range plan.Nodes {
		if node.ID == "" {
			issues = append(issues, canvasValidationIssue{
				Severity: "error",
				Code:     "missing_node_id",
				Message:  "node has empty id",
			})
			continue
		}
		if known[node.ID] {
			issues = append(issues, canvasValidationIssue{
				Severity: "error",
				Code:     "duplicate_node_id",
				Message:  "node id is duplicated",
				NodeID:   node.ID,
			})
		}
		known[node.ID] = true
		if node.Text != "" {
			known[node.Text] = true
		}
		if node.W <= 0 || node.H <= 0 {
			issues = append(issues, canvasValidationIssue{
				Severity: "error",
				Code:     "invalid_node_size",
				Message:  "node width and height must be positive",
				NodeID:   node.ID,
			})
		}
	}

	for _, conn := range plan.Connections {
		if conn.From == "" || conn.To == "" {
			issues = append(issues, canvasValidationIssue{
				Severity: "error",
				Code:     "invalid_connection",
				Message:  "connection must include from and to",
				From:     conn.From,
				To:       conn.To,
			})
			continue
		}
		if !known[conn.From] || !known[conn.To] {
			issues = append(issues, canvasValidationIssue{
				Severity: "error",
				Code:     "orphan_connection",
				Message:  "connection endpoint is not present in plan nodes",
				From:     conn.From,
				To:       conn.To,
			})
		}
	}

	for i := 0; i < len(plan.Nodes); i++ {
		for j := i + 1; j < len(plan.Nodes); j++ {
			if canvasNodesOverlap(plan.Nodes[i], plan.Nodes[j]) {
				issues = append(issues, canvasValidationIssue{
					Severity: "warning",
					Code:     "overlapping_nodes",
					Message:  fmt.Sprintf("nodes %q and %q overlap", plan.Nodes[i].ID, plan.Nodes[j].ID),
					NodeID:   plan.Nodes[i].ID,
				})
			}
		}
	}

	ok := true
	for _, issue := range issues {
		if issue.Severity == "error" {
			ok = false
			break
		}
	}
	return canvasValidationReport{
		OK:         ok,
		IssueCount: len(issues),
		Issues:     issues,
	}
}

func canvasNodesOverlap(a, b canvasNode) bool {
	if a.W <= 0 || a.H <= 0 || b.W <= 0 || b.H <= 0 {
		return false
	}
	return a.X < b.X+b.W && a.X+a.W > b.X && a.Y < b.Y+b.H && a.Y+a.H > b.Y
}

func canvasRole(name, hub string) string {
	if name == hub {
		return "hub"
	}
	return "card"
}

func slugCanvasID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func splitCanvasKeywords(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func readAllOrFile(path string, stdin io.Reader) ([]byte, error) {
	if path != "" {
		return os.ReadFile(path)
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("empty input; pass --spec/--plan/--input or pipe JSON on stdin")
	}
	return data, nil
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
