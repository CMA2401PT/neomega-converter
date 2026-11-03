// Package fixpalette 将固定 16³ 的世界存储接入独立的方块属性修复规则。
package fixpalette

import (
	"iter"
	"maps"

	"celler/layer_storage"
	"neomega-blocks/ir"
	fixer "neomega-viewer-core/fixer/java-postprocess"
)

type Column map[int32]*layer_storage.Palette[ir.IRBlock]
type ChunkNeighbours [8]Column

// 八邻居依次为西北、北、东北、西、东、西南、南、东南。
const (
	NW = iota
	N
	NE
	W
	E
	SW
	S
	SE
)

var offsets = [8][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}

// FixColumn 修复整列，只复制发生变化的 Palette；邻居必须来自修复前的快照。
func FixColumn(f *fixer.Fixer, center Column, neighbours ChunkNeighbours) Column {
	grid := make(map[[3]int]*layer_storage.Palette[ir.IRBlock])
	for y, blocks := range center {
		grid[[3]int{0, int(y), 0}] = blocks
	}
	for side, column := range neighbours {
		for y, blocks := range column {
			grid[[3]int{offsets[side][0], int(y), offsets[side][1]}] = blocks
		}
	}
	return fix(f, center, grid)
}

// FixSection 用六面邻居修复预览子区块，方向为西、东、下、上、北、南。
func FixSection(f *fixer.Fixer, blocks *layer_storage.Palette[ir.IRBlock], neighbours [6]*layer_storage.Palette[ir.IRBlock]) *layer_storage.Palette[ir.IRBlock] {
	grid := map[[3]int]*layer_storage.Palette[ir.IRBlock]{{}: blocks}
	for side, adjacent := range neighbours {
		if adjacent != nil {
			pos := [3]int{}
			pos[side/2] = 2*(side%2) - 1
			grid[pos] = adjacent
		}
	}
	return fix(f, Column{0: blocks}, grid)[0]
}

func fix(f *fixer.Fixer, center Column, grid map[[3]int]*layer_storage.Palette[ir.IRBlock]) Column {
	changes := f.Fix(fixer.Input{
		Bounds: fixer.HorizontalBounds{Max: [2]int{16, 16}},
		Blocks: func(accept func(ir.IRBlock) bool) iter.Seq2[[3]int, ir.IRBlock] {
			return func(yield func([3]int, ir.IRBlock) bool) {
				for y, blocks := range center {
					palette := blocks.Palette[:blocks.LastPaletteIdx+1]
					selected := make([]bool, len(palette))
					active := false
					for i, state := range palette {
						selected[i] = accept(state)
						active = active || selected[i]
					}
					if !active {
						continue
					}
					for i := range 4096 {
						local := layer_storage.At(i>>8, i&15, i>>4&15)
						index := int(blocks.GetIndex(local))
						if selected[index] && !yield([3]int{int(local.X), int(y)*16 + int(local.Y), int(local.Z)}, palette[index]) {
							return
						}
					}
				}
			}
		},
		Get: func(x, y, z int) (ir.IRBlock, bool) {
			blocks := grid[[3]int{x >> 4, y >> 4, z >> 4}]
			if blocks == nil {
				return "minecraft:air []", false
			}
			return blocks.Get(layer_storage.At(x, y, z)), true
		},
	})
	result := maps.Clone(center)
	for pos, state := range changes {
		y := int32(pos[1] >> 4)
		if result[y] == center[y] {
			copy := &layer_storage.Palette[ir.IRBlock]{}
			copy.CopyFrom(center[y])
			result[y] = copy
		}
		result[y].Set(layer_storage.At(pos[0], pos[1], pos[2]), state)
	}
	return result
}
