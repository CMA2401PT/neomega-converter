package export

import (
	"reflect"
	"testing"

	"celler"
	"celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	"neomega-converter"
	"neomega-converter/fixer"
	nbt "neomega-nbt/presets/v1"
)

func javaState(t *testing.T, text string) ir.IRBlock {
	t.Helper()
	state, ok := converts.JavaCanon(ir.IRBlock(text).Tidy())
	if !ok {
		t.Fatalf("unknown Java state %s", text)
	}
	return state
}

func writeConverted(t *testing.T, world *java.World, source *converter.ConvertedChunk) *java.Chunk {
	t.Helper()
	chunk, err := ToJavaChunk(source, DataVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err = world.WriteChunk(java.Overworld, chunk); err != nil {
		t.Fatal(err)
	}
	return chunk
}

func TestFixWorldConnectionsAndNBT(t *testing.T) {
	world := openJava(t)
	fence := javaState(t, "minecraft:oak_fence []")
	for x := range int32(2) {
		source := converted(celler.ChunkPos{X: x}, 0, 1)
		localX := int32(15)
		if x == 1 {
			localX = 0
		}
		source.Sections[0].Blocks.Set(layer_storage.At(localX, 5, 8), fence)
		chunk := writeConverted(t, world, source)
		section, err := chunk.Section(0)
		if err != nil {
			t.Fatal(err)
		}
		section.NBT["BlockLight"] = nbt.Array([]int8{1, 2, 3})
		section.NBT["custom"] = nbt.Compound{"keep": int64(42)}
		section.Biomes = new([64]string)
		for i := range section.Biomes {
			section.Biomes[i] = "minecraft:desert"
		}
		if err = chunk.SetSection(section); err != nil {
			t.Fatal(err)
		}
		if err = chunk.SetSection(&java.Section{Y: 2, NBT: nbt.Compound{"SkyLight": nbt.Array([]int8{7, 8})}}); err != nil {
			t.Fatal(err)
		}
		// 未使用的生物群系调色板条目也是原 NBT，写方块时不应顺便规范化它。
		values, _ := nbt.Compounds(chunk.NBT.Value["sections"])
		values[0]["biomes"] = nbt.Compound{
			"palette": []string{"minecraft:desert", "minecraft:plains"},
			"data":    nbt.Array([]int64{0}),
		}
		chunk.NBT.Value["custom_root"] = nbt.Compound{"foo": "bar"}
		if err = world.WriteChunk(java.Overworld, chunk); err != nil {
			t.Fatal(err)
		}
	}
	before, err := world.ReadChunk(java.Overworld, celler.ChunkPos{})
	if err != nil {
		t.Fatal(err)
	}
	f, err := fixer.New()
	if err != nil {
		t.Fatal(err)
	}
	report, err := FixWorld(world, f, FixOptions{})
	if err != nil || report.Chunks != 2 || report.ChangedSections != 2 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	for x := range int32(2) {
		chunk, err := world.ReadChunk(java.Overworld, celler.ChunkPos{X: x})
		if err != nil {
			t.Fatal(err)
		}
		section, err := chunk.Section(0)
		if err != nil {
			t.Fatal(err)
		}
		localX, side := 15, "east"
		if x == 1 {
			localX, side = 0, "west"
		}
		if !section.Blocks.Get(layer_storage.At(localX, 5, 8)).States().CheckKey(side, "true") {
			t.Fatal("boundary fence disconnected")
		}
	}
	after, err := world.ReadChunk(java.Overworld, celler.ChunkPos{})
	if err != nil {
		t.Fatal(err)
	}
	// 移除唯一允许改变的 block_states，再比较完整文档以覆盖光照、生物群系和未知字段。
	beforeSections, _ := nbt.Compounds(before.NBT.Value["sections"])
	afterSections, _ := nbt.Compounds(after.NBT.Value["sections"])
	delete(beforeSections[0], "block_states")
	delete(afterSections[0], "block_states")
	if !reflect.DeepEqual(before.NBT, after.NBT) {
		t.Fatalf("unrelated NBT changed:\nbefore=%v\nafter=%v", before.NBT, after.NBT)
	}
}

func TestFixWorldUsesOriginalNeighbours(t *testing.T) {
	// 两柱互为邻居；两侧楼梯修复后都会失去朝外实心面，栅栏必须仍按原值连接。
	for _, test := range []struct {
		name                    string
		delta                   celler.ChunkPos
		firstEdge, secondEdge   layer_storage.Position
		firstFacing, lastFacing string
		forward, backward       int
	}{
		{
			name: "same row", delta: celler.ChunkPos{X: 1},
			firstEdge: layer_storage.At(15, 5, 8), secondEdge: layer_storage.At(0, 5, 8),
			firstFacing: "north", lastFacing: "south", forward: fixer.E, backward: fixer.W,
		},
		{
			name: "cross row", delta: celler.ChunkPos{Z: 1},
			firstEdge: layer_storage.At(8, 5, 15), secondEdge: layer_storage.At(8, 5, 0),
			firstFacing: "east", lastFacing: "west", forward: fixer.S, backward: fixer.N,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			world := openJava(t)
			first, second := converted(celler.ChunkPos{}, 0), converted(test.delta, 0)
			stair := func(facing string) ir.IRBlock {
				return javaState(t, `minecraft:oak_stairs ["facing"="`+facing+`","half"="bottom","shape"="inner_right"]`)
			}
			first.Sections[0].Blocks.Set(test.firstEdge, stair(test.firstFacing))
			second.Sections[0].Blocks.Set(test.secondEdge, javaState(t, "minecraft:oak_fence []"))
			firstFence, secondStair := test.firstEdge, test.secondEdge
			firstFence.Y, secondStair.Y = 7, 7
			first.Sections[0].Blocks.Set(firstFence, javaState(t, "minecraft:oak_fence []"))
			second.Sections[0].Blocks.Set(secondStair, stair(test.lastFacing))
			writeConverted(t, world, first)
			writeConverted(t, world, second)
			f, err := fixer.New()
			if err != nil {
				t.Fatal(err)
			}
			var firstNeighbours, secondNeighbours fixer.ChunkNeighbours
			firstNeighbours[test.forward] = second
			secondNeighbours[test.backward] = first
			fixedFirst := f.Fix(first, firstNeighbours)
			fixedSecond := f.Fix(second, secondNeighbours)
			for _, pair := range []struct {
				center, fixed, other *converter.ConvertedChunk
				side                 int
				at                   layer_storage.Position
			}{
				{center: first, fixed: fixedFirst, other: fixedSecond, side: test.forward, at: firstFence},
				{center: second, fixed: fixedSecond, other: fixedFirst, side: test.backward, at: test.secondEdge},
			} {
				var neighbours fixer.ChunkNeighbours
				neighbours[pair.side] = pair.other
				sequential := f.Fix(pair.center, neighbours)
				if pair.fixed.Sections[0].Blocks.Get(pair.at) == sequential.Sections[0].Blocks.Get(pair.at) {
					t.Fatal("fixture does not distinguish original and fixed neighbours")
				}
			}
			report, err := FixWorld(world, f, FixOptions{})
			if err != nil || report.ChangedSections != 2 || report.Chunks != 2 {
				t.Fatalf("report=%+v err=%v", report, err)
			}
			for _, want := range []*converter.ConvertedChunk{fixedFirst, fixedSecond} {
				chunk, err := world.ReadChunk(java.Overworld, want.Pos)
				if err != nil {
					t.Fatal(err)
				}
				section, err := chunk.Section(0)
				if err != nil {
					t.Fatal(err)
				}
				for index := range 4096 {
					at := layer_storage.At(index>>8, index&15, index>>4&15)
					if got, expected := section.Blocks.Get(at), want.Sections[0].Blocks.Get(at); got != expected {
						t.Fatalf("chunk=%v at=%v: got=%s want=%s", want.Pos, at, got, expected)
					}
				}
			}
		})
	}
}

