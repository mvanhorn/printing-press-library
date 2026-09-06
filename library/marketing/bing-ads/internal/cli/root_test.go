// Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"

	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/bing-ads/internal/cliutil"

	"github.com/spf13/cobra"
)

func TestDeclaredAPISurfaceReachable(t *testing.T) {
	expected := []string{
		"ad-insight",
		"ad-insight apply-recommendations",
		"ad-insight dismiss-recommendations",
		"ad-insight get-auction-insight-data",
		"ad-insight get-audience-breakdown",
		"ad-insight get-audience-full-estimation",
		"ad-insight get-auto-apply-opt-in-status",
		"ad-insight get-bid-landscape-by-ad-group-ids",
		"ad-insight get-bid-landscape-by-campaign-ids",
		"ad-insight get-bid-landscape-by-keyword-ids",
		"ad-insight get-bid-opportunities",
		"ad-insight get-budget-opportunities",
		"ad-insight get-domain-categories",
		"ad-insight get-estimated-bid-by-keyword-ids",
		"ad-insight get-estimated-bid-by-keywords",
		"ad-insight get-estimated-position-by-keyword-ids",
		"ad-insight get-estimated-position-by-keywords",
		"ad-insight get-historical-keyword-performance",
		"ad-insight get-historical-search-count",
		"ad-insight get-keyword-categories",
		"ad-insight get-keyword-demographics",
		"ad-insight get-keyword-idea-categories",
		"ad-insight get-keyword-ideas",
		"ad-insight get-keyword-locations",
		"ad-insight get-keyword-opportunities",
		"ad-insight get-keyword-traffic-estimates",
		"ad-insight get-performance-insights-detail-data-by-account-id",
		"ad-insight get-recommendations",
		"ad-insight get-text-asset-suggestions-by-final-urls",
		"ad-insight retrieve-recommendations",
		"ad-insight set-auto-apply-opt-in-status",
		"ad-insight suggest-keywords-for-url",
		"ad-insight suggest-keywords-from-existing-keywords",
		"ad-insight tag-recommendations",
		"bulk",
		"bulk download-campaigns-by-account-ids",
		"bulk download-campaigns-by-campaign-ids",
		"bulk get-download-status",
		"bulk get-upload-status",
		"bulk get-upload-url",
		"bulk upload-entity-records",
		"campaign-management",
		"campaign-management add-ad-extensions",
		"campaign-management add-ad-group-criterions",
		"campaign-management add-ad-groups",
		"campaign-management add-ads",
		"campaign-management add-asset-groups",
		"campaign-management add-audience-groups",
		"campaign-management add-audiences",
		"campaign-management add-bid-strategies",
		"campaign-management add-brand-kits",
		"campaign-management add-budgets",
		"campaign-management add-campaign-conversion-goals",
		"campaign-management add-campaign-criterions",
		"campaign-management add-campaigns",
		"campaign-management add-conversion-goals",
		"campaign-management add-conversion-value-rules",
		"campaign-management add-data-exclusions",
		"campaign-management add-experiments",
		"campaign-management add-html5s",
		"campaign-management add-import-jobs",
		"campaign-management add-keywords",
		"campaign-management add-labels",
		"campaign-management add-linked-in-segments",
		"campaign-management add-list-items-to-shared-list",
		"campaign-management add-media",
		"campaign-management add-negative-keywords-to-entities",
		"campaign-management add-new-customer-acquisition-goals",
		"campaign-management add-seasonality-adjustments",
		"campaign-management add-shared-entity",
		"campaign-management add-uet-tags",
		"campaign-management add-videos",
		"campaign-management appeal-editorial-rejections",
		"campaign-management apply-asset-group-listing-group-actions",
		"campaign-management apply-customer-list-items",
		"campaign-management apply-customer-list-user-data",
		"campaign-management apply-hotel-group-actions",
		"campaign-management apply-offline-conversion-adjustments",
		"campaign-management apply-offline-conversions",
		"campaign-management apply-online-conversion-adjustments",
		"campaign-management apply-product-partition-actions",
		"campaign-management create-asset-group-recommendation",
		"campaign-management create-brand-kit-recommendation",
		"campaign-management create-responsive-ad-recommendation",
		"campaign-management create-responsive-search-ad-recommendation",
		"campaign-management delete-ad-extensions",
		"campaign-management delete-ad-extensions-associations",
		"campaign-management delete-ad-group-criterions",
		"campaign-management delete-ad-groups",
		"campaign-management delete-ads",
		"campaign-management delete-asset-groups",
		"campaign-management delete-audience-group-asset-group-associations",
		"campaign-management delete-audience-groups",
		"campaign-management delete-audiences",
		"campaign-management delete-bid-strategies",
		"campaign-management delete-brand-kits",
		"campaign-management delete-budgets",
		"campaign-management delete-campaign-conversion-goals",
		"campaign-management delete-campaign-criterions",
		"campaign-management delete-campaigns",
		"campaign-management delete-data-exclusions",
		"campaign-management delete-experiments",
		"campaign-management delete-html5s",
		"campaign-management delete-import-jobs",
		"campaign-management delete-keywords",
		"campaign-management delete-label-associations",
		"campaign-management delete-labels",
		"campaign-management delete-linked-in-segments",
		"campaign-management delete-list-items-from-shared-list",
		"campaign-management delete-media",
		"campaign-management delete-negative-keywords-from-entities",
		"campaign-management delete-seasonality-adjustments",
		"campaign-management delete-shared-entities",
		"campaign-management delete-shared-entity-associations",
		"campaign-management delete-videos",
		"campaign-management get-account-migration-statuses",
		"campaign-management get-account-properties",
		"campaign-management get-ad-extension-ids-by-account-id",
		"campaign-management get-ad-extensions-associations",
		"campaign-management get-ad-extensions-by-ids",
		"campaign-management get-ad-extensions-editorial-reasons",
		"campaign-management get-ad-group-criterions-by-ids",
		"campaign-management get-ad-groups-by-campaign-id",
		"campaign-management get-ad-groups-by-ids",
		"campaign-management get-ads-by-ad-group-id",
		"campaign-management get-ads-by-editorial-status",
		"campaign-management get-ads-by-ids",
		"campaign-management get-annotation-opt-out",
		"campaign-management get-asset-group-listing-groups-by-ids",
		"campaign-management get-asset-groups-by-campaign-id",
		"campaign-management get-asset-groups-by-ids",
		"campaign-management get-asset-groups-editorial-reasons",
		"campaign-management get-audience-group-asset-group-associations-by-asset-group-ids",
		"campaign-management get-audience-group-asset-group-associations-by-audience-group-ids",
		"campaign-management get-audience-groups-by-ids",
		"campaign-management get-audiences-by-ids",
		"campaign-management get-bid-strategies-by-ids",
		"campaign-management get-bmc-stores-by-customer-id",
		"campaign-management get-brand-kits-by-account-id",
		"campaign-management get-brand-kits-by-ids",
		"campaign-management get-bsc-countries",
		"campaign-management get-budgets-by-ids",
		"campaign-management get-campaign-criterions-by-ids",
		"campaign-management get-campaign-ids-by-bid-strategy-ids",
		"campaign-management get-campaign-ids-by-budget-ids",
		"campaign-management get-campaign-sizes-by-account-id",
		"campaign-management get-campaigns-by-account-id",
		"campaign-management get-campaigns-by-ids",
		"campaign-management get-clipchamp-templates",
		"campaign-management get-config-value",
		"campaign-management get-conversion-goals-by-ids",
		"campaign-management get-conversion-goals-by-tag-ids",
		"campaign-management get-conversion-value-rules-by-account-id",
		"campaign-management get-conversion-value-rules-by-ids",
		"campaign-management get-data-exclusions-by-account-id",
		"campaign-management get-data-exclusions-by-ids",
		"campaign-management get-diagnostics",
		"campaign-management get-editorial-reasons-by-ids",
		"campaign-management get-experiments-by-ids",
		"campaign-management get-file-import-upload-url",
		"campaign-management get-geo-locations-file-url",
		"campaign-management get-health-check",
		"campaign-management get-html5s-by-ids",
		"campaign-management get-import-entity-ids-mapping",
		"campaign-management get-import-jobs-by-ids",
		"campaign-management get-import-results",
		"campaign-management get-keywords-by-ad-group-id",
		"campaign-management get-keywords-by-editorial-status",
		"campaign-management get-keywords-by-ids",
		"campaign-management get-label-associations-by-entity-ids",
		"campaign-management get-label-associations-by-label-ids",
		"campaign-management get-labels-by-ids",
		"campaign-management get-list-items-by-shared-list",
		"campaign-management get-media-associations",
		"campaign-management get-media-meta-data-by-account-id",
		"campaign-management get-media-meta-data-by-ids",
		"campaign-management get-negative-keywords-by-entity-ids",
		"campaign-management get-negative-sites-by-ad-group-ids",
		"campaign-management get-negative-sites-by-campaign-ids",
		"campaign-management get-new-customer-acquisition-goals-by-account-id",
		"campaign-management get-offline-conversion-report-by-goal-ids",
		"campaign-management get-offline-conversion-reports",
		"campaign-management get-profile-data-file-url",
		"campaign-management get-responsive-ad-recommendation-job",
		"campaign-management get-seasonality-adjustments-by-account-id",
		"campaign-management get-seasonality-adjustments-by-ids",
		"campaign-management get-shared-entities",
		"campaign-management get-shared-entities-by-account-id",
		"campaign-management get-shared-entity-associations-by-entity-ids",
		"campaign-management get-shared-entity-associations-by-shared-entity-ids",
		"campaign-management get-supported-clipchamp-audio",
		"campaign-management get-supported-fonts",
		"campaign-management get-uet-tag-auth-key",
		"campaign-management get-uet-tags-by-ids",
		"campaign-management get-videos-by-ids",
		"campaign-management refine-asset-group-recommendation",
		"campaign-management refine-responsive-ad-recommendation",
		"campaign-management refine-responsive-search-ad-recommendation",
		"campaign-management search-companies",
		"campaign-management set-account-properties",
		"campaign-management set-ad-extensions-associations",
		"campaign-management set-audience-group-asset-group-associations",
		"campaign-management set-label-associations",
		"campaign-management set-negative-sites-to-ad-groups",
		"campaign-management set-negative-sites-to-campaigns",
		"campaign-management set-shared-entity-associations",
		"campaign-management update-ad-extensions",
		"campaign-management update-ad-group-criterions",
		"campaign-management update-ad-groups",
		"campaign-management update-ads",
		"campaign-management update-annotation-opt-out",
		"campaign-management update-asset-groups",
		"campaign-management update-audience-groups",
		"campaign-management update-audiences",
		"campaign-management update-bid-strategies",
		"campaign-management update-brand-kits",
		"campaign-management update-budgets",
		"campaign-management update-campaign-criterions",
		"campaign-management update-campaigns",
		"campaign-management update-conversion-goals",
		"campaign-management update-conversion-value-rules",
		"campaign-management update-conversion-value-rules-status",
		"campaign-management update-data-exclusions",
		"campaign-management update-experiments",
		"campaign-management update-import-jobs",
		"campaign-management update-keywords",
		"campaign-management update-labels",
		"campaign-management update-linked-in-segments",
		"campaign-management update-new-customer-acquisition-goals",
		"campaign-management update-seasonality-adjustments",
		"campaign-management update-shared-entities",
		"campaign-management update-uet-tags",
		"campaign-management update-videos",
		"customer-billing",
		"customer-billing add-insertion-order",
		"customer-billing check-feature-adoption-coupon-eligibility",
		"customer-billing claim-feature-adoption-coupons",
		"customer-billing dispatch-coupons",
		"customer-billing distribute-coupons",
		"customer-billing get-account-monthly-spend",
		"customer-billing get-billing-documents",
		"customer-billing get-billing-documents-info",
		"customer-billing get-billing-groups",
		"customer-billing get-coupon-info",
		"customer-billing get-ungrouped-accounts",
		"customer-billing redeem-coupon",
		"customer-billing search-coupons",
		"customer-billing search-insertion-orders",
		"customer-billing update-billing-group-accounts",
		"customer-billing update-insertion-order",
		"customer-management",
		"customer-management add-account",
		"customer-management add-client-links",
		"customer-management add-prepay-account",
		"customer-management delete-account",
		"customer-management delete-customer",
		"customer-management delete-user",
		"customer-management dismiss-notifications",
		"customer-management find-accounts",
		"customer-management find-accounts-or-customers-info",
		"customer-management get-accessible-customer",
		"customer-management get-account",
		"customer-management get-account-pilot-features",
		"customer-management get-accounts-info",
		"customer-management get-current-user",
		"customer-management get-customer",
		"customer-management get-customer-pilot-features",
		"customer-management get-customers-info",
		"customer-management get-linked-accounts-and-customers-info",
		"customer-management get-notifications",
		"customer-management get-pilot-features-countries",
		"customer-management get-user",
		"customer-management get-user-mfa-status",
		"customer-management get-users-info",
		"customer-management map-account-id-to-external-account-ids",
		"customer-management map-customer-id-to-external-customer-id",
		"customer-management search-accounts",
		"customer-management search-client-links",
		"customer-management search-customers",
		"customer-management search-user-invitations",
		"customer-management send-user-invitation",
		"customer-management signup-customer",
		"customer-management update-account",
		"customer-management update-client-links",
		"customer-management update-customer",
		"customer-management update-prepay-account",
		"customer-management update-user",
		"customer-management update-user-roles",
		"customer-management upgrade-customer-to-agency",
		"customer-management validate-address",
		"reporting",
		"reporting poll-generate-report",
		"reporting submit-generate-report",
	}
	actual := make(map[string]struct{}, len(expected))
	type pendingCommand struct {
		command *cobra.Command
		path    string
	}
	queue := make([]pendingCommand, 0, len(expected))
	for _, child := range RootCmd().Commands() {
		queue = append(queue, pendingCommand{command: child, path: child.Name()})
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		actual[current.path] = struct{}{}
		for _, child := range current.command.Commands() {
			queue = append(queue, pendingCommand{
				command: child,
				path:    strings.TrimSpace(current.path + " " + child.Name()),
			})
		}
	}

	var missing []string
	for _, commandPath := range expected {
		if _, ok := actual[commandPath]; !ok {
			missing = append(missing, commandPath)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("declared API command paths missing from generated Cobra tree: %s", strings.Join(missing, ", "))
	}
}

func TestNoDuplicateCommandNames(t *testing.T) {
	type pendingCommand struct {
		command *cobra.Command
		path    string
	}
	queue := []pendingCommand{}
	queue = append(queue, pendingCommand{command: RootCmd(), path: ""})
	var duplicates []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		seen := map[string]struct{}{}
		for _, child := range current.command.Commands() {
			childPath := strings.TrimSpace(current.path + " " + child.Name())
			if _, exists := seen[child.Name()]; exists {
				duplicates = append(duplicates, childPath)
			} else {
				seen[child.Name()] = struct{}{}
			}
			queue = append(queue, pendingCommand{command: child, path: childPath})
		}
	}
	if len(duplicates) > 0 {
		t.Fatalf("generated Cobra tree contains duplicate sibling command names: %s", strings.Join(duplicates, ", "))
	}
}
func TestWriteCredentialSaveErrorEnvelope(t *testing.T) {
	var out bytes.Buffer
	cause := &cliutil.CredentialsPermissionError{
		Path: "/tmp/credentials.toml",
		Err:  errors.New("unsafe permissions"),
	}
	if !writeCredentialSaveErrorEnvelope(&out, &rootFlags{asJSON: true}, fmt.Errorf("saving token: %w", cause)) {
		t.Fatal("permission failure envelope was not written")
	}

	var payload struct {
		Saved               bool   `json:"saved"`
		CredentialsPath     string `json:"credentials_path"`
		PermissionsVerified bool   `json:"permissions_verified"`
		Error               string `json:"error"`
		Code                int    `json:"code"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("permission failure envelope must be valid JSON: %v\n%s", err, out.String())
	}
	if !payload.Saved || payload.CredentialsPath != cause.Path || payload.PermissionsVerified || payload.Error == "" || payload.Code == 0 {
		t.Fatalf("permission failure envelope = %+v, want saved path, unsafe permissions, error, and non-zero code", payload)
	}
}

// TestIsCobraUsageError covers the six pre-RunE error shapes Cobra and
// pflag can produce before any user RunE runs. Each must be detected so
// the caller can wrap in usageErr() and yield exit code 2.
func TestIsCobraUsageError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		errIn error
		want  bool
	}{
		{"nil", nil, false},
		{"unknown flag", errors.New("unknown flag: --foob"), true},
		{"unknown shorthand flag", errors.New("unknown shorthand flag: 'x' in -x"), true},
		{"unknown command", errors.New("unknown command \"foo\" for \"cli\""), true},
		{"required flag (singular)", errors.New("required flag \"query\" not set"), true},
		{"required flag(s) (plural)", errors.New("required flag(s) \"query\", \"vault\" not set"), true},
		{"flag needs argument", errors.New("flag needs an argument: --query"), true},
		{"invalid argument", errors.New("invalid argument \"abc\" for \"--limit\" flag: strconv.ParseInt: parsing \"abc\": invalid syntax"), true},
		// Non-usage errors must NOT be flagged — they should retain their
		// original (or wrapped *cliError) exit code.
		{"API error pass-through", errors.New("API returned 500"), false},
		{"network error pass-through", errors.New("dial tcp: connection refused"), false},
		// Pattern-tightness regressions: application RunE errors that
		// happen to mention "required flag" / "flag needs an argument" /
		// "invalid argument" as prose must NOT classify as usage errors.
		// The anchored prefixes guard against this; if the patterns are
		// ever loosened back to Contains() these tests catch the drift.
		{"app msg containing 'required flag' as prose", errors.New("the required flag config is missing from the manifest"), false},
		{"app msg containing 'flag needs an argument' as prose", errors.New("this flag needs an argument upstream before retry"), false},
		{"app msg containing 'invalid argument' as prose", errors.New("invalid argument provided to handler at /foo"), false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isCobraUsageError(tc.errIn); got != tc.want {
				t.Errorf("isCobraUsageError(%v) = %v, want %v", tc.errIn, got, tc.want)
			}
		})
	}
}

// TestIsCobraUsageError_SurvivesHintRewrap covers the hint-suggestion
// path in Execute(): after rewrapping an "unknown flag" error with
// fmt.Errorf("%w\nhint: ...", err, ...), the rewrapped error must still
// classify as a usage error so the exit code stays at 2 rather than
// silently falling through to 1.
func TestIsCobraUsageError_SurvivesHintRewrap(t *testing.T) {
	t.Parallel()
	original := errors.New("unknown flag: --foob")
	wrapped := fmt.Errorf("%w\nhint: did you mean --foo?", original)
	if !isCobraUsageError(wrapped) {
		t.Errorf("hint-rewrapped unknown-flag error must still classify as usage error, got: %v", wrapped)
	}
}

// TestExitCode_UsageError_WrappedAsCode2 covers the end-to-end contract:
// after Execute() wraps a Cobra usage error in usageErr(), ExitCode()
// must return 2, not 1. This is the user-visible promise — a bad flag
// exits 2 (POSIX usage convention), not the generic-error-1 fallback.
func TestExitCode_UsageError_WrappedAsCode2(t *testing.T) {
	t.Parallel()
	wrapped := usageErr(errors.New("unknown flag: --foob"))
	if got := ExitCode(wrapped); got != 2 {
		t.Errorf("ExitCode(usageErr(...)) = %d, want 2 (POSIX usage convention)", got)
	}
}

// TestFilterFields covers --select projection against the four payload
// shapes printed CLIs see in practice: bare arrays, direct objects,
// list envelopes (Stripe/GitHub/Notion-style wrapper + array), and
// flat objects. The envelope cases guard against a regression where
// wrapper-key + array responses returned `{}` because the selector
// heads matched the inner record fields, not the wrapper key.
func TestFilterFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		input  string
		fields string
		want   string
	}{
		{
			name:   "bare array element-wise",
			input:  `[{"id":"a","name":"x","other":"y"},{"id":"b","name":"z","other":"w"}]`,
			fields: "id,name",
			want:   `[{"id":"a","name":"x"},{"id":"b","name":"z"}]`,
		},
		{
			name:   "direct object top-level match",
			input:  `{"id":"a","name":"b","other":"c"}`,
			fields: "id,name",
			want:   `{"id":"a","name":"b"}`,
		},
		{
			name:   "envelope single array sibling (orgo {projects:[...]})",
			input:  `{"projects":[{"id":"a","name":"x","other":"y"}]}`,
			fields: "id,name",
			want:   `{"projects":[{"id":"a","name":"x"}]}`,
		},
		{
			name:   "envelope with metadata sibling (github {total_count,items:[...]})",
			input:  `{"total_count":2,"items":[{"id":"a","name":"x","other":"y"},{"id":"b","name":"z","other":"w"}]}`,
			fields: "id,name",
			want:   `{"items":[{"id":"a","name":"x"},{"id":"b","name":"z"}],"total_count":2}`,
		},
		{
			name:   "envelope with metadata sibling (stripe {object,data:[...]})",
			input:  `{"object":"list","data":[{"id":"a","name":"x","other":"y"}]}`,
			fields: "id,name",
			want:   `{"data":[{"id":"a","name":"x"}],"object":"list"}`,
		},
		{
			name:   "selector matches envelope key (no descent into array)",
			input:  `{"projects":[{"id":"a","name":"x","other":"y"}]}`,
			fields: "projects",
			want:   `{"projects":[{"id":"a","name":"x","other":"y"}]}`,
		},
		{
			name:   "dotted-path through envelope key still descends",
			input:  `{"projects":[{"id":"a","name":"x","other":"y"}]}`,
			fields: "projects.id",
			want:   `{"projects":[{"id":"a"}]}`,
		},
		{
			name:   "flat object no match preserves input",
			input:  `{"a":1,"b":2}`,
			fields: "c",
			want:   `{"a":1,"b":2}`,
		},
		{
			name:   "unknown selector preserves nested array objects",
			input:  `{"items":[{"id":"a","name":"Alpha"},{"id":"b","name":"Beta"}]}`,
			fields: "missing",
			want:   `{"items":[{"id":"a","name":"Alpha"},{"id":"b","name":"Beta"}]}`,
		},
		{
			// Null pagination cursors are common envelope metadata.
			// json.Unmarshal accepts JSON null into []json.RawMessage as
			// a nil slice without error, so the array check must reject
			// nil explicitly or null siblings would be coerced to `[]`.
			name:   "envelope preserves null sibling verbatim",
			input:  `{"items":[{"id":"a","name":"x"}],"next_cursor":null}`,
			fields: "id,name",
			want:   `{"items":[{"id":"a","name":"x"}],"next_cursor":null}`,
		},
		{
			// Without a real array sibling the envelope fallback does not
			// fire, but an invalid selector still preserves the input.
			name:   "flat object with null sibling no match preserves input",
			input:  `{"a":1,"b":null}`,
			fields: "c",
			want:   `{"a":1,"b":null}`,
		},
		{
			// Multiple array siblings at the same level each receive the
			// selector independently. Documents that the projection fans
			// out across every array, not just the first one found.
			name:   "envelope with two array siblings filters both",
			input:  `{"events":[{"id":"e1","other":"x"}],"speakers":[{"id":"s1","other":"y"}]}`,
			fields: "id",
			want:   `{"events":[{"id":"e1"}],"speakers":[{"id":"s1"}]}`,
		},
		{
			// Generic object descent supports type-keyed envelopes such as
			// {"data":{"items":[...]}} while keeping the fail-closed
			// behavior for objects with no collection below them.
			name:   "nested object envelope descends into collection",
			input:  `{"data":{"items":[{"id":"a","other":"y"}]}}`,
			fields: "id",
			want:   `{"data":{"items":[{"id":"a"}]}}`,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filterFields(json.RawMessage(tc.input), tc.fields)
			// Normalize both sides through json.Unmarshal+Marshal so
			// map-iteration order does not produce false negatives.
			var gotV, wantV interface{}
			if err := json.Unmarshal(got, &gotV); err != nil {
				t.Fatalf("got is invalid json: %v (raw=%s)", err, string(got))
			}
			if err := json.Unmarshal([]byte(tc.want), &wantV); err != nil {
				t.Fatalf("want is invalid json: %v (raw=%s)", err, tc.want)
			}
			gotBytes, _ := json.Marshal(gotV)
			wantBytes, _ := json.Marshal(wantV)
			if string(gotBytes) != string(wantBytes) {
				t.Errorf("filterFields(%q, %q) = %s, want %s",
					tc.input, tc.fields, string(gotBytes), string(wantBytes))
			}
		})
	}
}
