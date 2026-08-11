package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Candidate retrieval for the reconcile decision engine.
//
// SearchIssuesByTeam answers "which issues match this text" and throws the
// relevance number away: it selects i.data only and orders by the implicit
// rank, so nothing quantitative reaches Go. Its LIMIT is also fixed at 50,
// which means a caller cannot widen the pool it scores over. Reconcile needs
// both, so it gets its own read path here rather than a behaviour change to
// the existing one.

// FTS match-expression join modes.
//
// MatchModeAll reproduces IssueSearchFTSQuery exactly: bare phrases separated
// by a space, which FTS5 joins with an implicit AND. MatchModeAny joins with
// an explicit OR, which is what a whole-title query needs, since requiring
// every token of a ten word title to be present usually matches nothing.
const (
	MatchModeAny = "any"
	MatchModeAll = "all"
)

// FTSQueryOptions shapes the MATCH expression built by IssueSearchFTSQueryWith.
// Zero values are not defaults: the caller owns every number, and the CLI
// surfaces each one as a flag.
type FTSQueryOptions struct {
	MatchMode   string // MatchModeAny or MatchModeAll
	MinTokenLen int    // tokens shorter than this are dropped
	MaxTokens   int    // cap on tokens, in order of appearance
}

// IssueSearchTokens splits text into the same tokens IssueSearchFTSQuery
// feeds to FTS5: runs of letters and digits, allowing interior underscores
// and hyphens, with those trimmed off the ends. Lowercased so callers can
// intersect two token sets without re-folding.
//
// Exported because the reconcile scorer has to recompute which query terms a
// candidate matched. FTS5 exposes no per-term match information through SQL
// (matchinfo is an FTS4 feature, and xInst is C-only), so the only way to
// report matched terms is to tokenise both sides in Go with these rules and
// intersect. That recomputation is unstemmed, so it can report fewer matched
// terms than bm25 actually rewarded.
func IssueSearchTokens(text string) []string {
	raw := ftsQueryTokenPattern.FindAllString(text, -1)
	tokens := make([]string, 0, len(raw))
	for _, token := range raw {
		token = strings.Trim(token, "_-")
		if token == "" {
			continue
		}
		tokens = append(tokens, strings.ToLower(token))
	}
	return tokens
}

// IssueSearchFTSQueryWith builds a safe FTS5 MATCH expression from prose,
// honouring a join mode and the token filters. It returns the expression and
// the tokens that survived filtering, in order of appearance, so the caller
// can report exactly which terms it searched for.
//
// Each token is quoted for the same reason IssueSearchFTSQuery quotes: raw
// issue keys such as SYMPH-309 and hyphenated prose such as follow-ups parse
// as FTS operators otherwise.
func IssueSearchFTSQueryWith(query string, opts FTSQueryOptions) (string, []string) {
	all := IssueSearchTokens(query)
	kept := make([]string, 0, len(all))
	seen := make(map[string]struct{}, len(all))
	for _, token := range all {
		if opts.MinTokenLen > 0 && len([]rune(token)) < opts.MinTokenLen {
			continue
		}
		if _, dup := seen[token]; dup {
			continue
		}
		seen[token] = struct{}{}
		kept = append(kept, token)
		if opts.MaxTokens > 0 && len(kept) >= opts.MaxTokens {
			break
		}
	}
	if len(kept) == 0 {
		return "", nil
	}
	quoted := make([]string, 0, len(kept))
	for _, token := range kept {
		quoted = append(quoted, `"`+strings.ReplaceAll(token, `"`, `""`)+`"`)
	}
	join := " "
	if opts.MatchMode == MatchModeAny {
		join = " OR "
	}
	return strings.Join(quoted, join), kept
}

// CandidateQuery is one scored retrieval against the local FTS index.
type CandidateQuery struct {
	Match       string  // MATCH expression from IssueSearchFTSQueryWith
	TeamID      string  // "" means no team predicate
	Limit       int     // SQL LIMIT, supplied by the caller, never assumed
	WeightID    float64 // bm25 column weight for identifier
	WeightTitle float64 // bm25 column weight for title
	WeightDesc  float64 // bm25 column weight for description
	ExcludeID   string  // source mode: the source issue's own id
}

// IssueCandidate is one row of the candidate set, carrying the relevance
// number the existing search path discards.
type IssueCandidate struct {
	ID          string
	Identifier  string
	Title       string
	Description string
	StateName   string
	StateType   string
	TeamID      string
	ProjectID   string
	UpdatedAt   time.Time
	CreatedAt   time.Time
	SyncedAt    time.Time

	// Bm25 is the raw bm25() value: negative for a match, more negative for
	// a better one. It is corpus-relative, so it has no absolute meaning
	// across stores of different sizes.
	Bm25 float64

	// MatchedColumns names the indexed columns FTS5 highlighted for this
	// row, drawn from identifier, title and description.
	MatchedColumns []string
}

// Column indexes into the issues_fts virtual table, which is declared as
// fts5(identifier, title, description, ...).
const (
	issuesFTSColIdentifier  = 0
	issuesFTSColTitle       = 1
	issuesFTSColDescription = 2
)

