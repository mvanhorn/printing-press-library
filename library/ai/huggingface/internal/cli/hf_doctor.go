// Stack-aware doctor command per seed plan. Replaces the framework doctor
// at registration time (see root.go: newHFDoctorCmd is registered instead of
// newDoctorCmd). Emits a structured runtime-shape that agents call once at
// session start and branch all subsequent calls on.
package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface/internal/hfx"
)

type doctorReport struct {
	hfx.Envelope
	NoWriteRequested     bool   `json:"no_write_requested"`
	TTY                  bool   `json:"tty"`
	JSONSupported        bool   `json:"json_supported"`
	StateWritable        bool   `json:"state_writable"`
	StateDir             string `json:"state_dir"`
	HasLiveConfig        bool   `json:"has_live_config"`
	LiveConfigPath       string `json:"live_config_path,omitempty"`
	HasHarness           bool   `json:"has_harness"`
	HarnessDir           string `json:"harness_dir,omitempty"`
	BackendMatrixSource  string `json:"backend_matrix_source"`
	BackendMatrixAgeDays int    `json:"backend_matrix_age_days"`
	BackendMatrixWarning string `json:"backend_matrix_warning,omitempty"`
	HFTokenPresent       bool   `json:"hf_token_present"`
	HFTokenSource        string `json:"hf_token_source,omitempty"`
	HFReachable          bool   `json:"hf_reachable"`
	HFLatencyMs          int64  `json:"hf_latency_ms,omitempty"`
	HFError              string `json:"hf_error,omitempty"`
	RateLimitRemaining   int    `json:"rate_limit_remaining"`
	ProxyInUse           bool   `json:"proxy_in_use"`
	ProxyEnv             string `json:"proxy_env,omitempty"`
	Explain              string `json:"explain,omitempty"`
}

func newHFDoctorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Single-call structured runtime probe for agents and humans.",
		Long: `Doctor emits a structured runtime-shape covering TTY, JSON support,
state writability, live-config presence, harness presence, backend-matrix age,
HF reachability + latency, rate-limit remaining, and proxy presence. Agents
call this once at session start and branch all subsequent calls on the result.`,
		Example: `  huggingface-pp-cli doctor --json
  huggingface-pp-cli doctor --json --explain
  huggingface-pp-cli doctor --state-dir /tmp/hf-test`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			rep := doctorReport{Envelope: hfx.NewEnvelope("doctor")}

			// Runtime self-report
			rep.NoWriteRequested = flags.noWrite
			rep.TTY = isTerminal(os.Stdout)
			rep.JSONSupported = true

			// State dir + writability
			stateDir, _ := hfx.StateDir(flags.stateDir)
			rep.StateDir = stateDir
			rep.StateWritable = stateDirWritable(stateDir)

			// Live config (data/openclaw.json)
			cfgCandidates := []string{"data/openclaw.json", filepath.Join("..", "data", "openclaw.json")}
			if root := hfOpenclawRoot(); root != "" {
				cfgCandidates = append(cfgCandidates, filepath.Join(root, "data", "openclaw.json"))
			}
			cfgPath := firstExistingPath(cfgCandidates)
			if cfgPath != "" {
				rep.HasLiveConfig = true
				rep.LiveConfigPath = cfgPath
			}

			// Harness dir
			harnessCandidates := []string{"workspace/scripts/model-eval-harness"}
			if root := hfOpenclawRoot(); root != "" {
				harnessCandidates = append(harnessCandidates, filepath.Join(root, "workspace", "scripts", "model-eval-harness"))
			}
			harness := firstExistingPath(harnessCandidates)
			if harness != "" {
				rep.HasHarness = true
				rep.HarnessDir = harness
			}

			// Backend matrix
			matrix, src, err := hfx.LoadBackendMatrix(stateDir, flags.backendSupport)
			if err == nil {
				rep.BackendMatrixSource = src
				rep.BackendMatrixAgeDays = hfx.MatrixAgeDays(matrix)
				if rep.BackendMatrixAgeDays > 90 {
					rep.BackendMatrixWarning = fmt.Sprintf("oldest source_checked is %d days old; refresh entries", rep.BackendMatrixAgeDays)
				}
			}

			// HF token
			if v, src := resolveHFToken(); v != "" {
				rep.HFTokenPresent = true
				rep.HFTokenSource = src
			}

			// HF reachability — single cheap probe
			start := time.Now()
			httpClient := &http.Client{Timeout: 5 * time.Second}
			req, _ := http.NewRequest("GET", "https://huggingface.co/api/models?limit=1", nil)
			if rep.HFTokenPresent {
				if v, _ := resolveHFToken(); v != "" {
					req.Header.Set("Authorization", "Bearer "+v)
				}
			}
			req.Header.Set("User-Agent", "huggingface-pp-cli/0.1.0")
			resp, err := httpClient.Do(req)
			rep.HFLatencyMs = time.Since(start).Milliseconds()
			if err != nil {
				rep.HFError = err.Error()
			} else {
				defer resp.Body.Close()
				rep.HFReachable = resp.StatusCode >= 200 && resp.StatusCode < 400
				if !rep.HFReachable {
					rep.HFError = fmt.Sprintf("HTTP %d", resp.StatusCode)
				}
				// Rate-limit snapshot
				rl, reset, _ := hfx.ParseRateLimitHeader(resp.Header)
				rep.RateLimitRemaining = rl
				_ = reset
			}

			// Proxy
			for _, env := range []string{"HTTPS_PROXY", "HTTP_PROXY", "https_proxy", "http_proxy"} {
				if v := os.Getenv(env); v != "" {
					rep.ProxyInUse = true
					rep.ProxyEnv = env + "=" + v
					break
				}
			}

			if flags.explain {
				rep.Explain = explainDoctor(rep)
			}

			// JSON path
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
			}

			// Human path
			fmt.Fprintf(cmd.OutOrStdout(), "huggingface-pp-cli doctor (as_of %s)\n\n", rep.AsOf)
			fmt.Fprintf(cmd.OutOrStdout(), "  Runtime:    tty=%v  json=%v  state_writable=%v\n", rep.TTY, rep.JSONSupported, rep.StateWritable)
			fmt.Fprintf(cmd.OutOrStdout(), "  State dir:  %s\n", rep.StateDir)
			fmt.Fprintf(cmd.OutOrStdout(), "  Live cfg:   %v %s\n", rep.HasLiveConfig, rep.LiveConfigPath)
			fmt.Fprintf(cmd.OutOrStdout(), "  Harness:    %v %s\n", rep.HasHarness, rep.HarnessDir)
			fmt.Fprintf(cmd.OutOrStdout(), "  Matrix:     source=%s  age=%dd\n", rep.BackendMatrixSource, rep.BackendMatrixAgeDays)
			if rep.BackendMatrixWarning != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "              WARNING: %s\n", rep.BackendMatrixWarning)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  HF token:   present=%v source=%s\n", rep.HFTokenPresent, rep.HFTokenSource)
			fmt.Fprintf(cmd.OutOrStdout(), "  HF probe:   reachable=%v latency=%dms err=%s\n", rep.HFReachable, rep.HFLatencyMs, rep.HFError)
			fmt.Fprintf(cmd.OutOrStdout(), "  Rate limit: remaining=%d\n", rep.RateLimitRemaining)
			fmt.Fprintf(cmd.OutOrStdout(), "  Proxy:      in_use=%v %s\n", rep.ProxyInUse, rep.ProxyEnv)
			if rep.Explain != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", rep.Explain)
			}
			return nil
		},
	}
	return cmd
}

