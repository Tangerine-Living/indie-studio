package assets

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"pixelbench/internal/apperr"
	"pixelbench/internal/project"
)

func sheetInfo() AnimationInfo {
	return AnimationInfo{
		FrameWidth:  2,
		FrameHeight: 2,
		FrameCount:  2,
		Columns:     2,
		Rows:        1,
		Margin:      1,
		Spacing:     1,
		FPS:         8,
		Loop:        true,
	}
}

func encodeSheet(t *testing.T, info AnimationInfo) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, info.SheetWidth(), info.SheetHeight()))
	colors := []color.NRGBA{
		{R: 255, A: 255},
		{R: 0, G: 0, B: 255, A: 255},
	}
	for index := 0; index < info.FrameCount; index++ {
		x0, y0 := info.FrameOrigin(index)
		c := colors[index%len(colors)]
		for y := 0; y < info.FrameHeight; y++ {
			for x := 0; x < info.FrameWidth; x++ {
				img.SetNRGBA(x0+x, y0+y, c)
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode sheet: %v", err)
	}
	return buf.Bytes()
}

func TestSaveSheetWritesFramesAndMetadata(t *testing.T) {
	store := newProject(t)
	asset := seededAsset(t, store)
	root := store.Root()
	info := sheetInfo()

	saved, err := SaveSheet(store, asset.ID, encodeSheet(t, info), info)
	if err != nil {
		t.Fatalf("save sheet: %v", err)
	}
	if saved.CurrentVersion != "0001" {
		t.Errorf("current version = %q, want 0001", saved.CurrentVersion)
	}
	if saved.Dimensions.W != 2 || saved.Dimensions.H != 2 {
		t.Errorf("dimensions = %+v, want 2x2 (one frame)", saved.Dimensions)
	}
	if saved.Animation == nil {
		t.Fatal("animation metadata missing")
	}
	if saved.Animation.FrameCount != 2 || saved.Animation.Columns != 2 || saved.Animation.FPS != 8 {
		t.Errorf("animation = %+v", saved.Animation)
	}

	dir := project.AssetDir(root, asset.ID)
	for _, rel := range []string{
		"versions/0001/output.png",
		"versions/0001/meta.json",
		"versions/0001/frames-aseprite.json",
		"versions/0001/frames-texturepacker.json",
		"frames/000.png",
		"frames/001.png",
		"thumb.png",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing %s: %v", rel, err)
		}
	}

	raw, err := os.ReadFile(filepath.Join(dir, "frames/001.png"))
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	frame, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	if frame.Bounds().Dx() != 2 || frame.Bounds().Dy() != 2 {
		t.Errorf("frame bounds = %v, want 2x2", frame.Bounds())
	}
	r, g, b, a := frame.At(0, 0).RGBA()
	if r != 0 || g != 0 || b != 0xffff || a != 0xffff {
		t.Errorf("frame 1 colour = %d,%d,%d,%d, want blue", r, g, b, a)
	}
}

