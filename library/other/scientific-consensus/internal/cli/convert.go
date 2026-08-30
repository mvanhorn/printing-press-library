// Hand-authored novel command: DOI <-> PMID converter. Not generated.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/source"
	"github.com/spf13/cobra"
)

// convertResult is the output of a single DOI<->PMID conversion. On no match,
// Found is false and Message explains it; the command still exits 0.
type convertResult struct {
	Input     string `json:"input"`
	InputType string `json:"input_type"` // "doi" or "pmid"
	Found     bool   `json:"found"`
	DOI       string `json:"doi,omitempty"`
	PMID      string `json:"pmid,omitempty"`
	Title     string `json:"title,omitempty"`
	Message   string `json:"message,omitempty"`
}

func newConvertCmd(flags *rootFlags) *cobra.Command {
	var doi, pmid string

	cmd := &cobra.Command{
		Use:   "convert",
		Short: "Convert between a DOI and a PMID",
		Long: "Resolve a DOI to its PubMed ID, or a PMID to its DOI, using OpenAlex (the same\n" +
			"keyless data source the other commands use — no new dependencies). Provide\n" +
			"exactly one of --doi or --pmid. On no match, reports \"no mapping found\"\n" +
			"rather than failing.",
		Example: "  scientific-consensus-pp-cli convert --doi 10.1038/s41586-020-2649-2\n" +
			"  scientific-consensus-pp-cli convert --pmid 32759779 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would convert an identifier")
				return nil
			}
			// Exactly one of --doi / --pmid. (a=="") == (b=="") is true when both
			// are empty or both are set.
			if (doi == "") == (pmid == "") {
				return usageErr(errors.New("provide exactly one of --doi or --pmid"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var res convertResult
			if doi != "" {
				res, err = convertDOIToPMID(ctx, c, doi)
			} else {
				res, err = convertPMIDToDOI(ctx, c, pmid)
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return emit(cmd, flags, res, func(w io.Writer) { renderConvert(w, res) })
		},
	}
	cmd.Flags().StringVar(&doi, "doi", "", "DOI to resolve to a PMID")
	cmd.Flags().StringVar(&pmid, "pmid", "", "PMID to resolve to a DOI")
	return cmd
}

// openAlexIDLookup is the minimal slice of an OpenAlex work needed to map
// between identifiers.
type openAlexIDLookup struct {
	ID          string `json:"id"`
	DOI         string `json:"doi"`
	Title       string `json:"title"`
	DisplayName string `json:"display_name"`
	IDs         struct {
		Pmid string `json:"pmid"`
	} `json:"ids"`
}

// lookupWorkByID fetches a single OpenAlex work by a namespaced id such as
// "doi:10.x/y" or "pmid:12345". A 404 (unknown identifier) is reported as a
// nil work, not an error, so the caller renders "no mapping found".
func lookupWorkByID(ctx context.Context, c apiGetter, entityID string) (*openAlexIDLookup, error) {
	raw, err := c.Get(ctx, "/works/"+entityID, map[string]string{
		"select": "id,doi,title,display_name,ids",
		"mailto": defaultMailto,
	})
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var w openAlexIDLookup
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, fmt.Errorf("parsing OpenAlex work: %w", err)
	}
	return &w, nil
}

// isNotFound reports whether err is an OpenAlex 404 (identifier not in the
// index), so the converter can treat it as "no mapping" rather than a failure.
func isNotFound(err error) bool {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 404
	}
	return strings.Contains(err.Error(), "HTTP 404")
}

func convertDOIToPMID(ctx context.Context, c apiGetter, doi string) (convertResult, error) {
	norm := source.NormDOI(doi)
	res := convertResult{Input: norm, InputType: "doi"}
	w, err := lookupWorkByID(ctx, c, "doi:"+norm)
	if err != nil {
		return res, err
	}
	pmid := ""
	if w != nil {
		pmid = extractPMID(w.IDs.Pmid)
	}
	if pmid == "" {
		res.Message = "no mapping found for " + norm
		return res, nil
	}
	res.Found = true
	res.DOI = norm
	res.PMID = pmid
	res.Title = workTitle(w)
	return res, nil
}

func convertPMIDToDOI(ctx context.Context, c apiGetter, pmid string) (convertResult, error) {
	clean := normPMID(pmid)
	res := convertResult{Input: clean, InputType: "pmid"}
	w, err := lookupWorkByID(ctx, c, "pmid:"+clean)
	if err != nil {
		return res, err
	}
	doi := ""
	if w != nil {
		doi = source.NormDOI(w.DOI)
	}
	if doi == "" {
		res.Message = "no mapping found for " + clean
		return res, nil
	}
	res.Found = true
	res.PMID = clean
	res.DOI = doi
	res.Title = workTitle(w)
	return res, nil
}

// normPMID strips a leading "pmid:" prefix and surrounding space so callers may
// pass "12345" or "pmid:12345" interchangeably.
func normPMID(pmid string) string {
	p := strings.TrimSpace(pmid)
	p = strings.TrimPrefix(strings.ToLower(p), "pmid:")
	return strings.TrimSpace(p)
}

func workTitle(w *openAlexIDLookup) string {
	if w.Title != "" {
		return w.Title
	}
	return w.DisplayName
}

func renderConvert(w io.Writer, r convertResult) {
	if !r.Found {
		fmt.Fprintln(w, r.Message)
		return
	}
	fmt.Fprintf(w, "DOI:   %s\n", r.DOI)
	fmt.Fprintf(w, "PMID:  %s\n", r.PMID)
	if r.Title != "" {
		fmt.Fprintf(w, "Title: %s\n", truncate(r.Title, 100))
	}
}
