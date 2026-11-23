package project

import "pixelbench/internal/apperr"

const maxMapEdge = 1024

// ExpandMap grows a map by the given number of tiles on each side. Existing
// tiles and objects are shifted so the old content keeps its position; the new
// border starts empty and is ready for painting or AI-assisted filling.
func ExpandMap(m *GameMap, top, right, bottom, left int) error {
	if m == nil {
		return apperr.New("INVALID_MAP", "地图数据为空")
	}
	edges := []int{top, right, bottom, left}
	for _, edge := range edges {
		if edge < 0 {
			return apperr.New("INVALID_MAP", "扩展量不能为负")
		}
		if edge > 256 {
			return apperr.New("INVALID_MAP", "单边最多扩展 256 格")
		}
	}
	if top+right+bottom+left == 0 {
		return apperr.New("INVALID_MAP", "请至少在一个方向扩展")
	}
	width := m.Width + left + right
	height := m.Height + top + bottom
	if width > maxMapEdge || height > maxMapEdge {
		return apperr.Newf("INVALID_MAP", "扩展后地图不能超过 %d 格", maxMapEdge)
	}
	m.Width = width
	m.Height = height
	if left == 0 && top == 0 {
		return nil
	}
	for i := range m.Layers {
		for j := range m.Layers[i].Tiles {
			m.Layers[i].Tiles[j].X += left
			m.Layers[i].Tiles[j].Y += top
		}
	}
	for i := range m.Objects {
		m.Objects[i].X += float64(left)
		m.Objects[i].Y += float64(top)
	}
	return nil
}