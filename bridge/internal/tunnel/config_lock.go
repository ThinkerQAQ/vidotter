package tunnel

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

// OS-released locks also serialize Save with a worker starting at login.
func configurationLock(root string) (func(), error) {
	dir := filepath.Join(root, "runtime")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err := protectPrivate(dir, true); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, "configuration.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = protectPrivate(file.Name(), false); err != nil {
		file.Close()
		return nil, err
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err = lockWorker(file); err == nil {
			return func() { _ = file.Close() }, nil
		}
		if time.Now().After(deadline) {
			file.Close()
			return nil, errors.New("another configuration operation is in progress")
		}
		time.Sleep(30 * time.Millisecond)
	}
}
func workerConfiguration(root string) (persistedSetup, string, []byte, error) {
	release, err := configurationLock(root)
	if err != nil {
		return persistedSetup{}, "", nil, err
	}
	defer release()
	setup, err := readSetup(root)
	if err != nil {
		return setup, "", nil, err
	}
	key, err := savedKey(root)
	if err != nil {
		return setup, "", nil, err
	}
	profile, err := os.ReadFile(filepath.Join(root, "runtime", "tunnel.yaml"))
	return setup, key, profile, err
}
