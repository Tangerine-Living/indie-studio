package project

import (
	"pixelbench/internal/apperr"
	"pixelbench/internal/fsutil"
)

// seedTables writes the three starter tables a new project opens with. They
// show a typed enum, a numeric range, a boolean and a cross-table reference,
// so the validation panel has something real to point at.
func seedTables(root string) error {
	float := func(value float64) *float64 { return &value }

	itemName := TableColumn{ID: NewID(), Name: "名称", Key: "name", Type: ColumnText, Required: true}
	itemKind := TableColumn{
		ID: NewID(), Name: "类型", Key: "kind", Type: ColumnEnum, Required: true,
		Options: []string{"消耗品", "装备", "材料"},
	}
	itemPrice := TableColumn{ID: NewID(), Name: "售价", Key: "price", Type: ColumnInt, Min: float(0)}
	itemStack := TableColumn{ID: NewID(), Name: "堆叠上限", Key: "stack", Type: ColumnInt, Min: float(1)}
	itemSellable := TableColumn{ID: NewID(), Name: "可出售", Key: "sellable", Type: ColumnBool}
	items := DataTable{
		SchemaVersion: SchemaVersion,
		ID:            NewID(),
		Name:          "物品",
		Key:           "items",
		Note:          "背包里的每件东西都在这里。掉落表通过引用指向本表的行。",
		Columns:       []TableColumn{itemName, itemKind, itemPrice, itemStack, itemSellable},
		Rows: []TableRow{
			{ID: NewID(), Values: map[string]any{
				itemName.ID: "苹果", itemKind.ID: "消耗品", itemPrice.ID: float64(12),
				itemStack.ID: float64(99), itemSellable.ID: true,
			}},
			{ID: NewID(), Values: map[string]any{
				itemName.ID: "面包", itemKind.ID: "消耗品", itemPrice.ID: float64(8),
				itemStack.ID: float64(99), itemSellable.ID: true,
			}},
			{ID: NewID(), Values: map[string]any{
				itemName.ID: "铁剑", itemKind.ID: "装备", itemPrice.ID: float64(320),
				itemStack.ID: float64(1), itemSellable.ID: true,
			}},
			{ID: NewID(), Values: map[string]any{
				itemName.ID: "旧地图", itemKind.ID: "材料", itemPrice.ID: float64(60),
				itemStack.ID: float64(1), itemSellable.ID: false,
			}},
		},
	}

	skillName := TableColumn{ID: NewID(), Name: "名称", Key: "name", Type: ColumnText, Required: true}
	skillKind := TableColumn{
		ID: NewID(), Name: "类型", Key: "kind", Type: ColumnEnum, Required: true,
		Options: []string{"攻击", "辅助", "被动"},
	}
	skillCooldown := TableColumn{ID: NewID(), Name: "冷却（秒）", Key: "cooldown", Type: ColumnNumber, Min: float(0)}
	skillCost := TableColumn{ID: NewID(), Name: "消耗", Key: "cost", Type: ColumnInt, Min: float(0)}
	skillNote := TableColumn{ID: NewID(), Name: "说明", Key: "note", Type: ColumnText}
	skills := DataTable{
		SchemaVersion: SchemaVersion,
		ID:            NewID(),
		Name:          "技能",
		Key:           "skills",
		Note:          "战斗与探索技能。冷却可以是小数，消耗按整数计算。",
		Columns:       []TableColumn{skillName, skillKind, skillCooldown, skillCost, skillNote},
		Rows: []TableRow{
			{ID: NewID(), Values: map[string]any{
				skillName.ID: "挥砍", skillKind.ID: "攻击", skillCooldown.ID: float64(0.8),
				skillCost.ID: float64(0), skillNote.ID: "向前方挥出一击，命中后短暂硬直。",
			}},
			{ID: NewID(), Values: map[string]any{
				skillName.ID: "突刺", skillKind.ID: "攻击", skillCooldown.ID: float64(2.5),
				skillCost.ID: float64(6), skillNote.ID: "向前突进两格，可以穿过低矮障碍。",
			}},
			{ID: NewID(), Values: map[string]any{
				skillName.ID: "回复术", skillKind.ID: "辅助", skillCooldown.ID: float64(8),
				skillCost.ID: float64(12), skillNote.ID: "恢复少量生命，对友方也生效。",
			}},
			{ID: NewID(), Values: map[string]any{
				skillName.ID: "疾行", skillKind.ID: "被动", skillCooldown.ID: float64(0),
				skillCost.ID: float64(0), skillNote.ID: "脱离战斗时移动速度提高 20%。",
			}},
		},
	}

	enemyName := TableColumn{ID: NewID(), Name: "名称", Key: "name", Type: ColumnText, Required: true}
	enemyHP := TableColumn{ID: NewID(), Name: "生命", Key: "hp", Type: ColumnInt, Min: float(1)}
	enemyAttack := TableColumn{ID: NewID(), Name: "攻击", Key: "attack", Type: ColumnInt, Min: float(0)}
	enemySpeed := TableColumn{ID: NewID(), Name: "速度", Key: "speed", Type: ColumnNumber, Min: float(0), Max: float(10)}
	enemyDrop := TableColumn{
		ID: NewID(), Name: "掉落", Key: "drop", Type: ColumnRef, RefTableID: items.ID,
	}
	enemies := DataTable{
		SchemaVersion: SchemaVersion,
		ID:            NewID(),
		Name:          "敌人",
		Key:           "enemies",
		Note:          "镇外会遇到的敌人。掉落引用「物品」表的行，删掉那一行后这里会报错。",
		Columns:       []TableColumn{enemyName, enemyHP, enemyAttack, enemySpeed, enemyDrop},
		Rows: []TableRow{
			{ID: NewID(), Values: map[string]any{
				enemyName.ID: "野狗", enemyHP.ID: float64(30), enemyAttack.ID: float64(4),
				enemySpeed.ID: float64(6), enemyDrop.ID: items.Rows[1].ID,
			}},
			{ID: NewID(), Values: map[string]any{
				enemyName.ID: "盗贼", enemyHP.ID: float64(60), enemyAttack.ID: float64(7),
				enemySpeed.ID: float64(7), enemyDrop.ID: items.Rows[0].ID,
			}},
			{ID: NewID(), Values: map[string]any{
				enemyName.ID: "骷髅兵", enemyHP.ID: float64(90), enemyAttack.ID: float64(6),
				enemySpeed.ID: float64(3), enemyDrop.ID: items.Rows[2].ID,
			}},
		},
	}

	for _, table := range []*DataTable{&items, &skills, &enemies} {
		if err := ValidateTable(table); err != nil {
			return err
		}
		if err := fsutil.WriteJSONAtomic(tableFile(root, table.ID), table, 0o644); err != nil {
			return apperr.Wrap("IO", "无法写入示例数值表", err)
		}
	}
	return nil
}