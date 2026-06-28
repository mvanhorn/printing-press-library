package cli

import (
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/biz-insurance-finder/internal/insurance"
	"github.com/spf13/cobra"
)

func newWarningsCmd(f *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "warnings",
		Short: "Surface the underwriting landmines relevant to your profile",
		Long:  "warnings lists the coverage landmines from the live multi-carrier quote run: the foreign-products exclusion (the #1 importer trap), the GL Coverage B IP gap, specialty-market routing, and the lead-capture / consent process lessons.",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := f.loadProfile()
			if err != nil {
				return err
			}
			ws := insurance.Warnings(p)
			return f.emit(cmd, func(w io.Writer) { renderWarnings(w, ws) }, ws)
		},
	}
}

func renderWarnings(w io.Writer, ws []insurance.Warning) {
	fmt.Fprintf(w, "%s\n\n", bold("Underwriting warnings"))
	for _, x := range ws {
		fmt.Fprintf(w, "  %s %s\n", severityTag(x.Severity), bold(x.Title))
		fmt.Fprintf(w, "      %s\n", x.Detail)
	}
}

func severityTag(sev string) string {
	switch sev {
	case insurance.SeverityCritical:
		return red("[CRITICAL]")
	case insurance.SeverityImportant:
		return yellow("[IMPORTANT]")
	default:
		return dim("[info]")
	}
}
