package converter

import (
	"maps"

	core "neomega-converter-core"
	"neomega-converter/fixpalette"
	fixer "neomega-viewer-core/fixer/java-postprocess"
)

// FixChunk 修复转换结果中的 Java 方块，保留实体、诊断、生物群系以及未改变的 section。
// 八邻居按 NW/N/NE/W/E/SW/S/SE 提供修复前的结果，缺失柱子用 nil 表示。
func FixChunk(f *fixer.Fixer, center *core.ConvertedChunk, neighbours [8]*core.ConvertedChunk) *core.ConvertedChunk {
	columns := [9]fixpalette.Column{}
	inputs := [9]*core.ConvertedChunk{8: center}
	copy(inputs[:8], neighbours[:])
	for index, chunk := range inputs {
		if chunk == nil {
			continue
		}
		columns[index] = make(fixpalette.Column, len(chunk.Sections))
		for y, section := range chunk.Sections {
			columns[index][y] = section.Blocks
		}
	}
	fixed := fixpalette.FixColumn(f, columns[8], fixpalette.ChunkNeighbours(columns[:8]))
	result := *center
	result.Sections = maps.Clone(center.Sections)
	for y, blocks := range fixed {
		if blocks != center.Sections[y].Blocks {
			section := *center.Sections[y]
			section.Blocks = blocks
			result.Sections[y] = &section
		}
	}
	return &result
}
