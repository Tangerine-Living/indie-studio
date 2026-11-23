package project

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"pixelbench/internal/apperr"
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ValidID reports whether id is safe to use as a directory name.
func ValidID(id string) bool { return safeIDPattern.MatchString(id) }

// ProjectFile is the path of project.json inside a project root.
func ProjectFile(root string) string { return filepath.Join(root, "project.json") }

// AssetsDir is the asset library directory.
func AssetsDir(root string) string { return filepath.Join(root, "assets") }

// QuestsDir is the quest directory.
func QuestsDir(root string) string { return filepath.Join(root, "quests") }

// PalettesDir is the palette directory.
func PalettesDir(root string) string { return filepath.Join(root, "palettes") }

// MapsDir is the map directory.
func MapsDir(root string) string { return filepath.Join(root, "maps") }

// EntitiesDir is the setting library directory.
func EntitiesDir(root string) string { return filepath.Join(root, "entities") }

// TablesDir is the game-data table directory.
func TablesDir(root string) string { return filepath.Join(root, "tables") }

// ExportsDir is where engine-ready exports are written.
func ExportsDir(root string) string { return filepath.Join(root, "exports") }

// WorkbenchDir holds derived caches, logs and recoverable trash.
func WorkbenchDir(root string) string { return filepath.Join(root, ".workbench") }

func assetRoot(root, id string) string { return filepath.Join(AssetsDir(root), id) }

func assetManifest(root, id string) string {
	return filepath.Join(assetRoot(root, id), "asset.json")
}

// AssetDir is the directory that holds one asset.
func AssetDir(root, id string) string { return assetRoot(root, id) }

// AssetManifestPath is the asset.json path of one asset.
func AssetManifestPath(root, id string) string { return assetManifest(root, id) }

func questFile(root, id string) string {
	return filepath.Join(QuestsDir(root), id+".json")
}

func entityFile(root, id string) string {
	return filepath.Join(EntitiesDir(root), id+".json")
}

func tableFile(root, id string) string {
	return filepath.Join(TablesDir(root), id+".json")
}

// MapPath is the .map.json file of one map.
func MapPath(root, id string) string {
	return filepath.Join(MapsDir(root), id+".map.json")
}

func paletteFile(root, id string) string {
	return filepath.Join(PalettesDir(root), id+".json")
}

// EnsureLayout creates the standard project directory tree.
func EnsureLayout(root string) error {
	dirs := []string{
		AssetsDir(root),
		QuestsDir(root),
		EntitiesDir(root),
		TablesDir(root),
		PalettesDir(root),
		MapsDir(root),
		filepath.Join(root, "exports"),
		filepath.Join(WorkbenchDir(root), "cache", "thumbnails"),
		filepath.Join(WorkbenchDir(root), "logs"),
		filepath.Join(WorkbenchDir(root), "autosave"),
		filepath.Join(WorkbenchDir(root), "trash", "quests"),
		filepath.Join(WorkbenchDir(root), "trash", "entities"),
		filepath.Join(WorkbenchDir(root), "trash", "tables"),
		filepath.Join(WorkbenchDir(root), "trash", "assets"),
		filepath.Join(WorkbenchDir(root), "trash", "maps"),
		filepath.Join(WorkbenchDir(root), "backups"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return apperr.Wrap("IO", "无法创建项目目录", err).WithDetail(dir)
		}
	}
	return nil
}

// SanitizeProjectName validates a user-provided project/folder name. Chinese
// characters, spaces and dots inside the name are allowed.
func SanitizeProjectName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", apperr.New("INVALID_NAME", "项目名称不能为空")
	}
	if len([]rune(name)) > 80 {
		return "", apperr.New("INVALID_NAME", "项目名称不能超过 80 个字符")
	}
	if strings.ContainsAny(name, "<>:\"/\\|?*") {
		return "", apperr.New("INVALID_NAME", `项目名称不能包含 < > : " / \ | ? *`)
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return "", apperr.New("INVALID_NAME", "项目名称不能以点或空格结尾")
	}
	if name == "." || name == ".." {
		return "", apperr.New("INVALID_NAME", "项目名称不合法")
	}
	upper := strings.ToUpper(name)
	for _, reserved := range []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"} {
		if upper == reserved {
			return "", apperr.New("INVALID_NAME", fmt.Sprintf("%s 是 Windows 保留名称，请换一个", name))
		}
	}
	return name, nil
}

// NormalizeRoot turns a user-picked file or directory into a project root.
func NormalizeRoot(path string) string {
	path = strings.Trim(strings.TrimSpace(path), `"`)
	if path == "" {
		return ""
	}
	cleaned := filepath.Clean(path)
	if strings.EqualFold(filepath.Base(cleaned), "project.json") {
		cleaned = filepath.Dir(cleaned)
	}
	if abs, err := filepath.Abs(cleaned); err == nil {
		cleaned = abs
	}
	return cleaned
}
