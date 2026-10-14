//go:build windows

package portable

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestProtectDirRemovesExplicitEveryoneGrant(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "credential.bin")
	if err := os.WriteFile(child, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:(A;;FA;;;WD)(A;;FA;;;BA)(A;;FA;;;SY)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err = windows.SetNamedSecurityInfo(child, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	if err = ProtectDir(dir); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, child} {
		actual, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(actual.String(), ";;;WD)") {
			t.Fatal("explicit Everyone access survived protection")
		}
	}
}
