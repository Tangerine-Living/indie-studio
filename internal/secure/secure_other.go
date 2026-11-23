//go:build !windows

// Package secure stores small secrets. Outside Windows there is no DPAPI, so
// values are only base64-wrapped; the desktop target is Windows.
package secure

import (
	"encoding/base64"
	"errors"
	"strings"
)

// Available reports whether real OS-level encryption is in use.
func Available() bool { return false }

// Protect wraps plain text in base64.
func Protect(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	return b64Prefix + base64.StdEncoding.EncodeToString([]byte(plain)), nil
}

// Unprotect reverses Protect. DPAPI blobs cannot be read off Windows.
func Unprotect(value string) (string, error) {
	if strings.HasPrefix(strings.TrimSpace(value), dpapiPrefix) {
		return "", errors.New("DPAPI 密钥只能在 Windows 上解密")
	}
	raw, err := decode(value)
	if err != nil {
		return "", err
	}
	if raw == nil {
		return "", nil
	}
	return string(raw), nil
}
