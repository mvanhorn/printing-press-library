// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/coinlocker-navi/internal/locker"

	"github.com/spf13/cobra"
)

var lockerIDRe = regexp.MustCompile(`^\d{1,7}$`)

type lockerView struct {
	Meta   lockerMeta    `json:"meta"`
	Result locker.Detail `json:"results"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newLockerCmd(flags))
		addNovelCommandIfAbsent(root, newVacancyCmd(flags))
	})
}

func newLockerCmd(flags *rootFlags) *cobra.Command {
	var idFlag string
	cmd := &cobra.Command{
		Use:   "locker [id]",
		Short: "One Coin Locker Navi locker in full, with its five nearest neighbours",
		Long: strings.Trim(`
Read one locker record from コインロッカーなび by its id (the number in
/cl/<id>; near returns it). Output has sizes with yen and box count, payment
methods with an IC-card verdict, change machine, hours, note, coordinates and
the five nearest neighbours with distance. Fields the source marks 情報なし
are null. The source shows no record date; fetched_at is the observation time.`, "\n"),
		Example: strings.Trim(`
  coinlocker-navi-pp-cli locker 2685 --agent
  coinlocker-navi-pp-cli locker 3366 --agent --select meta.fetched_at,results.name,results.sizes,results.hours`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "id=2685",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "locker: GET www.coinlocker-navi.com/cl/{id}")
			}
			id, err := lockerIDArg(args, idFlag)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if !lockerIDRe.MatchString(id) {
				return usageErr(fmt.Errorf("locker id %q: want digits, e.g. 2685", id))
			}
			// Live-only: there is no local store, so reject --data-source local
			// before any network call.
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newLockerClient(flags)
			fetchedAt := nowJST()
			d, err := c.Detail(ctx, id)
			if err != nil {
				return classifyLockerErr("coinlocker-navi locker "+id, err)
			}
			meta := newLockerMeta("locker", c, fetchedAt)
			meta.Query = map[string]any{"id": id}
			meta.Notes = []string{locker.NoRecordDateNote}
			view := lockerView{Meta: meta, Result: d}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := newSafeTextWriter(cmd.OutOrStdout())
			fmt.Fprintf(w, "%s  [%s]  %s\n", d.Name, d.ServiceJA, d.URL)
			fmt.Fprintf(w, "sizes:   %s\n", sizesText(d.Locker))
			fmt.Fprintf(w, "payment: %s\n", paymentText(d.Locker))
			fmt.Fprintf(w, "hours:   %s\n", hoursText(d.Hours))
			fmt.Fprintf(w, "change:  %s\n", strOr(d.ChangeMachineRaw, "unknown"))
			fmt.Fprintf(w, "note:    %s\n", strings.ReplaceAll(strOr(d.Note, "-"), "\n", " / "))
			if d.MapURL != nil {
				fmt.Fprintf(w, "map:     %s\n", *d.MapURL)
			}
			if len(d.Neighbours) > 0 {
				fmt.Fprintln(w, "\nneighbours:")
				return renderLockerTable(cmd, d.Neighbours, meta)
			}
			fmt.Fprintf(w, "\nfetched %s; %s\n", meta.FetchedAt, locker.NoRecordDateNote)
			return nil
		},
	}
	cmd.Flags().StringVar(&idFlag, "id", "", "Locker id (same as the positional id)")
	return cmd
}

func sizesText(l locker.Locker) string {
	if !l.SizesKnown {
		return "unknown"
	}
	parts := make([]string, 0, len(l.Sizes))
	for _, s := range l.Sizes {
		lab := strOr(s.Label, s.LabelJA)
		p := lab
		if s.PriceYen != nil {
			p += fmt.Sprintf(" ¥%d", *s.PriceYen)
		}
		if s.Count != nil {
			p += fmt.Sprintf(" x%d", *s.Count)
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, ", ")
}

func paymentText(l locker.Locker) string {
	if !l.PaymentKnown {
		return "unknown"
	}
	return strings.Join(l.PaymentJA, " ")
}

func hoursText(h locker.Hours) string {
	switch h.Kind {
	case locker.HoursClockRange:
		s := *h.Open + "-" + *h.Close
		if h.Overnight {
			s += " (overnight)"
		}
		return s
	case locker.HoursFirstToLastTrain:
		return "first-last train"
	case locker.Hours24h:
		return "24h"
	case locker.HoursUnknown:
		return "unknown"
	}
	return strOr(h.Raw, "unknown")
}

func liveText(l locker.Locker) string {
	if l.Live == nil {
		return "-"
	}
	var parts []string
	switch {
	case l.Live.Ekicube != nil:
		for _, b := range l.Live.Ekicube.Boxes {
			parts = append(parts, fmt.Sprintf("%s:%d", b.Key, b.ReservableEmpty))
		}
		// Name the matched bank's area: the bank is a proximity match and
		// may be a neighbouring bank, so the reader must see which one.
		area := strings.TrimSpace(strOr(l.Live.Ekicube.Area.JA, "") + " " + strOr(l.Live.Ekicube.Name.JA, ""))
		if area == "" {
			area = "nearest bank"
		}
		return fmt.Sprintf("ekicube [%s] reservable %s", truncateWidth(area, 24), strings.Join(parts, " "))
	case l.Live.Maihama != nil:
		for _, s := range l.Live.Maihama.Sizes {
			if s.Empty != nil {
				parts = append(parts, fmt.Sprintf("%s:%d", strOr(s.Label, s.LabelJA), *s.Empty))
			}
		}
		return "empty " + strings.Join(parts, " ")
	}
	return "-"
}

func renderLockerTable(cmd *cobra.Command, rows []locker.Locker, meta lockerMeta) error {
	w := newSafeTextWriter(cmd.OutOrStdout())
	if len(rows) == 0 {
		fmt.Fprintln(w, "No matching lockers.")
		for _, n := range meta.Notes {
			fmt.Fprintln(w, "note: "+n)
		}
		return nil
	}
	tw := &widthTable{}
	tw.add("ID", "DIST", "NAME", "SIZES", "PAY", "HOURS", "GATE", "LIVE")
	for _, r := range rows {
		dist := "-"
		if r.DistanceM != nil {
			dist = fmt.Sprintf("%dm", *r.DistanceM)
		}
		name := r.Name
		if r.Service != "coin_locker" {
			name += " [" + r.ServiceJA + "]"
		}
		tw.add(r.ID, dist, truncateWidth(name, 40), sizesText(r), truncateWidth(paymentText(r), 24), hoursText(r.Hours), strOr(r.Gate, "-"), liveText(r))
	}
	if err := tw.write(w); err != nil {
		return err
	}
	fmt.Fprintf(w, "\nfetched %s; %s\n", meta.FetchedAt, locker.NoRecordDateNote)
	return nil
}

// lockerIDArg takes the id from the one positional argument or --id.
func lockerIDArg(args []string, idFlag string) (string, error) {
	idFlag = strings.TrimSpace(idFlag)
	switch {
	case len(args) > 1:
		return "", fmt.Errorf("give one locker id, e.g. 2685")
	case len(args) == 1 && idFlag != "" && strings.TrimSpace(args[0]) != idFlag:
		return "", fmt.Errorf("positional id %q and --id %q differ", args[0], idFlag)
	case len(args) == 1:
		return strings.TrimSpace(args[0]), nil
	case idFlag != "":
		return idFlag, nil
	}
	return "", fmt.Errorf("give one locker id, e.g. 2685")
}
