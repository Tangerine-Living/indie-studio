package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
)

type lockInfo struct {
	PID       int       `json:"pid"`
	Host      string    `json:"host"`
	Token     string    `json:"token"`
	StartedAt time.Time `json:"startedAt"`
	App       string    `json:"app"`
}

func lockFile(root string) string {
	return filepath.Join(WorkbenchDir(root), "project.lock")
}

// acquireLock refuses to open a project that another live Indie Studio process
// has open, unless force is true.
func acquireLock(root string, force bool) (string, error) {
	path := lockFile(root)
	if data, err := os.ReadFile(path); err == nil {
		var existing lockInfo
		if json.Unmarshal(data, &existing) == nil && existing.PID > 0 {
			host, _ := os.Hostname()
			sameHost := existing.Host == "" || existing.Host == host
			alive := sameHost && processAlive(existing.PID)
			stale := !existing.StartedAt.IsZero() && time.Since(existing.StartedAt) > 24*time.Hour
			if !force && alive && !stale {
				return "", apperr.New("PROJECT_LOCKED", "项目已被另一个 Indie Studio 实例打开").
					WithDetail(fmt.Sprintf("PID %d", existing.PID))
			}
		}
	}

	host, _ := os.Hostname()
	token := NewID()
	info := lockInfo{
		PID:       os.Getpid(),
		Host:      host,
		Token:     token,
		StartedAt: time.Now().UTC(),
		App:       "pixelbench",
	}
	if err := fsutil.WriteJSONAtomic(path, info, 0o644); err != nil {
		return "", apperr.Wrap("IO", "无法锁定项目", err)
	}
	return token, nil
}

// releaseLock only removes the lock it owns. The PID check is a fallback for
// lock files written by older builds where Token was empty.
func releaseLock(root, token string) {
	path := lockFile(root)
	var info lockInfo
	if err := fsutil.ReadJSON(path, &info); err != nil {
		return
	}
	if info.Token != "" && info.Token != token {
		return
	}
	if info.Token == "" && info.PID != os.Getpid() {
		return
	}
	_ = os.Remove(path)
}
