package cli

// pp:novel-static-reference

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type htmlAuditResult struct {
	Path           string         `json:"path"`
	HasInstall     bool           `json:"has_install"`
	FoundProjectID string         `json:"found_project_id,omitempty"`
	Calls          map[string]int `json:"calls"`
	MaskCount      int            `json:"mask_count"`
	UnmaskCount    int            `json:"unmask_count"`
	Warnings       []string       `json:"warnings,omitempty"`
}

func newAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit local Microsoft Clarity instrumentation",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}
	cmd.AddCommand(newAuditHTMLCmd(flags))
	return cmd
}

func newAuditHTMLCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "html <file>",
		Short: "Check an HTML file for Clarity install and client API calls",
		Example: `  clarity-pp-cli audit html ./index.html
  clarity-pp-cli audit html ./index.html --json --select found_project_id,calls`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			result, err := auditHTMLFile(args[0])
			if err != nil {
				return err
			}
			if shouldPrintStructured(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			return printHTMLAudit(cmd, result)
		},
	}
	return cmd
}

func auditHTMLFile(path string) (htmlAuditResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return htmlAuditResult{}, notFoundErr(fmt.Errorf("reading %s: %w", path, err))
	}
	result := auditHTML(path, string(data))
	return result, nil
}

func auditHTML(path, body string) htmlAuditResult {
	result := htmlAuditResult{
		Path:  path,
		Calls: map[string]int{},
	}

	tagRe := regexp.MustCompile(`(?i)clarity\.ms/tag/([^"'\s<>]+)`)
	if match := tagRe.FindStringSubmatch(body); len(match) == 2 {
		result.HasInstall = true
		result.FoundProjectID = strings.TrimRight(match[1], `/;`)
	}

	apiRe := regexp.MustCompile(`(?i)window\.clarity\s*\(\s*['"]([^'"]+)['"]`)
	for _, match := range apiRe.FindAllStringSubmatch(body, -1) {
		if len(match) != 2 {
			continue
		}
		result.Calls[strings.ToLower(match[1])]++
	}

	result.MaskCount = strings.Count(body, "data-clarity-mask")
	result.UnmaskCount = strings.Count(body, "data-clarity-unmask")
	if !result.HasInstall {
		result.Warnings = append(result.Warnings, "no Clarity tag script found")
	}
	if result.MaskCount > 0 && result.UnmaskCount > 0 {
		result.Warnings = append(result.Warnings, "file contains both mask and unmask attributes; confirm the intended element scope")
	}
	return result
}

func printHTMLAudit(cmd *cobra.Command, result htmlAuditResult) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Install: %t\n", result.HasInstall)
	if result.FoundProjectID != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Project ID: %s\n", result.FoundProjectID)
	}
	if len(result.Calls) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Client calls:")
		for _, name := range []string{"consent", "identify", "set", "event", "upgrade"} {
			if count := result.Calls[name]; count > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %d\n", name, count)
			}
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Mask attributes: %d\n", result.MaskCount)
	fmt.Fprintf(cmd.OutOrStdout(), "Unmask attributes: %d\n", result.UnmaskCount)
	for _, warning := range result.Warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
	}
	return nil
}
