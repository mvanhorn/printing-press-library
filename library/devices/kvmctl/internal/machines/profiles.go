package machines

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ProfileStore struct{ Path string }

func (s ProfileStore) Save(inv Inventory) error {
	if err := inv.Validate(); err != nil {
		return err
	}
	if s.Path == "" {
		return fmt.Errorf("profile path is required")
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	_ = os.Chmod(dir, 0700)
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() || st.Mode().Perm() != 0700 {
		return fmt.Errorf("unsafe profile directory")
	}
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".profiles-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(append(data, '\n'))
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, s.Path); err != nil {
		return err
	}
	return os.Chmod(s.Path, 0600)
}
func (s ProfileStore) Load() (Inventory, error) {
	var inv Inventory
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return inv, err
	}
	if err = json.Unmarshal(data, &inv); err != nil {
		return inv, fmt.Errorf("parse profiles: %w", err)
	}
	return inv, inv.Validate()
}
