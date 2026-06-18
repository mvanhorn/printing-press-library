// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const defaultRonanRxRoute = "WELCOME_FLOW"

var (
	e164RE           = regexp.MustCompile(`^\+[1-9]\d{7,14}$`)
	opaqueTokenRE    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~-]{7,127}$`)
	dateLikeRE       = regexp.MustCompile(`(?i)\b(?:\d{1,2}[/-]\d{1,2}[/-]\d{2,4}|\d{4}-\d{2}-\d{2}|(?:jan|feb|mar|apr|may|jun|jul|aug|sep|sept|oct|nov|dec)[a-z]*\.?\s+\d{1,2},?\s+\d{2,4})\b`)
	dollarLikeRE     = regexp.MustCompile(`(?i)(?:\$|usd\s*)\d+(?:[.,]\d{2})?\b`)
	doseLikeRE       = regexp.MustCompile(`(?i)\b\d+(?:\.\d+)?\s*(?:mg|mcg|g|ml|iu|units?|tabs?|tablets?|caps?|capsules?)\b`)
	addressLikeRE    = regexp.MustCompile(`(?i)\b\d{1,6}\s+[A-Za-z0-9.'-]+(?:\s+[A-Za-z0-9.'-]+){0,4}\s+(?:st|street|ave|avenue|rd|road|dr|drive|ln|lane|blvd|boulevard|way|ct|court|pl|place)\b`)
	fullNameLikeRE   = regexp.MustCompile(`\b[A-Z][a-z]{2,}\s+[A-Z][a-z]{2,}\b`)
	emailLikeRE      = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	phoneLikeRE      = regexp.MustCompile(`(?:\+\d[\d .()\-]{7,}\d|\(\d{3}\) ?\d{3}[ -]?\d{4}|\b\d{3}[ -]\d{3}[ -]\d{4}\b)`)
	bearerTokenRE    = regexp.MustCompile(`(?i)\b(?:bearer|api[_-]?key|secret|password|token=)[A-Za-z0-9._~+/=-]{8,}`)
	htmlScriptSrcRE  = regexp.MustCompile(`(?is)<script[^>]+src=["']([^"']+)["']`)
	htmlMetaRE       = regexp.MustCompile(`(?is)<meta[^>]+(?:property|name)=["'](?:og:title|og:description|twitter:title|twitter:description|description)["'][^>]+content=["']([^"']*)["']`)
	htmlButtonRE     = regexp.MustCompile(`(?is)<(?:a|button)\b[^>]*(?:sms:|id=["'][^"']*(?:sms|text|fallback)[^"']*["'])`)
	htmlQueryParamRE = regexp.MustCompile(`(?i)\b(?:from|text)\b`)
)

var safeRoutingIDs = map[string]bool{
	defaultRonanRxRoute: true,
}

var sensitiveURLKeys = map[string]bool{
	"address": true, "birthdate": true, "diagnosis": true, "dob": true, "dose": true,
	"drug": true, "email": true, "lab": true, "med": true, "medication": true,
	"name": true, "patient": true, "phone": true, "price": true,
}

var drugOrDiagnosisTerms = []string{
	"adderall", "amoxicillin", "anxiety", "atorvastatin", "cancer", "depression",
	"diabetes", "diagnosis", "hiv", "hypertension", "insulin", "lab result",
	"lisinopril", "metformin", "mounjaro", "ozempic", "pregnancy", "semaglutide",
	"wegovy", "zepbound",
}

var commonNameTokens = map[string]bool{
	"alice": true, "bob": true, "david": true, "john": true, "jane": true,
	"maria": true, "mary": true, "michael": true, "patient": true, "smith": true,
}

type inviteLinkOptions struct {
	BaseURL                string
	FrontDoorPath          string
	From                   string
	Routing                string
	Token                  string
	PrefillBody            string
	PatientAuthoredPrefill bool
	DevMode                bool
}

type inviteLinkResult struct {
	FrontDoorLink            string                `json:"front_door_link"`
	InviteLink               string                `json:"invite_link"`
	InviteLinkDeprecated     bool                  `json:"invite_link_deprecated"`
	SMSURIPreview            string                `json:"sms_uri_preview"`
	Recipient                string                `json:"recipient"`
	PrefillBody              string                `json:"prefill_body"`
	PatientAction            string                `json:"patient_action"`
	FirstSender              string                `json:"first_sender"`
	OutboundSendPerformed    bool                  `json:"outbound_send_performed"`
	PointerNotPayload        bool                  `json:"pointer_not_payload"`
	PHIInBody                bool                  `json:"phi_in_body"`
	PHIInURL                 bool                  `json:"phi_in_url"`
	RequiresFrontDoorHandoff bool                  `json:"requires_front_door_handoff"`
	FrontDoorContract        frontDoorContract     `json:"front_door_contract"`
	FrontDoorCheck           *frontDoorCheckResult `json:"front_door_check,omitempty"`
	Warnings                 []string              `json:"warnings"`
	Errors                   []string              `json:"errors"`
}

type frontDoorContract struct {
	FrontDoorPublishesHTTPSURL           bool     `json:"front_door_publishes_https_url"`
	BrowserMustBuildSMSURI               bool     `json:"browser_must_build_sms_uri"`
	RequiredQueryParams                  []string `json:"required_query_params"`
	SafeQueryParams                      []string `json:"safe_query_params"`
	RecipientParam                       string   `json:"recipient_param"`
	BodyParam                            string   `json:"body_param"`
	SMSURIScheme                         string   `json:"sms_uri_scheme"`
	CanonicalSMSURIPreview               string   `json:"canonical_sms_uri_preview"`
	IOSBodySeparator                     string   `json:"ios_body_separator"`
	AndroidBodySeparator                 string   `json:"android_body_separator"`
	FallbackButtonRequired               bool     `json:"fallback_button_required"`
	AutoredirectAllowedWithFallback      bool     `json:"autoredirect_allowed_with_fallback"`
	MustNotInlinePHIOrTokenInPreviewMeta bool     `json:"must_not_inline_phi_or_token_in_preview_meta"`
	ExpectedPatientAction                string   `json:"expected_patient_action"`
}

type frontDoorCheckResult struct {
	URL                              string            `json:"url"`
	Checked                          bool              `json:"checked"`
	FetchAttempted                   bool              `json:"fetch_attempted"`
	HTTPStatus                       int               `json:"http_status,omitempty"`
	Strategy                         string            `json:"strategy"`
	BrowserCheckPluggable            bool              `json:"browser_check_pluggable"`
	ReferencesFromAndTextParams      bool              `json:"references_from_and_text_params"`
	SameOriginJSOrMarkupBuildsSMSURI bool              `json:"same_origin_js_or_markup_builds_sms_uri"`
	NoPHIInline                      bool              `json:"no_phi_inline"`
	NoTokenInPreviewMeta             bool              `json:"no_token_in_preview_meta"`
	HasFallbackButton                bool              `json:"has_fallback_button"`
	DistinguishesPlatformSeparators  bool              `json:"distinguishes_platform_separators"`
	IgnoresSafeParams                bool              `json:"ignores_safe_params"`
	Allowed                          bool              `json:"allowed"`
	Warnings                         []string          `json:"warnings"`
	Errors                           []string          `json:"errors"`
	Checks                           map[string]string `json:"checks"`
}

type sendPreflightOptions struct {
	Mode             string
	AllowedHosts     []string
	InboundMessageID string
	InboundAt        string
	LocalMessages    []map[string]any
}

type sendGuardResult struct {
	Mode            string            `json:"mode,omitempty"`
	WouldSend       bool              `json:"would_send"`
	Allowed         bool              `json:"allowed"`
	Reasons         []string          `json:"reasons,omitempty"`
	BlockingReasons []string          `json:"blocking_reasons"`
	Evidence        map[string]any    `json:"evidence"`
	Checks          map[string]string `json:"checks"`
}

type phiFinding struct {
	Contains bool
	Reasons  []string
}

func buildInviteURL(base, from, routing, token string) (*url.URL, error) {
	result, err := buildInviteSchema(inviteLinkOptions{
		BaseURL: base,
		From:    from,
		Routing: routing,
		Token:   token,
	})
	if err != nil {
		return nil, err
	}
	return url.Parse(result.FrontDoorLink)
}

func buildInviteSchema(opts inviteLinkOptions) (*inviteLinkResult, error) {
	errorsOut := []string{}
	warnings := []string{}
	routing := strings.TrimSpace(opts.Routing)
	token := strings.TrimSpace(opts.Token)
	from := strings.TrimSpace(opts.From)
	if !validateE164(from) {
		errorsOut = append(errorsOut, "--from must be an E.164 phone number such as +16282893046")
	}
	if err := validateRouting(routing); err != nil {
		errorsOut = append(errorsOut, err.Error())
	}
	if err := validateOpaqueToken(token); err != nil {
		errorsOut = append(errorsOut, err.Error())
	}
	body := strings.TrimSpace(opts.PrefillBody)
	if body == "" {
		body = routing + " " + token
	} else if !opts.PatientAuthoredPrefill {
		errorsOut = append(errorsOut, "--prefill-body is allowed only with --patient-authored-prefill because it is the patient's first inbound message")
	}
	bodyPHI := analyzePHI(body)
	if bodyPHI.Contains {
		errorsOut = append(errorsOut, "prefill body contains PHI-shaped content: "+strings.Join(bodyPHI.Reasons, ", "))
	}
	frontDoor, err := normalizeFrontDoorURL(opts.BaseURL, opts.FrontDoorPath, from, body, opts.DevMode)
	if err != nil {
		errorsOut = append(errorsOut, err.Error())
	}
	if len(errorsOut) > 0 {
		return nil, errors.New(strings.Join(errorsOut, "; "))
	}
	if opts.FrontDoorPath == "" {
		if parsed, _ := url.Parse(opts.BaseURL); parsed != nil && strings.Trim(parsed.Path, "/") == "start" {
			warnings = append(warnings, "base URL path /start/ looks generic; publish only if that page performs the sms: handoff, or pass --front-door-path /start/text/")
		}
	}
	urlPHI := analyzeURLPHI(frontDoor, from)
	smsURI := buildSMSURI(from, body)
	return &inviteLinkResult{
		FrontDoorLink:            frontDoor.String(),
		InviteLink:               frontDoor.String(),
		InviteLinkDeprecated:     true,
		SMSURIPreview:            smsURI,
		Recipient:                from,
		PrefillBody:              body,
		PatientAction:            "patient_taps_link_then_sends_prefilled_message",
		FirstSender:              "patient",
		OutboundSendPerformed:    false,
		PointerNotPayload:        !bodyPHI.Contains && !urlPHI.Contains,
		PHIInBody:                bodyPHI.Contains,
		PHIInURL:                 urlPHI.Contains,
		RequiresFrontDoorHandoff: true,
		FrontDoorContract: frontDoorContract{
			FrontDoorPublishesHTTPSURL:           true,
			BrowserMustBuildSMSURI:               true,
			RequiredQueryParams:                  []string{"from", "text"},
			SafeQueryParams:                      []string{"from", "text"},
			RecipientParam:                       "from",
			BodyParam:                            "text",
			SMSURIScheme:                         "sms",
			CanonicalSMSURIPreview:               smsURI,
			IOSBodySeparator:                     "?&body=",
			AndroidBodySeparator:                 "?body=",
			FallbackButtonRequired:               true,
			AutoredirectAllowedWithFallback:      true,
			MustNotInlinePHIOrTokenInPreviewMeta: true,
			ExpectedPatientAction:                "patient_taps_link_then_sends_prefilled_message",
		},
		Warnings: warnings,
		Errors:   []string{},
	}, nil
}

func normalizeFrontDoorURL(base, frontDoorPath, from, body string, devMode bool) (*url.URL, error) {
	if strings.TrimSpace(base) == "" {
		return nil, fmt.Errorf("--base-url is required")
	}
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("--base-url must be an absolute URL")
	}
	if u.Scheme != "https" {
		if !(devMode && (u.Scheme == "http") && isLocalhost(u.Hostname())) {
			return nil, fmt.Errorf("--base-url must use https unless --dev-mode is set for localhost")
		}
	}
	if frontDoorPath != "" {
		if strings.Contains(frontDoorPath, "?") || strings.Contains(frontDoorPath, "#") {
			return nil, fmt.Errorf("--front-door-path must be a path only, without query or fragment")
		}
		if !strings.HasPrefix(frontDoorPath, "/") {
			return nil, fmt.Errorf("--front-door-path must start with /")
		}
		u.Path = frontDoorPath
		u.RawQuery = ""
		u.Fragment = ""
	}
	q := u.Query()
	q.Set("from", from)
	q.Set("text", body)
	u.RawQuery = q.Encode()
	return u, nil
}

