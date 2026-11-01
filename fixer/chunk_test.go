package fixer

import (
	"testing"

	"celler"
	"celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter"
	nbt "neomega-nbt/presets/v1"
)

func TestChunkDoorsAndOwnership(t *testing.T) {
	// 两半跨越负高度 section，必须先收齐整根柱子的修改才能交换互补属性。
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	lower, upper := palette(airChunk(t)), palette(airChunk(t))
	var original [2]ir.IRBlock
	for i, half := range []string{"lower", "upper"} {
		original[i] = stateBlock(t, "spruce_door", map[string]string{
			"half": half, "facing": "east", "hinge": "right", "open": "true", "powered": "true",
		})
	}
	lowerPos, upperPos := layer_storage.At(4, 15, 9), layer_storage.At(4, 0, 9)
	lowerBefore, upperBefore := roundtripDoorBlock(t, original[0]), roundtripDoorBlock(t, original[1])
	lower.Set(lowerPos, lowerBefore)
	upper.Set(upperPos, upperBefore)
	untouched := &converter.ConvertedSection{Blocks: palette(airChunk(t))}
	biomes := &[64]string{"minecraft:plains"}
	center := &converter.ConvertedChunk{
		Pos: celler.ChunkPos{X: -7, Z: 4}, Dimension: 2,
		Sections: map[int32]*converter.ConvertedSection{
			-1: {Blocks: lower, Biomes: biomes}, 0: {Blocks: upper}, 2: untouched,
		},
		BlockEntities: []java.BlockEntity{{Pos: celler.BlockPos{X: -108, Y: -1, Z: 73}, NBT: nbt.Compound{"id": "test"}}},
		Diagnostics:   []converter.Diagnostic{{Kind: converter.UnknownBlock, State: "example:missing []", Count: 2}},
	}
	fixed := f.Fix(center, ChunkNeighbours{})
	assertDoorRestored(t, fixed.Sections[-1].Blocks.Get(lowerPos), original[0], lowerBefore)
	assertDoorRestored(t, fixed.Sections[0].Blocks.Get(upperPos), original[1], upperBefore)
	assertBlock(t, lower.Get(lowerPos), lowerBefore)
	assertBlock(t, upper.Get(upperPos), upperBefore)
	if fixed == center || fixed.Sections[-1] == center.Sections[-1] || fixed.Sections[0] == center.Sections[0] || fixed.Sections[2] != untouched || fixed.Sections[-1].Biomes != biomes {
		t.Fatal("changed sections must be copied and unchanged sections shared")
	}
	if fixed.Pos != center.Pos || fixed.Dimension != center.Dimension || fixed.BlockEntities[0].Pos != center.BlockEntities[0].Pos || fixed.BlockEntities[0].NBT["id"] != "test" || fixed.Diagnostics[0] != center.Diagnostics[0] {
		t.Fatal("chunk metadata changed")
	}
}

func TestChunkNeighbourCoordinates(t *testing.T) {
	// 八个方向各放不同状态，检查统一取格函数的负坐标和邻居下标。
	positions := [8]position{
		NW: {x: -1, y: -17, z: -1}, N: {x: 3, y: -17, z: -1}, NE: {x: 16, y: -17, z: -1},
		W: {x: -1, y: -17, z: 4}, E: {x: 16, y: -17, z: 4},
		SW: {x: -1, y: -17, z: 16}, S: {x: 3, y: -17, z: 16}, SE: {x: 16, y: -17, z: 16},
	}
	var neighbours ChunkNeighbours
	names := [8]string{"stone", "dirt", "cobblestone", "gold_block", "iron_block", "diamond_block", "emerald_block", "sand"}
	for side, pos := range positions {
		blocks := palette(airChunk(t))
		blocks.Set(layer_storage.At(pos.x, pos.y, pos.z), stateBlock(t, names[side], nil))
		neighbours[side] = &converter.ConvertedChunk{Sections: map[int32]*converter.ConvertedSection{-2: {Blocks: blocks}}}
	}
	center := &converter.ConvertedChunk{Sections: map[int32]*converter.ConvertedSection{-2: {Blocks: palette(airChunk(t))}}}
	f := &Fixer{passes: []pass{func(view snapshot) map[position]ir.IRBlock {
		for side, pos := range positions {
			got, exists := view.get(pos.x, pos.y, pos.z)
			if !exists {
				t.Fatalf("missing neighbour %d", side)
			}
			assertBlock(t, got, stateBlock(t, names[side], nil))
		}
		got, exists := view.get(0, -17, 0)
		if !exists {
			t.Fatal("stored air marked missing")
		}
		assertBlock(t, got, "minecraft:air []")
		got, exists = view.get(0, -1, 0)
		if exists {
			t.Fatal("missing section marked present")
		}
		assertBlock(t, got, "minecraft:air []")
		return nil
	}}}
	fixed := f.Fix(center, neighbours)
	if fixed == center || fixed.Sections[-2] != center.Sections[-2] {
		t.Fatal("unchanged chunk must retain its section")
	}
}

func TestChunkRedstoneAcrossTwoAxes(t *testing.T) {
	// 西邻居的上一 section 可以排除坡道；缺这份数据时仍保留输入形状。
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	wire := stateBlock(t, "redstone_wire", map[string]string{
		"power": "11", "north": "up", "east": "up", "south": "up", "west": "up",
	})
	blocks := palette(airChunk(t))
	blocks.Set(layer_storage.At(0, 15, 8), wire)
	blocks.Set(layer_storage.At(1, 15, 8), wire)
	center := &converter.ConvertedChunk{Sections: map[int32]*converter.ConvertedSection{
		0: {Blocks: blocks}, 1: {Blocks: palette(airChunk(t))},
	}}
	west := &converter.ConvertedChunk{Sections: map[int32]*converter.ConvertedSection{
		0: {Blocks: palette(airChunk(t))}, 1: {Blocks: palette(airChunk(t))},
	}}
	neighbours := ChunkNeighbours{W: west}
	fixed := f.Fix(center, neighbours)
	want := stateBlock(t, "redstone_wire", map[string]string{
		"power": "11", "north": "none", "east": "side", "south": "none", "west": "side",
	})
	assertBlock(t, fixed.Sections[0].Blocks.Get(layer_storage.At(0, 15, 8)), want)
	assertBlock(t, blocks.Get(layer_storage.At(0, 15, 8)), wire)
	west.Sections[1].Blocks.Set(layer_storage.At(15, 0, 8), wire)
	fixed = f.Fix(center, neighbours)
	assertBlock(t, fixed.Sections[0].Blocks.Get(layer_storage.At(0, 15, 8)), wire)
	assertBlock(t, west.Sections[1].Blocks.Get(layer_storage.At(15, 0, 8)), wire)
	delete(west.Sections, 1)
	fixed = f.Fix(center, neighbours)
	assertBlock(t, fixed.Sections[0].Blocks.Get(layer_storage.At(0, 15, 8)), wire)
}
