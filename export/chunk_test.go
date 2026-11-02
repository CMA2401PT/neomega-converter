package export

import (
	"reflect"
	"testing"

	"celler"
	"celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter"
	nbt "neomega-nbt/presets/v1"
)

func openJava(t *testing.T) *java.World {
	t.Helper()
	world, err := java.OpenDirectory(t.TempDir(), java.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := world.Close(); err != nil {
			t.Error(err)
		}
	})
	return world
}

func converted(pos celler.ChunkPos, ys ...int32) *converter.ConvertedChunk {
	result := &converter.ConvertedChunk{Pos: pos, Sections: make(map[int32]*converter.ConvertedSection)}
	for _, y := range ys {
		result.Sections[y] = &converter.ConvertedSection{Blocks: layer_storage.MakePaletteStorage(ir.IRBlock("minecraft:air []"))}
	}
	return result
}

func TestToJavaChunkRoundTrip(t *testing.T) {
	source := converted(celler.ChunkPos{X: -2, Z: 3}, -5, -2, 0, 20)
	source.Sections[-2].Blocks.Set(layer_storage.At(3, 4, 5), ir.IRBlock(`minecraft:oak_fence ["waterlogged"="true"]`).Tidy())
	source.Sections[-2].Blocks.Set(layer_storage.At(1, 2, 3), "minecraft:torch []")
	source.Sections[-2].Blocks.Set(layer_storage.At(4, 5, 6), "minecraft:oak_wall_sign []")
	source.Sections[0].Blocks.Set(layer_storage.At(15, 15, 15), "minecraft:glass_pane []")
	biomes := new([64]string)
	for i := range biomes {
		biomes[i] = "minecraft:plains"
	}
	source.Sections[0].Biomes = biomes
	pos := celler.BlockPos{X: -31, Y: -31, Z: 49}
	source.BlockEntities = []java.BlockEntity{
		{Pos: pos, NBT: nbt.Compound{"id": "minecraft:chest", "x": pos.X, "y": pos.Y, "z": pos.Z, "Items": []nbt.Compound{}}},
		{Pos: celler.BlockPos{Y: 320}, NBT: nbt.Compound{}},
	}
	chunk, err := ToJavaChunk(source, DataVersion)
	if err != nil {
		t.Fatal(err)
	}
	world := openJava(t)
	if err = world.WriteChunk(java.Overworld, chunk); err != nil {
		t.Fatal(err)
	}
	got, err := world.ReadChunk(java.Overworld, source.Pos)
	if err != nil {
		t.Fatal(err)
	}
	if got.NBT.Value["yPos"] != int32(-4) || got.NBT.Value["Status"] != "minecraft:full" || got.NBT.Value["isLightOn"] != int8(0) {
		t.Fatalf("loading fields: %v", got.NBT.Value)
	}
	ys, err := got.SectionYs()
	if err != nil || !reflect.DeepEqual(ys, []int32{-2, 0}) {
		t.Fatalf("ys=%v: %v", ys, err)
	}
	section, err := got.Section(-2)
	if err != nil {
		t.Fatal(err)
	}
	if section.Blocks.Get(layer_storage.At(3, 4, 5)) != source.Sections[-2].Blocks.Get(layer_storage.At(3, 4, 5)) {
		t.Fatal("waterlogged changed")
	}
	if _, exists := section.NBT["biomes"]; exists {
		t.Fatal("nil biomes written")
	}
	section, err = got.Section(0)
	if err != nil || !reflect.DeepEqual(section.Biomes, biomes) {
		t.Fatalf("biomes=%v: %v", section, err)
	}
	entities, issues, err := got.BlockEntities()
	if err != nil || len(issues) != 0 || len(entities) != 1 || !reflect.DeepEqual(entities[0].NBT, source.BlockEntities[0].NBT) {
		t.Fatalf("entities=%v issues=%v err=%v", entities, issues, err)
	}
	want := make([]any, 24)
	for i := range want {
		want[i] = []int16{}
	}
	want[2] = []int16{3 | 4<<4 | 5<<8}
	want[4] = []int16{4095}
	if !reflect.DeepEqual(got.NBT.Value["PostProcessing"], want) {
		t.Fatalf("PostProcessing=%#v", got.NBT.Value["PostProcessing"])
	}
}

func TestToJavaChunkMissingBottomSections(t *testing.T) {
	// 底部缺失时仍按主世界的 -4 起算，section 2 必须登记到下标 6。
	source := converted(celler.ChunkPos{}, 2)
	source.Sections[2].Blocks.Set(layer_storage.At(3, 4, 5), "minecraft:oak_fence []")
	chunk, err := ToJavaChunk(source, DataVersion)
	if err != nil {
		t.Fatal(err)
	}
	world := openJava(t)
	if err = world.WriteChunk(java.Overworld, chunk); err != nil {
		t.Fatal(err)
	}
	got, err := world.ReadChunk(java.Overworld, source.Pos)
	if err != nil {
		t.Fatal(err)
	}
	if got.NBT.Value["yPos"] != int32(-4) {
		t.Fatalf("yPos=%v, want -4", got.NBT.Value["yPos"])
	}
	post := got.NBT.Value["PostProcessing"].([]any)
	if len(post) != 24 {
		t.Fatalf("PostProcessing length=%d, want 24", len(post))
	}
	for index, entries := range post {
		want := []int16{}
		if index == 6 {
			want = []int16{3 | 4<<4 | 5<<8}
		}
		if !reflect.DeepEqual(entries, want) {
			t.Fatalf("PostProcessing[%d]=%v, want %v", index, entries, want)
		}
	}
}

func TestShapeUpdateList(t *testing.T) {
	for _, name := range []string{"oak_fence", "nether_brick_fence", "oak_fence_gate", "glass_pane", "red_stained_glass_pane", "iron_bars", "oak_stairs", "cobblestone_wall", "redstone_wire", "tripwire", "chorus_plant"} {
		if !needsShapeUpdate("minecraft:" + name) {
			t.Errorf("missing %s", name)
		}
	}
	for _, name := range []string{"stone", "torch", "oak_wall_sign", "oak_wall_hanging_sign", "chorus_flower", "oak_leaves", "tripwire_hook", "rail"} {
		if needsShapeUpdate("minecraft:" + name) {
			t.Errorf("unsafe %s", name)
		}
	}
}

func TestDimensionHeight(t *testing.T) {
	for _, dim := range []int32{0, 1, 2} {
		source := converted(celler.ChunkPos{}, -4, 0, 15, 19)
		source.Dimension = dim
		chunk, err := ToJavaChunk(source, DataVersion)
		if err != nil {
			t.Fatal(err)
		}
		ys, err := chunk.SectionYs()
		if err != nil {
			t.Fatal(err)
		}
		want, bottom := []int32{0, 15}, int32(0)
		if dim == 0 {
			want, bottom = []int32{-4, 0, 15, 19}, -4
		}
		if !reflect.DeepEqual(ys, want) || chunk.NBT.Value["yPos"] != bottom {
			t.Fatalf("dim=%d ys=%v yPos=%v", dim, ys, chunk.NBT.Value["yPos"])
		}
	}
	for dim, want := range map[int32]java.Dimension{0: java.Overworld, 1: java.Nether, 2: java.End} {
		if got, ok := JavaDimension(dim); !ok || got != want {
			t.Fatalf("dim %d: %s %v", dim, got, ok)
		}
	}
	if _, ok := JavaDimension(3); ok {
		t.Fatal("unknown dimension mapped")
	}
}
