// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type habiticaChore struct {
	Text     string   `yaml:"text" json:"text"`
	Notes    string   `yaml:"notes" json:"notes,omitempty"`
	Type     string   `yaml:"type" json:"type,omitempty"`
	Priority float64  `yaml:"priority" json:"priority,omitempty"`
	Date     string   `yaml:"date" json:"date,omitempty"`
	Tags     []string `yaml:"tags" json:"tags,omitempty"`
}

func readHabiticaChores(path string) ([]habiticaChore, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading chore plan: %w", err)
	}
	var chores []habiticaChore
	if err := yaml.Unmarshal(contents, &chores); err == nil && len(chores) > 0 {
		return chores, nil
	}
	var document struct {
		Chores []habiticaChore `yaml:"chores"`
	}
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return nil, fmt.Errorf("parsing chore plan YAML: %w", err)
	}
	if len(document.Chores) == 0 {
		return nil, fmt.Errorf("chore plan must be a non-empty YAML list or contain a chores: list")
	}
	return document.Chores, nil
}

func newNovelPlanChoresCmd(flags *rootFlags) *cobra.Command {
	var flagFile string
	var apply bool
	cmd := &cobra.Command{
		Use:         "chores",
		Short:       "Preview a chore batch as Habitica quests and create it only after an explicit apply confirmation.",
		Example:     "  habitica-pp-cli plan chores --file chores.yaml --dry-run --agent",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(flagFile) == "" {
				return usageErr(errors.New("--file is required"))
			}
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"file": flagFile, "apply": apply, "action": "would parse the chore plan and create one Habitica task per entry"})
			}
			chores, err := readHabiticaChores(flagFile)
			if err != nil {
				return err
			}
			for i := range chores {
				if strings.TrimSpace(chores[i].Text) == "" {
					return usageErr(fmt.Errorf("chore %d needs text", i+1))
				}
				if chores[i].Type == "" {
					chores[i].Type = "todo"
				}
			}
			preview := map[string]any{"file": flagFile, "apply": apply, "chores": chores}
			if !apply {
				return flags.printJSON(cmd, preview)
			}
			if !flags.yes {
				return usageErr(errors.New("creating chores requires --apply --yes"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			headers, err := habiticaHeaders()
			if err != nil {
				return err
			}
			created := make([]any, 0, len(chores))
			for _, chore := range chores {
				result, _, err := c.PostWithHeaders(ctx, "/tasks/user", chore, headers)
				if err != nil {
					return fmt.Errorf("creating %q: %w", chore.Text, err)
				}
				data, err := habiticaData(result)
				if err != nil {
					return err
				}
				created = append(created, data)
			}
			return flags.printJSON(cmd, map[string]any{"created": created, "count": len(created)})
		},
	}
	cmd.Flags().StringVar(&flagFile, "file", "", "YAML chore plan (a list or a chores: list)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Create the previewed chores; also requires --yes")
	return cmd
}
