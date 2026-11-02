package fixer

import (
	"iter"
	"maps"

	"celler/layer_storage"
	"neomega-blocks/geometry/solidtable"
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	java "neomega-blocks/presets/java/v1"
	javasolid "neomega-blocks/presets/java/v1/solidtable"
	"neomega-converter"
)

type ChunkNeighbours [8]*converter.ConvertedChunk

// 邻居保持转换后的原始状态，按 NW/N/NE/W/E/SW/S/SE 提供整根柱子。

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

var chunkOffsets = [8][2]int{
	NW: {-1, -1}, N: {0, -1}, NE: {1, -1}, W: {-1, 0},
	E: {1, 0}, SW: {-1, 1}, S: {0, 1}, SE: {1, 1},
}

type position struct{ x, y, z int }

// 规则改动按方块定位，x、z 相对中心区块原点，y 为世界高度。

type sectionKey struct{ x, y, z int }

// 网格按 section 寻址，独立的键类型避免将 section 坐标当作方块坐标。

type blockGetter func(x, y, z int) (ir.IRBlock, bool)

// 缺数据时返回空气及 false，让红石规则保留无法排除坡道的状态。

type snapshot struct {
	// 中心随 pass 更新，邻居保留输入原值；目标索引仅属于当前 pass。
	sections map[int32]*layer_storage.Palette[ir.IRBlock]
	get      blockGetter
	solid    map[ir.IRBlock]uint8
	targets  map[int32][]bool
}

type pass struct {
	// 筛选由驱动统一完成，规则只接收当前轮目标，先收齐整根柱子的改动。
	cares func(ir.IRBlock) bool
	run   func(snapshot) map[position]ir.IRBlock
}

type Fixer struct {
	// 规则和实心表只读共享，调用中的状态与缓存由各次修复独立持有。
	solid  solidtable.Table
	passes []pass
}

func New() (*Fixer, error) {
	// 独立加载面位数据即可完成邻接判断，无需几何运行时。
	solid, err := javasolid.Bundled()
	if err != nil {
		return nil, err
	}
	f := &Fixer{solid: solid}
	f.passes = []pass{
		{cares: connectionTarget, run: f.connections},
		{cares: fireTarget, run: f.fireConnections},
		{cares: stairTarget, run: f.stairShapes},
		{cares: doorTarget, run: f.doorHalves},
		{cares: snowyTarget, run: f.snowyGround},
		{cares: redstoneTarget, run: f.redstoneConnections},
		{cares: leafTarget, run: f.leafDistances},
		{cares: plantTarget, run: f.plantHeads},
		{cares: campfireTarget, run: f.campfireSignal},
		{cares: noteBlockTarget, run: f.noteBlockInstrument},
	}
	return f, nil
}

func (f *Fixer) Fix(center *converter.ConvertedChunk, neighbours ChunkNeighbours) *converter.ConvertedChunk {
	// 只替换中心有改动的 section，元数据和未变调色板继续共享输入。
	sections := make(map[int32]*layer_storage.Palette[ir.IRBlock], len(center.Sections))
	grid := make(map[sectionKey]*layer_storage.Palette[ir.IRBlock])
	for y, section := range center.Sections {
		blocks := completePalette(section.Blocks)
		sections[y] = blocks
		grid[sectionKey{y: int(y)}] = blocks
	}
	for side, chunk := range neighbours {
		if chunk == nil {
			continue
		}
		offset := chunkOffsets[side]
		for y, section := range chunk.Sections {
			grid[sectionKey{x: offset[0], y: int(y), z: offset[1]}] = completePalette(section.Blocks)
		}
	}
	f.run(sections, grid)
	result := *center
	result.Sections = maps.Clone(center.Sections)
	for y, blocks := range sections {
		if blocks != center.Sections[y].Blocks {
			section := *center.Sections[y]
			section.Blocks = blocks
			result.Sections[y] = &section
		}
	}
	return &result
}

func (f *Fixer) FixSection(
	section *layer_storage.Palette[ir.IRBlock],
	neighbours [6]*layer_storage.Palette[ir.IRBlock],
) *layer_storage.Palette[ir.IRBlock] {
	// 单 section 预览使用局部高度，六面邻居按 west/east/down/up/north/south 排列。
	section = completePalette(section)
	sections := map[int32]*layer_storage.Palette[ir.IRBlock]{0: section}
	grid := map[sectionKey]*layer_storage.Palette[ir.IRBlock]{{}: section}
	for side, blocks := range neighbours {
		if blocks != nil {
			grid[sectionKey(sideOffset(side))] = completePalette(blocks)
		}
	}
	f.run(sections, grid)
	return sections[0]
}

func completePalette(blocks *layer_storage.Palette[ir.IRBlock]) *layer_storage.Palette[ir.IRBlock] {
	// 仅在补全确实改变状态时复制，完整调色板可继续与调用方共享。
	for _, block := range blocks.Palette[:blocks.LastPaletteIdx+1] {
		if completeBlock(block) != block {
			return layer_storage.ConvertPalette(blocks, completeBlock)
		}
	}
	return blocks
}

