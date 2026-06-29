package insurance

import "strings"

// FillField is one step in a fill plan: a value to enter, or a human-gated
// action to pause for. The agent-driven filler types AutoFill fields and stops
// at every HumanGated step.
type FillField struct {
	Label      string `json:"label"`           // the human-visible field label to locate on the form
	Value      string `json:"value,omitempty"` // value to enter (empty for gates)
	Type       string `json:"type,omitempty"`  // text|email|tel|date|select|checkbox (a hint; the filler adapts to the live form)
	HumanGated bool   `json:"human_gated"`     // true => the filler MUST stop and let the user do it
	Gate       string `json:"gate,omitempty"`  // captcha|account|gov_id|payment|submit (set when HumanGated)
	Note       string `json:"note,omitempty"`
}

// FillPlan is everything the agent-driven filler needs to drive one provider's
// quote form: where to start, what to type, and where to stop for the human.
type FillPlan struct {
	ProviderID   string      `json:"provider_id"`
	ProviderName string      `json:"provider_name"`
	QuoteURL     string      `json:"quote_url"`
	SubmitNote   string      `json:"submit_note,omitempty"`
	AutoFill     []FillField `json:"auto_fill"`   // safe to type automatically (data the user already provided)
	HumanGates   []FillField `json:"human_gates"` // CAPTCHA / account / gov-id / payment / submit
}

// Human-gate kinds.
const (
	GateCaptcha = "captcha"
	GateAccount = "account"
	GateGovID   = "gov_id"
	GatePayment = "payment"
	GateSubmit  = "submit"
)

// GenerateFillPlan turns the stored profile into a per-provider fill plan. The
// AutoFill steps are exactly the answer-sheet values the user already provided;
// the HumanGates are the steps the tool must never do on its own. It is a pure
// function.
func GenerateFillPlan(p Profile, prov Provider) FillPlan {
	sheet := GenerateAnswerSheet(p, prov)
	fp := FillPlan{
		ProviderID:   prov.ID,
		ProviderName: prov.Name,
		QuoteURL:     prov.StartURL(),
		SubmitNote:   prov.SubmitNote,
	}
	for _, f := range sheet.Fields {
		fp.AutoFill = append(fp.AutoFill, FillField{
			Label:      f.Field,
			Value:      f.Value,
			Type:       inferFieldType(f.Field),
			HumanGated: false,
			Note:       f.Note,
		})
	}
	fp.HumanGates = []FillField{
		{Label: "CAPTCHA / \"I'm not a robot\"", Gate: GateCaptcha, HumanGated: true,
			Note: "Solve it yourself when it appears - the filler cannot and will not."},
		{Label: "Account creation / password", Gate: GateAccount, HumanGated: true,
			Note: "If the form requires an account, create it yourself; the filler never types a password."},
		{Label: "EIN / SSN / government ID", Gate: GateGovID, HumanGated: true,
			Note: "Not stored in the profile. Type it in yourself, only when the form genuinely requires it."},
		{Label: "Payment / card / bank info", Gate: GatePayment, HumanGated: true,
			Note: "Only at bind time. Never entered during a quote run."},
		{Label: "Final submit", Gate: GateSubmit, HumanGated: true,
			Note: submitGateNote(prov)},
	}
	return fp
}

func submitGateNote(prov Provider) string {
	if prov.SubmitNote != "" {
		return "Two-gate submit: review every filled value, then YOU click submit. " + prov.SubmitNote
	}
	return "Two-gate submit: review every filled value, then YOU click submit. The filler never submits."
}

// inferFieldType is a best-effort hint for the filler; it always adapts to the
// actual input it finds on the live form.
func inferFieldType(label string) string {
	l := strings.ToLower(label)
	switch {
	case strings.Contains(l, "email"):
		return "email"
	case strings.Contains(l, "phone"):
		return "tel"
	case strings.Contains(l, "date"):
		return "date"
	case strings.Contains(l, "consent"):
		return "checkbox"
	case strings.Contains(l, "sells on amazon"),
		strings.Contains(l, "prior claims"),
		strings.Contains(l, "products-completed"),
		strings.Contains(l, "trade-show"),
		strings.Contains(l, "gl limits"),
		strings.Contains(l, "revenue"):
		return "select"
	default:
		return "text"
	}
}