func buildSMSURI(recipient, body string) string {
	return "sms:" + recipient + "?&body=" + url.QueryEscape(body)
}

func validateE164(s string) bool {
	return e164RE.MatchString(strings.TrimSpace(s))
}

func validateRouting(routing string) error {
	if routing == "" {
		return fmt.Errorf("--routing is required")
	}
	if !safeRoutingIDs[routing] {
		return fmt.Errorf("--routing must be an allowlisted non-PHI route ID such as %s", defaultRonanRxRoute)
	}
	if finding := analyzePHI(routing); finding.Contains {
		return fmt.Errorf("--routing contains PHI-shaped content: %s", strings.Join(finding.Reasons, ", "))
	}
	return nil
}

func validateOpaqueToken(token string) error {
	if token == "" {
		return fmt.Errorf("--token is required")
	}
	if !opaqueTokenRE.MatchString(token) {
		return fmt.Errorf("--token must be an opaque 8-128 character value using only letters, numbers, dot, underscore, tilde, or dash")
	}
	lower := strings.ToLower(token)
	for name := range commonNameTokens {
		if lower == name || strings.Contains(lower, name+"-") || strings.Contains(lower, "-"+name) || strings.Contains(lower, name+"_") || strings.Contains(lower, "_"+name) {
			return fmt.Errorf("--token looks like a name or patient descriptor")
		}
	}
	if finding := analyzePHI(token); finding.Contains {
		return fmt.Errorf("--token contains PHI-shaped content: %s", strings.Join(finding.Reasons, ", "))
	}
	return nil
}

