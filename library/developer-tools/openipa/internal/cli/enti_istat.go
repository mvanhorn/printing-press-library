// Hand-written addition: enti istat — IPA/ISTAT bidirectional lookup via CKAN — preserve on regeneration.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openipa/internal/client"
	"github.com/spf13/cobra"
)

// ckanAOOResourceID: one row per AOO — used for --codice (IPA→ISTAT, limit=1).
const ckanAOOResourceID = "cdaded04-f84e-4193-a720-47d6d5f422aa"

// ckanEntiResourceID: one row per entity with Tipologia — used for --istat (ISTAT→IPA).
const ckanEntiResourceID = "d09adf99-dc10-4349-8c53-27b1e5aa97b6"

func newEntiIstatCmd(flags *rootFlags) *cobra.Command {
	var codiceIPA string
	var codiceISTAT string

	cmd := &cobra.Command{
		Use:   "istat",
		Short: "Abbina codice IPA e codice ISTAT tramite il dataset IPA open data",
		Long: `Ricerca bidirezionale tra codici IPA e codici ISTAT del comune
tramite il dataset CKAN del portale IPA open data (indicepa.gov.it/ipa-dati).

  --codice <IPA>   codice IPA → Codice_comune_ISTAT e dati di sede
  --istat <ISTAT>  codice ISTAT → lista enti IPA nel comune`,
		Example: `  openipa-pp-cli enti istat --codice c_g273
  openipa-pp-cli enti istat --istat 082053`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if codiceIPA == "" && codiceISTAT == "" && !flags.dryRun {
				return fmt.Errorf("specify --codice <IPA> or --istat <ISTAT>")
			}
			if codiceIPA != "" && codiceISTAT != "" {
				return fmt.Errorf("--codice and --istat are mutually exclusive")
			}

			c := client.NewCKAN(flags.timeout)
			c.DryRun = flags.dryRun

			// --codice: AOO dataset (one row/AOO, same ISTAT for all AOOs of an entity → limit=1).
			// --istat:  Enti dataset (one row/entity, includes Tipologia and Codice_Categoria).
			var resourceID, filterKey, filterVal, limit string
			if codiceIPA != "" {
				resourceID = ckanAOOResourceID
				filterKey = "Codice_IPA"
				filterVal = codiceIPA
				limit = "1"
			} else {
				resourceID = ckanEntiResourceID
				filterKey = "Codice_comune_ISTAT"
				filterVal = codiceISTAT
				limit = "500"
			}

			filtersJSON, err := json.Marshal(map[string]string{filterKey: filterVal})
			if err != nil {
				return fmt.Errorf("building filter: %w", err)
			}

			params := map[string]string{
				"resource_id": resourceID,
				"filters":     string(filtersJSON),
				"limit":       limit,
			}

			raw, statusCode, err := c.GetJSON("/api/3/action/datastore_search", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// In dry-run GetJSON returns the {"dry_run": true} sentinel, not a
			// CKAN response — skip parsing/records logic and fall through to the
			// dry-run envelope below.
			var records []map[string]any
			if !flags.dryRun {
				var total int
				records, total, err = parseCKANDatastore(raw)
				if err != nil {
					return err
				}

				if len(records) == 0 {
					if codiceIPA != "" {
						return fmt.Errorf("nessun risultato per codice IPA %q", codiceIPA)
					}
					return fmt.Errorf("nessun risultato per codice ISTAT %q", codiceISTAT)
				}

				// Warn when CKAN returned fewer records than the true total (--istat direction).
				if codiceISTAT != "" && total > len(records) {
					fmt.Fprintf(os.Stderr, "warning: results truncated — %d of %d total records returned; use a smaller area or filter further\n",
						len(records), total)
				}
				// Warn when entity has AOOs in multiple municipalities (--codice direction).
				if codiceIPA != "" && total > 1 {
					fmt.Fprintf(os.Stderr, "warning: entity has %d AOOs across potentially different municipalities — only the first ISTAT code is returned\n",
						total)
				}

				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					if err := printAutoTable(cmd.OutOrStdout(), records); err != nil {
						fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
					} else {
						return nil
					}
				}
			}

			items, err := json.Marshal(records)
			if err != nil {
				return err
			}

			filtered := items
			if flags.selectFields != "" {
				filtered = filterFields(filtered, flags.selectFields)
			} else if flags.compact {
				filtered = compactFields(filtered)
			}

			envelope := map[string]any{
				"action":   "get",
				"resource": "enti/istat",
				"path":     "/api/3/action/datastore_search",
				"status":   statusCode,
				"success":  statusCode >= 200 && statusCode < 300,
			}
			if flags.dryRun {
				envelope["dry_run"] = true
				envelope["status"] = 0
				envelope["success"] = false
			}
			var parsed any
			if err := json.Unmarshal(filtered, &parsed); err == nil {
				envelope["data"] = parsed
			}
			envelopeJSON, err := json.Marshal(envelope)
			if err != nil {
				return err
			}
			return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
		},
	}
	cmd.Flags().StringVar(&codiceIPA, "codice", "", "Codice IPA (es. 'c_g273') → restituisce Codice_comune_ISTAT")
	cmd.Flags().StringVar(&codiceISTAT, "istat", "", "Codice ISTAT del comune (es. '082053') → restituisce gli enti IPA nel comune")
	return cmd
}
