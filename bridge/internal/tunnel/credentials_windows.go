//go:build windows

package tunnel

import (
	"encoding/base64"
	"errors"
	"golang.org/x/sys/windows"
	"unsafe"
)

func encodeCredentials(key string) (persistedCredentials, error) {
	raw := []byte(key)
	defer clear(raw)
	if len(raw) == 0 {
		return persistedCredentials{}, errors.New("key required")
	}
	input := windows.DataBlob{Size: uint32(len(raw)), Data: &raw[0]}
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return persistedCredentials{}, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	return persistedCredentials{Protected: base64.StdEncoding.EncodeToString(unsafe.Slice(output.Data, int(output.Size)))}, nil
}
func decodeCredentials(saved persistedCredentials) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(saved.Protected)
	if err != nil || len(raw) == 0 {
		return "", errors.New("invalid protected key")
	}
	input := windows.DataBlob{Size: uint32(len(raw)), Data: &raw[0]}
	var output windows.DataBlob
	if err = windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data)))
	plain := unsafe.Slice(output.Data, int(output.Size))
	defer clear(plain)
	return string(plain), nil
}
