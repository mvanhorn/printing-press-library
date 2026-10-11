package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/store"
	"github.com/spf13/cobra"
)

type guestInfo struct {
	Unit     int    `json:"unit"`
	Name     string `json:"name"`
	Phone    string `json:"phone,omitempty"`
	Email    string `json:"email,omitempty"`
	Adults   int    `json:"adults"`
	Children int    `json:"children"`
	Pets     int    `json:"pets"`
	CheckIn  string `json:"check_in"`
	CheckOut string `json:"check_out"`
	Channel  string `json:"channel"`
}

func newGuestsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var unit int

	cmd := &cobra.Command{
		Use:   "guests",
		Short: "Show current guests across all units",
		Example: strings.Trim(`
  hostfully-pp-cli guests
  hostfully-pp-cli guests --unit 5
  hostfully-pp-cli guests --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "guests")
			}

			var reservations []reservation
			if mockMode() {
				reservations = mockHostfullyReservations(time.Now())
			} else {
				if dbPath == "" {
					dbPath = defaultDBPath("hostfully-pp-cli")
				}
				db, err := store.OpenWithContext(cmd.Context(), dbPath)
				if err != nil {
					return fmt.Errorf("opening database: %w (run 'hostfully-pp-cli sync' first)", err)
				}
				defer db.Close()

				reservations, err = loadReservations(db)
				if err != nil {
					return fmt.Errorf("loading reservations: %w", err)
				}
			}

			today := time.Now().Truncate(24 * time.Hour)

			var current []guestInfo
			occupiedUnits := map[int]bool{}

			for _, r := range reservations {
				ciDate := r.CheckIn.Truncate(24 * time.Hour)
				coDate := r.CheckOut.Truncate(24 * time.Hour)
				if !ciDate.After(today) && coDate.After(today) {
					if unit > 0 && r.Unit != unit {
						continue
					}
					current = append(current, guestInfo{
						Unit:     r.Unit,
						Name:     fmt.Sprintf("%s %s", r.GuestFirst, r.GuestLast),
						Phone:    formatPhone(r.Phone),
						Email:    r.Email,
						Adults:   r.Adults,
						Children: r.Children,
						Pets:     r.Pets,
						CheckIn:  r.CheckIn.Format("Jan 2"),
						CheckOut: r.CheckOut.Format("Jan 2"),
						Channel:  formatChannel(r.Channel),
					})
					occupiedUnits[r.Unit] = true
				}
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), current, flags)
			}

			if unit > 0 && len(current) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Unit %d is currently vacant.\n", unit)
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "Unit\tGuest\tPhone\tCheck-out\tChannel")
			for _, g := range current {
				fmt.Fprintf(w, "#%d\t%s\t%s\t%s\t%s\n",
					g.Unit, g.Name, g.Phone, g.CheckOut, g.Channel)
			}
			if err := w.Flush(); err != nil {
				return err
			}

			if unit == 0 {
				var vacant []string
				for _, u := range unitNumbers() {
					if !occupiedUnits[u] {
						vacant = append(vacant, fmt.Sprintf("#%d", u))
					}
				}
				if len(vacant) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "\nVacant: %s\n", strings.Join(vacant, ", "))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&unit, "unit", 0, "Show guest for a specific unit only")

	return cmd
}
