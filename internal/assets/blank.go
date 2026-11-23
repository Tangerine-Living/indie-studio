package assets

import (
	"strings"
	"time"

	"pixelbench/internal/apperr"
	"pixelbench/internal/project"
)

// BlankAssetInput describes a hand-drawn asset created from an empty canvas.
type BlankAssetInput struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	PaletteID string `json:"paletteId"`
}

// CreateBlank writes a draft asset with no source file and no versions. The
// editor opens it on a blank canvas; saving the first version fills in
// versions/0001 and the thumbnail.
func CreateBlank(store *project.Store, input BlankAssetInput) (project.Asset, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return project.Asset{}, apperr.New("INVALID_ASSET", "素材名称不能为空")
	}
	if len([]rune(name)) > 80 {
		return project.Asset{}, apperr.New("INVALID_ASSET", "素材名称不能超过 80 个字符")
	}
	if !allowedTypes[input.Type] {
		return project.Asset{}, apperr.New("INVALID_ASSET", "素材类型不合法").WithDetail(input.Type)
	}
	if input.Width < 1 || input.Height < 1 || input.Width > 2048 || input.Height > 2048 {
		return project.Asset{}, apperr.New("INVALID_ASSET", "画布尺寸需在 1–2048 像素之间")
	}

	paletteID := strings.TrimSpace(input.PaletteID)
	if paletteID == "" {
		paletteID = store.Session().Project.Settings.DefaultPaletteID
	}
	now := time.Now().UTC()
	asset := project.Asset{
		SchemaVersion: project.SchemaVersion,
		ID:            project.NewID(),
		Name:          name,
		Type:          input.Type,
		Tags:          []string{},
		Status:        project.AssetDraft,
		Dimensions:    project.Dimensions{W: input.Width, H: input.Height},
		Pivot:         project.Point{X: input.Width / 2, Y: input.Height - 1},
		PaletteID:     paletteID,
		QuestIDs:      []string{},
		License:       "original",
		Source:        project.AssetSource{Kind: "blank"},
		Versions:      []project.AssetVersion{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return store.SaveAsset(asset)
}