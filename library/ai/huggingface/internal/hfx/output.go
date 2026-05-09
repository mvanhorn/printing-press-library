package hfx

// Envelope is the standard wrapper around every JSON response from any
// command (per seed: "Output carries `as-of` timestamp" + stable schema).
//
// Concrete commands embed Envelope into their typed response struct so
// JSON tags inline cleanly:
//
//	type DoctorReport struct {
//	    hfx.Envelope
//	    TTY bool `json:"tty"`
//	    ...
//	}
type Envelope struct {
	SchemaVersion int    `json:"schema_version"`
	AsOf          string `json:"as_of"`
	Command       string `json:"command,omitempty"`
}

// NewEnvelope returns a fresh envelope stamped with the current time.
func NewEnvelope(command string) Envelope {
	return Envelope{
		SchemaVersion: SchemaVersion,
		AsOf:          AsOf(),
		Command:       command,
	}
}
