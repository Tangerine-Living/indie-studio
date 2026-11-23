package project

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
)

// Store owns one open project and serialises access to its files.
type Store struct {
	mu        sync.RWMutex
	root      string
	project   Project
	lockToken string
	closed    bool
}

// Create creates a new project directory and opens it.
func Create(opts CreateOptions) (*Store, error) {
	if strings.TrimSpace(opts.ParentDir) == "" {
		return nil, apperr.New("INVALID_PARENT", "请选择项目存放的父文件夹")
	}
	name, err := SanitizeProjectName(opts.Name)
	if err != nil {
		return nil, err
	}
	parent := NormalizeRoot(opts.ParentDir)
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return nil, apperr.New("INVALID_PARENT", "父文件夹不存在").WithDetail(parent)
	}

	root := filepath.Join(parent, name)
	if entries, err := os.ReadDir(root); err == nil {
		if len(entries) > 0 {
			if _, statErr := os.Stat(ProjectFile(root)); statErr == nil {
				return nil, apperr.New("PROJECT_EXISTS", "同名项目已存在").WithDetail(root)
			}
			return nil, apperr.New("PROJECT_EXISTS", "目标文件夹已存在且不为空").WithDetail(root)
		}
	} else if !os.IsNotExist(err) {
		return nil, apperr.Wrap("IO", "无法检查目标文件夹", err).WithDetail(root)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, apperr.Wrap("IO", "无法创建项目文件夹", err).WithDetail(root)
	}
	if err := EnsureLayout(root); err != nil {
		return nil, err
	}

	project := defaultProject(opts, name)
	if err := ValidateProject(&project); err != nil {
		return nil, err
	}
	if err := fsutil.WriteJSONAtomic(ProjectFile(root), project, 0o644); err != nil {
		return nil, apperr.Wrap("IO", "无法写入 project.json", err)
	}

	palette := DefaultPalette()
	if err := fsutil.WriteJSONAtomic(paletteFile(root, palette.ID), palette, 0o644); err != nil {
		return nil, apperr.Wrap("IO", "无法写入默认调色板", err)
	}

	// The town template ships the demo content; a blank project starts with
	// only project.json and the default palette.
	if project.Settings.Template == "town-square" {
		quest := welcomeQuest()
		quest.Brief.TileSize = project.Settings.TileSize
		quest.Brief.PaletteID = project.Settings.DefaultPaletteID

		grass, err := seedAsset(root, project.Settings.DefaultPaletteID, quest.ID)
		if err != nil {
			return nil, err
		}
		atlas, err := seedTileAtlas(root, project.Settings.DefaultPaletteID, quest.ID)
		if err != nil {
			return nil, err
		}
		townMap, err := seedTownMap(root, project, atlas)
		if err != nil {
			return nil, err
		}
		if err := seedEntities(root, grass, atlas, townMap); err != nil {
			return nil, err
		}
		if err := seedTables(root); err != nil {
			return nil, err
		}
		quest.Links.MapIDs = []string{townMap.ID}
		if err := ValidateQuest(&quest); err != nil {
			return nil, err
		}
		if err := fsutil.WriteJSONAtomic(questFile(root, quest.ID), quest, 0o644); err != nil {
			return nil, apperr.Wrap("IO", "无法写入示例任务", err)
		}
	}

	if err := writeProjectDocs(root, project); err != nil {
		return nil, err
	}
	lockToken, err := acquireLock(root, false)
	if err != nil {
		return nil, err
	}

	store := &Store{root: root, project: project, lockToken: lockToken}
	session := store.Session()
	if err := TouchRecent(session); err != nil {
		slog.Warn("cannot update recent projects", "error", err)
	}
	return store, nil
}

