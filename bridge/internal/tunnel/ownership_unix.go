//go:build !windows

package tunnel

// Adapted from ThinkerQAQ/remote-browser-mcp client at 789d085 (#17, #18), Apache-2.0.

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
)

func preserveExistingOwner(temp string, existing os.FileInfo) error {
	if existing == nil {
		return nil
	}
	stat, ok := existing.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	// Ordinary user updates already own the new file; avoid privileged chown.
	if stat.Uid == uint32(os.Geteuid()) && stat.Gid == uint32(os.Getegid()) {
		return nil
	}
	return os.Chown(temp, int(stat.Uid), int(stat.Gid))
}
func verifyServiceDataOwner(root, account string) error {
	target, err := user.Lookup(account)
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(target.Uid)
	if err != nil {
		return err
	}
	info, err := os.Stat(filepath.Join(root, "runtime", "credentials.json"))
	if err != nil {
		return err
	}
	actual, ok := info.Sys().(*syscall.Stat_t)
	if !ok || actual.Uid != uint32(uid) {
		return fmt.Errorf("service account must own the private runtime credentials")
	}
	return nil
}

func assignServiceOwner(file, account string) error {
	u, e := user.Lookup(account)
	if e != nil {
		return e
	}
	uid, e := strconv.Atoi(u.Uid)
	if e != nil {
		return e
	}
	gid, e := strconv.Atoi(u.Gid)
	if e != nil {
		return e
	}
	if e = os.Chown(file, uid, gid); e != nil {
		return e
	}
	return os.Chmod(file, 0600)
}
