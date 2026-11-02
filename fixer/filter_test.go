package fixer

import (
	"testing"

	"celler"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter"
)

func TestFilteredPassesMatchFullScan(t *testing.T) {
	// 参考驱动强制每轮逐格取值再筛选，独立验证调色板索引路径与跳过 section 的行为。
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	full := *f
	full.passes = make([]pass, len(f.passes))
	changedByPass := make([]int, len(f.passes))
	for index, original := range f.passes {
		full.passes[index] = pass{
			cares: func(ir.IRBlock) bool { return true },
			run: func(view snapshot) map[position]ir.IRBlock {
				view.targets = nil
				changes := original.run(view)
				changedByPass[index] += len(changes)
				return changes
			},
		}
	}
	chunks := make(map[celler.ChunkPos]*converter.ConvertedChunk)
	for z := int32(-1); z <= 1; z++ {
		for x := int32(-1); x <= 1; x++ {
			pos := celler.ChunkPos{X: x, Z: z}
			chunks[pos] = mixedTargetChunk(t, pos)
		}
	}
	for pos, center := range chunks {
		var neighbours ChunkNeighbours
		for side, offset := range chunkOffsets {
			neighbours[side] = chunks[celler.ChunkPos{
				X: pos.X + int32(offset[0]), Z: pos.Z + int32(offset[1]),
			}]
		}
		got, want := f.Fix(center, neighbours), full.Fix(center, neighbours)
		for y, section := range got.Sections {
			for index := range 4096 {
				at := layer_storage.At(index>>8, index&15, index>>4&15)
				if actual, expected := section.Blocks.Get(at), want.Sections[y].Blocks.Get(at); actual != expected {
					t.Fatalf("chunk=%v section=%d at=%v: got %s, want %s", pos, y, at, actual, expected)
				}
			}
		}
	}
	for index, count := range changedByPass {
		if count == 0 {
			t.Fatalf("fixture did not exercise changes from pass %d", index)
		}
	}
}

func mixedTargetChunk(t *testing.T, pos celler.ChunkPos) *converter.ConvertedChunk {
	// 原木独占底层；其余目标混在空气中，并穿过 section、柱子的边界。
	t.Helper()
	chunk := &converter.ConvertedChunk{Pos: pos, Sections: make(map[int32]*converter.ConvertedSection)}
	for _, y := range []int32{-2, -1, 0, 1, 2, 4} {
		chunk.Sections[y] = &converter.ConvertedSection{Blocks: palette(airChunk(t))}
	}
	put := func(x, y, z int, name string, states map[string]string) {
		chunk.Sections[int32(y>>4)].Blocks.Set(layer_storage.At(x, y, z), stateBlock(t, name, states))
	}
	put(8, -17, 8, "oak_log", nil)
	for y := -16; y <= -12; y++ {
		put(8, y, 8, "oak_leaves", map[string]string{"persistent": "false", "distance": "7"})
	}
	put(9, -15, 8, "oak_leaves", map[string]string{"persistent": "true", "distance": "7"})
	put(4, -1, 4, "spruce_door", map[string]string{"half": "lower", "facing": "east", "open": "true"})
	put(4, 0, 4, "spruce_door", map[string]string{"half": "upper", "hinge": "right"})
	for _, p := range []position{{x: 0, y: 5, z: 8}, {x: 15, y: 5, z: 8}, {x: 8, y: 5, z: 0}, {x: 8, y: 5, z: 15}} {
		put(p.x, p.y, p.z, "oak_fence", nil)
	}
	put(15, 8, 9, "glass_pane", nil)
	put(0, 8, 9, "iron_bars", nil)
	put(6, 5, 6, "fire", nil)
	put(6, 5, 5, "stone", nil)
	put(8, 8, 0, "oak_stairs", map[string]string{"shape": "inner_right"})
	put(8, 8, 15, "oak_stairs", map[string]string{"facing": "west"})
	put(3, 15, 3, "grass_block", nil)
	put(3, 16, 3, "snow", nil)
	for x := range 2 {
		put(x, 15, 4, "redstone_wire", map[string]string{"north": "up", "east": "up"})
	}
	for _, plant := range []struct {
		x, dy int
		name  string
	}{
		{x: 2, dy: 1, name: "kelp"},
		{x: 4, dy: 1, name: "twisting_vines"},
		{x: 6, dy: -1, name: "weeping_vines"},
		{x: 8, dy: -1, name: "cave_vines"},
	} {
		put(plant.x, 15, 12, plant.name, nil)
		put(plant.x, 15+plant.dy, 12, plant.name+"_plant", nil)
	}
	put(10, -1, 10, "hay_block", nil)
	put(10, 0, 10, "campfire", nil)
	put(12, 15, 12, "note_block", nil)
	put(12, 16, 12, "zombie_head", nil)
	// 索引 0 也可能是目标，且单值调色板没有索引数组。
	chunk.Sections[2].Blocks = layer_storage.MakePaletteStorage(stateBlock(t, "kelp", nil))
	if pos.X == 0 {
		put(0, 32, 0, "stone", nil)
	}
	chunk.Sections[4].Blocks = layer_storage.MakePaletteStorage(stateBlock(t, "stone", nil))
	return chunk
}

