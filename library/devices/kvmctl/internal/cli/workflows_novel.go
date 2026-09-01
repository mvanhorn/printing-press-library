// PATCH(library): expose repository-backed workflow lifecycle commands.
package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/sequence"
	"github.com/spf13/cobra"
)

// pp:data-source local
func init() { registerNovelCommand(registerWorkflowRepositoryCommands) }

func registerWorkflowRepositoryCommands(root *cobra.Command, flags *rootFlags) {
	addNovelCommandIfAbsent(root, newWorkflowListNovelCmd(flags))
	addNovelCommandIfAbsent(root, newWorkflowInspectNovelCmd(flags))
	addNovelCommandIfAbsent(root, newWorkflowAuthorizeNovelCmd(flags))
	addNovelCommandIfAbsent(root, newWorkflowExecuteNovelCmd(flags))
}

func workflowRepository(cmd *cobra.Command, path string) (sequence.Repository, error) {
	if strings.TrimSpace(path) == "" {
		return sequence.Repository{}, usageErr(fmt.Errorf("--repository is required"))
	}
	return sequence.LoadWorkflowRepository(path)
}

func workflowName(args []string, flag string) (string, error) {
	if flag != "" && len(args) > 0 {
		return "", usageErr(fmt.Errorf("workflow name provided twice"))
	}
	if flag != "" {
		return flag, nil
	}
	if len(args) != 1 {
		return "", usageErr(fmt.Errorf("workflow name is required"))
	}
	return args[0], nil
}

func newWorkflowListNovelCmd(flags *rootFlags) *cobra.Command {
	var repository string
	cmd := &cobra.Command{Use: "workflow-list", Short: "List repository workflows in deterministic order", Args: cobra.MaximumNArgs(0), Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		r, err := workflowRepository(cmd, repository)
		if err != nil {
			return err
		}
		items := make([]map[string]any, 0, len(r.List()))
		for _, d := range r.List() {
			items = append(items, map[string]any{"name": d.Name, "revision": d.Revision, "target": d.Target, "target_independent": d.TargetIndependent})
		}
		return kvmdJSON(flags, cmd, map[string]any{"workflows": items})
	}}
	cmd.Flags().StringVar(&repository, "repository", "", "workflow repository JSON path")
	return cmd
}

func newWorkflowInspectNovelCmd(flags *rootFlags) *cobra.Command {
	var repository, nameFlag, revision, target string
	cmd := &cobra.Command{Use: "workflow-inspect [name]", Short: "Inspect a workflow with action values redacted", Args: cobra.MaximumNArgs(1), Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		name, err := workflowName(args, nameFlag)
		if err != nil {
			return err
		}
		r, err := workflowRepository(cmd, repository)
		if err != nil {
			return err
		}
		data, err := r.Inspect(name, revision, target)
		if err != nil {
			return err
		}
		return kvmdJSON(flags, cmd, map[string]any{"workflow": rawJSON(data)})
	}}
	cmd.Flags().StringVar(&repository, "repository", "", "workflow repository JSON path")
	cmd.Flags().StringVar(&nameFlag, "name", "", "workflow name")
	cmd.Flags().StringVar(&revision, "revision", "", "expected immutable revision")
	cmd.Flags().StringVar(&target, "target", "", "target identity")
	return cmd
}

