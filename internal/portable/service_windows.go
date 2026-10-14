//go:build windows

package portable

import (
	"golang.org/x/sys/windows"
	"syscall"
)

// Elevation is requested only after the user opts into automatic Windows startup.
func RequestServiceInstallation(exe, dir string) error {
	verb, e := windows.UTF16PtrFromString("runas")
	if e != nil {
		return e
	}
	file, e := windows.UTF16PtrFromString(exe)
	if e != nil {
		return e
	}
	args, e := windows.UTF16PtrFromString("-data " + syscall.EscapeArg(dir) + " service install-start")
	if e != nil {
		return e
	}
	return windows.ShellExecute(0, verb, file, args, nil, windows.SW_HIDE)
}
