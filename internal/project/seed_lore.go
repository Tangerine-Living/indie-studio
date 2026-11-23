package project

import (
	"os"

	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
)

// seedEntities writes two linked setting entries so a new project shows what
// the setting library is for: one character and the place they arrive at.
func seedEntities(root string, grass, atlas Asset, townMap GameMap) error {
	if err := os.MkdirAll(EntitiesDir(root), 0o755); err != nil {
		return apperr.Wrap("IO", "无法创建设定目录", err)
	}

	plaza := Entity{
		SchemaVersion: SchemaVersion,
		ID:            NewID(),
		Kind:          EntityPlace,
		Name:          "小镇广场",
		Status:        EntityStable,
		Aliases:       []string{"石铺广场"},
		Summary:       "小镇唯一的石铺广场，白天是集市，入夜只剩一盏路灯。",
		Body:          "广场铺着磨损的青石板，中间那圈颜色最深，是常年摆摊留下的痕迹。\n\n北侧是镇长家，东侧通往码头。节日时会在这里搭台，平时午后就只剩几条野狗和收摊的小贩。",
		Tags:          []string{"城镇", "集市"},
		Fields: []EntityField{
			{Key: "人口", Value: "约 400 人"},
			{Key: "归属", Value: "雾湾镇"},
		},
		Relations: []EntityRelation{},
		AssetIDs:  []string{atlas.ID, grass.ID},
		MapIDs:    []string{townMap.ID},
	}
	traveler := Entity{
		SchemaVersion: SchemaVersion,
		ID:            NewID(),
		Kind:          EntityCharacter,
		Name:          "旅人",
		Status:        EntityDraft,
		Aliases:       []string{"外乡人"},
		Summary:       "刚从北边来到小镇的年轻人，背着一只补过三次的布包。",
		Body:          "他不肯说自己的名字，只说要在这里等一个人。\n\n说话很少，付钱很准时。布包里除了干粮，还有一张被雨泡皱的地图。",
		Tags:          []string{"主角"},
		Fields: []EntityField{
			{Key: "年龄", Value: "二十出头"},
			{Key: "随身物", Value: "布包、旧地图"},
		},
		Relations: []EntityRelation{
			{EntityID: plaza.ID, Label: "初到"},
		},
		AssetIDs: []string{},
		MapIDs:   []string{},
	}

	for _, entity := range []Entity{plaza, traveler} {
		if err := ValidateEntity(&entity); err != nil {
			return err
		}
		if err := fsutil.WriteJSONAtomic(entityFile(root, entity.ID), entity, 0o644); err != nil {
			return apperr.Wrap("IO", "无法写入示例设定", err)
		}
	}
	return nil
}
