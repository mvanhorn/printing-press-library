package consumer

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// FlexInt unmarshals JSON numbers or numeric strings into an int.
type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*f = 0
		return nil
	}
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		*f = FlexInt(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" {
		*f = 0
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("flex int: %q", s)
	}
	*f = FlexInt(v)
	return nil
}

// FlexID unmarshals reservation identifiers that may be int or string in wire JSON.
type FlexID string

func (f *FlexID) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*f = ""
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = FlexID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		*f = FlexID(n.String())
		return nil
	}
	return fmt.Errorf("flex id: %s", string(b))
}

func (f FlexID) String() string { return string(f) }
