package cli

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface-pp-cli/internal/hfx"
)

type evalCandidatesResponse struct {
	hfx.Envelope
	Base       string                       `json:"base"`
	TargetSize string                       `json:"target_size"`
	Emit       string                       `json:"emit"`
	HarnessDir string                       `json:"harness_dir,omitempty"`
	Models     map[string]evalCandidateRow  `json:"models,omitempty"`
	Script     []string                     `json:"add_model_script,omitempty"`
	Explain    string                       `json:"explain,omitempty"`
}

type evalCandidateRow struct {
	Path        string `json:"path"`
	Label       string `json:"label"`
	HFID        string `json:"hf_id"`
	Quant       string `json:"quant"`
	SizeGB      string `json:"size_gb,omitempty"`
	Uploader    string `json:"uploader"`
	UploaderRep bool   `json:"uploader_rep"`
}

func newHFEvalCandidatesCmd(flags *rootFlags) *cobra.Command {
	var baseModel, targetSize, preferStr, uploadersStr, emit, downloadRoot string
	cmd := &cobra.Command{
		Use:   "eval-candidates",
		Short: "Emit model-eval-harness-ready candidate list (wires HF discovery into the eval loop).",
		Long: `eval-candidates wraps find-quants and emits the result in the model-eval-harness
input format. Two emit shapes:

  --emit harness-input    JSON object ready to merge into models.json (default)
  --emit add-model-script Shell lines with one 'node run.mjs --add-model ...' per candidate

Closes the loop between HF discovery and the existing eval pipeline.

Path resolution: if --download-root is set, the emitted 'path' is the absolute
local target where the user should download the GGUF. Otherwise emits the HF id
directly (the harness can resolve it via huggingface_hub).`,
		Example: `  huggingface-pp-cli eval-candidates --base Qwen/Qwen2.5-7B --target-size 8g
  huggingface-pp-cli eval-candidates --base Qwen/Qwen3-MoE-A14B --target-size 25g --emit add-model-script
  huggingface-pp-cli eval-candidates --base Qwen/Qwen2.5-7B --download-root ~/.local/models --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if baseModel == "" {
				return hfNotFound("--base is required (e.g. --base Qwen/Qwen2.5-7B)")
			}
			emitMode := strings.ToLower(strings.TrimSpace(emit))
			switch emitMode {
			case "", "harness-input":
				emitMode = "harness-input"
			case "add-model-script", "script":
				emitMode = "add-model-script"
			default:
				return hfNotFound("--emit %q invalid (use: harness-input, add-model-script)", emit)
			}

			ctx := cmd.Context()
			opts := parseFindQuantsOpts(preferStr, uploadersStr, targetSize, flags.limit)
			core, err := findQuantsCore(ctx, baseModel, opts)
			if err != nil {
				return err
			}
			if len(core.Results) == 0 {
				return hfNotFound("no candidates found for base=%q size<=%s (try widening uploaders or removing --prefer)", baseModel, targetSize)
			}

			// Discover harness dir for completeness reporting (not required to emit).
			harnessCands := []string{"workspace/scripts/model-eval-harness"}
			if root := hfOpenclawRoot(); root != "" {
				harnessCands = append(harnessCands, filepath.Join(root, "workspace", "scripts", "model-eval-harness"))
			}
			harnessDir := firstExistingPath(harnessCands)

			models := map[string]evalCandidateRow{}
			scriptLines := []string{}
			for _, q := range core.Results {
				key := evalCandidateKey(q.ID, q.Quant)
				path := q.ID + ":" + filepath.Base(q.Quant) // default: HF-id-style path the harness resolves
				if downloadRoot != "" {
					// Local target: <root>/<repo-name>-<quant>.gguf
					repoName := strings.ReplaceAll(q.ID, "/", "-")
					path = filepath.Join(expandHome(downloadRoot), repoName+"-"+q.Quant+".gguf")
				}
				label := fmt.Sprintf("%s — %s (%s)", baseModel, q.Quant, q.Uploader)
				models[key] = evalCandidateRow{
					Path:        path,
					Label:       label,
					HFID:        q.ID,
					Quant:       q.Quant,
					SizeGB:      q.SizeGB,
					Uploader:    q.Uploader,
					UploaderRep: q.UploaderRep,
				}
				scriptLines = append(scriptLines, fmt.Sprintf(`node workspace/scripts/model-eval-harness/run.mjs --add-model %s %q %q`,
					key, path, label))
			}

			// Stable sort emitted keys for reproducibility
			keys := make([]string, 0, len(models))
			for k := range models {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			resp := evalCandidatesResponse{
				Envelope:   hfx.NewEnvelope("eval-candidates"),
				Base:       baseModel,
				TargetSize: targetSize,
				Emit:       emitMode,
				HarnessDir: harnessDir,
			}
			if emitMode == "harness-input" {
				resp.Models = models
			} else {
				resp.Script = scriptLines
			}
			if flags.explain {
				resp.Explain = fmt.Sprintf("explain: emitted %d candidates derived from find-quants(base=%s, max=%s); use --emit add-model-script to get ready-to-run 'node run.mjs --add-model' lines.",
					len(models), baseModel, targetSize)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}

			// Human path mirrors the script form so the user can copy-paste
			fmt.Fprintf(cmd.OutOrStdout(), "eval-candidates: %d entries for %s (target<=%s)\n", len(keys), baseModel, targetSize)
			if harnessDir == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "  WARNING: no harness dir found at workspace/scripts/model-eval-harness; emission is informational")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  harness dir: %s\n", harnessDir)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "")
			if emitMode == "add-model-script" {
				for _, line := range scriptLines {
					fmt.Fprintln(cmd.OutOrStdout(), line)
				}
			} else {
				for _, k := range keys {
					m := models[k]
					rep := " "
					if m.UploaderRep {
						rep = "*"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s%-35s  %s  %-12s  %s\n", rep, k, m.HFID, m.Quant, m.SizeGB)
				}
			}
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&baseModel, "base", "", "Base model HF id (required, e.g. Qwen/Qwen2.5-7B)")
	cmd.Flags().StringVar(&targetSize, "target-size", "25g", "Max GGUF size per candidate (default: 25g)")
	cmd.Flags().StringVar(&preferStr, "prefer", "", "Preferred quants (comma-separated, e.g. iq4_nl,q4_k_m)")
	cmd.Flags().StringVar(&uploadersStr, "uploaders", "", "Restrict to specific uploaders (default: trusted set)")
	cmd.Flags().StringVar(&emit, "emit", "harness-input", "Emission shape: harness-input (default) or add-model-script")
	cmd.Flags().StringVar(&downloadRoot, "download-root", "", "Local download root for emitted paths (e.g. ~/.local/models). When unset, paths are HF ids the harness resolves itself.")
	return cmd
}

// evalCandidateKey is the harness models.json key for a (repo, quant) pair.
// Slug-safe: lowercase, hyphenated, no slashes. Stable across runs so re-eval
// against the same candidate keeps the historical row.
func evalCandidateKey(hfID, quant string) string {
	slug := strings.ToLower(strings.ReplaceAll(hfID, "/", "-"))
	slug = strings.ReplaceAll(slug, "_", "-")
	q := strings.ToLower(quant)
	q = strings.ReplaceAll(q, "_", "-")
	return slug + "-" + q
}
