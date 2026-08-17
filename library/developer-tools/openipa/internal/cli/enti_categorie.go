// Hand-written addition: enti categorie — live category list from CKAN — preserve on regeneration.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openipa/internal/client"
	"github.com/spf13/cobra"
)

// ckanCategorieResourceID is the CKAN datastore resource for IPA entity categories.
const ckanCategorieResourceID = "84ebb2e7-0e61-427b-a1dd-ab8bb2a84f07"

func newEntiCategorieCmd(flags *rootFlags) *cobra.Command {
	var cerca string

	cmd := &cobra.Command{
		Use:   "categorie",
		Short: "Lista le categorie IPA degli enti con i relativi codici",
		Long: `Restituisce la lista completa delle categorie IPA degli enti
dal dataset CKAN del portale IPA open data (indicepa.gov.it/ipa-dati).

Il codice categoria può essere usato con: openipa-pp-cli sede enti --categoria <codice>`,
		Example: `  openipa-pp-cli enti categorie
  openipa-pp-cli enti categorie --cerca scuola
  openipa-pp-cli enti categorie --cerca comune`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.NewCKAN(flags.timeout)
			c.DryRun = flags.dryRun

			params := map[string]string{
				"resource_id": ckanCategorieResourceID,
				"limit":       "200",
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
				returned := len(records) // server-returned count, before the client-side --cerca filter

				// Client-side filter when --cerca is set.
				if cerca != "" {
					q := strings.ToLower(cerca)
					var filtered []map[string]any
					for _, r := range records {
						nome := strings.ToLower(fmt.Sprintf("%v", r["Nome_categoria"]))
						tip := strings.ToLower(fmt.Sprintf("%v", r["Tipologia_categoria"]))
						if strings.Contains(nome, q) || strings.Contains(tip, q) {
							filtered = append(filtered, r)
						}
					}
					records = filtered
				}

				if total > 200 {
					fmt.Fprintf(os.Stderr, "warning: results truncated — %d of %d total records returned\n",
						returned, total)
				}

				if len(records) == 0 {
					if cerca != "" {
						return fmt.Errorf("nessuna categoria trovata per %q", cerca)
					}
					return fmt.Errorf("nessuna categoria disponibile")
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
				"resource": "enti/categorie",
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
	cmd.Flags().StringVar(&cerca, "cerca", "", "Filtra per testo in nome o tipologia (es. 'scuola', 'comune')")
	return cmd
}
