//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// OpenPath reveals a folder or file in Explorer.
func OpenPath(target string) error {
	if target == "" {
		return fmt.Errorf("empty path")
	}
	cmd := exec.Command("explorer.exe", filepath.Clean(target))
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}
