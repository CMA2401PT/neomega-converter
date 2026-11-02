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
	// 左柱的内拐楼梯修复成直楼梯后会失去东实心面；右柱栅栏必须仍按原始内拐楼梯连接。
	world := openJava(t)
	left, right := converted(celler.ChunkPos{}, 0), converted(celler.ChunkPos{X: 1}, 0)
	left.Sections[0].Blocks.Set(
		layer_storage.At(15, 5, 8),
		javaState(t, `minecraft:oak_stairs ["facing"="north","half"="bottom","shape"="inner_right"]`),
	)
	right.Sections[0].Blocks.Set(layer_storage.At(0, 5, 8), javaState(t, "minecraft:oak_fence []"))
	writeConverted(t, world, left)
	writeConverted(t, world, right)
	f, err := fixer.New()
	if err != nil {
		t.Fatal(err)
	}
	fixedLeft := f.Fix(left, fixer.ChunkNeighbours{fixer.E: right})
	originalResult := f.Fix(right, fixer.ChunkNeighbours{fixer.W: left})
	sequentialResult := f.Fix(right, fixer.ChunkNeighbours{fixer.W: fixedLeft})
	at := layer_storage.At(0, 5, 8)
	want := originalResult.Sections[0].Blocks.Get(at)
	if want == sequentialResult.Sections[0].Blocks.Get(at) {
		t.Fatal("fixture does not distinguish neighbour order")
	}
	report, err := FixWorld(world, f, FixOptions{})
	if err != nil || report.ChangedSections != 2 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	got, err := world.ReadChunk(java.Overworld, right.Pos)
	if err != nil {
		t.Fatal(err)
	}
	section, err := got.Section(0)
	if err != nil {
		t.Fatal(err)
	}
	if section.Blocks.Get(at) != want {
		t.Fatalf("got=%s want=%s", section.Blocks.Get(at), want)
	}
}

func TestFixWorldWaitsForAllDimensions(t *testing.T) {
	// 第二个维度读取失败时，第一个维度已算出的结果也不能提前写回。
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
	if err == nil || report.ChangedSections != 0 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	after, err := world.ReadChunk(java.Overworld, source.Pos)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.NBT, after.NBT) {
		t.Fatal("wrote changes before all fixes completed")
	}
}
