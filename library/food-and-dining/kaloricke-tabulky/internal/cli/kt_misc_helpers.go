package cli

import "encoding/json"

// jsonUnmarshalImpl wraps the stdlib json.Unmarshal. Centralizing it
// lets us avoid pulling encoding/json into every novel-feature file.
func jsonUnmarshalImpl(raw []byte, dst any) error {
	return json.Unmarshal(raw, dst)
}