// Open opens an existing project. force ignores a lock held by a live process.
func Open(root string, force bool) (*Store, error) {
	root = NormalizeRoot(root)
	if root == "" {
		return nil, apperr.New("PROJECT_NOT_FOUND", "请选择项目文件夹")
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, apperr.New("PROJECT_NOT_FOUND", "项目文件夹不存在").WithDetail(root)
	}
	if !info.IsDir() {
		return nil, apperr.New("PROJECT_NOT_FOUND", "选择的不是项目文件夹").WithDetail(root)
	}
	if _, err := os.Stat(ProjectFile(root)); err != nil {
		return nil, apperr.New("PROJECT_NOT_FOUND", "该文件夹中没有 project.json").WithDetail(root)
	}

	var project Project
	if err := fsutil.ReadJSON(ProjectFile(root), &project); err != nil {
		return nil, apperr.Wrap("INVALID_PROJECT", "无法读取 project.json", err)
	}
	migrated, err := migrateProject(root, &project)
	if err != nil {
		return nil, err
	}
	if err := ValidateProject(&project); err != nil {
		return nil, err
	}
	if migrated {
		if err := fsutil.WriteJSONAtomic(ProjectFile(root), project, 0o644); err != nil {
			return nil, apperr.Wrap("IO", "迁移后无法写入 project.json", err)
		}
	}
	if err := EnsureLayout(root); err != nil {
		return nil, err
	}
	if err := ensureDefaultPalette(root); err != nil {
		return nil, err
	}
	lockToken, err := acquireLock(root, force)
	if err != nil {
		return nil, err
	}

	store := &Store{root: root, project: project, lockToken: lockToken}
	session := store.Session()
	if err := TouchRecent(session); err != nil {
		slog.Warn("cannot update recent projects", "error", err)
	}
	return store, nil
}

// Root returns the project directory.
func (s *Store) Root() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root
}

// Session returns a deep copy of the current session.
func (s *Store) Session() Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Session{Path: s.root, Project: cloneProject(s.project)}
}

func (s *Store) ensureOpen() error {
	if s.closed {
		return apperr.New("NO_PROJECT", "项目已关闭")
	}
	return nil
}

// Close releases the project lock.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	releaseLock(s.root, s.lockToken)
	return nil
}

// SaveProject writes the project manifest after validating it.
func (s *Store) SaveProject(project Project) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return Session{}, err
	}
	if err := ValidateProject(&project); err != nil {
		return Session{}, err
	}
	project.UpdatedAt = time.Now().UTC()
	if err := fsutil.WriteJSONAtomic(ProjectFile(s.root), project, 0o644); err != nil {
		return Session{}, apperr.Wrap("IO", "无法保存 project.json", err)
	}
	s.project = project
	return Session{Path: s.root, Project: cloneProject(project)}, nil
}

// ListQuests returns all valid quests, newest first.
func (s *Store) ListQuests() ([]Quest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(QuestsDir(s.root))
	if err != nil {
		if os.IsNotExist(err) {
			return []Quest{}, nil
		}
		return nil, apperr.Wrap("IO", "无法读取任务目录", err)
	}
	quests := make([]Quest, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var quest Quest
		if err := fsutil.ReadJSON(filepath.Join(QuestsDir(s.root), entry.Name()), &quest); err != nil {
			slog.Warn("skipping unreadable quest", "file", entry.Name(), "error", err)
			continue
		}
		if err := ValidateQuest(&quest); err != nil {
			slog.Warn("skipping invalid quest", "file", entry.Name(), "error", err)
			continue
		}
		quests = append(quests, quest)
	}
	sort.Slice(quests, func(i, j int) bool { return quests[i].UpdatedAt.After(quests[j].UpdatedAt) })
	return quests, nil
}

// SaveQuest writes one quest.
func (s *Store) SaveQuest(quest Quest) (Quest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return Quest{}, err
	}
	if quest.Brief.PaletteID == "" {
		quest.Brief.PaletteID = s.project.Settings.DefaultPaletteID
	}
	if quest.Brief.TileSize == 0 {
		quest.Brief.TileSize = s.project.Settings.TileSize
	}
	if err := ValidateQuest(&quest); err != nil {
		return Quest{}, err
	}
	quest.UpdatedAt = time.Now().UTC()
	if err := fsutil.WriteJSONAtomic(questFile(s.root, quest.ID), quest, 0o644); err != nil {
		return Quest{}, apperr.Wrap("IO", "无法保存任务", err)
	}
	return quest, nil
}

