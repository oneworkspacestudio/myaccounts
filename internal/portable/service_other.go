//go:build !windows

package portable

import "errors"

func RequestServiceInstallation(exe, dir string) error {
	return errors.New("automatic Windows startup is only available on Windows")
}
