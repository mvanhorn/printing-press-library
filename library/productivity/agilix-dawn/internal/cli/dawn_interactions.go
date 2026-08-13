// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel commands: typed quiz/interaction authoring inside a lesson.
//
// Interactions live at concept.section[].instruction[].interaction[]. Writes use
// the same safe read-modify-write PUT /concept path as the rest of `edit`
// (preview by default, --apply to write, blocked under verify/dogfood).

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// findInstruction locates section -> instruction inside a fetched concept.
func findInstruction(obj map[string]any, sectionID, instructionID string) (map[string]any, error) {
	for _, s := range sectionsOf(obj) {
		sm, ok := asMap(s)
		if !ok || sm["id"] != sectionID {
			continue
		}
		for _, in := range arrOf(sm["instruction"]) {
			im, ok := asMap(in)
			if ok && im["id"] == instructionID {
				return im, nil
			}
		}
		return nil, fmt.Errorf("instruction %s not found in section %s", instructionID, sectionID)
	}
	return nil, fmt.Errorf("section %s not found", sectionID)
}

func newEditInteractionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "interaction",
		Short: "Author quiz interactions inside a lesson (add-choice, add-response, remove)",
		Long: "Author quiz interactions inside a lesson.\n\n" +
			"add-choice builds a multiple-choice question; add-response builds a short-answer\n" +
			"question. For other interaction types (likert, flashcards, sorting, surveys) use\n" +
			"'edit patch'. All commands preview by default; pass --apply to write.",
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newEditInteractionAddChoiceCmd(flags))
	cmd.AddCommand(newEditInteractionAddResponseCmd(flags))
	cmd.AddCommand(newEditInteractionRemoveCmd(flags))
	return cmd
}

// edit interaction add-choice <course-id> <section-id> <instruction-id>
func newEditInteractionAddChoiceCmd(flags *rootFlags) *cobra.Command {
	var body string
	var options []string
	var answers []int
	var points int
	var shuffle, apply bool
	cmd := &cobra.Command{
		Use:     "add-choice <course-id> <section-id> <instruction-id>",
		Short:   "Add a multiple-choice question to a lesson",
		Long:    "Add a multiple-choice question to a lesson.\n\n--answer uses 1-based option numbers (e.g. --answer 2 marks the 2nd --option correct); repeat --answer for multiple correct options.",
		Example: "  agilix-dawn-pp-cli edit interaction add-choice c_216... s_dd8... i_cb8... --body \"2+2=?\" --option 3 --option 4 --option 5 --answer 2 --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would add a multiple-choice question")
				return nil
			}
			if len(args) < 3 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("course-id, section-id and instruction-id are required"))
			}
			if body == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--body (the question text) is required"))
			}
			if len(options) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least two --option values are required"))
			}
			if len(answers) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one --answer (1-based option number) is required"))
			}
			answerIdx := make([]any, 0, len(answers))
			for _, a := range answers {
				if a < 1 || a > len(options) {
					return usageErr(fmt.Errorf("--answer %d is out of range (there are %d options)", a, len(options)))
				}
				answerIdx = append(answerIdx, a-1) // 0-based for the API
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			obj, err := fetchConceptRaw(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			instr, err := findInstruction(obj, args[1], args[2])
			if err != nil {
				return err
			}
			choices := make([]any, 0, len(options))
			for _, o := range options {
				choices = append(choices, map[string]any{"body": o})
			}
			newID := genID("q_")
			inter := map[string]any{
				"id": newID, "type": "choice", "title": "", "body": body,
				"choice": choices,
				"action": []any{map[string]any{"answer": answerIdx, "feedback": "", "points": points}},
				"points": points, "position": 0.0, "status": "enabled",
				"duration": 0.0, "supplemental": []any{},
			}
			if shuffle {
				inter["shuffle"] = true
			}
			instr["interaction"] = append(arrOf(instr["interaction"]), inter)
			summary := fmt.Sprintf("edit interaction add-choice %s/%s/%s: new question %s (%d options, %d correct)",
				args[0], args[1], args[2], newID, len(options), len(answers))
			return previewOrApply(cmd, flags, apply, c, obj, summary)
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "The question text")
	cmd.Flags().StringArrayVar(&options, "option", nil, "An answer option (repeat for each option)")
	cmd.Flags().IntSliceVar(&answers, "answer", nil, "1-based option number(s) that are correct (repeatable)")
	cmd.Flags().IntVar(&points, "points", 1, "Points awarded for a correct answer")
	cmd.Flags().BoolVar(&shuffle, "shuffle", false, "Shuffle the options when shown")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the change (default previews only)")
	return cmd
}