// DeleteQuest moves a quest into the recoverable trash.
func (s *Store) DeleteQuest(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if !ValidID(id) {
		return apperr.New("INVALID_QUEST", "任务 ID 不合法")
	}
	src := questFile(s.root, id)
	if _, err := os.Stat(src); err != nil {
		return apperr.New("QUEST_NOT_FOUND", "任务不存在")
	}
	dst := filepath.Join(WorkbenchDir(s.root), "trash", "quests", fmt.Sprintf("%s-%d.json", id, time.Now().Unix()))
	if err := os.Rename(src, dst); err != nil {
		if err := fsutil.CopyFile(src, dst); err != nil {
			return apperr.Wrap("IO", "无法删除任务", err)
		}
		_ = os.Remove(src)
	}
	return nil
}

// ListEntities returns every valid setting entry, newest first.
func (s *Store) ListEntities() ([]Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(EntitiesDir(s.root))
	if err != nil {
		if os.IsNotExist(err) {
			return []Entity{}, nil
		}
		return nil, apperr.Wrap("IO", "无法读取设定目录", err)
	}
	entities := make([]Entity, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var entity Entity
		if err := fsutil.ReadJSON(filepath.Join(EntitiesDir(s.root), entry.Name()), &entity); err != nil {
			slog.Warn("skipping unreadable entity", "file", entry.Name(), "error", err)
			continue
		}
		if err := ValidateEntity(&entity); err != nil {
			slog.Warn("skipping invalid entity", "file", entry.Name(), "error", err)
			continue
		}
		entities = append(entities, entity)
	}
	sort.Slice(entities, func(i, j int) bool { return entities[i].UpdatedAt.After(entities[j].UpdatedAt) })
	return entities, nil
}

// SaveEntity writes one setting entry.
func (s *Store) SaveEntity(entity Entity) (Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return Entity{}, err
	}
	if err := ValidateEntity(&entity); err != nil {
		return Entity{}, err
	}
	entity.UpdatedAt = time.Now().UTC()
	if err := os.MkdirAll(EntitiesDir(s.root), 0o755); err != nil {
		return Entity{}, apperr.Wrap("IO", "无法创建设定目录", err)
	}
	if err := fsutil.WriteJSONAtomic(entityFile(s.root, entity.ID), entity, 0o644); err != nil {
		return Entity{}, apperr.Wrap("IO", "无法保存设定", err)
	}
	return entity, nil
}

// DeleteEntity moves one setting entry into the recoverable trash.
func (s *Store) DeleteEntity(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if !ValidID(id) {
		return apperr.New("INVALID_ENTITY", "设定 ID 不合法")
	}
	src := entityFile(s.root, id)
	if _, err := os.Stat(src); err != nil {
		return apperr.New("ENTITY_NOT_FOUND", "设定不存在")
	}
	dst := filepath.Join(WorkbenchDir(s.root), "trash", "entities", fmt.Sprintf("%s-%d.json", id, time.Now().Unix()))
	if err := os.Rename(src, dst); err != nil {
		if err := fsutil.CopyFile(src, dst); err != nil {
			return apperr.Wrap("IO", "无法删除设定", err)
		}
		_ = os.Remove(src)
	}
	dropEntityRelations(s.root, id)
	return nil
}

// dropEntityRelations removes links to a deleted entity from the remaining
// entries, so no dangling relation survives the deletion.
func dropEntityRelations(root, id string) {
	entries, err := os.ReadDir(EntitiesDir(root))
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		file := filepath.Join(EntitiesDir(root), entry.Name())
		var entity Entity
		if err := fsutil.ReadJSON(file, &entity); err != nil {
			continue
		}
		kept := make([]EntityRelation, 0, len(entity.Relations))
		for _, relation := range entity.Relations {
			if relation.EntityID != id {
				kept = append(kept, relation)
			}
		}
		if len(kept) == len(entity.Relations) {
			continue
		}
		entity.Relations = kept
		if err := fsutil.WriteJSONAtomic(file, entity, 0o644); err != nil {
			slog.Warn("cannot drop deleted entity relation", "file", entry.Name(), "error", err)
		}
	}
}

// ListTables returns every valid game-data table, newest first.
func (s *Store) ListTables() ([]DataTable, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	return s.readTables()
}

