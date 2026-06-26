package packet

import "time"

// Change proposes replacing a profile section's content.
type Change struct {
	Section    string `json:"section"`
	Before     string `json:"before"`
	After      string `json:"after"`
	SourceNote string `json:"source_note"`
}

// Packet is an immutable proposed profile edit set tied to a snapshot.
type Packet struct {
	ID            string    `json:"id"`
	SnapshotID    string    `json:"snapshot_id"`
	CreatedAt     time.Time `json:"created_at"`
	Opportunity   string    `json:"opportunity"`
	OpportunityID string    `json:"opportunity_id"`
	Status        string    `json:"status"`
	RiskNotes     []string  `json:"risk_notes"`
	Changes       []Change  `json:"changes"`
}

// ApplyLog records every attempted apply decision.
type ApplyLog struct {
	ID                 string    `json:"id"`
	PacketID           string    `json:"packet_id"`
	CreatedAt          time.Time `json:"created_at"`
	PrivacyStatus      string    `json:"privacy_status"`
	SensitiveStatus    string    `json:"sensitive_status"`
	Result             string    `json:"result"`
	DryRun             bool      `json:"dry_run"`
	ConfirmationSource string    `json:"confirmation_source"`
}
