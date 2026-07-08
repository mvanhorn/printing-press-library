package security

const (
	ReasonPII                   = "PII"
	ReasonShieldEncrypted       = "shield_encrypted"
	ReasonPolymorphicUnreadable = "polymorphic_target_unreadable"
	ReasonComplianceFailClosed  = "compliance_fail_closed"
	ReasonContentScan           = "content_scan"
)

type RedactionMarker map[string]string

func MarkRedacted(fields map[string]any, key string, reason string) {
	if fields == nil || key == "" || reason == "" {
		return
	}
	fields[key] = RedactionMarker{"redacted": reason}
}

func ProvenanceCounter(prov *Provenance, reason string) {
	if prov == nil || reason == "" {
		return
	}
	if prov.Redactions == nil {
		prov.Redactions = map[string]int{}
	}
	prov.Redactions[reason]++
	switch reason {
	case ReasonShieldEncrypted:
		prov.Counters.Shield++
	case ReasonPolymorphicUnreadable:
		prov.Counters.Polymorphic++
	case ReasonContentScan:
		prov.Counters.ContentScan++
	}
}