// readTables loads the table directory; callers must hold the store lock.
func (s *Store) readTables() ([]DataTable, error) {
	entries, err := os.ReadDir(TablesDir(s.root))
	if err != nil {
		if os.IsNotExist(err) {
			return []DataTable{}, nil
		}
		return nil, apperr.Wrap("IO", "无法读取数值表目录", err)
	}
	tables := make([]DataTable, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var table DataTable
		if err := fsutil.ReadJSON(filepath.Join(TablesDir(s.root), entry.Name()), &table); err != nil {
			slog.Warn("skipping unreadable table", "file", entry.Name(), "error", err)
			continue
		}
		if err := ValidateTable(&table); err != nil {
			slog.Warn("skipping invalid table", "file", entry.Name(), "error", err)
			continue
		}
		tables = append(tables, table)
	}
	sort.Slice(tables, func(i, j int) bool { return tables[i].UpdatedAt.After(tables[j].UpdatedAt) })
	return tables, nil
}

// SaveTable writes one table. Table keys share a project-level namespace
// because exports are addressed by key.
func (s *Store) SaveTable(table DataTable) (DataTable, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return DataTable{}, err
	}
	if err := ValidateTable(&table); err != nil {
		return DataTable{}, err
	}
	existing, err := s.readTables()
	if err != nil {
		return DataTable{}, err
	}
	for _, other := range existing {
		if other.ID != table.ID && other.Key == table.Key {
			return DataTable{}, apperr.Newf("TABLE_KEY_TAKEN", "表键「%s」已被「%s」使用", table.Key, other.Name).
				WithDetail("表键导出为一组文件，项目内必须唯一。")
		}
	}
	table.UpdatedAt = time.Now().UTC()
	if err := fsutil.WriteJSONAtomic(tableFile(s.root, table.ID), table, 0o644); err != nil {
		return DataTable{}, apperr.Wrap("IO", "无法保存数值表", err)
	}
	return table, nil
}

// DeleteTable moves one table into the project trash. A table used by another
// table's reference column is kept, so no column silently loses its target.
func (s *Store) DeleteTable(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if !ValidID(id) {
		return apperr.New("INVALID_TABLE", "数值表 ID 不合法")
	}
	src := tableFile(s.root, id)
	if _, err := os.Stat(src); err != nil {
		return apperr.New("TABLE_NOT_FOUND", "数值表不存在")
	}
	tables, err := s.readTables()
	if err != nil {
		return err
	}
	users := make([]string, 0, 2)
	for _, table := range tables {
		if table.ID == id {
			continue
		}
		for _, col := range table.Columns {
			if col.Type == ColumnRef && col.RefTableID == id {
				users = append(users, table.Name)
				break
			}
		}
	}
	if len(users) > 0 {
		return apperr.New("TABLE_IN_USE", "这张表还被其他表引用，不能删除").
			WithDetail(strings.Join(users, "、") + " 把它的行当作引用目标。先把这些列改成别的表。")
	}
	dst := filepath.Join(WorkbenchDir(s.root), "trash", "tables", fmt.Sprintf("%s-%d.json", id, time.Now().Unix()))
	if err := os.Rename(src, dst); err != nil {
		if err := fsutil.CopyFile(src, dst); err != nil {
			return apperr.Wrap("IO", "无法删除数值表", err)
		}
		_ = os.Remove(src)
	}
	return nil
}

// GetAsset returns one validated asset manifest.
func (s *Store) GetAsset(id string) (Asset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureOpen(); err != nil {
		return Asset{}, err
	}
	if !ValidID(id) {
		return Asset{}, apperr.New("INVALID_ASSET", "素材 ID 不合法")
	}
	var asset Asset
	if err := fsutil.ReadJSON(assetManifest(s.root, id), &asset); err != nil {
		if os.IsNotExist(err) {
			return Asset{}, apperr.New("ASSET_NOT_FOUND", "素材不存在").WithDetail(id)
		}
		return Asset{}, apperr.Wrap("IO", "无法读取素材清单", err)
	}
	if err := ValidateAsset(&asset); err != nil {
		return Asset{}, err
	}
	return asset, nil
}