func newWorkflowAuthorizeNovelCmd(flags *rootFlags) *cobra.Command {
	var repository, nameFlag, target, authStore string
	var revision string
	var ttl time.Duration
	var yes bool
	cmd := &cobra.Command{Use: "workflow-authorize [name]", Short: "Authorize one workflow execution and return a one-time token", Args: cobra.MaximumNArgs(1), Annotations: map[string]string{"mcp:local-write": "true", "pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		name, err := workflowName(args, nameFlag)
		if err != nil {
			return err
		}
		if !yes {
			return fmt.Errorf("physical execution authorization requires --yes")
		}
		if strings.TrimSpace(target) == "" {
			return usageErr(fmt.Errorf("--target is required"))
		}
		r, err := workflowRepository(cmd, repository)
		if err != nil {
			return err
		}
		list := r.List()
		var found *sequence.WorkflowDefinition
		for i := range list {
			if list[i].Name == name {
				found = &list[i]
				break
			}
		}
		if found == nil {
			return fmt.Errorf("unknown workflow")
		}
		if revision == "" {
			revision = found.Revision
		}
		plan, err := r.Resolve(name, revision, target)
		if err != nil {
			return err
		}
		if flags.dryRun {
			return kvmdJSON(flags, cmd, map[string]any{"dry_run": true, "authorized": false, "name": name, "revision": revision, "target": target})
		}
		if authStore == "" {
			authStore = filepath.Join(filepath.Dir(repository), ".workflow-authorizations.json")
		}
		token, err := sequence.NewAuthorizer(sequence.NewStore(authStore), nil).Authorize(plan, target, yes, ttl)
		if err != nil {
			return err
		}
		return kvmdJSON(flags, cmd, map[string]any{"authorized": true, "name": name, "revision": revision, "target": target, "token": token, "expires_in_ms": ttl.Milliseconds()})
	}}
	cmd.Flags().StringVar(&repository, "repository", "", "workflow repository JSON path")
	cmd.Flags().StringVar(&nameFlag, "name", "", "workflow name")
	cmd.Flags().StringVar(&revision, "revision", "", "expected immutable revision")
	cmd.Flags().StringVar(&target, "target", "", "target identity")
	cmd.Flags().DurationVar(&ttl, "ttl", 10*time.Second, "authorization lifetime (max 30s)")
	cmd.Flags().BoolVar(&yes, "yes", false, "explicitly authorize physical execution")
	cmd.Flags().StringVar(&authStore, "auth-store", "", "authorization store path")
	return cmd
}

func newWorkflowExecuteNovelCmd(flags *rootFlags) *cobra.Command {
	var repository, nameFlag, revision, target, token, authStore, journal string
	var yes bool
	cmd := &cobra.Command{Use: "workflow-execute [name]", Short: "Execute an authorized workflow on KVMD", Args: cobra.MaximumNArgs(1), Annotations: map[string]string{"mcp:destructive": "true", "pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		name, err := workflowName(args, nameFlag)
		if err != nil {
			return err
		}
		if strings.TrimSpace(revision) == "" {
			return usageErr(fmt.Errorf("--revision is required"))
		}
		if strings.TrimSpace(token) == "" {
			return usageErr(fmt.Errorf("--token is required"))
		}
		if !yes {
			return fmt.Errorf("physical execution requires --yes")
		}
		if strings.TrimSpace(target) == "" {
			return usageErr(fmt.Errorf("--target is required"))
		}
		r, err := workflowRepository(cmd, repository)
		if err != nil {
			return err
		}
		plan, err := r.Resolve(name, revision, target)
		if err != nil {
			return err
		}
		if flags.dryRun {
			return kvmdJSON(flags, cmd, map[string]any{"dry_run": true, "executed": false, "name": name, "revision": revision, "target": target, "actions": len(plan.Actions)})
		}
		if authStore == "" {
			authStore = filepath.Join(filepath.Dir(repository), ".workflow-authorizations.json")
		}
		if journal == "" {
			journal = filepath.Join(filepath.Dir(repository), ".workflow-journal.jsonl")
		}
		c, err := kvmdClient(flags)
		if err != nil {
			return err
		}
		if err := sequence.ExecuteAuthorized(context.Background(), sequence.NewAuthorizer(sequence.NewStore(authStore), nil), sequence.NewExecutor(), sequence.NewKVMDDevice(c), token, target, plan, sequence.NewJournal(journal)); err != nil {
			return err
		}
		return kvmdJSON(flags, cmd, map[string]any{"executed": true, "name": name, "revision": revision, "target": target, "actions": len(plan.Actions)})
	}}
	cmd.Flags().StringVar(&repository, "repository", "", "workflow repository JSON path")
	cmd.Flags().StringVar(&nameFlag, "name", "", "workflow name")
	cmd.Flags().StringVar(&revision, "revision", "", "immutable workflow revision")
	cmd.Flags().StringVar(&target, "target", "", "target identity")
	cmd.Flags().StringVar(&token, "token", "", "one-time authorization token")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm physical execution")
	cmd.Flags().StringVar(&authStore, "auth-store", "", "authorization store path")
	cmd.Flags().StringVar(&journal, "journal", "", "redacted JSONL journal path")
	return cmd
}
