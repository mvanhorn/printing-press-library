// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/courtlistener/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/courtlistener/internal/config"
	"github.com/spf13/cobra"
)

func clGet(ctx context.Context, flags *rootFlags, path string, params url.Values, authRequired bool) (map[string]any, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(cfg.AuthHeader())
	if authRequired && token == "" {
		return nil, errors.New("this command requires CourtListener API credentials; set COURTLISTENER_TOKEN_AUTH or run auth set-token")
	}
	if token != "" {
		if !strings.HasPrefix(strings.ToLower(token), "token ") {
			token = "Token " + token
		}
		// The generated client owns configured base URLs, bounded retry, request
		// timeouts, caching, and --rate-limit. Supply CourtListener's required
		// scheme through its normal static-header slot.
		cfg.AuthHeaderVal = token
	}
	c := client.New(cfg, flags.timeout, flags.rateLimit)
	c.NoCache = flags.noCache
	raw, err := c.GetWithHeadersValues(ctx, path, params, nil)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var out map[string]any
	if err := decoder.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func clReferenceID(value any) string {
	s := strings.TrimSpace(fmt.Sprint(value))
	if s == "" || s == "<nil>" {
		return ""
	}
	s = strings.TrimRight(s, "/")
	if slash := strings.LastIndex(s, "/"); slash >= 0 {
		s = s[slash+1:]
	}
	return s
}

func clFirstString(row map[string]any, fields ...string) string {
	for _, field := range fields {
		value := strings.TrimSpace(fmt.Sprint(row[field]))
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return ""
}

// clDocketTimeline joins documents to their docket entry and applies a stable,
// oldest-first source chronology. Undated entries are retained at the end.
func clDocketTimeline(entries, documents []map[string]any) []map[string]any {
	documentsByEntry := map[string][]map[string]any{}
	var unlinked []map[string]any
	for _, document := range documents {
		entryID := clReferenceID(document["docket_entry"])
		if entryID == "" {
			unlinked = append(unlinked, document)
			continue
		}
		documentsByEntry[entryID] = append(documentsByEntry[entryID], document)
	}
	timeline := make([]map[string]any, 0, len(entries)+len(unlinked))
	for _, entry := range entries {
		entryID := clReferenceID(entry["id"])
		documents := documentsByEntry[entryID]
		sort.SliceStable(documents, func(i, j int) bool { return clReferenceIDLess(documents[i]["id"], documents[j]["id"]) })
		timeline = append(timeline, map[string]any{
			"date":        clFirstString(entry, "date_filed", "date_created", "date_modified"),
			"entry_id":    entry["id"],
			"document_id": nil,
			"entry":       entry,
			"documents":   documents,
		})
		delete(documentsByEntry, entryID)
	}
	for _, documents := range documentsByEntry {
		unlinked = append(unlinked, documents...)
	}
	for _, document := range unlinked {
		timeline = append(timeline, map[string]any{
			"date":        clFirstString(document, "date_filed", "date_created", "date_modified"),
			"entry_id":    document["docket_entry"],
			"document_id": document["id"],
			"entry":       nil,
			"documents":   []map[string]any{document},
		})
	}
	sort.SliceStable(timeline, func(i, j int) bool {
		left, right := fmt.Sprint(timeline[i]["date"]), fmt.Sprint(timeline[j]["date"])
		if left == "" {
			return false
		}
		if right == "" {
			return true
		}
		if left != right {
			return left < right
		}
		leftEntry, rightEntry := timeline[i]["entry_id"], timeline[j]["entry_id"]
		if clReferenceID(leftEntry) != clReferenceID(rightEntry) {
			return clReferenceIDLess(leftEntry, rightEntry)
		}
		return clReferenceIDLess(timeline[i]["document_id"], timeline[j]["document_id"])
	})
	return timeline
}

func clReferenceIDLess(leftValue, rightValue any) bool {
	left, right := clReferenceID(leftValue), clReferenceID(rightValue)
	leftNumber, leftOK := new(big.Int).SetString(left, 10)
	rightNumber, rightOK := new(big.Int).SetString(right, 10)
	if leftOK && rightOK {
		return leftNumber.Cmp(rightNumber) < 0
	}
	return left < right
}

func clResults(response map[string]any) []map[string]any {
	raw, ok := response["results"].([]any)
	if !ok {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if row, ok := item.(map[string]any); ok {
			out = append(out, row)
		}
	}
	return out
}

func docketBundle(ctx context.Context, flags *rootFlags, id string) (map[string]any, error) {
	docket, err := clGet(ctx, flags, "/dockets/"+url.PathEscape(id)+"/", nil, true)
	if err != nil {
		return nil, err
	}
	params := url.Values{"docket": {id}, "page_size": {"100"}}
	entries, err := clGet(ctx, flags, "/docket-entries/", params, true)
	if err != nil {
		return nil, err
	}
	parties, err := clGet(ctx, flags, "/parties/", params, true)
	if err != nil {
		return nil, err
	}
	attorneys, err := clGet(ctx, flags, "/attorneys/", params, true)
	if err != nil {
		return nil, err
	}
	documents, err := clGet(ctx, flags, "/recap-documents/", url.Values{"docket_entry__docket": {id}, "page_size": {"100"}}, true)
	if err != nil {
		return nil, err
	}
	entryRows, documentRows := clResults(entries), clResults(documents)
	pagination := map[string]any{"entries_next": entries["next"], "parties_next": parties["next"], "attorneys_next": attorneys["next"], "documents_next": documents["next"]}
	complete := entries["next"] == nil && parties["next"] == nil && attorneys["next"] == nil && documents["next"] == nil
	return map[string]any{"docket": docket, "timeline": clDocketTimeline(entryRows, documentRows), "entries": entryRows, "parties": clResults(parties), "attorneys": clResults(attorneys), "documents": documentRows, "complete_observation": complete, "pagination": pagination}, nil
}

func emitCL(cmd *cobra.Command, flags *rootFlags, source string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": source, "provider": "CourtListener / RECAP", "retrieved_at": time.Now().UTC().Format(time.RFC3339)})
}

func clCaveats() []string {
	return []string{"RECAP coverage is incomplete and does not represent every PACER docket or filing.", "Document metadata does not guarantee that a free local file is available.", "Judge and case history are descriptive evidence and must not be used for outcome prediction or causal scoring."}
}