func TestSaveSheetHonoursPerFrameDurationsAndTags(t *testing.T) {
	store := newProject(t)
	asset := seededAsset(t, store)
	info := sheetInfo()
	info.Durations = []int{60, 140}
	info.Tags = []string{" idle ", "walk", "idle", ""}

	saved, err := SaveSheet(store, asset.ID, encodeSheet(t, info), info)
	if err != nil {
		t.Fatalf("save sheet: %v", err)
	}
	if saved.Animation == nil {
		t.Fatal("animation metadata missing")
	}
	if want := []int{60, 140}; len(saved.Animation.FrameDurations) != 2 ||
		saved.Animation.FrameDurations[0] != want[0] || saved.Animation.FrameDurations[1] != want[1] {
		t.Errorf("frame durations = %v, want %v", saved.Animation.FrameDurations, want)
	}
	if len(saved.Animation.Tags) != 2 || saved.Animation.Tags[0] != "idle" || saved.Animation.Tags[1] != "walk" {
		t.Errorf("tags = %v, want [idle walk]", saved.Animation.Tags)
	}

	dir := project.AssetDir(store.Root(), asset.ID)
	var aseprite struct {
		Frames map[string]struct {
			Duration int `json:"duration"`
		} `json:"frames"`
		Meta struct {
			FrameTags []struct {
				Name string `json:"name"`
				From int    `json:"from"`
				To   int    `json:"to"`
			} `json:"frameTags"`
		} `json:"meta"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "versions/0001/frames-aseprite.json"))
	if err != nil {
		t.Fatalf("read aseprite json: %v", err)
	}
	if err := json.Unmarshal(raw, &aseprite); err != nil {
		t.Fatalf("parse aseprite json: %v", err)
	}
	if got := aseprite.Frames["000"].Duration; got != 60 {
		t.Errorf("frame 000 duration = %d, want 60", got)
	}
	if got := aseprite.Frames["001"].Duration; got != 140 {
		t.Errorf("frame 001 duration = %d, want 140", got)
	}
	if len(aseprite.Meta.FrameTags) != 2 ||
		aseprite.Meta.FrameTags[0].Name != "idle" || aseprite.Meta.FrameTags[1].Name != "walk" {
		t.Errorf("frame tags = %+v, want idle, walk", aseprite.Meta.FrameTags)
	}
	for _, tag := range aseprite.Meta.FrameTags {
		if tag.From != 0 || tag.To != 1 {
			t.Errorf("tag %s range = %d..%d, want 0..1", tag.Name, tag.From, tag.To)
		}
	}

	var texturePacker struct {
		Animations map[string][]string `json:"animations"`
	}
	raw, err = os.ReadFile(filepath.Join(dir, "versions/0001/frames-texturepacker.json"))
	if err != nil {
		t.Fatalf("read texturepacker json: %v", err)
	}
	if err := json.Unmarshal(raw, &texturePacker); err != nil {
		t.Fatalf("parse texturepacker json: %v", err)
	}
	if len(texturePacker.Animations["idle"]) != 2 || len(texturePacker.Animations["walk"]) != 2 {
		t.Errorf("animations = %v, want idle and walk with 2 frames each", texturePacker.Animations)
	}
}

func TestSaveSheetFallsBackToUniformDurations(t *testing.T) {
	store := newProject(t)
	asset := seededAsset(t, store)
	info := sheetInfo()
	// A zero entry falls back to 1000/fps (125ms); a positive one is clamped.
	info.Durations = []int{0, 99999}

	saved, err := SaveSheet(store, asset.ID, encodeSheet(t, info), info)
	if err != nil {
		t.Fatalf("save sheet: %v", err)
	}
	got := saved.Animation.FrameDurations
	if len(got) != 2 || got[0] != 125 || got[1] != maxFrameDurationMs {
		t.Errorf("frame durations = %v, want [125 %d]", got, maxFrameDurationMs)
	}
	if len(saved.Animation.Tags) != 0 {
		t.Errorf("tags = %v, want none", saved.Animation.Tags)
	}

	var aseprite struct {
		Frames map[string]struct {
			Duration int `json:"duration"`
		} `json:"frames"`
		Meta struct {
			FrameTags []struct {
				Name string `json:"name"`
			} `json:"frameTags"`
		} `json:"meta"`
	}
	raw, err := os.ReadFile(filepath.Join(project.AssetDir(store.Root(), asset.ID), "versions/0001/frames-aseprite.json"))
	if err != nil {
		t.Fatalf("read aseprite json: %v", err)
	}
	if err := json.Unmarshal(raw, &aseprite); err != nil {
		t.Fatalf("parse aseprite json: %v", err)
	}
	if len(aseprite.Meta.FrameTags) != 1 || aseprite.Meta.FrameTags[0].Name != "default" {
		t.Errorf("frame tags = %+v, want a single default tag", aseprite.Meta.FrameTags)
	}
}

func TestSaveSheetRejectsMismatchedLayout(t *testing.T) {
	store := newProject(t)
	asset := seededAsset(t, store)
	info := sheetInfo()

	wrong := info
	wrong.Columns = 1
	wrong.Rows = 2
	_, err := SaveSheet(store, asset.ID, encodeSheet(t, info), wrong)
	if !apperr.Is(err, "INVALID_SHEET") {
		t.Fatalf("error = %v, want INVALID_SHEET", err)
	}
}

func TestSaveSheetScalesPivotToFrameSize(t *testing.T) {
	store := newProject(t)
	asset, err := CreateBlank(store, BlankAssetInput{
		Name:   "锚点",
		Type:   project.AssetSprite,
		Width:  32,
		Height: 32,
	})
	if err != nil {
		t.Fatal(err)
	}
	info := AnimationInfo{
		FrameWidth:  8,
		FrameHeight: 8,
		FrameCount:  2,
		Columns:     2,
		Rows:        1,
		FPS:         8,
		Loop:        true,
	}
	saved, err := SaveSheet(store, asset.ID, encodeSheet(t, info), info)
	if err != nil {
		t.Fatalf("SaveSheet: %v", err)
	}
	if saved.Dimensions != (project.Dimensions{W: 8, H: 8}) {
		t.Fatalf("dimensions = %+v", saved.Dimensions)
	}
	if saved.Pivot != (project.Point{X: 4, Y: 7}) {
		t.Fatalf("pivot = %+v, want {4,7}", saved.Pivot)
	}
}

func TestSaveStillVersionClearsAnimationAndScalesPivot(t *testing.T) {
	store := newProject(t)
	asset, err := CreateBlank(store, BlankAssetInput{
		Name:   "单帧",
		Type:   project.AssetSprite,
		Width:  32,
		Height: 32,
	})
	if err != nil {
		t.Fatal(err)
	}
	sheet := AnimationInfo{
		FrameWidth:  8,
		FrameHeight: 8,
		FrameCount:  2,
		Columns:     2,
		Rows:        1,
		FPS:         8,
		Loop:        true,
	}
	if _, err := SaveSheet(store, asset.ID, encodeSheet(t, sheet), sheet); err != nil {
		t.Fatal(err)
	}
	still := AnimationInfo{FrameWidth: 16, FrameHeight: 16, FrameCount: 1, Columns: 1, Rows: 1, FPS: 8}
	saved, err := SaveVersion(store, asset.ID, encodeSheet(t, still))
	if err != nil {
		t.Fatalf("SaveVersion: %v", err)
	}
	if saved.Animation != nil {
		t.Fatalf("animation = %+v, want nil", saved.Animation)
	}
	if saved.Dimensions != (project.Dimensions{W: 16, H: 16}) {
		t.Fatalf("dimensions = %+v", saved.Dimensions)
	}
	if saved.Pivot != (project.Point{X: 8, Y: 14}) {
		t.Fatalf("pivot = %+v, want {8,14}", saved.Pivot)
	}
}

func TestSaveSheetRejectsInvalidOptionsAndMissingAsset(t *testing.T) {
	store := newProject(t)
	asset := seededAsset(t, store)

	info := sheetInfo()
	info.FrameWidth = 0
	if _, err := SaveSheet(store, asset.ID, encodeSheet(t, sheetInfo()), info); err == nil {
		t.Fatal("expected invalid frame size error")
	}
	_, err := SaveSheet(store, asset.ID, nil, sheetInfo())
	if err == nil {
		t.Fatal("expected empty payload error")
	}
	if _, err := SaveSheet(store, "missing", encodeSheet(t, sheetInfo()), sheetInfo()); err == nil {
		t.Fatal("expected missing asset error")
	}
}
