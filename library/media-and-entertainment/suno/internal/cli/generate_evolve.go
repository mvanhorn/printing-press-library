// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: Add focused reroll command that mutates one generation axis.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newGenerateEvolveCmd(flags *rootFlags) *cobra.Command {
	var mutate, tagsAdd, tagsRemove, personaID, mv string
	cmd := &cobra.Command{
		Use:     "evolve <clip-id>",
		Short:   "Mutate one axis of an existing clip and re-roll",
		Example: "  suno-pp-cli generate evolve 9baa5d3c-02fb-466d-80f9-a4edfc9f0a65 --mutate tags --tags-add bossa",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			raw, err := readClipRaw(cmd.Context(), args[0])
			if err != nil {
				return notFoundErr(fmt.Errorf("clip %q not found in local store", args[0]))
			}
			obj := unmarshalObject(raw)
			body := map[string]any{}
			for _, key := range []string{"gpt_description_prompt", "prompt", "negative_tags", "title"} {
				if s := stringAtAny(obj, []string{key}, []string{"metadata", key}); s != "" {
					body[key] = s
				}
			}
			tags := clipTags(obj)
			if model := clipModel(obj); model != "" {
				body["mv"] = model
			}
			if pid := clipPersonaID(obj); pid != "" {
				body["persona_id"] = pid
			}
			if boolAtAny(obj, []string{"make_instrumental"}, []string{"metadata", "make_instrumental"}) {
				body["make_instrumental"] = true
			}
			switch mutate {
			case "tags-add":
				tags = append(tags, splitList(tagsAdd)...)
			case "tags-remove":
				remove := map[string]bool{}
				for _, t := range splitList(tagsRemove) {
					remove[strings.ToLower(t)] = true
				}
				var kept []string
				for _, t := range tags {
					if !remove[strings.ToLower(t)] {
						kept = append(kept, t)
					}
				}
				tags = kept
			case "persona":
				if personaID == "" {
					return usageErr(fmt.Errorf("--persona is required for --mutate persona"))
				}
				body["persona_id"] = personaID
			case "model":
				if mv == "" {
					return usageErr(fmt.Errorf("--mv is required for --mutate model"))
				}
				body["mv"] = mv
			case "instrumental-toggle":
				body["make_instrumental"] = !boolAtAny(obj, []string{"make_instrumental"}, []string{"metadata", "make_instrumental"})
			default:
				return usageErr(fmt.Errorf("invalid --mutate %q", mutate))
			}
			if len(tags) > 0 {
				body["tags"] = strings.Join(tags, ", ")
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), body, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// PATCH(greptile #577 P1 round 3): mirror the budget cap check from
			// runGenerateCreate. `generate evolve` reaches the same 10-credit
			// /api/generate/v2-web/ endpoint and must honor the persisted cap.
			if budgetStore, berr := openExistingStore(cmd.Context()); berr == nil && budgetStore != nil {
				capLimit, period, exceeded, eerr := budgetCapExceeded(cmd.Context(), budgetStore)
				budgetStore.Close()
				if eerr == nil && exceeded {
					return fmt.Errorf("budget cap reached: %s cap of %d credits would be exceeded by submitting this generation (10 credits per call). Raise the cap with `suno-pp-cli budget set %s <N>` or clear it with `suno-pp-cli budget clear`", period, capLimit, period)
				}
			}
			data, _, err := c.Post("/api/generate/v2-web/", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&mutate, "mutate", "", "Mutation axis: tags-add, tags-remove, persona, model, instrumental-toggle")
	cmd.Flags().StringVar(&tagsAdd, "tags-add", "", "Comma-separated tags to add")
	cmd.Flags().StringVar(&tagsRemove, "tags-remove", "", "Comma-separated tags to remove")
	cmd.Flags().StringVar(&personaID, "persona", "", "Persona ID for persona mutation")
	cmd.Flags().StringVar(&mv, "mv", "", "Model version for model mutation")
	return cmd
}