// ListAssets returns all valid asset manifests, newest first.
func (s *Store) ListAssets() ([]Asset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(AssetsDir(s.root))
	if err != nil {
		if os.IsNotExist(err) {
			return []Asset{}, nil
		}
		return nil, apperr.Wrap("IO", "无法读取素材目录", err)
	}
	assets := make([]Asset, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !ValidID(entry.Name()) {
			continue
		}
		var asset Asset
		manifest := assetManifest(s.root, entry.Name())
		if err := fsutil.ReadJSON(manifest, &asset); err != nil {
			slog.Warn("skipping unreadable asset", "file", manifest, "error", err)
			continue
		}
		if err := ValidateAsset(&asset); err != nil {
			slog.Warn("skipping invalid asset", "file", manifest, "error", err)
			continue
		}
		assets = append(assets, asset)
	}
	sort.Slice(assets, func(i, j int) bool { return assets[i].UpdatedAt.After(assets[j].UpdatedAt) })
	return assets, nil
}

// SaveAsset writes one asset manifest, creating its directory if needed.
func (s *Store) SaveAsset(asset Asset) (Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return Asset{}, err
	}
	if err := ValidateAsset(&asset); err != nil {
		return Asset{}, err
	}
	if err := os.MkdirAll(assetRoot(s.root, asset.ID), 0o755); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法创建素材目录", err)
	}
	asset.UpdatedAt = time.Now().UTC()
	if err := fsutil.WriteJSONAtomic(assetManifest(s.root, asset.ID), asset, 0o644); err != nil {
		return Asset{}, apperr.Wrap("IO", "无法保存素材", err)
	}
	return asset, nil
}

// DeleteAsset moves the whole asset directory (manifest, source, versions and
// thumbnails) into the recoverable project trash.
func (s *Store) DeleteAsset(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if !ValidID(id) {
		return apperr.New("INVALID_ASSET", "素材 ID 不合法")
	}
	src := assetRoot(s.root, id)
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return apperr.New("ASSET_NOT_FOUND", "素材不存在").WithDetail(id)
	}
	dst := filepath.Join(
		WorkbenchDir(s.root),
		"trash",
		"assets",
		fmt.Sprintf("%s-%d", id, time.Now().Unix()),
	)
	if err := fsutil.MoveDir(src, dst); err != nil {
		return apperr.Wrap("IO", "无法删除素材", err)
	}
	return nil
}

// ListPalettes returns all palettes, sorted by name.
func (s *Store) ListPalettes() ([]Palette, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(PalettesDir(s.root))
	if err != nil {
		if os.IsNotExist(err) {
			return []Palette{}, nil
		}
		return nil, apperr.Wrap("IO", "无法读取调色板目录", err)
	}
	palettes := make([]Palette, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var palette Palette
		if err := fsutil.ReadJSON(filepath.Join(PalettesDir(s.root), entry.Name()), &palette); err != nil {
			slog.Warn("skipping unreadable palette", "file", entry.Name(), "error", err)
			continue
		}
		if err := ValidatePalette(&palette); err != nil {
			slog.Warn("skipping invalid palette", "file", entry.Name(), "error", err)
			continue
		}
		palettes = append(palettes, palette)
	}
	sort.Slice(palettes, func(i, j int) bool { return palettes[i].Name < palettes[j].Name })
	return palettes, nil
}

// SavePalette writes one palette.
func (s *Store) SavePalette(palette Palette) (Palette, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return Palette{}, err
	}
	if err := ValidatePalette(&palette); err != nil {
		return Palette{}, err
	}
	if err := fsutil.WriteJSONAtomic(paletteFile(s.root, palette.ID), palette, 0o644); err != nil {
		return Palette{}, apperr.Wrap("IO", "无法保存调色板", err)
	}
	return palette, nil
}

