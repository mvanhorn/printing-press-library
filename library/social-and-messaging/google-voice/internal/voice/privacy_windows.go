//go:build windows

package voice

import "fmt"

func verifyPrivateDirectory(path string) error {
	return fmt.Errorf("Voice private storage directory validation is currently supported on Unix; Windows storage is unavailable")
}
