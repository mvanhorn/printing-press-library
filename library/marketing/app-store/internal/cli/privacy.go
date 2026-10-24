// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newPrivacyCmd(flags))
	})
}

// privacyView is the privacy command's JSON: the App Privacy label types,
// each with its data categories and data types (and, where Apple groups a
// type by purpose, the purposes).
type privacyView struct {
	App          string                 `json:"app"`
	Title        string                 `json:"title"`
	Storefront   string                 `json:"storefront"`
	PolicyURL    string                 `json:"policyUrl"`
	PrivacyTypes []appstore.PrivacyType `json:"privacyTypes"`
	Note         string                 `json:"note,omitempty"`
}

func newPrivacyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "privacy <app>",
		Short: "Fetch an app's privacy nutrition labels live: data types per category, linked or tracking",
		Long: strings.Trim(`
Show an app's App Privacy labels from its product page: each label type
(Data Used to Track You, Data Linked to You, Data Not Linked to You, Data Not
Collected) with its data categories and the data types in each, the purposes
Apple groups them by, and the privacy policy URL. Read from the
robots-allowed product page: no token and no proxy.

Each fresh page is recorded; the store keeps a new privacy snapshot only when
the labels change, so changes and watch-listing can date a label change. An
app with no labels returns an empty privacyTypes list and a note.

Use this for the privacy labels alone; 'app --full' shows them with the rest
of the listing. An unknown id exits 3.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli privacy 324684580 --json
  app-store-pp-cli privacy com.spotify.client --country gb --select privacyTypes.title`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			"pp:happy-args":       "app=324684580",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "privacy")
			}
			ref, err := s2aOneAppArg(cmd, args)
			if err != nil {
				return err
			}
			cc, err := refCountry(cmd, flags, ref)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newAppStoreClient(flags)
			if err != nil {
				return err
			}
			st, err := openAppStoreStore(cmd, flags)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			attachCaptureLog(c, st)

			lk, err := lookupAppRef(ctx, c, ref, cc)
			if err != nil {
				return err
			}
			page, err := s2aProductPage(ctx, c, lk, cc, "")
			if err != nil {
				return err
			}
			s2aWarnStore(cmd, "the privacy snapshot", s2aRecordProductPage(ctx, cmd, st, cc, page, lk))

			view := privacyView{
				App:          lk.AppID(),
				Title:        lk.TrackName,
				Storefront:   cc,
				PolicyURL:    page.Privacy.PolicyURL,
				PrivacyTypes: append(make([]appstore.PrivacyType, 0, len(page.Privacy.Types)), page.Privacy.Types...),
			}
			if len(view.PrivacyTypes) == 0 {
				switch {
				case page.Privacy.Unreadable:
					// A parse gap must not read as "no labels".
					return apiErr(fmt.Errorf("the %s product page has privacy labels this version cannot read (Apple may have changed the page); none are reported rather than a false \"no labels\"", cc))
				case page.Privacy.NoneShown:
					view.Note = fmt.Sprintf("the %s product page shows no privacy labels for this app (the developer may not have provided privacy details)", cc)
				default:
					view.Note = fmt.Sprintf("the %s product page has no App Privacy section, so it shows no privacy labels for this app", cc)
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if view.Note != "" {
					s2aPrintln(cmd.ErrOrStderr(), "note: "+view.Note)
				}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return s2aPrintPrivacyHuman(cmd.OutOrStdout(), view)
		},
	}
	s2bDBFlag(cmd)
	return cmd
}

func s2aPrintPrivacyHuman(w io.Writer, v privacyView) error {
	s2aPrintf(w, "%s (%s) privacy labels, storefront %s\n", cliutil.ScrubTerminal(v.Title), v.App, v.Storefront)
	if v.PolicyURL != "" {
		s2aPrintf(w, "Policy: %s\n", cliutil.ScrubTerminal(v.PolicyURL))
	}
	if v.Note != "" {
		s2aPrintln(w, v.Note)
		return nil
	}
	for _, t := range v.PrivacyTypes {
		s2aPrintf(w, "\n%s\n", cliutil.ScrubTerminal(t.Title))
		if len(t.Categories) == 0 && t.Detail != "" {
			s2aPrintf(w, "  %s\n", cliutil.ScrubTerminal(t.Detail))
		}
		for _, c := range t.Categories {
			types := ""
			if len(c.DataTypes) > 0 {
				types = ": " + strings.Join(c.DataTypes, ", ")
			}
			s2aPrintf(w, "  %s%s\n", cliutil.ScrubTerminal(c.Title), cliutil.ScrubTerminal(types))
		}
	}
	return nil
}
