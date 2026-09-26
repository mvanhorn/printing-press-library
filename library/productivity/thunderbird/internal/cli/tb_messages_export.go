// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type tbExportedFile struct {
	ID     string `json:"id"`
	Format string `json:"format"`
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
}

func newTBMessagesExportCmd(flags *rootFlags) *cobra.Command {
	var format, outDir string
	var force bool
	cmd := &cobra.Command{
		Use:   "export <id> [id...]",
		Short: "Export messages as the original .eml or as JSON",
		Long: `Export one or more messages. --format eml (default) writes the original
RFC822 bytes from the mbox; --format json writes headers, flags, full text
body and attachment list. Without --output the result goes to stdout (eml
accepts a single id there, and --json without --format selects json); with --output each message becomes <id>.eml or
<id>.json in that directory. Existing files are kept unless --force.`,
		Example: strings.Trim(`
  thunderbird-pp-cli messages export 3f9a1c2b7d4e > message.eml
  thunderbird-pp-cli messages export 3f9a1c2b7d4e 7b2e9d0c4a1f --output ./export
  thunderbird-pp-cli messages export 3f9a1c2b7d4e --format json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "local", "pp:typed-exit-codes": "0,2,3", "pp:happy-args": "id=0123456789ab"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "messages export")
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("expected at least one message id\nUsage: %s <id> [id...]", cmd.CommandPath()))
			}
			format = strings.ToLower(strings.TrimSpace(format))
			if !cmd.Flags().Changed("format") && outDir == "" && flags.asJSON {
				format = "json"
			}
			if format != "eml" && format != "json" {
				return usageErr(fmt.Errorf("--format must be eml or json, got %q", format))
			}
			if outDir == "" && format == "eml" && len(args) > 1 {
				return usageErr(fmt.Errorf("exporting several messages as eml needs --output <dir>"))
			}
			db, err := tbStoreFor(cmd, flags, "messages")
			if err != nil || db == nil {
				return err
			}
			type item struct {
				id   string
				data []byte
				det  tbMessageDetail
			}
			items := make([]item, 0, len(args))
			for _, ref := range args {
				d, err := tbGetMessage(db, ref)
				if err != nil {
					_ = db.Close()
					return err
				}
				raw, err := tbReadRaw(d)
				if err != nil {
					_ = db.Close()
					return err
				}
				it := item{id: d.ID, data: raw}
				if format == "json" {
					it.det = tbBuildDetail(d, raw, true, nil)
					if it.data, err = json.MarshalIndent(it.det, "", "  "); err != nil {
						_ = db.Close()
						return err
					}
				}
				items = append(items, it)
			}
			_ = db.Close()
			if outDir == "" {
				if format == "eml" {
					_, err := cmd.OutOrStdout().Write(items[0].data)
					return err
				}
				dets := make([]tbMessageDetail, 0, len(items))
				for _, it := range items {
					dets = append(dets, it.det)
				}
				return printJSONFiltered(cmd.OutOrStdout(), dets, flags)
			}
			files := make([]tbExportedFile, 0, len(items))
			var clash []string
			for _, it := range items {
				p := filepath.Join(outDir, it.id+"."+format)
				if _, err := os.Lstat(p); err == nil && !force {
					clash = append(clash, p)
				}
				files = append(files, tbExportedFile{ID: it.id, Format: format, Path: p, Bytes: len(it.data)})
			}
			if len(clash) > 0 {
				return fmt.Errorf("refusing to overwrite existing file(s): %s (use --force)", strings.Join(clash, ", "))
			}
			if err := os.MkdirAll(outDir, 0o700); err != nil {
				return err
			}
			for i, it := range items {
				if err := tbWriteNewFile(files[i].Path, it.data, force); err != nil {
					return err
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), files, flags)
			}
			for _, f := range files {
				fmt.Fprintf(tbHumanOut(cmd), "exported %s (%s)\n", f.Path, tbHumanBytes(int64(f.Bytes)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "eml", "Output format: eml (original bytes) or json")
	tbOutputDirFlag(cmd, &outDir, "Directory to write <id>.eml / <id>.json files into (default: stdout)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing files in --output")
	return cmd
}