func TestFixWithoutTargetsSharesAllPalettes(t *testing.T) {
	// 无关 section 连同带有已弃用目标条目的调色板都不应因筛选而复制。
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	center := &converter.ConvertedChunk{Sections: make(map[int32]*converter.ConvertedSection)}
	for index, name := range []string{"air", "stone", "mushroom_stem", "oak_fence_gate"} {
		center.Sections[int32(index-2)] = &converter.ConvertedSection{
			Blocks: layer_storage.MakePaletteStorage(stateBlock(t, name, nil)),
		}
	}
	blocks := center.Sections[0].Blocks
	blocks.Set(layer_storage.At(1, 2, 3), stateBlock(t, "fire", nil))
	blocks.Set(layer_storage.At(1, 2, 3), stateBlock(t, "mushroom_stem", nil))
	fixed := f.Fix(center, ChunkNeighbours{})
	for y, section := range center.Sections {
		if fixed.Sections[y].Blocks != section.Blocks {
			t.Fatalf("section %d palette copied without target blocks", y)
		}
	}
}

func TestPassTargetsRefreshBetweenPasses(t *testing.T) {
	// 后一轮必须看见新加入的状态索引；没有目标的 pass 不应被调用。
	air, stone := stateBlock(t, "air", nil), stateBlock(t, "stone", nil)
	blocks := layer_storage.MakePaletteStorage(air)
	f := &Fixer{passes: []pass{
		{
			cares: func(block ir.IRBlock) bool { return block == air },
			run: func(view snapshot) map[position]ir.IRBlock {
				return map[position]ir.IRBlock{{x: 2, y: 3, z: 4}: stone}
			},
		},
		{
			cares: func(block ir.IRBlock) bool { return block == stone },
			run: func(view snapshot) map[position]ir.IRBlock {
				count := 0
				for pos, block := range view.targetBlocks(func(ir.IRBlock) bool {
					t.Fatal("target predicate called per cell")
					return false
				}) {
					count++
					if pos != (position{x: 2, y: 3, z: 4}) || block != stone {
						t.Fatalf("unexpected target %v %s", pos, block)
					}
				}
				if count != 1 {
					t.Fatalf("got %d targets, want 1", count)
				}
				return nil
			},
		},
		{
			cares: func(ir.IRBlock) bool { return false },
			run: func(snapshot) map[position]ir.IRBlock {
				t.Fatal("pass without targets ran")
				return nil
			},
		},
	}}
	f.FixSection(blocks, [6]*layer_storage.Palette[ir.IRBlock]{})
}
