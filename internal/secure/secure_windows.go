//go:build windows

// Package secure stores small secrets (AI provider API keys) encrypted with
// the Windows Data Protection API. Values never enter a project folder or a
// log line; only the app config directory holds the ciphertext.
package secure

import (
	"encoding/base64"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Available reports whether real OS-level encryption is in use.
func Available() bool { return true }

// Protect encrypts plain text for the current Windows user.
func Protect(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	data := []byte(plain)
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	blob := unsafe.Slice(out.Data, out.Size)
	return dpapiPrefix + base64.StdEncoding.EncodeToString(blob), nil
}

// Unprotect decrypts a value produced by Protect.
func Unprotect(value string) (string, error) {
	raw, err := decode(value)
	if err != nil {
		return "", err
	}
	if raw == nil {
		return "", nil
	}
	in := windows.DataBlob{Size: uint32(len(raw)), Data: &raw[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return "", err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return string(unsafe.Slice(out.Data, out.Size)), nil
}
