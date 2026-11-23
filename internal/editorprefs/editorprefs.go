// Package editorprefs stores app-wide pixel-editor preferences. They live next
// to the other user config files, never inside a project.
package editorprefs

import (
	"os"
	"path/filepath"
	"strings"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
)

// Open modes for the first view of an asset.
const (
	OpenFit    = "fit"
	OpenActual = "actual"
)

type Settings struct {
	ShowGrid bool   `json:"showGrid"`
	OpenMode string `json:"openMode"`
}

func DefaultSettings() Settings {
	return Settings{ShowGrid: true, OpenMode: OpenFit}
}

func settingsFile() (string, error) {
	dir, err := project.ConfigDir()
	if err != nil {
		return "", apperr.Wrap("CONFIG", "无法定位应用配置目录", err)
	}
	return filepath.Join(dir, "editor.json"), nil
}

// Load reads editor.json, falling back to defaults when it is missing or bad.
func Load() Settings {
	settings := DefaultSettings()
	file, err := settingsFile()
	if err != nil {
		return settings
	}
	if err := fsutil.ReadJSON(file, &settings); err != nil && !os.IsNotExist(err) {
		return DefaultSettings()
	}
	settings.normalize()
	return settings
}

// Save normalizes and persists the preferences atomically.
func Save(settings Settings) (Settings, error) {
	settings.normalize()
	file, err := settingsFile()
	if err != nil {
		return Settings{}, err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return Settings{}, apperr.Wrap("CONFIG", "无法创建应用配置目录", err)
	}
	if err := fsutil.WriteJSONAtomic(file, settings, 0o600); err != nil {
		return Settings{}, apperr.Wrap("CONFIG", "无法保存编辑器设置", err)
	}
	return settings, nil
}

func (s *Settings) normalize() {
	if strings.EqualFold(strings.TrimSpace(s.OpenMode), OpenActual) {
		s.OpenMode = OpenActual
	} else {
		s.OpenMode = OpenFit
	}
}
