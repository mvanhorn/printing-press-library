package machines

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type TargetStateStore struct{ Path string }

func (s TargetStateStore) Save(name string) error {
	if name == "" {
		return fmt.Errorf("target name is required")
	}
	d := filepath.Dir(s.Path)
	if err := os.MkdirAll(d, 0700); err != nil {
		return err
	}
	data, _ := json.Marshal(map[string]string{"selected": name})
	tmp, err := os.CreateTemp(d, ".target-*")
	if err != nil {
		return err
	}
	n := tmp.Name()
	defer os.Remove(n)
	_ = tmp.Chmod(0600)
	if _, err = tmp.Write(append(data, '\n')); err == nil {
		err = tmp.Close()
	}
	if err != nil {
		return err
	}
	if err = os.Rename(n, s.Path); err != nil {
		return err
	}
	return os.Chmod(s.Path, 0600)
}
func (s TargetStateStore) Load() (string, error) {
	b, err := os.ReadFile(s.Path)
	if err != nil {
		return "", err
	}
	var v struct {
		Selected string `json:"selected"`
	}
	if err = json.Unmarshal(b, &v); err != nil || v.Selected == "" {
		return "", fmt.Errorf("invalid target state")
	}
	return v.Selected, nil
}