// DeletePalette moves a palette into the recoverable trash. The project
// default and palettes referenced by assets or quest briefs are protected.
func (s *Store) DeletePalette(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return apperr.New("INVALID_PALETTE", "调色板 ID 为空")
	}
	if id == s.project.Settings.DefaultPaletteID {
		return apperr.New("PALETTE_DEFAULT", "不能删除项目默认调色板，先把默认切换成其他调色板")
	}
	file := paletteFile(s.root, id)
	if _, err := os.Stat(file); err != nil {
		return apperr.New("PALETTE_NOT_FOUND", "调色板不存在").WithDetail(id)
	}
	assets, quests := s.paletteReferences(id)
	if assets+quests > 0 {
		return apperr.Newf("PALETTE_IN_USE", "这个调色板还被 %d 个素材、%d 个任务引用，先改掉引用再删", assets, quests)
	}
	trash := filepath.Join(WorkbenchDir(s.root), "trash", "palettes")
	if err := os.MkdirAll(trash, 0o755); err != nil {
		return apperr.Wrap("IO", "无法创建回收目录", err)
	}
	dst := filepath.Join(trash, fmt.Sprintf("%s-%d.json", id, time.Now().Unix()))
	if err := os.Rename(file, dst); err != nil {
		return apperr.Wrap("IO", "无法删除调色板", err)
	}
	return nil
}

func (s *Store) paletteReferences(id string) (assets, quests int) {
	count := func(dir string, check func([]byte) bool) int {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return 0
		}
		total := 0
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err == nil && check(raw) {
				total++
			}
		}
		return total
	}
	assets = count(AssetsDir(s.root), func(raw []byte) bool {
		var asset struct {
			PaletteID string `json:"paletteId"`
		}
		return json.Unmarshal(raw, &asset) == nil && asset.PaletteID == id
	})
	quests = count(QuestsDir(s.root), func(raw []byte) bool {
		var quest struct {
			Brief struct {
				PaletteID string `json:"paletteId"`
			} `json:"brief"`
		}
		return json.Unmarshal(raw, &quest) == nil && quest.Brief.PaletteID == id
	})
	return assets, quests
}

// ListMaps returns summaries of every valid map, newest first.
func (s *Store) ListMaps() ([]MapSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureOpen(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(MapsDir(s.root))
	if err != nil {
		if os.IsNotExist(err) {
			return []MapSummary{}, nil
		}
		return nil, apperr.Wrap("IO", "无法读取地图目录", err)
	}
	maps := make([]MapSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".map.json") {
			continue
		}
		var m GameMap
		if err := fsutil.ReadJSON(filepath.Join(MapsDir(s.root), entry.Name()), &m); err != nil {
			slog.Warn("skipping unreadable map", "file", entry.Name(), "error", err)
			continue
		}
		if err := ValidateMap(&m); err != nil {
			slog.Warn("skipping invalid map", "file", entry.Name(), "error", err)
			continue
		}
		maps = append(maps, m.Summary())
	}
	sort.Slice(maps, func(i, j int) bool { return maps[i].UpdatedAt.After(maps[j].UpdatedAt) })
	return maps, nil
}

// LoadMap reads and validates one map.
func (s *Store) LoadMap(id string) (GameMap, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureOpen(); err != nil {
		return GameMap{}, err
	}
	if !ValidID(id) {
		return GameMap{}, apperr.New("INVALID_MAP", "地图 ID 不合法")
	}
	var m GameMap
	if err := fsutil.ReadJSON(MapPath(s.root, id), &m); err != nil {
		if os.IsNotExist(err) {
			return GameMap{}, apperr.New("MAP_NOT_FOUND", "地图不存在").WithDetail(id)
		}
		return GameMap{}, apperr.Wrap("IO", "无法读取地图", err)
	}
	if err := ValidateMap(&m); err != nil {
		return GameMap{}, err
	}
	return m, nil
}

// SaveMap writes one map atomically after validation.
func (s *Store) SaveMap(m GameMap) (GameMap, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return GameMap{}, err
	}
	if err := ValidateMap(&m); err != nil {
		return GameMap{}, err
	}
	if err := os.MkdirAll(MapsDir(s.root), 0o755); err != nil {
		return GameMap{}, apperr.Wrap("IO", "无法创建地图目录", err)
	}
	m.UpdatedAt = time.Now().UTC()
	if err := fsutil.WriteJSONAtomic(MapPath(s.root, m.ID), m, 0o644); err != nil {
		return GameMap{}, apperr.Wrap("IO", "无法保存地图", err)
	}
	return m, nil
}

