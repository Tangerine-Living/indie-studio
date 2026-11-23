package project

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"pixelbench/internal/fsutil"
)

const maxRecent = 12

// ConfigDir is the per-user PixelBench configuration directory. Secrets and
// app-level settings live here, never inside a project folder.
func ConfigDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("PIXELBENCH_CONFIG_DIR")); override != "" {
		return override, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "PixelBench"), nil
}

func recentFile() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "recent.json"), nil
}

// LoadRecent returns recent projects whose folders still contain project.json.
func LoadRecent() []RecentProject {
	file, err := recentFile()
	if err != nil {
		return []RecentProject{}
	}
	var entries []RecentProject
	if err := fsutil.ReadJSON(file, &entries); err != nil {
		return []RecentProject{}
	}
	valid := entries[:0]
	for _, entry := range entries {
		if entry.Path == "" {
			continue
		}
		if _, err := os.Stat(ProjectFile(entry.Path)); err != nil {
			continue
		}
		valid = append(valid, entry)
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].LastOpenedAt.After(valid[j].LastOpenedAt) })
	if len(valid) > maxRecent {
		valid = valid[:maxRecent]
	}
	return valid
}

// TouchRecent moves a project to the top of the recent list.
func TouchRecent(session Session) error {
	file, err := recentFile()
	if err != nil {
		return err
	}
	entries := LoadRecent()
	now := time.Now().UTC()
	found := false
	for i := range entries {
		if samePath(entries[i].Path, session.Path) {
			entries[i].Name = session.Project.Name
			entries[i].LastOpenedAt = now
			found = true
			break
		}
	}
	if !found {
		entries = append([]RecentProject{{
			Path:         session.Path,
			Name:         session.Project.Name,
			LastOpenedAt: now,
		}}, entries...)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].LastOpenedAt.After(entries[j].LastOpenedAt) })
	if len(entries) > maxRecent {
		entries = entries[:maxRecent]
	}
	return fsutil.WriteJSONAtomic(file, entries, 0o644)
}

// RemoveRecent drops one path from the recent list.
func RemoveRecent(projectPath string) error {
	file, err := recentFile()
	if err != nil {
		return err
	}
	entries := LoadRecent()
	filtered := entries[:0]
	for _, entry := range entries {
		if !samePath(entry.Path, projectPath) {
			filtered = append(filtered, entry)
		}
	}
	return fsutil.WriteJSONAtomic(file, filtered, 0o644)
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
