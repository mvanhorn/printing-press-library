// pp:client-call
// Real implementation: the generator stub used the generic extractHTMLResponse
// path, which cannot drive the Liferay session+pagination search. Delegates to
// the shared gaclient core (see ga_core.go / search.go).
package cli

import (
	"github.com/spf13/cobra"
)

func newProvvedimentiCercaCmd(flags *rootFlags) *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use:         "cerca [testo]",
		Short:       "Cerca provvedimenti per testo, tipo, sede, anno, numero o NRG.",
		Example:     "  giustizia-amministrativa-pp-cli provvedimenti cerca \"appalto\" --tipo sentenza --sede roma",
		Annotations: map[string]string{"pp:endpoint": "provvedimenti.cerca", "pp:method": "GET", "pp:path": "/web/guest/dcsnprr", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			pos := ""
			if len(args) > 0 {
				pos = args[0]
			}
			return runGASearch(cmd, flags, f.opts(pos))
		},
	}
	addSearchFlags(cmd, &f)
	return cmd
}
