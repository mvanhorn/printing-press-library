// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
)

type qdsReport struct {
	Version         string       `json:"version"`
	ReleaseDate     string       `json:"release_date,omitempty"`
	InMatrix        bool         `json:"in_compatibility_matrix"`
	LTSStatus       string       `json:"lts_status"`
	LTSEndDate      string       `json:"lts_end_date,omitempty"`
	LTSSource       string       `json:"lts_source,omitempty"`
	AddedHardware   string       `json:"added_hardware,omitempty"`
	RemovedHardware string       `json:"removed_hardware,omitempty"`
	KnownIssues     []supportRef `json:"known_issues"`
	Awareness       []supportRef `json:"awareness"`
	MatrixRows      int          `json:"matrix_rows_scanned"`
	ScannedArticles int          `json:"scanned_articles"`
	Note            string       `json:"note,omitempty"`
}

func newNovelQdsCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		limit  int
		full   bool
	)
	cmd := &cobra.Command{
		Use:   "qds <version>",
		Short: "For one Q-SYS Designer release: known issues, LTS status and end date, and which hardware was removed.",
		Long: strings.Trim(`
Qds is the pre-upgrade briefing for a single Q-SYS Designer release.

It joins two sources the vendor keeps apart. The hardware-support matrix on
help.qsys.com gives the release date and the hardware added and removed in that
release. The support knowledge base carries the known-issues and awareness
articles, which is where LTS designations and their end dates are announced.
Neither page can be queried by version, so today this means reading a 59-row
table and then searching the knowledge base by hand.

Articles are matched to the release line, so asking for 10.0 also picks up
articles that name 10.0.2, and articles naming a different line are excluded.
Articles that name no version at all are excluded here (unlike bom risks),
because a release briefing that included every version-agnostic article would
not be a briefing.

LTS status is reported as "lts" only when an article in the local corpus
designates this release as LTS. "unknown" means no such article was found - it
is not a claim that the release lacks LTS. The source URL is always returned so
the designation can be verified against the vendor.

Exits 3 when the version is in neither the matrix nor the knowledge base, so a
script can tell a typo from a quiet release.
`, "\n"),
		Example: strings.Trim(`
  qsys-pp-cli qds 10.0 --agent
  qsys-pp-cli qds 9.4 --full
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:happy-args":          "version=10.0",
			"pp:typed-exit-codes":    "0,3",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "qds")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a Q-SYS Designer version is required, e.g. 10.0"))
			}
			version := strings.TrimSpace(args[0])
			if limit <= 0 {
				limit = 10
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			rep := qdsReport{
				Version:     version,
				LTSStatus:   "unknown",
				KnownIssues: make([]supportRef, 0, limit),
				Awareness:   make([]supportRef, 0, limit),
			}

			dbPath = corpusDBPath(dbPath)
			if corpusMissing(cmd, flags, dbPath) {
				rep.Note = "no local corpus; run `qsys-pp-cli harvest`"
				return finishQDS(cmd, flags, rep, false)
			}
			st, err := openCorpus(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			db := st.DB()

			rep.MatrixRows, err = countRows(ctx, db, `SELECT COUNT(*) FROM qsys_compat`)
			if err != nil {
				return err
			}
			if err := loadCompatRelease(ctx, db, &rep); err != nil {
				return err
			}

			stored, err := supportHarvested(ctx, db)
			if err != nil {
				return err
			}
			if stored == 0 {
				rep.Note = supportHarvestHint + "; known issues and LTS status are unavailable"
				return finishQDS(cmd, flags, rep, !rep.InMatrix)
			}

			articles, err := loadSupportArticles(ctx, db, releaseCategories)
			if err != nil {
				return err
			}
			rep.ScannedArticles = len(articles)
			for _, a := range articles {
				text := a.Title + " " + a.Body
				versions := articleVersions(text)
				// A release briefing keeps only articles that name this
				// release line. Version-agnostic articles are the general
				// knowledge base, not news about this release.
				if len(versions) == 0 || !versionRelevant(version, versions) {
					continue
				}
				ref := supportRef{
					Title:            a.Title,
					Category:         a.Category,
					URL:              a.URL,
					DesignerVersions: versions,
					Excerpt:          excerpt(a.Body, excerptLen(full)),
				}
				if rep.LTSStatus == "unknown" {
					if end, ok := detectLTSForVersion(version, text, versions); ok {
						rep.LTSStatus, rep.LTSEndDate, rep.LTSSource = "lts", end, a.URL
					}
				}
				if a.Category == "known-issues" {
					rep.KnownIssues = append(rep.KnownIssues, ref)
				} else {
					rep.Awareness = append(rep.Awareness, ref)
				}
			}
			sortRefs(rep.KnownIssues)
			sortRefs(rep.Awareness)
			if len(rep.KnownIssues) > limit {
				rep.KnownIssues = rep.KnownIssues[:limit]
			}
			if len(rep.Awareness) > limit {
				rep.Awareness = rep.Awareness[:limit]
			}

			missing := !rep.InMatrix && len(rep.KnownIssues) == 0 && len(rep.Awareness) == 0
			switch {
			case missing:
				rep.Note = fmt.Sprintf("Q-SYS Designer %s is in neither the compatibility matrix (%d rows) nor the knowledge base (%d articles scanned)", version, rep.MatrixRows, rep.ScannedArticles)
			case rep.MatrixRows == 0:
				rep.Note = "compatibility matrix is empty; run `qsys-pp-cli harvest --only compat`"
			case rep.LTSStatus == "unknown":
				rep.Note = "no article in the local corpus designates this release as LTS; that is not a claim that it is not an LTS release"
			}
			return finishQDS(cmd, flags, rep, missing)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Corpus database path")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum articles per category")
	cmd.Flags().BoolVar(&full, "full", false, "Return untruncated article text")
	return cmd
}

// loadCompatRelease fills the matrix-derived fields for one release. The
// vendor's own wording ("No changes.") is stored verbatim rather than being
// reinterpreted as an empty list, because the two mean different things.
func loadCompatRelease(ctx context.Context, db *sql.DB, rep *qdsReport) error {
	var release, added, removed sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT release_date, added_hardware, removed_hardware FROM qsys_compat WHERE qds_version = ?`,
		rep.Version).Scan(&release, &added, &removed)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading compatibility row for %s: %w", rep.Version, err)
	}
	rep.InMatrix = true
	rep.ReleaseDate = release.String
	rep.AddedHardware = added.String
	rep.RemovedHardware = removed.String
	return nil
}

