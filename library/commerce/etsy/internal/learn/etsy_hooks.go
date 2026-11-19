package learn

import (
	"context"
	"database/sql"
)

// EtsyDropResourceEntities lets the host CLI veto dereferencing a resource row during recall.
// When non-nil and it returns true for a hit's resource type, Recall must not read the row.
var EtsyDropResourceEntities func(resourceType string) bool

// EtsyCandidateFilter lets the host veto surfacing open candidates during recall. When non-nil it is
// called once per recall; a nil return means "show everything", otherwise the returned predicate
// reports whether a candidate's query family must be hidden.
var EtsyCandidateFilter func(ctx context.Context, db *sql.DB) func(family string) bool

// EtsyWarningBuyerCandidatesHidden is the top-level warning code recall emits when it hid candidates.
const EtsyWarningBuyerCandidatesHidden = "buyer_candidates_hidden"
