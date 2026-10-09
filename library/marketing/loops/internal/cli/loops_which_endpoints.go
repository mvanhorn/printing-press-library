package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// PATCH(loops-which-indexes-api-commands): `which` ranks the curated feature
// index plus every visible command that maps to a Loops API endpoint, so
// endpoint commands added by a spec update (for example campaign metrics) are
// discoverable without hand-maintaining the curated list.
func loopsWhichIndex(root *cobra.Command) []whichEntry {
	index := append([]whichEntry{}, whichIndex...)
	if root == nil {
		return index
	}
	seen := make(map[string]bool, len(index))
	for _, entry := range index {
		seen[entry.Command] = true
	}
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, child := range cmd.Commands() {
			if child.Hidden || !child.IsAvailableCommand() {
				continue
			}
			if child.Annotations["pp:endpoint"] != "" {
				path := strings.TrimSpace(strings.TrimPrefix(child.CommandPath(), root.Name()))
				if path != "" && !seen[path] {
					seen[path] = true
					index = append(index, whichEntry{
						Command:     path,
						Description: child.Short,
						Group:       strings.Fields(path)[0],
					})
				}
			}
			walk(child)
		}
	}
	walk(root)
	return index
}
