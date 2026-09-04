package cli

import "github.com/spf13/cobra"

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newResolveCmd(flags))
		// Splitwise API truth (prior patch splitwise-get-group-no-error-path-probe):
		// GET /get_group/{id} answers HTTP 200 with the "non-group" object for an
		// unknown id, so the generic invalid-argument error-path probe cannot apply.
		// The command file is generated, so the annotation is attached here, from a
		// hand-authored hook, to survive regeneration.
		if getGroup, _, err := root.Find([]string{"get-group"}); err == nil && getGroup != nil && getGroup.Name() == "get-group" {
			if getGroup.Annotations == nil {
				getGroup.Annotations = map[string]string{}
			}
			getGroup.Annotations["pp:no-error-path-probe"] = "true"
		}
	})
}