// highlightOpen and highlightClose are the markers wrapped around matched
// text by highlight(). They are control characters (STX and ETX) rather than
// printable strings so that no user-authored issue text can forge a marker
// and fake a column match.
const (
	highlightOpen  = "\x02"
	highlightClose = "\x03"
)

// SearchIssueCandidates runs one FTS5 query and returns the matching issues
// with their bm25 score and matched columns.
//
// state_type is read with json_extract because the issues table persists
// state_name but no state_type column. Results are ordered by score
// ascending, which is correct for bm25: SQLite returns negative values and
// better matches are more negative.
func (s *Store) SearchIssueCandidates(q CandidateQuery) ([]IssueCandidate, error) {
	if strings.TrimSpace(q.Match) == "" {
		return nil, nil
	}
	if q.Limit <= 0 {
		return nil, fmt.Errorf("SearchIssueCandidates: Limit must be positive, got %d", q.Limit)
	}

	var sb strings.Builder
	args := make([]any, 0, 8)

	sb.WriteString(`SELECT i.id, IFNULL(i.identifier, ''), IFNULL(i.title, ''), IFNULL(i.description, ''),
		IFNULL(i.state_name, ''),
		IFNULL(json_extract(i.data, '$.state.type'), '') AS state_type,
		IFNULL(i.team_id, ''), IFNULL(i.project_id, ''),
		IFNULL(i.updated_at, ''), IFNULL(i.created_at, ''), IFNULL(i.synced_at, ''),
		bm25(issues_fts, ?, ?, ?) AS score,
		highlight(issues_fts, ?, ?, ?) AS h_identifier,
		highlight(issues_fts, ?, ?, ?) AS h_title,
		highlight(issues_fts, ?, ?, ?) AS h_description
	FROM issues i
	JOIN issues_fts ON i.rowid = issues_fts.rowid
	WHERE issues_fts MATCH ?`)

	args = append(args, q.WeightID, q.WeightTitle, q.WeightDesc)
	args = append(args,
		issuesFTSColIdentifier, highlightOpen, highlightClose,
		issuesFTSColTitle, highlightOpen, highlightClose,
		issuesFTSColDescription, highlightOpen, highlightClose,
	)
	args = append(args, q.Match)

	// The team predicate is built conditionally rather than folded into an
	// always-present OR, so the team_id index stays usable. It is the only
	// cheap selectivity the issues table offers.
	if q.TeamID != "" {
		sb.WriteString(" AND i.team_id = ?")
		args = append(args, q.TeamID)
	}
	if q.ExcludeID != "" {
		sb.WriteString(" AND i.id <> ?")
		args = append(args, q.ExcludeID)
	}
	sb.WriteString(" ORDER BY score LIMIT ?")
	args = append(args, q.Limit)

	rows, err := s.db.Query(sb.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("searching issue candidates: %w", err)
	}
	defer rows.Close()

	var out []IssueCandidate
	for rows.Next() {
		var (
			c                                 IssueCandidate
			updatedAt, createdAt, syncedAt    string
			hIdentifier, hTitle, hDescription string
		)
		if err := rows.Scan(
			&c.ID, &c.Identifier, &c.Title, &c.Description,
			&c.StateName, &c.StateType,
			&c.TeamID, &c.ProjectID,
			&updatedAt, &createdAt, &syncedAt,
			&c.Bm25,
			&hIdentifier, &hTitle, &hDescription,
		); err != nil {
			return nil, fmt.Errorf("scanning issue candidate: %w", err)
		}
		c.UpdatedAt = parseStoreTime(updatedAt)
		c.CreatedAt = parseStoreTime(createdAt)
		c.SyncedAt = parseStoreTime(syncedAt)
		c.MatchedColumns = matchedColumns(
			[2]string{c.Identifier, hIdentifier},
			[2]string{c.Title, hTitle},
			[2]string{c.Description, hDescription},
		)
		out = append(out, c)
	}
	return out, rows.Err()
}

// IssueByRef returns the stored issue payload for a UUID or a TEAM-NUMBER
// identifier, or a nil payload when the store has neither. Reconcile needs it
// to read a --source or --target issue that the candidate query never
// surfaced, without paying for a live round trip.
func (s *Store) IssueByRef(ref string) (json.RawMessage, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, nil
	}
	column := "identifier"
	if IsUUID(ref) {
		column = "id"
	}
	rows, err := s.queryJSON(fmt.Sprintf(`SELECT data FROM issues WHERE %s = ? LIMIT 1`, column), ref)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// matchedColumns reports which indexed columns FTS5 wrapped in markers. Each
// pair is {raw value, highlight() output}: a column matched when the two
// differ, which happens only when highlight() inserted a marker.
func matchedColumns(identifier, title, description [2]string) []string {
	var cols []string
	if identifier[0] != identifier[1] {
		cols = append(cols, "identifier")
	}
	if title[0] != title[1] {
		cols = append(cols, "title")
	}
	if description[0] != description[1] {
		cols = append(cols, "description")
	}
	return cols
}

// parseStoreTime reads a timestamp column that may hold RFC3339 from the API
// or the SQLite CURRENT_TIMESTAMP shape. Returns the zero time when neither
// parses, which callers treat as "unknown age".
func parseStoreTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}
