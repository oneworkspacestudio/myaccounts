//go:build !windows

package portable

import (
	"errors"
	"os"
)

func ProtectDir(dir string) error { return os.Chmod(dir, 0700) }
func Protect(b []byte) ([]byte, error) {
	return nil, errors.New("operational credential storage requires Windows DPAPI")
}
func Unprotect(b []byte) ([]byte, error) {
	return nil, errors.New("operational credential storage requires Windows DPAPI")
}
