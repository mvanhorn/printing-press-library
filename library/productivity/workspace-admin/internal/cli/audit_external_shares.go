// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: audit external-shares.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

type externalSharesView struct {
	Shares       []driveExternalShare `json:"shares"`
	ScannedFiles int                  `json:"scanned_files"`
	MaxScanPages int                  `json:"max_scan_pages"`
	Note         string               `json:"note,omitempty"`
}

type driveFileRow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Owners []struct {
		EmailAddress string `json:"emailAddress"`
	} `json:"owners"`
	Permissions []struct {
		Type         string `json:"type"`
		EmailAddress string `json:"emailAddress"`
		Domain       string `json:"domain"`
	} `json:"permissions"`
}

func newNovelAuditExternalSharesCmd(flags *rootFlags) *cobra.Command {
	var flagDomain string
	var flagInternal string
	var flagLimit int
	var flagMaxScanPages int

	cmd := &cobra.Command{
		Use:   "external-shares",
		Short: "Find Drive files shared with anyone-with-link or external domains, joined to their owner.",
		Long: "Find every Drive file shared 'anyone with link' or with an external domain, joined to its owner.\n\n" +
			"Use for per-file external exposure. For a per-external-domain rollup use 'audit domain-graph' instead.\n\n" +
			"Company-owned domains are taken from --internal-domain; when omitted they are inferred from the\n" +
			"domains of the scanned files' owners.",
		Example:     "  workspace-admin-pp-cli audit external-shares --domain gmail.com --agent --select shares.name,shares.owner",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would scan up to %d Drive list pages for external shares\n", flagMaxScanPages)
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			internal := internalDomainSet(flagInternal)
			inferInternal := len(internal) == 0

			url := wsDriveBase + "/files"
			pageToken := ""
			scanned := 0
			scanCapHit := true
			var raw []driveFileRow

			for page := 1; page <= flagMaxScanPages; page++ {
				params := map[string]string{
					"pageSize":                  "100",
					"fields":                    "nextPageToken,files(id,name,owners(emailAddress),permissions(type,emailAddress,domain))",
					"supportsAllDrives":         "true",
					"includeItemsFromAllDrives": "true",
					"corpora":                   "allDrives",
				}
				if pageToken != "" {
					params["pageToken"] = pageToken
				}
				data, err := c.Get(ctx, url, params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var env struct {
					NextPageToken string          `json:"nextPageToken"`
					Files         json.RawMessage `json:"files"`
				}
				if err := json.Unmarshal(data, &env); err != nil {
					return fmt.Errorf("decoding Drive files response: %w", err)
				}
				var pageFiles []driveFileRow
				_ = json.Unmarshal(env.Files, &pageFiles)
				raw = append(raw, pageFiles...)
				scanned += len(pageFiles)
				if env.NextPageToken == "" || len(pageFiles) == 0 {
					scanCapHit = false
					break
				}
				pageToken = env.NextPageToken
			}

			if inferInternal {
				for _, f := range raw {
					for _, o := range f.Owners {
						if d := emailDomain(o.EmailAddress); d != "" {
							internal[d] = true
						}
					}
				}
			}

			shares := make([]driveExternalShare, 0)
			for _, f := range raw {
				owner := ""
				if len(f.Owners) > 0 {
					owner = f.Owners[0].EmailAddress
				}
				for _, p := range f.Permissions {
					ext, shareType, with := classifyPermission(p.Type, p.EmailAddress, p.Domain, internal)
					if !ext {
						continue
					}
					if flagDomain != "" {
						target := emailDomain(with)
						if target == "" {
							target = with
						}
						if target != flagDomain && with != flagDomain {
							continue
						}
					}
					shares = append(shares, driveExternalShare{
						FileID:       f.ID,
						Name:         f.Name,
						Owner:        owner,
						ShareType:    shareType,
						ExternalWith: with,
					})
				}
				if flagLimit > 0 && len(shares) >= flagLimit {
					shares = shares[:flagLimit]
					break
				}
			}

			view := externalSharesView{Shares: shares, ScannedFiles: scanned, MaxScanPages: flagMaxScanPages}
			if len(shares) == 0 && scanCapHit {
				view.Note = "scanned " + strconv.Itoa(scanned) + " files up to the page cap without finding external shares; raise --max-scan-pages to widen the search"
			}
			return emitAudit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagDomain, "domain", "", "Filter to a single external domain (e.g. gmail.com)")
	cmd.Flags().StringVar(&flagInternal, "internal-domain", "", "Comma-separated company-owned domains; inferred from file owners when omitted")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum external shares to return (0 = no limit)")
	cmd.Flags().IntVar(&flagMaxScanPages, "max-scan-pages", 5, "Maximum Drive list pages to scan")
	return cmd
}
