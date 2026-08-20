package omnilogic

import (
	"errors"
	"os"
)

var errFileNotExist = errors.New("file does not exist")

func removeFile(path string) error {
	err := os.Remove(path)
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return errFileNotExist
	}
	return err
}
