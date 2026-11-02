package fixer

import (
	"celler/layer_storage"
	"neomega-blocks/ir"
)

type sectionNeighbours [6][]ir.IRBlock

func palette(blocks []ir.IRBlock) *layer_storage.Palette[ir.IRBlock] {
	// 用调色板承接既有用例的逐格布局，保留每个用例的独立期望值。
	result := &layer_storage.Palette[ir.IRBlock]{}
	result.CopyFrom(&layer_storage.Plain[ir.IRBlock]{DefaultValue: "minecraft:air []", Data: blocks})
	return result
}

func fixSection(f *Fixer, blocks []ir.IRBlock, neighbours sectionNeighbours) []ir.IRBlock {
	var adjacent [6]*layer_storage.Palette[ir.IRBlock]
	for side, values := range neighbours {
		if values != nil {
			adjacent[side] = palette(values)
		}
	}
	fixed := f.FixSection(palette(blocks), adjacent)
	var result layer_storage.Plain[ir.IRBlock]
	result.CopyFrom(fixed)
	return result.Data
}

func runPass(
	cares func(ir.IRBlock) bool,
	run func(snapshot) map[position]ir.IRBlock,
	blocks []ir.IRBlock,
	neighbours sectionNeighbours,
) map[int]ir.IRBlock {
	// 直接调用规则也提供目标索引，遵守与正式驱动相同的输入约定。
	section := palette(blocks)
	grid := map[sectionKey]*layer_storage.Palette[ir.IRBlock]{{}: section}
	for side, values := range neighbours {
		if values != nil {
			grid[sectionKey(sideOffset(side))] = palette(values)
		}
	}
	sections := map[int32]*layer_storage.Palette[ir.IRBlock]{0: section}
	changes := run(snapshot{
		sections: sections,
		targets:  selectTargets(sections, cares),
		get:      getter(grid),
		solid:    make(map[ir.IRBlock]uint8),
	})
	result := make(map[int]ir.IRBlock, len(changes))
	for pos, state := range changes {
		result[cell(pos.x, pos.y, pos.z)] = state
	}
	return result
}
