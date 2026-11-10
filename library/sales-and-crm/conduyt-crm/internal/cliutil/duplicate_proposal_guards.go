package cliutil

import (
	"fmt"
	"regexp"
	"strings"
)

// Hand-written guards shared by the duplicate-proposal CLI commands and the MCP code-orchestration executor (identity rules v2,
// board #69). Not generated: the public OpenAPI describes the decision body only as a free object, so a reprint would send an
// unvalidated body. A reprint must keep these.

// ProposalDecisions are the only values POST /contacts/{id}/duplicate-proposals/{proposalId} accepts for `decision`.
// not_sure keeps both records visible; different is final (never shown again); merge records a pair that is ALREADY merged
// (it never merges: use the contacts merge command first, else the server answers 409 not_merged).
var ProposalDecisions = []string{"not_sure", "different", "merge"}

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// CheckUUID refuses a value that is not a UUID, before it can reach a request path.
func CheckUUID(name, value string) error {
	if !uuidRE.MatchString(value) {
		return fmt.Errorf("%s must be a UUID (got %q)", name, value)
	}
	return nil
}

// CheckProposalDecision requires exactly one of the three lower-case decision values.
func CheckProposalDecision(value string) error {
	for _, d := range ProposalDecisions {
		if value == d {
			return nil
		}
	}
	return fmt.Errorf("decision must be exactly one of %s (got %q); not_sure keeps both records visible, different is final, merge only records a pair that is already merged", strings.Join(ProposalDecisions, ", "), value)
}

// CheckProposalBody requires a body that is exactly { "decision": <one of the three> }, nothing else.
func CheckProposalBody(body map[string]any) error {
	for k := range body {
		if k != "decision" {
			return fmt.Errorf("the request body takes only \"decision\" (got %q)", k)
		}
	}
	v, ok := body["decision"]
	if !ok {
		return fmt.Errorf("decision is required: one of %s", strings.Join(ProposalDecisions, ", "))
	}
	s, isStr := v.(string)
	if !isStr {
		return fmt.Errorf("decision must be a string, one of %s", strings.Join(ProposalDecisions, ", "))
	}
	return CheckProposalDecision(s)
}