func TestFixWorldKeepsWrittenChangesOnLaterError(t *testing.T) {
	// 流式写回不回滚已完成的柱子，后续维度读取失败也应保留已写计数。
	world := openJava(t)
	source := converted(celler.ChunkPos{}, 0)
	source.Sections[0].Blocks.Set(
		layer_storage.At(8, 5, 8),
		javaState(t, `minecraft:oak_stairs ["shape"="inner_right"]`),
	)
	writeConverted(t, world, source)
	before, err := world.ReadChunk(java.Overworld, source.Pos)
	if err != nil {
		t.Fatal(err)
	}
	bad := java.NewChunk(celler.ChunkPos{}, DataVersion)
	bad.NBT.Value["sections"] = []nbt.Compound{{"Y": int8(0), "block_states": "broken"}}
	if err = world.WriteChunk(java.Nether, bad); err != nil {
		t.Fatal(err)
	}
	f, err := fixer.New()
	if err != nil {
		t.Fatal(err)
	}
	report, err := FixWorld(world, f, FixOptions{Dimensions: []java.Dimension{java.Overworld, java.Nether}})
	if err == nil || report.ChangedSections != 1 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	after, err := world.ReadChunk(java.Overworld, source.Pos)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(before.NBT, after.NBT) {
		t.Fatal("completed chunk was not written before later error")
	}
}

