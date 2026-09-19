package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		cmd, _, err := root.Find([]string{"items", "modifiers", "get-item"})
		if err != nil || cmd.Name() != "get-item" {
			return
		}
		cmd.Example = "  haven-hot-chicken-pp-cli items modifiers get-item 9656289 --location-id 14208 --agent"
		cmd.Annotations["pp:happy-args"] = "item_id=9656289;--location-id=14208"
	})
}