func analyzeURLPHI(u *url.URL, allowedFrom string) phiFinding {
	if u == nil {
		return phiFinding{Contains: true, Reasons: []string{"invalid_url"}}
	}
	reasons := []string{}
	component := strings.Join([]string{u.Hostname(), u.EscapedPath(), u.Fragment}, " ")
	if finding := analyzePHI(component); finding.Contains {
		reasons = append(reasons, finding.Reasons...)
	}
	for key, vals := range u.Query() {
		if isSensitiveURLKey(key) {
			reasons = append(reasons, "sensitive_query_key:"+key)
		}
		for _, v := range vals {
			if key == "from" && v == allowedFrom {
				continue
			}
			if finding := analyzePHI(v); finding.Contains {
				reasons = append(reasons, finding.Reasons...)
			}
		}
	}
	return dedupeFinding(reasons)
}

func analyzePHI(s string) phiFinding {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return phiFinding{}
	}
	reasons := []string{}
	checks := []struct {
		name string
		re   *regexp.Regexp
	}{
		{"email", emailLikeRE},
		{"phone", phoneLikeRE},
		{"date_or_dob", dateLikeRE},
		{"dollar_amount", dollarLikeRE},
		{"drug_or_dose", doseLikeRE},
		{"address", addressLikeRE},
		{"full_name", fullNameLikeRE},
		{"secret_or_token_value", bearerTokenRE},
	}
	for _, check := range checks {
		if check.re.MatchString(trimmed) {
			reasons = append(reasons, check.name)
		}
	}
	lower := strings.ToLower(trimmed)
	for _, term := range drugOrDiagnosisTerms {
		if strings.Contains(lower, term) {
			reasons = append(reasons, "drug_or_diagnosis:"+term)
		}
	}
	return dedupeFinding(reasons)
}

