package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type orgInfo struct {
	Key         string `json:"key"`
	Nome        string `json:"nome"`
	ParametroDB string `json:"parametro_db"`
}

var aziende = []orgInfo{
	{"aslbari", "ASL Bari", "ASL Bari"},
	{"aslbat", "ASL BT", "ASL BT"},
	{"aslfoggia", "ASL Foggia", "ASL Foggia"},
	{"asltaranto", "ASL Taranto", "ASL Taranto"},
	{"aslbrindisi", "ASL Brindisi", "ASL Brindisi"},
	{"asllecce", "ASL Lecce", "ASL Lecce"},
	{"aress", "ARESS", "ARESS"},
	{"ares", "ARES", "ARES"},
	{"ospedaliriuniti", "Ospedali Riuniti Foggia", "Azienda Ospedaliera Ospedali Riuniti - Foggia"},
	{"sdebellis", "IRCCS S. De Bellis", `I.R.C.C.S. "S. De Bellis" - Castellana Grotte`},
	{"giovannipaolo", "IRCCS G. Paolo II", `I.R.C.C.S Ospedale Oncologico "G. Paolo II" - Bari`},
	{"policlinicobari", "Policlinico Bari", "Azienda Ospedaliero Universitaria Consorziale Policlinico"},
	{"saslbr", "Sanitaservice ASL BR", "Sanitaservice ASL BR"},
}

func newOrgsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "orgs",
		Short:   "Lista le 13 aziende sanitarie pugliesi con i loro identificatori API",
		Example: "  aol-puglia-pp-cli orgs\n  aol-puglia-pp-cli orgs --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.asJSON {
				data, err := json.MarshalIndent(aziende, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "CHIAVE\tNOME BREVE\tNOME API (--azienda)")
			fmt.Fprintln(w, "------\t----------\t--------------------")
			for _, a := range aziende {
				fmt.Fprintf(w, "%s\t%s\t%s\n", a.Key, a.Nome, a.ParametroDB)
			}
			return w.Flush()
		},
	}
	// Ensure --json flag is available
	_ = flags
	return cmd
}

// ListAziende stampa la lista delle aziende su stderr a scopo diagnostico.
func ListAziende() {
	w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Aziende disponibili:")
	for _, a := range aziende {
		fmt.Fprintf(w, "  %-20s %s\n", a.Key, a.ParametroDB)
	}
	w.Flush()
}
