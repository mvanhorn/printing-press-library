package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"
	"github.com/spf13/cobra"
)

func newComponentsLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "components",
		Short: "Component operations via the Framer Server API",
	}
	cmd.AddCommand(newComponentsAddLiveCmd(flags))
	return cmd
}

func newComponentsAddLiveCmd(flags *rootFlags) *cobra.Command {
	var codeFileName string
	var codeFileID string
	var insertURL string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a code component instance to the canvas",
		Long: strings.Trim(`
Add an instance of a code component to the canvas. You can reference the
component by code file name (--name), code file ID (--id), or direct
insertURL (--url). When using --name or --id, the CLI automatically
resolves the component's insertURL from its exports.`, "\n"),
		Example: strings.Trim(`
  # Add by code file name (resolves insertURL automatically)
  framer-pp-cli components add --name Hero

  # Add by code file ID
  framer-pp-cli components add --id iZHtokM

  # Add by direct insertURL
  framer-pp-cli components add --url "https://framer.com/m/Hero-xxxx.js"`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if codeFileName == "" && codeFileID == "" && insertURL == "" {
				return usageErr(fmt.Errorf("one of --name, --id, or --url is required"))
			}
			if dryRunOK(flags) {
				ref := codeFileName
				if ref == "" {
					ref = codeFileID
				}
				if ref == "" {
					ref = insertURL
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would add component instance for %s\n", ref)
				return nil
			}
			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			payload := map[string]interface{}{}
			if insertURL != "" {
				payload["insertURL"] = insertURL
			} else if codeFileID != "" {
				payload["codeFileId"] = codeFileID
			} else if codeFileName != "" {
				payload["codeFileName"] = codeFileName
			}

			raw, err := bc.Call("components-add", mustJSON(payload))
			if err != nil {
				return err
			}

			if flags.asJSON {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var result struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				InsertURL string `json:"insertURL"`
			}
			json.Unmarshal(raw, &result)
			fmt.Fprintf(cmd.OutOrStdout(), "Added component instance %s (from %s) to Framer canvas\n", result.ID, result.InsertURL)
			return nil
		},
	}
	cmd.Flags().StringVar(&codeFileName, "name", "", "Code file name (resolves insertURL automatically)")
	cmd.Flags().StringVar(&codeFileID, "id", "", "Code file ID (resolves insertURL automatically)")
	cmd.Flags().StringVar(&insertURL, "url", "", "Direct component insertURL")
	return cmd
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