// stateDirWritable probes whether the state dir is writable WITHOUT creating it.
// If the dir does not exist, probes the parent. If --no-write is set this still
// reports honestly (we want agents to know the runtime can write, even if this
// invocation won't).
func stateDirWritable(dir string) bool {
	if dir == "" {
		return false
	}
	probe := dir
	for i := 0; i < 4; i++ {
		if fi, err := os.Stat(probe); err == nil && fi.IsDir() {
			// Try to create + remove a sentinel
			tmp := filepath.Join(probe, ".hfx-writable-probe")
			f, err := os.Create(tmp)
			if err != nil {
				return false
			}
			_ = f.Close()
			_ = os.Remove(tmp)
			return true
		}
		probe = filepath.Dir(probe)
		if probe == "/" || probe == "." {
			return false
		}
	}
	return false
}

// resolveHFToken honors HF_TOKEN, HUGGING_FACE_HUB_TOKEN, then the cached file
// at ~/.cache/huggingface/token (the path huggingface_hub uses).
func resolveHFToken() (string, string) {
	if v := os.Getenv("HF_TOKEN"); v != "" {
		return strings.TrimSpace(v), "env:HF_TOKEN"
	}
	if v := os.Getenv("HUGGING_FACE_HUB_TOKEN"); v != "" {
		return strings.TrimSpace(v), "env:HUGGING_FACE_HUB_TOKEN"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ""
	}
	cached := filepath.Join(home, ".cache", "huggingface", "token")
	if data, err := os.ReadFile(cached); err == nil && len(data) > 0 {
		return strings.TrimSpace(string(data)), "cache:~/.cache/huggingface/token"
	}
	return "", ""
}

func firstExistingPath(candidates []string) string {
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	return ""
}

func explainDoctor(rep doctorReport) string {
	var b strings.Builder
	b.WriteString("explain: ")
	if !rep.HFReachable {
		b.WriteString(fmt.Sprintf("HF unreachable (%s); commands that hit the API will exit 5 (rate limit) or 1 (transport). ", rep.HFError))
	} else {
		b.WriteString(fmt.Sprintf("HF reachable (%dms latency, %d req remaining in window). ", rep.HFLatencyMs, rep.RateLimitRemaining))
	}
	if !rep.HasLiveConfig {
		b.WriteString("No data/openclaw.json found; vs-current will exit 6. ")
	} else {
		b.WriteString("Live openclaw.json present; vs-current is operational. ")
	}
	if !rep.HasHarness {
		b.WriteString("No harness dir found; bench-history will exit 6. ")
	}
	if rep.BackendMatrixAgeDays > 90 {
		b.WriteString(fmt.Sprintf("Backend matrix is %d days stale; backend-check verdicts may be out of date. ", rep.BackendMatrixAgeDays))
	}
	if !rep.StateWritable {
		b.WriteString("State dir is not writable; --no-write is implicit. ")
	}
	return strings.TrimSpace(b.String())
}
