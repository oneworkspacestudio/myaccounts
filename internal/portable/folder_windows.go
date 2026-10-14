//go:build windows

package portable

import (
	"errors"
	"golang.org/x/sys/windows"
	"runtime"
	"unsafe"
)

// Native Windows folder selection keeps first-run data placement independent of developer runtimes.
func ChooseDataDir() (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := windows.NewLazySystemDLL("ole32.dll")
	coInit := ole.NewProc("CoInitializeEx")
	coUninit := ole.NewProc("CoUninitialize")
	free := ole.NewProc("CoTaskMemFree")
	hr, _, _ := coInit.Call(0, 2)
	if hr == 0 || hr == 1 {
		defer coUninit.Call()
	}
	shell := windows.NewLazySystemDLL("shell32.dll")
	browse := shell.NewProc("SHBrowseForFolderW")
	getPath := shell.NewProc("SHGetPathFromIDListW")
	title, _ := windows.UTF16PtrFromString("Choose an NTFS folder for MyAccounts data. Select a new dedicated folder, or an existing MyAccounts data folder.")
	display := make([]uint16, 260)
	info := struct {
		Owner    uintptr
		Root     uintptr
		Display  *uint16
		Title    *uint16
		Flags    uint32
		Callback uintptr
		Param    uintptr
		Image    int32
	}{Display: &display[0], Title: title, Flags: 0x41}
	pidl, _, _ := browse.Call(uintptr(unsafe.Pointer(&info)))
	if pidl == 0 {
		return "", errors.New("no data folder was selected")
	}
	defer free.Call(pidl)
	buffer := make([]uint16, 32768)
	ok, _, _ := getPath.Call(pidl, uintptr(unsafe.Pointer(&buffer[0])))
	if ok == 0 {
		return "", errors.New("selected folder has no filesystem path")
	}
	return windows.UTF16ToString(buffer), nil
}
