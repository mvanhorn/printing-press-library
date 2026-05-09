package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface-pp-cli/internal/hfx"
)

type vsCurrentResponse struct {
	hfx.Envelope
	Candidate          string             `json:"candidate"`
	Agent              string             `json:"agent"`
	ConfigPath         string             `json:"config_path"`
	Current            currentModelInfo   `json:"current"`
	ArchDelta          string             `json:"arch_delta,omitempty"`
	SizeDelta          string             `json:"size_delta,omitempty"`
	LicenseDelta       string             `json:"license_delta,omitempty"`
	BackendUnsupported []string           `json:"backend_unsupported,omitempty"`
	WouldReplace       bool               `json:"would_replace"`
	ReplaceRole        string             `json:"replace_role,omitempty"`
	Verdict            string             `json:"verdict"` // replace|hold|backend-block|info-only
	Explain            string             `json:"explain,omitempty"`
}

type currentModelInfo struct {
	Role    string `json:"role"`
	Model   string `json:"model"`
	Source  string `json:"source"`
}

func newHFVsCurrentCmd(flags *rootFlags) *cobra.Command {
	var configPath, agent string
	cmd := &cobra.Command{
		Use:   "vs-current <id>",
		Short: "Diff a candidate against the model the named agent currently runs.",
		Long: `vs-current reads data/openclaw.json (or --config-path) to find the model
the named agent (--agent, default 'main') currently runs across roles
(primary/fast/long/fallback). Then diffs candidate vs current along
arch, size, license, and backend support.

Verdicts: replace | hold | backend-block | info-only.

Exits 6 cleanly when no openclaw.json is reachable (rather than crash).`,
		Example: `  huggingface-pp-cli vs-current Qwen/Qwen3-MoE-A14B --agent main
  huggingface-pp-cli vs-current Qwen/Qwen3-MoE-A14B --config-path data/openclaw.json --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			candidate := args[0]
			ctx := cmd.Context()

			// Resolve openclaw.json
			path := configPath
			if path == "" {
				cands := []string{"data/openclaw.json", filepath.Join("..", "data", "openclaw.json")}
				if root := hfOpenclawRoot(); root != "" {
					cands = append(cands, filepath.Join(root, "data", "openclaw.json"))
				}
				path = firstExistingPath(cands)
			}
			if path == "" {
				return hfConfigMissing("no data/openclaw.json found (pass --config-path or run from the OpenClaw repo)")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return hfConfigMissing("reading %s: %v", path, err)
			}
			var cfg map[string]any
			if err := json.Unmarshal(data, &cfg); err != nil {
				return hfConfigMissing("parsing %s: %v", path, err)
			}

			currentRole, currentModel := extractAgentModel(cfg, agent)
			if currentModel == "" {
				return hfConfigMissing("agent %q has no resolved primary model in %s", agent, path)
			}

			// Fetch candidate card
			cand, status, err := hfFetchModel(ctx, candidate, hfTokenForRequests())
			if err != nil {
				if status == 404 {
					return hfNotFound("candidate %q not found", candidate)
				}
				if status == 429 {
					return hfRateLimited("rate limited (HTTP 429)")
				}
				return err
			}

			// Best-effort fetch current's card too — if upstream id is parseable as
			// an HF id (slashed). Fail-soft: if the current is a llama-server local
			// path, the diff is informational only.
			var current *hfModel
			if strings.Contains(currentModel, "/") {
				current, _, _ = hfFetchModel(ctx, currentModel, hfTokenForRequests())
			}

			resp := vsCurrentResponse{
				Envelope:    hfx.NewEnvelope("vs-current"),
				Candidate:   candidate,
				Agent:       agent,
				ConfigPath:  path,
				Current:     currentModelInfo{Role: currentRole, Model: currentModel, Source: "openclaw.json"},
				Verdict:     "info-only",
			}

			// License delta
			candLic := cand.CardData.License
			currLic := ""
			if current != nil {
				currLic = current.CardData.License
			}
			if candLic != "" && currLic != "" && !strings.EqualFold(candLic, currLic) {
				resp.LicenseDelta = fmt.Sprintf("%s → %s", currLic, candLic)
			}

			// Size delta (effective GGUF if both have it)
			candSize := candidateGGUFSize(cand)
			currSize := int64(0)
			if current != nil {
				currSize = candidateGGUFSize(current)
			}
			if candSize > 0 && currSize > 0 {
				resp.SizeDelta = fmt.Sprintf("%s → %s", hfHumanGB(currSize), hfHumanGB(candSize))
			}

			// Arch delta
			candArch := configArchitectures(cand.Config)
			currArch := []string{}
			if current != nil {
				currArch = configArchitectures(current.Config)
			}
			if len(candArch) > 0 && len(currArch) > 0 && strings.Join(candArch, ",") != strings.Join(currArch, ",") {
				resp.ArchDelta = fmt.Sprintf("%s → %s", strings.Join(currArch, ","), strings.Join(candArch, ","))
			} else if len(candArch) > 0 && len(currArch) == 0 {
				resp.ArchDelta = "current arch unknown (local model); candidate: " + strings.Join(candArch, ",")
			}

			// Backend support check using bundled matrix
			cfgParsed, _ := decodeConfigFromMap(cand.Config)
			features := []string{}
			for _, f := range []string{"moe", "mtp", "mla", "gqa", "sliding-window-attn", "rope-yarn"} {
				if hfx.ClassifyFeature(f, cfgParsed, "").Detected {
					features = append(features, f)
				}
			}
			stateDir, _ := hfx.StateDir(flags.stateDir)
			matrix, _, _ := hfx.LoadBackendMatrix(stateDir, flags.backendSupport)
			for _, f := range features {
				for _, backend := range []string{"llama.cpp/turboquant", "mlx"} {
					if entry, ok := hfx.LookupVerdict(matrix, f, backend); ok && strings.EqualFold(entry.Supported, "no") {
						resp.BackendUnsupported = append(resp.BackendUnsupported, backend+":"+f)
					}
				}
			}

			// Verdict synthesis
			if len(resp.BackendUnsupported) > 0 {
				resp.Verdict = "backend-block"
			} else if resp.ArchDelta != "" || resp.SizeDelta != "" {
				resp.WouldReplace = true
				resp.ReplaceRole = currentRole
				resp.Verdict = "replace"
			} else {
				resp.Verdict = "hold"
			}

			if flags.explain {
				resp.Explain = explainVsCurrent(resp)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "vs-current  agent=%s  config=%s\n", agent, path)
			fmt.Fprintf(cmd.OutOrStdout(), "  Current:    %s = %s\n", resp.Current.Role, resp.Current.Model)
			fmt.Fprintf(cmd.OutOrStdout(), "  Candidate:  %s\n", resp.Candidate)
			if resp.ArchDelta != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Arch:       %s\n", resp.ArchDelta)
			}
			if resp.SizeDelta != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Size:       %s\n", resp.SizeDelta)
			}
			if resp.LicenseDelta != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  License:    %s\n", resp.LicenseDelta)
			}
			if len(resp.BackendUnsupported) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Blocking:   %s\n", strings.Join(resp.BackendUnsupported, ", "))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Verdict:    %s\n", resp.Verdict)
			if resp.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", resp.Explain)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&configPath, "config-path", "", "Path to data/openclaw.json (default: auto-discover)")
	cmd.Flags().StringVar(&agent, "agent", "main", "Agent whose model to diff against (default: main)")
	return cmd
}

// extractAgentModel walks openclaw.json to find the agent's primary model.
// Real structure (verified 2026-05-09):
//   agents.list is an []object, each with id, default(bool), model.primary, escalation, ...
//   agents.defaults.model.primary is the inheritance target when an agent's
//     own .model is absent and .default == true.
//
// Resolution order:
//  1. agents.list[id==agentID].model.primary
//  2. agents.list[id==agentID].default==true → agents.defaults.model.primary
//  3. agents.defaults.model.primary (no agent match — fall through)
func extractAgentModel(cfg map[string]any, agentID string) (role, model string) {
	agents, _ := cfg["agents"].(map[string]any)
	if agents == nil {
		return "", ""
	}

	// agents.list is an array; iterate to find matching id.
	var matched map[string]any
	if list, ok := agents["list"].([]any); ok {
		for _, item := range list {
			if a, ok := item.(map[string]any); ok {
				if id, _ := a["id"].(string); id == agentID {
					matched = a
					break
				}
			}
		}
	}

	// 1. own model.primary
	if matched != nil {
		if m, ok := matched["model"].(map[string]any); ok {
			if p, ok := m["primary"].(string); ok && p != "" {
				return "primary", p
			}
		}
		// 2. default==true → defaults.model.primary
		if def, _ := matched["default"].(bool); def {
			if defaults, ok := agents["defaults"].(map[string]any); ok {
				if m, ok := defaults["model"].(map[string]any); ok {
					if p, ok := m["primary"].(string); ok && p != "" {
						return "default-inherited", p
					}
				}
			}
		}
	}

	// 3. defaults.model.primary as last resort
	if defaults, ok := agents["defaults"].(map[string]any); ok {
		if m, ok := defaults["model"].(map[string]any); ok {
			if p, ok := m["primary"].(string); ok && p != "" {
				return "default", p
			}
		}
	}
	return "", ""
}

func candidateGGUFSize(m *hfModel) int64 {
	if m == nil {
		return 0
	}
	if g := m.GGUF; g != nil {
		if total, ok := g["total"].(float64); ok && total > 0 {
			return int64(total)
		}
	}
	var total int64
	for _, s := range m.Siblings {
		if hfx.IsGGUF(s.Path) {
			size := s.Size
			if size == 0 && s.LFS != nil {
				size = s.LFS.Size
			}
			total += size
		}
	}
	return total
}

func configArchitectures(cfg map[string]any) []string {
	if cfg == nil {
		return nil
	}
	archs, _ := cfg["architectures"].([]any)
	out := []string{}
	for _, a := range archs {
		if s, ok := a.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func explainVsCurrent(r vsCurrentResponse) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("explain: comparing %s against %s='%s' from %s. ",
		r.Candidate, r.Current.Role, r.Current.Model, r.ConfigPath))
	switch r.Verdict {
	case "backend-block":
		b.WriteString(fmt.Sprintf("Verdict BLOCKED: %s. Resolve before downloading. ", strings.Join(r.BackendUnsupported, ", ")))
	case "replace":
		b.WriteString("Verdict REPLACE: candidate diverges on arch/size; pilot before flipping configs. ")
	case "hold":
		b.WriteString("Verdict HOLD: no material delta surfaced; bench-history may say more. ")
	}
	return strings.TrimSpace(b.String())
}
