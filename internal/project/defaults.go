package project

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewID returns a filename-safe, random identifier.
func NewID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

// DefaultNegativePrompt keeps AI drafts away from noisy, non-pixel output.
const DefaultNegativePrompt = "模糊，抗锯齿，照片写实，3D 渲染，文字，水印，多余肢体"

func defaultStyleGuide() StyleGuide {
	return StyleGuide{
		Perspective:    "3/4-topdown-orthographic",
		Outline:        "1px-dark",
		Light:          "top-left",
		MaxColors:      16,
		NegativePrompt: DefaultNegativePrompt,
	}
}

// DefaultPalette is PixelBench's built-in 16-colour palette. Its name stays
// generic: projects start from it regardless of their template or genre.
func DefaultPalette() Palette {
	return Palette{
		SchemaVersion: SchemaVersion,
		ID:            "default",
		Name:          "默认调色板",
		Source:        "pixelbench",
		Colors: []string{
			"#0f1b2a", "#1e2f45", "#2f4b5e", "#3e6b5a",
			"#4f8f4a", "#6fae5f", "#8ccf6f", "#b8e986",
			"#f2e6c9", "#e8c07a", "#d98f4a", "#b85c38",
			"#c94f4f", "#f2b84b", "#6ec6e8", "#f4f4f4",
		},
		Locked: false,
	}
}

func defaultProject(opts CreateOptions, name string) Project {
	now := time.Now().UTC()
	engine := strings.ToLower(strings.TrimSpace(opts.Engine))
	if engine == "" {
		engine = "godot"
	}
	tileSize := opts.TileSize
	if tileSize == 0 {
		tileSize = 32
	}
	template := opts.Template
	if template == "" {
		template = "blank"
	}
	// A blank project carries no example milestones; the town template ships
	// two so the dashboard has something to show.
	milestones := []Milestone{}
	if template == "town-square" {
		milestones = []Milestone{
			{ID: "ms1", Title: "可玩 Demo", DueAt: "", Done: false},
			{ID: "ms2", Title: "商店页面", DueAt: "", Done: false},
		}
	}
	return Project{
		SchemaVersion: SchemaVersion,
		ID:            NewID(),
		Name:          name,
		CreatedAt:     now,
		UpdatedAt:     now,
		Engine:        engine,
		Stage:         StageConcept,
		Settings: Settings{
			TileSize:         tileSize,
			DefaultPaletteID: "default",
			PixelScale:       4,
			UILanguage:       "zh-CN",
			Template:         template,
			StyleGuide:       defaultStyleGuide(),
		},
		Members: []Member{
			{ID: "m1", Name: "我", Role: "独立开发者"},
		},
		Milestones: milestones,
	}
}

func welcomeQuest() Quest {
	now := time.Now().UTC()
	return Quest{
		SchemaVersion: SchemaVersion,
		ID:            NewID(),
		Title:         "创建第一张角色素材",
		Type:          QuestTypeAsset,
		Status:        QuestTodo,
		Priority:      PriorityHigh,
		Assignee:      "m1",
		Description:   "用素材库建立第一个 32×32 的角色精灵，确认调色板、锚点与导出流程。",
		Brief: QuestBrief{
			AssetType:  AssetSprite,
			TileSize:   32,
			PaletteID:  "default",
			Style:      "3/4 俯视，1px 深色描边，光从左上方来",
			References: []string{},
			Prompt:     "",
		},
		Acceptance: []string{
			"32×32 像素，透明背景",
			"不超过 16 色",
			"锚点在脚底中心",
		},
		Links:     QuestLinks{AssetIDs: []string{}, MapIDs: []string{}, URLs: []string{}},
		Comments:  []Comment{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}
