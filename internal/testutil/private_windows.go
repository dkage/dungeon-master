package testutil

import (
	"os"
	"regexp"
	"strings"

	"golang.org/x/sys/windows"
)

func PrivateMode(path string, info os.FileInfo, _ os.FileMode) bool {
	if info == nil {
		return false
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return false
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return false
	}
	allowed := map[string]bool{"SY": true, "BA": true, user.User.Sid.String(): true}
	entries := regexp.MustCompile("\\(([^()]*)\\)").FindAllStringSubmatch(sd.String(), -1)
	if len(entries) != 3 {
		return false
	}
	for _, entry := range entries {
		fields := strings.Split(entry[1], ";")
		if len(fields) != 6 || fields[0] != "A" || fields[2] != "FA" || !allowed[fields[5]] {
			return false
		}
		delete(allowed, fields[5])
	}
	return len(allowed) == 0
}