func dedupeFinding(reasons []string) phiFinding {
	if len(reasons) == 0 {
		return phiFinding{}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		reason = strings.TrimSpace(reason)
		if reason == "" || seen[reason] {
			continue
		}
		seen[reason] = true
		out = append(out, reason)
	}
	sort.Strings(out)
	return phiFinding{Contains: len(out) > 0, Reasons: out}
}

func isSensitiveURLKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if sensitiveURLKeys[key] {
		return true
	}
	for sensitive := range sensitiveURLKeys {
		if strings.Contains(key, sensitive) {
			return true
		}
	}
	return false
}

func isLocalhost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func inheritedAllowedHosts() []string {
	raw := strings.TrimSpace(firstNonEmptyEnv("RONANRX_LINK_ALLOW_HOSTS", "LINQ_LINK_ALLOW_HOSTS"))
	if raw == "" {
		return nil
	}
	var hosts []string
	for _, part := range strings.Split(raw, ",") {
		if h := strings.TrimSpace(part); h != "" {
			hosts = append(hosts, h)
		}
	}
	return hosts
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

var getenv = os.Getenv

func evaluateSendGuard(chatID, routing, link string) sendGuardResult {
	return evaluateSendPreflight(chatID, routing, link, sendPreflightOptions{Mode: "real"})
}

func evaluateSendPreflight(chatID, routing, link string, opts sendPreflightOptions) sendGuardResult {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode == "" {
		mode = "real"
	}
	checks := map[string]string{}
	evidence := map[string]any{}
	reasons := []string{}
	add := func(name string, ok bool, reason string) {
		if ok {
			checks[name] = "pass"
			return
		}
		checks[name] = "fail: " + reason
		reasons = append(reasons, reason)
	}
	add("mode", mode == "synthetic" || mode == "demo" || mode == "real", "mode must be synthetic, demo, or real")
	placeholder := isPlaceholderChatID(chatID)
	evidence["placeholder_chat_id"] = placeholder
	if mode == "real" {
		add("chat_id", strings.TrimSpace(chatID) != "" && !placeholder, "real mode requires a non-placeholder inbound chat ID")
	} else {
		add("chat_id", strings.TrimSpace(chatID) != "", "chat ID placeholder is required in synthetic/demo mode")
	}
	add("routing", strings.TrimSpace(routing) != "", "routing text is required")
	add("link", strings.TrimSpace(link) != "", "opaque link is required")
	add("inbound_marker", strings.Contains(strings.ToUpper(routing), "INBOUND_OK"), "routing must include INBOUND_OK as a preflight marker")
	add("opt_out", !strings.Contains(strings.ToUpper(routing), "OPTED_OUT"), "recipient appears opted out")
	phiFinding := analyzePHI(routing + " " + link)
	add("phi_payload", !phiFinding.Contains, "message body or link contains PHI-shaped content: "+strings.Join(phiFinding.Reasons, ", "))

	hosts := append([]string{}, inheritedAllowedHosts()...)
	hosts = append(hosts, opts.AllowedHosts...)
	linkAudit := auditLink(link, hosts)
	linkAllowed, _ := linkAudit["allowed"].(bool)
	add("link_audit", linkAllowed, "pointer link failed link-audit")
	if len(hosts) == 0 {
		add("link_host_allowlist", mode != "real", "real mode requires --allow-host or RONANRX_LINK_ALLOW_HOSTS/LINQ_LINK_ALLOW_HOSTS")
	} else {
		checks["link_host_allowlist"] = "pass"
	}

	localEvidence := consentEvidence(chatID, opts.LocalMessages)
	evidence["local"] = localEvidence
	evidence["inbound_message_id"] = opts.InboundMessageID
	evidence["inbound_at"] = opts.InboundAt
	hasExplicitEvidence := strings.TrimSpace(opts.InboundMessageID) != "" || strings.TrimSpace(opts.InboundAt) != ""
	hasLocalEvidence, _ := localEvidence["inbound_first_satisfied_by_local_evidence"].(bool)
	evidence["explicit_preflight_evidence"] = hasExplicitEvidence
	if mode == "real" {
		add("inbound_evidence", hasLocalEvidence || hasExplicitEvidence, "real mode requires local inbound evidence or explicit inbound evidence flags; INBOUND_OK alone is not enough")
	} else {
		checks["inbound_evidence"] = "skip: synthetic/demo mode does not require live inbound evidence"
	}
	return sendGuardResult{
		Mode:            mode,
		WouldSend:       false,
		Allowed:         len(reasons) == 0,
		Reasons:         reasons,
		BlockingReasons: reasons,
		Evidence:        evidence,
		Checks:          checks,
	}
}

func isPlaceholderChatID(chatID string) bool {
	s := strings.ToLower(strings.TrimSpace(chatID))
	return s == "" || strings.Contains(s, "<") || strings.Contains(s, "placeholder") || strings.Contains(s, "demo") || strings.Contains(s, "synthetic") || s == "chat_id"
}

func auditLink(raw string, allowedHosts []string) map[string]any {
	checks := map[string]string{}
	reasons := []string{}
	sensitiveKeys := []string{}
	add := func(name string, ok bool, reason string) {
		if ok {
			checks[name] = "pass"
		} else {
			checks[name] = "fail: " + reason
			reasons = append(reasons, reason)
		}
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	add("parse", err == nil && u != nil && u.Host != "", "not a valid absolute URL")
	add("https", err == nil && u.Scheme == "https", "URL must use https")
	if err == nil && u != nil {
		hostAllowed := len(allowedHosts) == 0
		for _, h := range allowedHosts {
			h = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(h)), ".")
			host := strings.ToLower(u.Hostname())
			if host == h || strings.HasSuffix(host, "."+h) {
				hostAllowed = true
				break
			}
		}
		if len(allowedHosts) > 0 {
			add("allowlisted_host", hostAllowed, "host is not allowlisted")
		} else {
			checks["allowlisted_host"] = "skip: pass --allow-host to enforce RonanRx host policy"
		}
		for key := range u.Query() {
			if isSensitiveURLKey(key) {
				sensitiveKeys = append(sensitiveKeys, key)
			}
		}
		sort.Strings(sensitiveKeys)
		add("no_sensitive_query_keys", len(sensitiveKeys) == 0, "URL uses sensitive query key names")
		phiFinding := analyzeURLPHI(u, u.Query().Get("from"))
		add("no_phi_url", !phiFinding.Contains, "URL contains PHI-shaped data: "+strings.Join(phiFinding.Reasons, ", "))
		add("opaque_token_shape", urlTokensOpaque(u), "URL token/path payload must be opaque, not readable patient data")
	} else {
		add("no_sensitive_query_keys", false, "URL could not be parsed")
		add("no_phi_url", false, "URL could not be parsed")
		add("opaque_token_shape", false, "URL could not be parsed")
	}
	return map[string]any{
		"link":                 raw,
		"allowed":              len(reasons) == 0,
		"pointer_not_payload":  len(reasons) == 0,
		"phi_in_url":           containsReason(reasons, "PHI-shaped"),
		"sensitive_query_keys": sensitiveKeys,
		"reasons":              reasons,
		"checks":               checks,
	}
}

