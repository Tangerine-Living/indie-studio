package secure

import (
	"encoding/base64"
	"strings"
)

const (
	dpapiPrefix = "dpapi:"
	b64Prefix   = "b64:"
)

// decode strips the storage prefix and returns the raw payload bytes.
func decode(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	switch {
	case trimmed == "":
		return nil, nil
	case strings.HasPrefix(trimmed, dpapiPrefix):
		trimmed = strings.TrimPrefix(trimmed, dpapiPrefix)
	case strings.HasPrefix(trimmed, b64Prefix):
		trimmed = strings.TrimPrefix(trimmed, b64Prefix)
	}
	return base64.StdEncoding.DecodeString(trimmed)
}
