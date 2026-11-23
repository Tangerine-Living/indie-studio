//go:build !windows

package platform

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenPath reveals a folder or file in the platform file manager.
func OpenPath(target string) error {
	if target == "" {
		return fmt.Errorf("empty path")
	}
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "open"
	}
	return exec.Command(opener, target).Start()
}
