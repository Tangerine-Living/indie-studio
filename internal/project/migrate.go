package project

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
)

// migrateProject upgrades project data in memory. It returns true when the
// in-memory value changed and the caller should write it back.
func migrateProject(root string, project *Project) (bool, error) {
	if project.SchemaVersion > SchemaVersion {
		return false, apperr.Newf("SCHEMA_TOO_NEW",
			"项目由更新版本的 Indie Studio 创建（schema %d），请升级应用", project.SchemaVersion)
	}
	if project.SchemaVersion < 0 {
		project.SchemaVersion = 0
	}
	start := project.SchemaVersion
	migrated := false
	for i := 0; project.SchemaVersion < SchemaVersion; i++ {
		if i > 16 {
			return false, apperr.New("SCHEMA_UNSUPPORTED", "项目迁移链过长，已中止")
		}
		switch project.SchemaVersion {
		case 0:
			migrate0to1(project)
		default:
			return false, apperr.Newf("SCHEMA_UNSUPPORTED", "无法从 schema %d 迁移", project.SchemaVersion)
		}
		migrated = true
	}
	if migrated {
		if data, err := os.ReadFile(ProjectFile(root)); err == nil {
			backup := filepath.Join(WorkbenchDir(root), "backups",
				fmt.Sprintf("project-v%d-%s.json", start, time.Now().Format("20060102-150405")))
			_ = fsutil.WriteFileAtomic(backup, data, 0o644)
		}
	}
	return migrated, nil
}

func migrate0to1(project *Project) {
	project.SchemaVersion = 1
	if project.ID == "" {
		project.ID = NewID()
	}
	if project.Name == "" {
		project.Name = "未命名项目"
	}
	if project.Engine == "" {
		project.Engine = "godot"
	}
	if project.Stage == "" {
		project.Stage = StageConcept
	}
	if project.Settings.TileSize == 0 {
		project.Settings.TileSize = 32
	}
	if project.Settings.DefaultPaletteID == "" {
		project.Settings.DefaultPaletteID = "default"
	}
	if project.Settings.PixelScale == 0 {
		project.Settings.PixelScale = 4
	}
	if project.Settings.UILanguage == "" {
		project.Settings.UILanguage = "zh-CN"
	}
	if project.Settings.StyleGuide.Perspective == "" {
		project.Settings.StyleGuide = defaultStyleGuide()
	}
	now := time.Now().UTC()
	if project.CreatedAt.IsZero() {
		project.CreatedAt = now
	}
	if project.UpdatedAt.IsZero() {
		project.UpdatedAt = now
	}
	if len(project.Members) == 0 {
		project.Members = []Member{{ID: "m1", Name: "我", Role: "独立开发者"}}
	}
}
