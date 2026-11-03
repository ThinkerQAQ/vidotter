//go:build windows

package tunnel

// Adapted from ThinkerQAQ/remote-browser-mcp client at 789d085 (#17, #18), Apache-2.0.

import (
	"fmt"
	"golang.org/x/sys/windows"
)

func protectPrivate(path string, directory bool) error {
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return e
	}
	sd, e := windows.SecurityDescriptorFromString("D:P(A;;FA;;;" + user.User.Sid.String() + ")")
	if e != nil {
		return e
	}
	acl, _, e := sd.DACL()
	if e != nil {
		return e
	}
	flags := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	if e := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, flags, nil, nil, acl, nil); e != nil {
		return fmt.Errorf("could not restrict local private-data permissions: %w", e)
	}
	return nil
}
