// PATCH(library): expose executable target-bound sequences through novel hooks.
package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/config"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/sequence"
	"github.com/spf13/cobra"
)

// pp:data-source local
func init() { registerNovelCommand(registerSequenceCommands) }

func registerSequenceCommands(root *cobra.Command, flags *rootFlags) {
	parent := &cobra.Command{Use: "sequence", Short: "Validate and execute target-bound KVM sequences", Annotations: map[string]string{"pp:novel": "true"}}
	var file string
	validate := &cobra.Command{Use: "validate", Short: "Validate a sequence document without executing it", Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if file == "" {
			return usageErr(fmt.Errorf("--file is required"))
		}
		p, err := sequence.ReadDocument(file)
		if err != nil {
			return err
		}
		h, err := p.Hash()
		if err != nil {
			return err
		}
		return kvmdJSON(flags, cmd, map[string]any{"valid": true, "target": p.Target, "actions": len(p.Actions), "plan_hash": h})
	}}
	validate.Flags().StringVar(&file, "file", "", "JSON sequence document")
	parent.AddCommand(validate)

	var authFile, authTarget string
	var ttl time.Duration
	var approved bool
	authorize := &cobra.Command{Use: "authorize", Short: "Authorize one target-bound sequence execution", Annotations: map[string]string{"mcp:local-write": "true", "pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if file == "" {
			return usageErr(fmt.Errorf("--file is required"))
		}
		if !approved {
			return fmt.Errorf("physical execution authorization requires --yes")
		}
		p, err := sequence.ReadDocument(file)
		if err != nil {
			return err
		}
		cfg, err := loadSequenceConfig(flags)
		if err != nil {
			return err
		}
		if authFile == "" {
			authFile = filepath.Join(filepath.Dir(cfg.Path), "sequence-auth.json")
		}
		tok, err := sequence.NewAuthorizer(sequence.NewStore(authFile), nil).Authorize(p, authTarget, approved, ttl)
		if err != nil {
			return err
		}
		return kvmdJSON(flags, cmd, map[string]any{"authorized": true, "target": p.Target, "token": tok, "expires_in_ms": ttl.Milliseconds()})
	}}
	authorize.Flags().StringVar(&authTarget, "target", "", "target identity (must equal document target)")
	authorize.Flags().DurationVar(&ttl, "ttl", 10*time.Second, "authorization lifetime (max 30s)")
	authorize.Flags().BoolVar(&approved, "yes", false, "explicitly authorize physical execution")
	authorize.Flags().StringVar(&authFile, "auth-store", "", "authorization store path")
	authorize.Flags().StringVar(&file, "file", "", "JSON sequence document")
	parent.AddCommand(authorize)

	var token, target, journal string
	var yes bool
	run := &cobra.Command{Use: "run", Short: "Execute a previously authorized sequence on the target", Annotations: map[string]string{"mcp:destructive": "true", "pp:novel": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		if file == "" {
			return usageErr(fmt.Errorf("--file is required"))
		}
		if token == "" {
			return usageErr(fmt.Errorf("--token is required"))
		}
		if !yes {
			return fmt.Errorf("physical execution requires --yes")
		}
		p, err := sequence.ReadDocument(file)
		if err != nil {
			return err
		}
		cfg, err := loadSequenceConfig(flags)
		if err != nil {
			return err
		}
		if authFile == "" {
			authFile = filepath.Join(filepath.Dir(cfg.Path), "sequence-auth.json")
		}
		if journal == "" {
			journal = filepath.Join(filepath.Dir(cfg.Path), "sequence-journal.jsonl")
		}
		if target == "" {
			target = p.Target
		}
		c, err := kvmdClient(flags)
		if err != nil {
			return err
		}
		d := sequence.NewKVMDDevice(c)
		j := sequence.NewJournal(journal)
		if err := sequence.ExecuteAuthorized(context.Background(), sequence.NewAuthorizer(sequence.NewStore(authFile), nil), sequence.NewExecutor(), d, token, target, p, j); err != nil {
			return err
		}
		return kvmdJSON(flags, cmd, map[string]any{"executed": true, "target": target, "actions": len(p.Actions)})
	}}
	run.Flags().StringVar(&file, "file", "", "JSON sequence document")
	run.Flags().StringVar(&token, "token", "", "one-time authorization token")
	run.Flags().StringVar(&target, "target", "", "target identity (defaults to document target)")
	run.Flags().StringVar(&authFile, "auth-store", "", "authorization store path")
	run.Flags().StringVar(&journal, "journal", "", "redacted JSONL journal path")
	run.Flags().BoolVar(&yes, "yes", false, "confirm physical execution")
	parent.AddCommand(run)
	addNovelCommandIfAbsent(root, parent)
}

func loadSequenceConfig(flags *rootFlags) (*config.Config, error) {
	return config.Load(flags.configPath)
}
