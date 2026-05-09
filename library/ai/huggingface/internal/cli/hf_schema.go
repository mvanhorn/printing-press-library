package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface-pp-cli/internal/hfx"
)

// commandSchema is the JSON shape returned by `hf schema [<command>]`.
// Each command authored under the seed registers its output schema here.
// Defends against output drift: agents introspect rather than guess.
type commandSchema struct {
	Command       string             `json:"command"`
	Description   string             `json:"description"`
	OutputSchema  map[string]string  `json:"output_schema"`
	ExitCodes     map[string]string  `json:"exit_codes"`
	Stable        bool               `json:"stable"`
	SchemaVersion int                `json:"schema_version"`
	StateFiles    []string           `json:"state_files,omitempty"`
}

type schemaResponse struct {
	hfx.Envelope
	Commands []commandSchema `json:"commands"`
}

// hfCommandSchemas is the registry of stable schemas, keyed by command name.
// Adding a new command's schema here is the one place to do it; `hf schema`
// dumps from this registry. Order is the documentation order.
var hfCommandSchemas = []commandSchema{
	{
		Command:     "doctor",
		Description: "Single-call structured runtime probe.",
		OutputSchema: map[string]string{
			"tty":                     "bool",
			"json_supported":          "bool",
			"state_writable":          "bool",
			"state_dir":               "string",
			"has_live_config":         "bool",
			"live_config_path":        "string",
			"has_harness":             "bool",
			"harness_dir":             "string",
			"backend_matrix_source":   "string (bundled|override:<path>)",
			"backend_matrix_age_days": "int",
			"hf_token_present":        "bool",
			"hf_reachable":            "bool",
			"hf_latency_ms":           "int64",
			"rate_limit_remaining":    "int (-1 if unknown)",
			"proxy_in_use":            "bool",
			"explain":                 "string (only when --explain)",
		},
		ExitCodes: map[string]string{
			"0": "ok (probe completed; check fields for any 'no' verdicts)",
			"1": "transport error (cannot reach huggingface.co at all)",
		},
		Stable:        true,
		SchemaVersion: hfx.SchemaVersion,
	},
	{
		Command:     "schema",
		Description: "Dump JSON output schema for one or all commands.",
		OutputSchema: map[string]string{
			"commands": "array of {command, description, output_schema, exit_codes, stable}",
		},
		ExitCodes: map[string]string{
			"0": "ok",
			"2": "command not found",
		},
		Stable:        true,
		SchemaVersion: hfx.SchemaVersion,
	},
	{
		Command:     "model-card",
		Description: "Stack-relevant model card: MoE active params, effective GGUF size, training-data summary.",
		OutputSchema: map[string]string{
			"id":                  "string",
			"author":              "string",
			"library_name":        "string",
			"pipeline_tag":        "string",
			"license":             "string",
			"downloads":           "int",
			"likes":               "int",
			"last_modified":       "string (RFC3339)",
			"gated":               "string|bool",
			"private":             "bool",
			"tags":                "[]string",
			"base_model":          "string",
			"context_length":      "int",
			"total_params":        "int (estimate; 0 if unknown)",
			"moe_total_experts":   "int (0 if not MoE)",
			"moe_active_per_tok":  "int (0 if not MoE)",
			"effective_gguf_size": "string (e.g. 18.4 GB; empty if no GGUF siblings)",
			"siblings":            "array of {path, size_bytes, quant}",
		},
		ExitCodes: map[string]string{
			"0": "ok",
			"2": "model id not found",
			"5": "rate-limited",
		},
		Stable:        true,
		SchemaVersion: hfx.SchemaVersion,
	},
	{
		Command:     "find-quants",
		Description: "Sorted GGUF quant variants of a base model with uploader rep + size filter.",
		OutputSchema: map[string]string{
			"base_model": "string",
			"results":    "array of {id, uploader, uploader_rep, quant, quant_family, size_bytes, last_modified}",
		},
		ExitCodes: map[string]string{
			"0": "ok",
			"2": "no quants found for base model",
			"5": "rate-limited",
		},
		Stable:        true,
		SchemaVersion: hfx.SchemaVersion,
	},
	{
		Command:     "find-feature",
		Description: "Search models by architecture feature (mtp/mla/moe/gqa/sliding-window/rope-yarn) with backend-readiness verdict.",
		OutputSchema: map[string]string{
			"feature": "string",
			"results": "array of {id, detected, variant, evidence, confidence, backend_verdicts}",
		},
		ExitCodes: map[string]string{
			"0": "ok",
			"2": "no candidates matched",
			"5": "rate-limited",
		},
		Stable:        true,
		SchemaVersion: hfx.SchemaVersion,
	},
	{
		Command:     "vs-current",
		Description: "Diff candidate against the model the named agent currently runs (reads data/openclaw.json).",
		OutputSchema: map[string]string{
			"candidate":         "string (HF id)",
			"agent":             "string",
			"current":           "object {role, model, source}",
			"arch_delta":        "string",
			"size_delta":        "string",
			"license_delta":     "string",
			"would_replace":     "bool",
			"replace_role":      "string",
			"backend_unsupported": "[]string",
			"verdict":           "string (replace|hold|backend-block|info-only)",
		},
		ExitCodes: map[string]string{
			"0": "ok",
			"2": "candidate model not found",
			"6": "data/openclaw.json missing or unparseable",
		},
		Stable:        true,
		SchemaVersion: hfx.SchemaVersion,
	},
	{
		Command:     "backend-check",
		Description: "Backend-readiness verdict for a model's architecture.",
		OutputSchema: map[string]string{
			"id":        "string",
			"backends":  "array of {backend, verdicts: [{feature, supported, since, source, source_checked, notes, wiki_pointer}]}",
			"summary":   "string (overall verdict)",
		},
		ExitCodes: map[string]string{
			"0": "ok",
			"2": "model not found",
			"3": "all requested backends unsupported for this arch",
			"5": "rate-limited",
		},
		Stable:        true,
		SchemaVersion: hfx.SchemaVersion,
	},
}

func newHFSchemaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema [command]",
		Short: "Dump JSON output schema for one or all commands.",
		Long: `schema returns a stable JSON description of every command's output shape and
exit codes. Agents introspect this rather than guess against output drift.`,
		Example: `  huggingface-pp-cli schema
  huggingface-pp-cli schema doctor
  huggingface-pp-cli schema --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp := schemaResponse{Envelope: hfx.NewEnvelope("schema")}
			if len(args) == 0 {
				resp.Commands = hfCommandSchemas
			} else {
				want := strings.ToLower(args[0])
				for _, s := range hfCommandSchemas {
					if s.Command == want {
						resp.Commands = []commandSchema{s}
						break
					}
				}
				if len(resp.Commands) == 0 {
					return hfNotFound("no schema for command %q (try one of: doctor, schema, model-card, find-quants, find-feature, vs-current, backend-check)", args[0])
				}
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			// Human path: terse table
			for _, s := range resp.Commands {
				fmt.Fprintf(cmd.OutOrStdout(), "%s — %s (schema_version=%d, stable=%v)\n", s.Command, s.Description, s.SchemaVersion, s.Stable)
				fmt.Fprintln(cmd.OutOrStdout(), "  output_schema:")
				for k, v := range s.OutputSchema {
					fmt.Fprintf(cmd.OutOrStdout(), "    %-25s %s\n", k+":", v)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "  exit_codes:")
				for k, v := range s.ExitCodes {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s = %s\n", k, v)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	return cmd
}
