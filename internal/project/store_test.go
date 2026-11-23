package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pixelbench/internal/apperr"
)

func testConfig(t *testing.T) {
	t.Helper()
	t.Setenv("PIXELBENCH_CONFIG_DIR", t.TempDir())
}

func newTestStore(t *testing.T, name string) *Store {
	t.Helper()
	testConfig(t)
	store, err := Create(CreateOptions{
		ParentDir: t.TempDir(),
		Name:      name,
		Engine:    "godot",
		TileSize:  32,
		Template:  "town-square",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestDeletePaletteProtectsDefaultAndReferences(t *testing.T) {
	store := newTestStore(t, "调色板测试")

	palettes, err := store.ListPalettes()
	if err != nil {
		t.Fatal(err)
	}
	if palettes[0].Name != "默认调色板" {
		t.Fatalf("default palette name = %q", palettes[0].Name)
	}

	if err := store.DeletePalette("default"); !apperr.Is(err, "PALETTE_DEFAULT") {
		t.Fatalf("delete default err = %v", err)
	}

	custom := DefaultPalette()
	custom.ID = "custom-palette"
	custom.Name = "副本调色板"
	if _, err := store.SavePalette(custom); err != nil {
		t.Fatal(err)
	}

	quests, err := store.ListQuests()
	if err != nil || len(quests) == 0 {
		t.Fatalf("quests = %d err = %v", len(quests), err)
	}
	quest := quests[0]
	quest.ID = NewID()
	quest.Title = "引用调色板的任务"
	quest.Brief.PaletteID = custom.ID
	quest, err = store.SaveQuest(quest)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeletePalette(custom.ID); !apperr.Is(err, "PALETTE_IN_USE") {
		t.Fatalf("delete referenced err = %v", err)
	}
	if err := store.DeleteQuest(quest.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeletePalette(custom.ID); err != nil {
		t.Fatalf("delete unused err = %v", err)
	}
	remaining, err := store.ListPalettes()
	if err != nil {
		t.Fatal(err)
	}
	for _, palette := range remaining {
		if palette.ID == custom.ID {
			t.Fatal("deleted palette still listed")
		}
	}
	if _, err := os.Stat(filepath.Join(WorkbenchDir(store.Root()), "trash", "palettes")); err != nil {
		t.Fatalf("trash dir: %v", err)
	}
}

func TestCreateSeedsWorkspace(t *testing.T) {
	store := newTestStore(t, "我的 游戏")
	session := store.Session()

	if session.Project.Name != "我的 游戏" {
		t.Fatalf("name = %q", session.Project.Name)
	}
	if session.Path != store.Root() {
		t.Fatalf("session path = %q, root = %q", session.Path, store.Root())
	}
	if !ValidID(session.Project.ID) {
		t.Fatalf("invalid project id %q", session.Project.ID)
	}
	for _, rel := range []string{"project.json", "README.md", ".gitignore", "palettes/default.json", "exports/.gitkeep"} {
		if _, err := os.Stat(filepath.Join(store.Root(), filepath.FromSlash(rel))); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}

	quests, err := store.ListQuests()
	if err != nil {
		t.Fatal(err)
	}
	if len(quests) != 1 {
		t.Fatalf("quests = %d, want 1", len(quests))
	}

	assets, err := store.ListAssets()
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 {
		t.Fatalf("assets = %d, want 2 (grass + atlas)", len(assets))
	}
	thumb, err := store.AssetPath(assets[0].ID, "thumb.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(thumb); err != nil {
		t.Fatalf("missing thumbnail: %v", err)
	}

	maps, err := store.ListMaps()
	if err != nil {
		t.Fatal(err)
	}
	if len(maps) != 1 || maps[0].Name != "小镇广场" {
		t.Fatalf("maps = %+v", maps)
	}
	seeded, err := store.LoadMap(maps[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if seeded.Width != 40 || seeded.Height != 30 || len(seeded.Layers) != 6 || len(seeded.Objects) != 4 {
		t.Fatalf("seeded map = %dx%d, %d layers, %d objects", seeded.Width, seeded.Height, len(seeded.Layers), len(seeded.Objects))
	}
	if quests[0].Links.MapIDs[0] != seeded.ID {
		t.Fatalf("quest map link = %v, map = %s", quests[0].Links.MapIDs, seeded.ID)
	}

	palettes, err := store.ListPalettes()
	if err != nil {
		t.Fatal(err)
	}
	if len(palettes) != 1 || len(palettes[0].Colors) != 16 {
		t.Fatalf("palette = %+v", palettes)
	}
}

func TestBlankTemplateCreatesOnlyScaffold(t *testing.T) {
	testConfig(t)
	store, err := Create(CreateOptions{
		ParentDir: t.TempDir(),
		Name:      "空白 项目",
		Engine:    "tiled",
		TileSize:  16,
		Template:  "blank",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if store.Session().Project.Settings.Template != "blank" {
		t.Fatalf("template = %q", store.Session().Project.Settings.Template)
	}
	for _, rel := range []string{"project.json", "palettes/default.json"} {
		if _, err := os.Stat(filepath.Join(store.Root(), filepath.FromSlash(rel))); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}

	quests, err := store.ListQuests()
	if err != nil {
		t.Fatal(err)
	}
	assets, err := store.ListAssets()
	if err != nil {
		t.Fatal(err)
	}
	maps, err := store.ListMaps()
	if err != nil {
		t.Fatal(err)
	}
	entities, err := store.ListEntities()
	if err != nil {
		t.Fatal(err)
	}
	tables, err := store.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	if len(quests)+len(assets)+len(maps)+len(entities)+len(tables) != 0 {
		t.Fatalf("blank template seeded content: %d quests, %d assets, %d maps, %d entities, %d tables",
			len(quests), len(assets), len(maps), len(entities), len(tables))
	}

	palettes, err := store.ListPalettes()
	if err != nil {
		t.Fatal(err)
	}
	if len(palettes) != 1 || len(palettes[0].Colors) != 16 {
		t.Fatalf("palette = %+v", palettes)
	}
	if n := len(store.Session().Project.Milestones); n != 0 {
		t.Fatalf("blank project has %d default milestones", n)
	}
}

func TestSaveProjectQuestAndDelete(t *testing.T) {
	store := newTestStore(t, "保存测试")

	session := store.Session()
	session.Project.Name = "新的名字"
	saved, err := store.SaveProject(session.Project)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Project.Name != "新的名字" {
		t.Fatalf("saved name = %q", saved.Project.Name)
	}

	quests, err := store.ListQuests()
	if err != nil {
		t.Fatal(err)
	}
	quest := quests[0]
	quest.Status = QuestDone
	quest.Title = "已完成的任务"
	savedQuest, err := store.SaveQuest(quest)
	if err != nil {
		t.Fatal(err)
	}
	if savedQuest.Status != QuestDone || savedQuest.ID != quest.ID {
		t.Fatalf("saved quest = %+v", savedQuest)
	}

	fresh := Quest{
		Title:    "新任务",
		Type:     QuestTypeCode,
		Status:   QuestBacklog,
		Priority: PriorityLow,
	}
	created, err := store.SaveQuest(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || !ValidID(created.ID) {
		t.Fatalf("generated id = %q", created.ID)
	}
	if created.Brief.TileSize != 32 {
		t.Fatalf("brief tile size = %d", created.Brief.TileSize)
	}

	all, err := store.ListQuests()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("quests = %d, want 2", len(all))
	}

	if err := store.DeleteQuest(created.ID); err != nil {
		t.Fatal(err)
	}
	all, err = store.ListQuests()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("quests after delete = %d, want 1", len(all))
	}
	trash, err := os.ReadDir(filepath.Join(WorkbenchDir(store.Root()), "trash", "quests"))
	if err != nil {
		t.Fatal(err)
	}
	if len(trash) != 1 {
		t.Fatalf("trash entries = %d, want 1", len(trash))
	}
}

func TestDeleteAssetMovesToTrash(t *testing.T) {
	store := newTestStore(t, "删除 素材")
	assets, err := store.ListAssets()
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) == 0 {
		t.Fatal("seed produced no assets")
	}
	target := assets[0]

	if err := store.DeleteAsset(target.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(AssetDir(store.Root(), target.ID)); !os.IsNotExist(err) {
		t.Fatalf("asset dir still exists: %v", err)
	}
	remaining, err := store.ListAssets()
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != len(assets)-1 {
		t.Fatalf("assets after delete = %d, want %d", len(remaining), len(assets)-1)
	}
	trash, err := os.ReadDir(filepath.Join(WorkbenchDir(store.Root()), "trash", "assets"))
	if err != nil {
		t.Fatal(err)
	}
	if len(trash) != 1 {
		t.Fatalf("trash entries = %d, want 1", len(trash))
	}
	if err := store.DeleteAsset(target.ID); err == nil {
		t.Fatal("second delete should report a missing asset")
	}
}

func TestProjectStageAndCover(t *testing.T) {
	store := newTestStore(t, "阶段测试")
	session := store.Session()
	if session.Project.Stage != StageConcept {
		t.Fatalf("default stage = %q, want %q", session.Project.Stage, StageConcept)
	}

	project := session.Project
	project.Stage = StageProduction
	project.CoverAssetID = "../escape"
	saved, err := store.SaveProject(project)
	if err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	if saved.Project.Stage != StageProduction {
		t.Fatalf("stage = %q", saved.Project.Stage)
	}
	if saved.Project.CoverAssetID != "" {
		t.Fatalf("unsafe cover id kept: %q", saved.Project.CoverAssetID)
	}

	project = saved.Project
	project.Stage = "shipping"
	if _, err := store.SaveProject(project); err == nil {
		t.Fatal("expected an invalid stage to be rejected")
	}
}

func TestLockIsOwnedByToken(t *testing.T) {
	store := newTestStore(t, "锁测试")
	root := store.Root()

	if _, err := Open(root, false); !apperr.Is(err, "PROJECT_LOCKED") {
		t.Fatalf("second open error = %v, want PROJECT_LOCKED", err)
	}

	forced, err := Open(root, true)
	if err != nil {
		t.Fatalf("force open: %v", err)
	}

	// Closing the original store must not release the forced store's lock.
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root, false); !apperr.Is(err, "PROJECT_LOCKED") {
		t.Fatalf("open after closing stale store = %v, want PROJECT_LOCKED", err)
	}

	if err := forced.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(root, false)
	if err != nil {
		t.Fatalf("reopen after force close: %v", err)
	}
	_ = reopened.Close()
}

func TestCreateRejectsBadNamesAndDuplicates(t *testing.T) {
	testConfig(t)
	parent := t.TempDir()
	opts := CreateOptions{ParentDir: parent, Name: "重复项目", TileSize: 32, Engine: "godot"}

	first, err := Create(opts)
	if err != nil {
		t.Fatal(err)
	}
	_ = first.Close()

	if _, err := Create(opts); !apperr.Is(err, "PROJECT_EXISTS") {
		t.Fatalf("duplicate create error = %v, want PROJECT_EXISTS", err)
	}

	for _, name := range []string{"bad/name", "bad:name", "CON"} {
		bad := opts
		bad.Name = name
		if _, err := Create(bad); !apperr.Is(err, "INVALID_NAME") {
			t.Fatalf("name %q error = %v, want INVALID_NAME", name, err)
		}
	}
}

func TestEntityLifecycle(t *testing.T) {
	store := newTestStore(t, "设定库测试")

	seeded, err := store.ListEntities()
	if err != nil {
		t.Fatal(err)
	}
	if len(seeded) < 2 {
		t.Fatalf("seeded entities = %d, want at least 2", len(seeded))
	}

	entity := Entity{
		Kind:      EntityCharacter,
		Name:      "  店长  ",
		Aliases:   []string{"老板", " 老板 ", ""},
		Summary:   "杂货铺的店长。",
		Body:      "她记得每一个赊过账的人。",
		Fields:    []EntityField{{Key: "年龄", Value: "五十"}, {Key: " ", Value: "丢掉"}},
		Relations: []EntityRelation{{EntityID: seeded[0].ID, Label: " 熟人 "}, {EntityID: entitySelfID(), Label: "自己"}},
	}
	entity.ID = entitySelfID()
	saved, err := store.SaveEntity(entity)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidID(saved.ID) {
		t.Fatalf("saved id = %q", saved.ID)
	}
	if saved.Name != "店长" {
		t.Fatalf("name = %q, want trimmed", saved.Name)
	}
	if len(saved.Aliases) != 1 || saved.Aliases[0] != "老板" {
		t.Fatalf("aliases = %v", saved.Aliases)
	}
	if len(saved.Fields) != 1 {
		t.Fatalf("fields = %v", saved.Fields)
	}
	if len(saved.Relations) != 1 || saved.Relations[0].Label != "熟人" {
		t.Fatalf("relations = %v", saved.Relations)
	}
	if _, err := os.Stat(entityFile(store.Root(), saved.ID)); err != nil {
		t.Fatalf("entity file missing: %v", err)
	}

	list, err := store.ListEntities()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(seeded)+1 {
		t.Fatalf("entities = %d, want %d", len(list), len(seeded)+1)
	}

	if err := store.DeleteEntity(saved.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteEntity(saved.ID); !apperr.Is(err, "ENTITY_NOT_FOUND") {
		t.Fatalf("second delete = %v, want ENTITY_NOT_FOUND", err)
	}
}

// entitySelfID exists so the fixture can reference its own ID, which
// validation must drop from the relation list.
func entitySelfID() string { return "0123456789abcdef0123456789abcdef" }

func TestDeleteEntityDropsIncomingRelations(t *testing.T) {
	store := newTestStore(t, "关系清理")

	target, err := store.SaveEntity(Entity{Kind: EntityFaction, Name: "盐商行会"})
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.SaveEntity(Entity{
		Kind:      EntityCharacter,
		Name:      "账房",
		Relations: []EntityRelation{{EntityID: target.ID, Label: "受雇于"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteEntity(target.ID); err != nil {
		t.Fatal(err)
	}
	list, err := store.ListEntities()
	if err != nil {
		t.Fatal(err)
	}
	for _, entity := range list {
		if entity.ID != source.ID {
			continue
		}
		if len(entity.Relations) != 0 {
			t.Fatalf("relations after delete = %v, want none", entity.Relations)
		}
		return
	}
	t.Fatalf("source entity %s missing after delete", source.ID)
}

func tableByKey(tables []DataTable, key string) (DataTable, bool) {
	for _, table := range tables {
		if table.Key == key {
			return table, true
		}
	}
	return DataTable{}, false
}

func tableColumnByKey(table DataTable, key string) TableColumn {
	for _, col := range table.Columns {
		if col.Key == key {
			return col
		}
	}
	return TableColumn{}
}

func tablesByID(tables []DataTable) map[string]DataTable {
	out := make(map[string]DataTable, len(tables))
	for _, table := range tables {
		out[table.ID] = table
	}
	return out
}

func TestSeededTablesAreValid(t *testing.T) {
	store := newTestStore(t, "数值表示例")
	tables, err := store.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 3 {
		t.Fatalf("tables = %d, want 3", len(tables))
	}
	byID := tablesByID(tables)
	items, ok := tableByKey(tables, "items")
	if !ok || len(items.Rows) != 4 {
		t.Fatalf("items table = %+v", items)
	}
	enemies, ok := tableByKey(tables, "enemies")
	if !ok {
		t.Fatal("missing enemies table")
	}
	drop := tableColumnByKey(enemies, "drop")
	if drop.Type != ColumnRef || drop.RefTableID != items.ID {
		t.Fatalf("drop column = %+v, want ref to items", drop)
	}
	for _, table := range tables {
		if issues := TableIssues(table, byID); len(issues) != 0 {
			t.Fatalf("seeded table %s has issues: %v", table.Key, issues)
		}
	}
}

func TestSaveTableNormalisesCellsAndKeys(t *testing.T) {
	store := newTestStore(t, "数值表保存")
	tables, err := store.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	byID := tablesByID(tables)
	items, ok := tableByKey(tables, "items")
	if !ok {
		t.Fatal("missing items table")
	}
	items.Name = "  物品表  "
	items.Key = "Item List!"
	name := tableColumnByKey(items, "name")
	kind := tableColumnByKey(items, "kind")
	price := tableColumnByKey(items, "price")
	sellable := tableColumnByKey(items, "sellable")
	items.Rows = append(items.Rows,
		TableRow{ID: NewID(), Values: map[string]any{
			name.ID: "药草", kind.ID: "武器", price.ID: float64(3.9), sellable.ID: true,
		}},
		TableRow{ID: NewID(), Values: map[string]any{
			kind.ID: "消耗品", price.ID: float64(3),
		}},
	)

	saved, err := store.SaveTable(items)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Name != "物品表" || saved.Key != "item_list" {
		t.Fatalf("name/key = %q/%q", saved.Name, saved.Key)
	}
	herb := saved.Rows[len(saved.Rows)-2]
	if got := herb.Values[price.ID]; got != float64(3) {
		t.Fatalf("int cell = %#v, want 3", got)
	}
	if got := herb.Values[name.ID]; got != "药草" {
		t.Fatalf("text cell = %#v", got)
	}
	if got := herb.Values[sellable.ID]; got != true {
		t.Fatalf("bool cell = %#v", got)
	}

	byID[saved.ID] = saved
	issues := TableIssues(saved, byID)
	messages := make([]string, 0, len(issues))
	for _, issue := range issues {
		messages = append(messages, issue.Message)
	}
	for _, want := range []string{"不在可选范围内", "是必填项"} {
		found := false
		for _, message := range messages {
			if strings.Contains(message, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("issues %v missing %q", messages, want)
		}
	}
}

func TestTableKeyMustBeUnique(t *testing.T) {
	store := newTestStore(t, "表键唯一")
	tables, err := store.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	enemies, ok := tableByKey(tables, "enemies")
	if !ok {
		t.Fatal("missing enemies table")
	}
	enemies.ID = NewID()
	enemies.Key = "items"
	if _, err := store.SaveTable(enemies); !apperr.Is(err, "TABLE_KEY_TAKEN") {
		t.Fatalf("duplicate key = %v, want TABLE_KEY_TAKEN", err)
	}
}

func TestDeleteTableGuardsReferences(t *testing.T) {
	store := newTestStore(t, "删除数值表")
	tables, err := store.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	items, ok := tableByKey(tables, "items")
	if !ok {
		t.Fatal("missing items table")
	}
	enemies, ok := tableByKey(tables, "enemies")
	if !ok {
		t.Fatal("missing enemies table")
	}

	if err := store.DeleteTable(items.ID); !apperr.Is(err, "TABLE_IN_USE") {
		t.Fatalf("delete referenced table = %v, want TABLE_IN_USE", err)
	}

	drop := tableColumnByKey(enemies, "drop")
	for index := range enemies.Columns {
		if enemies.Columns[index].ID == drop.ID {
			enemies.Columns[index].Type = ColumnText
			enemies.Columns[index].RefTableID = ""
		}
	}
	if _, err := store.SaveTable(enemies); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteTable(items.ID); err != nil {
		t.Fatal(err)
	}
	list, err := store.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("tables after delete = %d, want 2", len(list))
	}
	if _, ok := tableByKey(list, "items"); ok {
		t.Fatal("items table still listed")
	}
	matches, err := filepath.Glob(filepath.Join(WorkbenchDir(store.Root()), "trash", "tables", "*.json"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("trash files = %v, %v", matches, err)
	}
	if err := store.DeleteTable(items.ID); !apperr.Is(err, "TABLE_NOT_FOUND") {
		t.Fatalf("second delete = %v, want TABLE_NOT_FOUND", err)
	}
}
