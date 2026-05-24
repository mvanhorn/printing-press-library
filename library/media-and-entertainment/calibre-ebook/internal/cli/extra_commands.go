package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newFtsEnableCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "enable",
		Short:       "Enable full-text search indexing for this library",
		Example:     "  calibre-ebook-pp-cli fts enable",
		Annotations: map[string]string{"pp:endpoint": "fts.enable", "pp:method": "POST", "pp:path": "/fts/enable"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			out, _, err := c.RunCalibredb("fts_index", "enable")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result := map[string]any{"success": true, "output": string(out)}
			return printResult(cmd, flags, result)
		},
	}
}

func newFtsDisableCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "disable",
		Short:       "Disable full-text search indexing for this library",
		Example:     "  calibre-ebook-pp-cli fts disable",
		Annotations: map[string]string{"pp:endpoint": "fts.disable", "pp:method": "POST", "pp:path": "/fts/disable"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			out, code, err := c.RunCalibredb("fts_index", "disable")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			_ = code
			result := map[string]any{"success": true, "output": string(out)}
			return printResult(cmd, flags, result)
		},
	}
}

func newLibraryCatalogCmd(flags *rootFlags) *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:         "catalog <output_path>",
		Short:       "Generate a catalog of books",
		Example:     "  calibre-ebook-pp-cli library catalog catalog.xml",
		Annotations: map[string]string{"pp:endpoint": "library.catalog", "pp:method": "POST", "pp:path": "/library/catalog"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			out, _, err := c.RunCalibredb("catalog", args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result := map[string]any{"path": args[0], "success": true, "output": string(out)}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&outputPath, "output", "", "Output path for catalog")
	return cmd
}

func newBooksSetCustomCmd(flags *rootFlags) *cobra.Command {
	var column string
	var value string

	cmd := &cobra.Command{
		Use:         "set-custom <id>",
		Short:       "Set a custom column value for a book",
		Example:     "  calibre-ebook-pp-cli books set-custom 42 --column #rating --value 5",
		Annotations: map[string]string{"pp:endpoint": "books.set-custom", "pp:method": "PUT", "pp:path": "/books/{id}/custom"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if column == "" {
				return fmt.Errorf("required flag --column not set")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			out, _, err := c.RunCalibredb("set_custom", args[0], "--column", column, "--value", value)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result := map[string]any{"id": args[0], "column": column, "value": value, "success": true, "output": string(out)}
			return printResult(cmd, flags, result)
		},
	}
	cmd.Flags().StringVar(&column, "column", "", "Custom column name (e.g. #rating)")
	cmd.Flags().StringVar(&value, "value", "", "Value to set")
	return cmd
}

func newLibraryCloneCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "clone <path>",
		Short:       "Clone the current library to a new path",
		Example:     "  calibre-ebook-pp-cli library clone /path/to/new/library",
		Annotations: map[string]string{"pp:endpoint": "library.clone", "pp:method": "POST", "pp:path": "/library/clone"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			out, _, err := c.RunCalibredb("clone", args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result := map[string]any{"path": args[0], "success": true, "output": string(out)}
			return printResult(cmd, flags, result)
		},
	}
	return cmd
}

func printResult(cmd *cobra.Command, flags *rootFlags, result map[string]any) error {
	data, _ := json.Marshal(result)
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printOutput(cmd.OutOrStdout(), data, true)
	}
	for k, v := range result {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s: %v\n", k, v)
	}
	return nil
}