// edit interaction add-response <course-id> <section-id> <instruction-id>
func newEditInteractionAddResponseCmd(flags *rootFlags) *cobra.Command {
	var body, input string
	var answers []string
	var points int
	var coachReview, apply bool
	cmd := &cobra.Command{
		Use:     "add-response <course-id> <section-id> <instruction-id>",
		Short:   "Add a short-answer question to a lesson",
		Long:    "Add a short-answer (free-text) question. Repeat --answer for each accepted answer string. Use --coach-review when a human must grade it.",
		Example: "  agilix-dawn-pp-cli edit interaction add-response c_216... s_dd8... i_cb8... --body \"Capital of Idaho?\" --answer Boise --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would add a short-answer question")
				return nil
			}
			if len(args) < 3 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("course-id, section-id and instruction-id are required"))
			}
			if body == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--body (the question text) is required"))
			}
			if input != "text" && input != "large" {
				return usageErr(fmt.Errorf("--input must be 'text' or 'large'"))
			}
			if len(answers) == 0 && !coachReview {
				return usageErr(fmt.Errorf("provide at least one --answer, or set --coach-review for human grading"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			obj, err := fetchConceptRaw(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			instr, err := findInstruction(obj, args[1], args[2])
			if err != nil {
				return err
			}
			ans := make([]any, 0, len(answers))
			for _, a := range answers {
				ans = append(ans, a)
			}
			newID := genID("q_")
			inter := map[string]any{
				"id": newID, "type": "response", "title": "", "body": body,
				"input":         input,
				"action":        []any{map[string]any{"answer": ans, "feedback": "", "points": points}},
				"awardCriteria": map[string]any{"coachReviewRequired": coachReview},
				"points":        points, "position": 0.0, "status": "enabled",
				"duration": 0.0, "supplemental": []any{},
			}
			instr["interaction"] = append(arrOf(instr["interaction"]), inter)
			summary := fmt.Sprintf("edit interaction add-response %s/%s/%s: new question %s (%d accepted answers, coachReview=%v)",
				args[0], args[1], args[2], newID, len(answers), coachReview)
			return previewOrApply(cmd, flags, apply, c, obj, summary)
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "The question text")
	cmd.Flags().StringArrayVar(&answers, "answer", nil, "An accepted answer (repeat for each)")
	cmd.Flags().StringVar(&input, "input", "text", "Input size: text (short) or large (paragraph)")
	cmd.Flags().IntVar(&points, "points", 1, "Points awarded for a correct answer")
	cmd.Flags().BoolVar(&coachReview, "coach-review", false, "Require a human coach to grade the answer")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the change (default previews only)")
	return cmd
}

// edit interaction remove <course-id> <section-id> <instruction-id> <interaction-id>
func newEditInteractionRemoveCmd(flags *rootFlags) *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:     "remove <course-id> <section-id> <instruction-id> <interaction-id>",
		Short:   "Remove a quiz interaction from a lesson",
		Example: "  agilix-dawn-pp-cli edit interaction remove c_216... s_dd8... i_cb8... q_abc... --apply",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would remove an interaction")
				return nil
			}
			if len(args) < 4 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("course-id, section-id, instruction-id and interaction-id are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			obj, err := fetchConceptRaw(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			instr, err := findInstruction(obj, args[1], args[2])
			if err != nil {
				return err
			}
			existing := arrOf(instr["interaction"])
			kept := make([]any, 0, len(existing))
			found := false
			for _, x := range existing {
				if xm, ok := asMap(x); ok && xm["id"] == args[3] {
					found = true
					continue
				}
				kept = append(kept, x)
			}
			if !found {
				return fmt.Errorf("interaction %s not found in instruction %s", args[3], args[2])
			}
			instr["interaction"] = kept
			summary := fmt.Sprintf("edit interaction remove %s/%s/%s/%s", args[0], args[1], args[2], args[3])
			return previewOrApply(cmd, flags, apply, c, obj, summary)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually write the change (default previews only)")
	return cmd
}