// DeleteMap moves a map into the recoverable trash.
func (s *Store) DeleteMap(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureOpen(); err != nil {
		return err
	}
	if !ValidID(id) {
		return apperr.New("INVALID_MAP", "地图 ID 不合法")
	}
	src := MapPath(s.root, id)
	if _, err := os.Stat(src); err != nil {
		return apperr.New("MAP_NOT_FOUND", "地图不存在")
	}
	dst := filepath.Join(WorkbenchDir(s.root), "trash", "maps", fmt.Sprintf("%s-%d.map.json", id, time.Now().Unix()))
	if err := os.Rename(src, dst); err != nil {
		if err := fsutil.CopyFile(src, dst); err != nil {
			return apperr.Wrap("IO", "无法删除地图", err)
		}
		_ = os.Remove(src)
	}
	return nil
}

// Summary projects the list payload.
func (m GameMap) Summary() MapSummary {
	return MapSummary{
		ID:        m.ID,
		Name:      m.Name,
		TileSize:  m.TileSize,
		Width:     m.Width,
		Height:    m.Height,
		UpdatedAt: m.UpdatedAt,
	}
}

// AssetPath resolves a file inside an asset directory, rejecting traversal.
func (s *Store) AssetPath(assetID, rel string) (string, error) {
	if !ValidID(assetID) {
		return "", apperr.New("INVALID_ASSET", "素材 ID 不合法")
	}
	rel = strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(rel), "\\", "/"), "/")
	if rel == "" {
		rel = "thumb.png"
	}
	cleaned := path.Clean(rel)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || path.IsAbs(cleaned) {
		return "", apperr.New("INVALID_PATH", "素材文件路径不合法")
	}
	root := s.Root()
	base := assetRoot(root, assetID)
	abs := filepath.Join(base, filepath.FromSlash(cleaned))
	if !fsutil.Within(base, abs) {
		return "", apperr.New("INVALID_PATH", "素材文件路径不合法")
	}
	return abs, nil
}

func ensureDefaultPalette(root string) error {
	file := paletteFile(root, "default")
	if _, err := os.Stat(file); err == nil {
		// Older projects named the built-in palette after the town template.
		// Rename only that exact name; a user-chosen name is left alone.
		var palette Palette
		if err := fsutil.ReadJSON(file, &palette); err == nil && palette.Name == "暖色小镇 16" {
			palette.Name = "默认调色板"
			if err := fsutil.WriteJSONAtomic(file, palette, 0o644); err != nil {
				return apperr.Wrap("IO", "无法更新默认调色板名称", err)
			}
		}
		return nil
	}
	palette := DefaultPalette()
	if err := fsutil.WriteJSONAtomic(file, palette, 0o644); err != nil {
		return apperr.Wrap("IO", "无法写入默认调色板", err)
	}
	return nil
}

func writeProjectDocs(root string, project Project) error {
	readme := fmt.Sprintf(`# %s

由 Indie Studio 创建的游戏项目。

- 项目文件：project.json
- 素材库：assets/
- 任务：quests/
- 设定库：entities/
- 数值表：tables/
- 调色板：palettes/
- 地图：maps/
- 导出：exports/

请把整个文件夹纳入 Git。.workbench/ 下的缓存、日志、回收站和备份已经写入 .gitignore。
`, project.Name)
	if err := fsutil.WriteFileAtomic(filepath.Join(root, "README.md"), []byte(readme), 0o644); err != nil {
		return apperr.Wrap("IO", "无法写入项目说明", err)
	}
	gitignore := `.workbench/cache/
.workbench/logs/
.workbench/autosave/
.workbench/trash/
.workbench/backups/
.workbench/project.lock
*.tmp
`
	if err := fsutil.WriteFileAtomic(filepath.Join(root, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		return apperr.Wrap("IO", "无法写入 .gitignore", err)
	}
	keep := filepath.Join(root, "exports", ".gitkeep")
	if err := fsutil.WriteFileAtomic(keep, []byte{}, 0o644); err != nil {
		return apperr.Wrap("IO", "无法初始化导出目录", err)
	}
	return nil
}

func cloneProject(p Project) Project {
	p.Members = append([]Member(nil), p.Members...)
	p.Milestones = append([]Milestone(nil), p.Milestones...)
	return p
}