func completeBlock(block ir.IRBlock) ir.IRBlock {
	// 默认属性表不保留通用含水标记，补全时需单独传递原值。
	if _, ok := java.HashToBlock(java.BlockToHash(block)); ok {
		return block
	}
	states := block.States()
	waterlogged, wet := states.Get("waterlogged")
	source := ir.IRBlock(block.NameWithNameSpace() + " " + string(states.Delete("waterlogged")))
	completed, ok := converts.JavaCanon(source)
	if !ok {
		return block
	}
	if wet {
		completed = ir.IRBlock(completed.NameWithNameSpace() + " " + string(completed.States().Set("waterlogged", waterlogged)))
	}
	return completed.Tidy()
}

func (f *Fixer) run(
	sections map[int32]*layer_storage.Palette[ir.IRBlock],
	grid map[sectionKey]*layer_storage.Palette[ir.IRBlock],
) {
	// 每轮按最新中心调色板重新选目标；首次写入才复制，收齐改动后统一应用。
	view := snapshot{sections: sections, get: getter(grid), solid: make(map[ir.IRBlock]uint8)}
	owned := make(map[int32]bool)
	for _, pass := range f.passes {
		view.targets = selectTargets(sections, pass.cares)
		if len(view.targets) == 0 {
			continue
		}
		changes := pass.run(view)
		for pos, state := range changes {
			y := int32(pos.y >> 4)
			if !owned[y] {
				copy := &layer_storage.Palette[ir.IRBlock]{}
				copy.CopyFrom(sections[y])
				sections[y] = copy
				grid[sectionKey{y: int(y)}] = copy
				owned[y] = true
			}
			sections[y].Set(layer_storage.At(pos.x, pos.y, pos.z), state)
		}
	}
}

func selectTargets(
	sections map[int32]*layer_storage.Palette[ir.IRBlock],
	cares func(ir.IRBlock) bool,
) map[int32][]bool {
	// 同一完整状态跨 section 复用判断；每轮重建索引，避免沿用已改名或扩容的调色板。
	targets := make(map[int32][]bool)
	cache := make(map[ir.IRBlock]bool)
	for y, section := range sections {
		indices := make([]bool, section.LastPaletteIdx+1)
		active := false
		for index, block := range section.Palette[:section.LastPaletteIdx+1] {
			matches, known := cache[block]
			if !known {
				matches = cares(block)
				cache[block] = matches
			}
			indices[index] = matches
			active = active || matches
		}
		if active {
			targets[y] = indices
		}
	}
	return targets
}

func getter(grid map[sectionKey]*layer_storage.Palette[ir.IRBlock]) blockGetter {
	// 算术右移与低四位同时支持负高度和跨区块坐标，两个入口共用这条寻址路径。
	return func(x, y, z int) (ir.IRBlock, bool) {
		section := grid[sectionKey{x: x >> 4, y: y >> 4, z: z >> 4}]
		if section == nil {
			return "minecraft:air []", false
		}
		return section.Get(layer_storage.At(x, y, z)), true
	}
}

func (view snapshot) targetBlocks() iter.Seq2[position, ir.IRBlock] {
	// 只读取驱动为当前轮选出的目标，避免规则重复指定筛选条件。
	return func(yield func(position, ir.IRBlock) bool) {
		for y, indices := range view.targets {
			section := view.sections[y]
			for index := range 4096 {
				local := layer_storage.At(index>>8, index&15, index>>4&15)
				paletteIndex := section.GetIndex(local)
				if !indices[paletteIndex] {
					continue
				}
				pos := position{x: int(local.X), y: int(y)*16 + int(local.Y), z: int(local.Z)}
				if !yield(pos, section.Palette[paletteIndex]) {
					return
				}
			}
		}
	}
}

func (f *Fixer) faces(block ir.IRBlock, cache map[ir.IRBlock]uint8) uint8 {
	// 面位按完整 Java 状态缓存，避免每个相邻格都重复计算 hash。
	if faces, exists := cache[block]; exists {
		return faces
	}
	faces := f.solid[java.BlockToHash(block)]
	cache[block] = faces
	return faces
}

func sideOffset(side int) position {
	// 方向编号与实心表的成对面位一致，异或 1 即反向。
	offset := [3]int{}
	offset[side/2] = 2*(side%2) - 1
	return position{x: offset[0], y: offset[1], z: offset[2]}
}

func neighbour(get blockGetter, pos position, side int) (ir.IRBlock, bool) {
	// 单面规则只需提供方向，跨 section 与跨区块寻址统一交给取格函数。
	delta := sideOffset(side)
	return get(pos.x+delta.x, pos.y+delta.y, pos.z+delta.z)
}