func auditLinkPreview(ctx context.Context, raw string, result map[string]any) map[string]any {
	checks, _ := result["checks"].(map[string]string)
	if checks == nil {
		checks = map[string]string{}
		result["checks"] = checks
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		checks["link_preview_fetch"] = "fail: " + err.Error()
		result["allowed"] = false
		result["pointer_not_payload"] = false
		result["reasons"] = appendReason(result["reasons"], "link preview fetch failed")
		return result
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		checks["link_preview_fetch"] = "warn: " + err.Error()
		result["preview_fetch_warning"] = err.Error()
		return result
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	html := string(body)
	checks["link_preview_fetch"] = "pass"
	previewValues := []string{}
	for _, match := range htmlMetaRE.FindAllStringSubmatch(html, -1) {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			previewValues = append(previewValues, match[1])
		}
	}
	previewText := strings.Join(previewValues, " ")
	finding := analyzePHI(previewText)
	if finding.Contains {
		checks["link_preview_generic"] = "fail: preview meta contains PHI-shaped content"
		result["allowed"] = false
		result["pointer_not_payload"] = false
		result["reasons"] = appendReason(result["reasons"], "link preview meta contains PHI-shaped content: "+strings.Join(finding.Reasons, ", "))
	} else {
		checks["link_preview_generic"] = "pass"
	}
	result["preview_meta_checked"] = len(previewValues)
	result["http_status"] = resp.StatusCode
	return result
}