// ltsRE detects an LTS designation, and ltsEndRE the date it runs to. QSC
// announces both in prose in the awareness articles, so this is extraction
// from text rather than a published field; the source URL is always returned
// alongside so the reader can check it.
var (
	ltsRE = regexp.MustCompile(`(?i)\blong[- ]term support\b|\bLTS\b`)

	ltsMonth = `(?:jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec)[a-z]*`
	ltsDate  = `(?:` + ltsMonth + `\s+\d{1,2},?\s+\d{4}|` + ltsMonth + `\s+\d{4}|\d{4}-\d{2}-\d{2}|\d{1,2}/\d{1,2}/\d{4})`
	ltsEndRE = regexp.MustCompile(`(?i)(?:lts\s+(?:support\s+)?(?:ends?|end date|expires?|expiration|through|until)|end[- ]of[- ](?:life|support|maintenance)|supported\s+(?:through|until)|support\s+ends?)[^\n]{0,40}?(` + ltsDate + `)`)
)

// detectLTS reports whether text designates the release as LTS, and the end
// date when one is stated. A designation with no date still returns true: "LTS
// with an unknown end date" is a materially different answer from "not LTS".
func detectLTS(text string) (string, bool) {
	if !ltsRE.MatchString(text) {
		return "", false
	}
	if m := ltsEndRE.FindStringSubmatch(text); m != nil {
		return strings.TrimSpace(m[1]), true
	}
	return "", true
}

// ltsSentenceCap bounds how far (in bytes) the sentence around a version
// mention may extend in each direction; a backstop against pathological text
// with no sentence punctuation.
const ltsSentenceCap = 400

// detectLTSForVersion attributes LTS wording to the queried release only. A
// single-release article can be trusted whole, but a multi-release article can
// say "9.8 is the LTS release" while merely mentioning the queried 9.10. Each
// LTS phrase is attributed to the version mention nearest to it inside the
// same sentence — in QSC's announcement prose the designated release is the
// subject sitting next to the designation ("Designer 9.8 is the LTS release
// ... over Designer 9.10"), so nearest-mention wins even when the sentence
// names another release further away.
func detectLTSForVersion(target, text string, versions []string) (string, bool) {
	if len(versions) <= 1 {
		return detectLTS(text)
	}
	mentions := designerVersionRE.FindAllStringSubmatchIndex(text, -1)
	seenSentence := map[int]struct{}{}
	for _, lm := range ltsRE.FindAllStringIndex(text, -1) {
		lo, hi := ltsSentenceBounds(text, lm[0], lm[1])
		// Matches arrive in order, so the first LTS phrase seen for a sentence
		// is its designating one ("is the LTS release"). A later phrase in the
		// same sentence is usually an elliptical continuation of that same
		// designation ("...; LTS support ends March 2027") — except when a
		// version mention sits immediately before it as its grammatical
		// subject ("... and Designer 10.0 is LTS through June 2028"), which is
		// an independent designation of that release.
		if _, dup := seenSentence[lo]; dup {
			m, subject := subjectMentionBefore(text, mentions, lm[0], lo)
			if !subject || !versionSeriesMatch(target, text[m[2]:m[3]]) {
				continue
			}
			if end, ok := detectLTS(text[lm[0]:hi]); ok {
				return end, ok
			}
			continue
		}
		seenSentence[lo] = struct{}{}
		best, bestDist := -1, hi-lo+1
		for i, m := range mentions {
			if m[2] < 0 || m[3] < 0 || m[2] < lo || m[3] > hi {
				continue
			}
			d := spanGap(m[2], m[3], lm[0], lm[1])
			if d < bestDist {
				bestDist, best = d, i
			}
		}
		if best < 0 || !versionSeriesMatch(target, text[mentions[best][2]:mentions[best][3]]) {
			continue
		}
		if end, ok := detectLTS(text[lo:hi]); ok {
			return end, ok
		}
	}
	return "", false
}

