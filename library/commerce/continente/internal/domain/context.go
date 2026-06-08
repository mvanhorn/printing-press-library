package domain

type Context struct {
	Kind  string `json:"kind,omitempty"`
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
}
