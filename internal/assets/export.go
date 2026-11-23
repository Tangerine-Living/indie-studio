package assets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
	"pixelbench/internal/project"
)

// ExportResult describes one asset export into the project's exports/ tree.
type ExportResult struct {
	// Dir is the absolute path of the exported version folder.
	Dir string `json:"dir"`
	// Files are the paths inside Dir, using forward slashes.
	Files []string `json:"files"`
}

// exportMarker records which asset owns an exports folder, so two assets that
// share a name never overwrite each other.
type exportMarker struct {
	AssetID    string    `json:"assetId"`
	AssetName  string    `json:"assetName"`
	Version    string    `json:"version"`
	ExportedAt time.Time `json:"exportedAt"`
}

// ExportAsset copies the asset's current immutable version into
// exports/<name>/<version>/. Sheets also bring their Aseprite/TexturePacker
// JSON and derived frames. Nothing in the asset library is modified.
func ExportAsset(store *project.Store, assetID string) (ExportResult, error) {
	asset, err := store.GetAsset(assetID)
	if err != nil {
		return ExportResult{}, err
	}
	if asset.CurrentVersion == "" {
		return ExportResult{}, apperr.New("NO_VERSION", "这个素材还没有可导出的版本")
	}
	versionDir := filepath.Join(project.AssetDir(store.Root(), asset.ID), "versions", asset.CurrentVersion)
	outputPath := filepath.Join(versionDir, "output.png")
	if _, statErr := os.Stat(outputPath); statErr != nil {
		return ExportResult{}, apperr.New("NO_VERSION", "当前版本的图片不存在").
			WithDetail(outputPath)
	}

	root := project.ExportsDir(store.Root())
	destDir := filepath.Join(root, exportFolderName(root, asset), asset.CurrentVersion)

	files := []string{"output.png"}
	if err := copyExportFile(outputPath, destDir, "output.png"); err != nil {
		return ExportResult{}, apperr.Wrap("IO", "无法写入导出图片", err)
	}
	for _, name := range []string{"meta.json", "frames-aseprite.json", "frames-texturepacker.json"} {
		src := filepath.Join(versionDir, name)
		if _, statErr := os.Stat(src); statErr != nil {
			continue
		}
		if err := copyExportFile(src, destDir, name); err != nil {
			return ExportResult{}, apperr.Wrap("IO", "无法写入导出元数据", err)
		}
		files = append(files, name)
	}
	if asset.Animation != nil {
		framesDir := filepath.Join(project.AssetDir(store.Root(), asset.ID), "frames")
		entries, readErr := os.ReadDir(framesDir)
		if readErr == nil {
			for _, entry := range entries {
				if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
					continue
				}
				rel := "frames/" + entry.Name()
				if err := copyExportFile(filepath.Join(framesDir, entry.Name()), destDir, rel); err != nil {
					return ExportResult{}, apperr.Wrap("IO", "无法写入导出帧", err)
				}
				files = append(files, rel)
			}
		}
	}

	marker := exportMarker{
		AssetID:    asset.ID,
		AssetName:  asset.Name,
		Version:    asset.CurrentVersion,
		ExportedAt: time.Now().UTC(),
	}
	if err := fsutil.WriteJSONAtomic(filepath.Join(destDir, "export.json"), marker, 0o644); err != nil {
		return ExportResult{}, apperr.Wrap("IO", "无法写入导出标记", err)
	}
	files = append(files, "export.json")

	return ExportResult{Dir: destDir, Files: files}, nil
}

// exportFolderName sanitizes the asset name and keeps it unique per asset.
func exportFolderName(root string, asset project.Asset) string {
	base := sanitizeExportName(asset.Name)
	target := filepath.Join(root, base, "export.json")
	var existing exportMarker
	if err := fsutil.ReadJSON(target, &existing); err == nil && existing.AssetID != asset.ID {
		suffix := shortAssetID(asset.ID)
		base = base + "-" + suffix
	}
	return base
}

// sanitizeExportName turns an asset name into a Windows-safe folder name.
func sanitizeExportName(name string) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			builder.WriteRune('_')
		default:
			if r < 0x20 {
				continue
			}
			builder.WriteRune(r)
		}
	}
	out := strings.Trim(strings.TrimSpace(builder.String()), " .")
	if runes := []rune(out); len(runes) > 64 {
		out = string(runes[:64])
	}
	if out == "" || out == "." || out == ".." {
		out = "asset"
	}
	switch strings.ToUpper(out) {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		out = "_" + out
	}
	return out
}

func shortAssetID(id string) string {
	var builder strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		}
		if builder.Len() == 8 {
			break
		}
	}
	out := builder.String()
	if out == "" {
		out = fmt.Sprintf("%08x", time.Now().UnixNano()&0xffffffff)
	}
	return out
}

func copyExportFile(src, destDir, rel string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(destDir, filepath.FromSlash(rel)), data, 0o644)
}