func appendReason(raw any, reason string) []string {
	out, _ := raw.([]string)
	out = append(out, reason)
	return out
}

func urlTokensOpaque(u *url.URL) bool {
	candidates := []string{}
	for _, segment := range strings.Split(strings.Trim(u.EscapedPath(), "/"), "/") {
		if segment != "" {
			if decoded, err := url.PathUnescape(segment); err == nil {
				candidates = append(candidates, decoded)
			}
		}
	}
	for key, vals := range u.Query() {
		if key == "from" || key == "text" {
			continue
		}
		for _, v := range vals {
			candidates = append(candidates, v)
		}
	}
	for _, candidate := range candidates {
		if candidate == "" || strings.EqualFold(candidate, "start") || strings.EqualFold(candidate, "text") || strings.EqualFold(candidate, "t") {
			continue
		}
		if analyzePHI(candidate).Contains {
			return false
		}
		if strings.Contains(candidate, " ") && !opaqueTokenRE.MatchString(strings.ReplaceAll(candidate, " ", "-")) {
			return false
		}
	}
	return true
}

func containsReason(reasons []string, needle string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, needle) {
			return true
		}
	}
	return false
}

func inspectFrontDoor(ctx context.Context, rawURL, expectedToken string, fetch bool) frontDoorCheckResult {
	result := frontDoorCheckResult{
		URL:                   rawURL,
		Strategy:              "static_html_fetch",
		BrowserCheckPluggable: true,
		Checks:                map[string]string{},
		NoPHIInline:           true,
		NoTokenInPreviewMeta:  true,
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		result.Errors = append(result.Errors, "url must be an absolute front-door URL")
		result.Allowed = false
		result.Checks["parse"] = "fail: invalid URL"
		return result
	}
	result.Checks["parse"] = "pass"
	if !fetch {
		result.Warnings = append(result.Warnings, "network fetch disabled; static page contract was not verified")
		result.Allowed = false
		return result
	}
	result.FetchAttempted = true
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		result.Allowed = false
		return result
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		result.Warnings = append(result.Warnings, "front-door fetch failed: "+err.Error())
		result.Allowed = false
		return result
	}
	defer resp.Body.Close()
	result.HTTPStatus = resp.StatusCode
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	html := string(body)
	lower := strings.ToLower(html)
	result.Checked = true
	result.ReferencesFromAndTextParams = strings.Contains(lower, "from") && strings.Contains(lower, "text")
	result.HasFallbackButton = htmlButtonRE.MatchString(html)
	result.DistinguishesPlatformSeparators = strings.Contains(lower, "?&body=") && strings.Contains(lower, "?body=")
	result.SameOriginJSOrMarkupBuildsSMSURI = strings.Contains(lower, "sms:") || sameOriginScriptReferences(html, u)
	result.IgnoresSafeParams = !result.ReferencesFromAndTextParams
	if finding := analyzePHI(html); finding.Contains {
		result.NoPHIInline = false
		result.Errors = append(result.Errors, "HTML contains PHI-shaped content: "+strings.Join(finding.Reasons, ", "))
	}
	for _, meta := range htmlMetaRE.FindAllStringSubmatch(html, -1) {
		if len(meta) > 1 && expectedToken != "" && strings.Contains(meta[1], expectedToken) {
			result.NoTokenInPreviewMeta = false
			result.Errors = append(result.Errors, "OpenGraph/Twitter/description meta exposes token value")
		}
	}
	if !result.ReferencesFromAndTextParams {
		result.Warnings = append(result.Warnings, "page does not appear to read from/text query params")
	}
	if !result.SameOriginJSOrMarkupBuildsSMSURI {
		result.Warnings = append(result.Warnings, "page does not show same-origin JS or markup capable of building an sms: URI")
	}
	if !result.HasFallbackButton {
		result.Warnings = append(result.Warnings, "page does not expose a detectable fallback button for blocked redirects")
	}
	if !result.DistinguishesPlatformSeparators {
		result.Warnings = append(result.Warnings, "page does not visibly distinguish iOS ?&body= from Android ?body= separator behavior")
	}
	result.Checks["references_from_text"] = passFail(result.ReferencesFromAndTextParams, "page ignores safe params")
	result.Checks["same_origin_sms_builder"] = passFail(result.SameOriginJSOrMarkupBuildsSMSURI, "missing sms builder")
	result.Checks["no_phi_inline"] = passFail(result.NoPHIInline, "inline PHI-shaped content detected")
	result.Checks["no_token_preview_meta"] = passFail(result.NoTokenInPreviewMeta, "token appears in preview meta")
	result.Checks["fallback_button"] = passFail(result.HasFallbackButton, "fallback button not detected")
	result.Checks["platform_separators"] = passFail(result.DistinguishesPlatformSeparators, "platform separator distinction not detected")
	result.Allowed = len(result.Errors) == 0 && result.ReferencesFromAndTextParams && result.SameOriginJSOrMarkupBuildsSMSURI && result.HasFallbackButton
	return result
}

func sameOriginScriptReferences(html string, pageURL *url.URL) bool {
	for _, match := range htmlScriptSrcRE.FindAllStringSubmatch(html, -1) {
		if len(match) < 2 {
			continue
		}
		src, err := url.Parse(match[1])
		if err != nil {
			continue
		}
		if src.IsAbs() && !strings.EqualFold(src.Hostname(), pageURL.Hostname()) {
			continue
		}
		return true
	}
	return false
}

func passFail(ok bool, reason string) string {
	if ok {
		return "pass"
	}
	return "fail: " + reason
}