func TestFixWorldRowsMatchWholeDimension(t *testing.T) {
	// 跨 region 的多行网格加空行、空洞，覆盖连续滑动与跳行时的加载和淘汰。
	world := openJava(t)
	fence := javaState(t, "minecraft:oak_fence []")
	for _, z := range []int32{-2, -1, 0, 1, 2, 4, 7, 8} {
		for x := int32(30); x <= 33; x++ {
			if (z == 1 && x == 33) || (z == 4 && x != 31) {
				continue
			}
			source := converted(celler.ChunkPos{X: x, Z: z}, -1, 0, 2)
			for _, at := range []layer_storage.Position{
				layer_storage.At(8, 5, 0), layer_storage.At(8, 5, 15),
				layer_storage.At(0, 5, 8), layer_storage.At(15, 5, 8),
			} {
				source.Sections[0].Blocks.Set(at, fence)
			}
			source.Sections[-1].Blocks.Set(
				layer_storage.At(4, 15, 4),
				javaState(t, `minecraft:oak_door ["half"="lower","facing"="east","open"="true"]`),
			)
			source.Sections[0].Blocks.Set(
				layer_storage.At(4, 0, 4),
				javaState(t, `minecraft:oak_door ["half"="upper","hinge"="right"]`),
			)
			writeConverted(t, world, source)
		}
	}
	lightOnly := java.NewChunk(celler.ChunkPos{X: 35, Z: 4}, DataVersion)
	if err := lightOnly.SetSection(&java.Section{Y: 0, NBT: nbt.Compound{"SkyLight": nbt.Array([]int8{7})}}); err != nil {
		t.Fatal(err)
	}
	if err := world.WriteChunk(java.Overworld, lightOnly); err != nil {
		t.Fatal(err)
	}

	// 参考实现先从磁盘读完整个维度，所有 Fix 完成后才允许写回。
	originals := make(map[celler.ChunkPos]*converter.ConvertedChunk)
	err := world.Scan(java.Overworld, java.Terrain, func(pos celler.ChunkPos) error {
		chunk, err := world.ReadChunk(java.Overworld, pos)
		if err != nil {
			return err
		}
		ys, err := chunk.SectionYs()
		if err != nil {
			return err
		}
		original := converted(pos)
		for _, y := range ys {
			section, err := chunk.Section(y)
			if err != nil {
				return err
			}
			if section.Blocks != nil {
				original.Sections[y] = &converter.ConvertedSection{Blocks: section.Blocks}
			}
		}
		originals[pos] = original
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := fixer.New()
	if err != nil {
		t.Fatal(err)
	}
	want := make(map[celler.ChunkPos]*converter.ConvertedChunk)
	expectedReport := FixReport{Chunks: uint64(len(originals))}
	for pos, original := range originals {
		var neighbours fixer.ChunkNeighbours
		for side, offset := range [8][2]int32{
			{-1, -1}, {0, -1}, {1, -1}, {-1, 0},
			{1, 0}, {-1, 1}, {0, 1}, {1, 1},
		} {
			neighbours[side] = originals[celler.ChunkPos{X: pos.X + offset[0], Z: pos.Z + offset[1]}]
		}
		fixed := f.Fix(original, neighbours)
		want[pos] = fixed
		for y, section := range fixed.Sections {
			if section.Blocks != original.Sections[y].Blocks {
				expectedReport.ChangedSections++
			}
		}
	}
	report, err := FixWorld(world, f, FixOptions{Dimensions: []java.Dimension{java.Overworld, java.Overworld}})
	if err != nil || report != expectedReport {
		t.Fatalf("report=%+v want=%+v err=%v", report, expectedReport, err)
	}
	for pos, expected := range want {
		chunk, err := world.ReadChunk(java.Overworld, pos)
		if err != nil {
			t.Fatal(err)
		}
		for y, expectedSection := range expected.Sections {
			section, err := chunk.Section(y)
			if err != nil {
				t.Fatal(err)
			}
			for index := range 4096 {
				at := layer_storage.At(index>>8, index&15, index>>4&15)
				if got, want := section.Blocks.Get(at), expectedSection.Blocks.Get(at); got != want {
					t.Fatalf("chunk=%v section=%d at=%v: got=%s want=%s", pos, y, at, got, want)
				}
			}
		}
	}
}