// subjectMentionBefore returns the version mention that immediately precedes
// position pos as its grammatical subject: the closest mention ending at or
// before pos, within the same sentence (>= lo), separated from pos by nothing
// but letters and spaces (linking words like "is", "remains", "is the") within
// a short span. Punctuation in the gap means the mention is not the subject.
func subjectMentionBefore(text string, mentions [][]int, pos, lo int) ([]int, bool) {
	const maxSubjectGap = 40
	var best []int
	for _, m := range mentions {
		if m[2] < 0 || m[3] < 0 || m[2] < lo || m[3] > pos {
			continue
		}
		if best == nil || m[3] > best[3] {
			best = m
		}
	}
	if best == nil || pos-best[3] > maxSubjectGap {
		return nil, false
	}
	for _, r := range text[best[3]:pos] {
		if r != ' ' && r != '\t' && !unicode.IsLetter(r) {
			return nil, false
		}
	}
	return best, true
}

// spanGap returns the byte distance between two half-open spans, 0 if they
// overlap or touch.
func spanGap(aLo, aHi, bLo, bHi int) int {
	if aHi <= bLo {
		return bLo - aHi
	}
	if bHi <= aLo {
		return aLo - bHi
	}
	return 0
}

// ltsSentenceBounds expands [lo,hi) to the enclosing sentence. A period
// between two digits is a version dot (9.8.1), not a sentence end.
func ltsSentenceBounds(text string, lo, hi int) (int, int) {
	isBoundary := func(i int) bool {
		switch text[i] {
		case '\n', '!', '?':
			return true
		case '.':
			return i == 0 || i+1 >= len(text) ||
				text[i-1] < '0' || text[i-1] > '9' ||
				text[i+1] < '0' || text[i+1] > '9'
		}
		return false
	}
	start := lo
	for start > 0 && lo-start < ltsSentenceCap && !isBoundary(start-1) {
		start--
	}
	end := hi
	for end < len(text) && end-hi < ltsSentenceCap && !isBoundary(end) {
		end++
	}
	if end < len(text) {
		end++ // keep the terminator so date regexes anchored to it still see context
	}
	return start, end
}

func sortRefs(refs []supportRef) {
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].Title < refs[j].Title })
}

func finishQDS(cmd *cobra.Command, flags *rootFlags, rep qdsReport, notFound bool) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if err := printJSONFiltered(cmd.OutOrStdout(), rep, flags); err != nil {
			return err
		}
	} else {
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "Q-SYS Designer %s\n", rep.Version)
		if rep.ReleaseDate != "" {
			fmt.Fprintf(w, "released    %s\n", rep.ReleaseDate)
		}
		fmt.Fprintf(w, "lts         %s", rep.LTSStatus)
		if rep.LTSEndDate != "" {
			fmt.Fprintf(w, " (ends %s)", rep.LTSEndDate)
		}
		fmt.Fprintln(w)
		if rep.LTSSource != "" {
			fmt.Fprintf(w, "lts source  %s\n", rep.LTSSource)
		}
		if rep.AddedHardware != "" {
			fmt.Fprintf(w, "\nHARDWARE ADDED\n%s\n", rep.AddedHardware)
		}
		if rep.RemovedHardware != "" {
			fmt.Fprintf(w, "\nHARDWARE REMOVED\n%s\n", rep.RemovedHardware)
		}
		printSupportRefs(w, "KNOWN ISSUES", rep.KnownIssues)
		printSupportRefs(w, "AWARENESS", rep.Awareness)
		if rep.Note != "" {
			fmt.Fprintf(w, "\nnote: %s\n", rep.Note)
		}
	}
	if notFound {
		return notFoundErr(fmt.Errorf("Q-SYS Designer %s not found in the local corpus", rep.Version))
	}
	return nil
}
