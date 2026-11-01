package bedrock

import (
	"reflect"
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter"
	"neomega-converter/bedrock/blockentities"
	"neomega-converter/bedrock/internal/blockstate"

	nbt "neomega-nbt/presets/v1"
)

func TestConvertState(t *testing.T) {
	// 公开入口保留内部入口的成功、失败与旧状态对齐语义。
	for _, source := range []ir.IRBlock{
		`minecraft:stone []`,
		`minecraft:wool ["color"="red"]`,
		`minecraft:oak_stairs ["weirdo_direction"=99]`,
		`example:missing []`,
	} {
		got, ok := ConvertState(source)
		want, wantOK := blockstate.ToJava(source)
		if got != want || ok != wantOK {
			t.Fatalf("%s: got %s/%v, want %s/%v", source, got, ok, want, wantOK)
		}
	}
}

func TestConvertBlockEntity(t *testing.T) {
	// 查无规则与有规则但执行失败必须能分辨，成功时返回规则的原始输出。
	input := blockentities.Input{
		Base:   `minecraft:red_bed ["facing"="north","occupied"="false","part"="foot"]`,
		Source: nbt.Compound{"color": int8(11)},
	}
	output, ok, err := ConvertBlockEntity("Bed", input)
	if !ok || err != nil {
		t.Fatalf("registered rule: %v, %v", ok, err)
	}
	assertBlock(t, output.State, `minecraft:blue_bed ["facing"="north","occupied"="false","part"="foot"]`)
	input.Source["color"] = int32(11)
	if _, ok, err := ConvertBlockEntity("Bed", input); !ok || err == nil {
		t.Fatalf("rule failure: %v, %v", ok, err)
	}
	output, ok, err = ConvertBlockEntity("example:missing", input)
	if ok || err != nil || !reflect.DeepEqual(output, blockentities.Result{}) {
		t.Fatalf("missing rule: %+v, %v, %v", output, ok, err)
	}
}

func TestConvertSection(t *testing.T) {
	// 非零区块和负高度同时覆盖实体定位、规则先于含水，以及与整根入口的一致性。
	pos := celler.ChunkPos{X: -2, Z: 3}
	base := cellerbedrock.NewLayer(cellerbedrock.Block{State: air})
	water := cellerbedrock.NewLayer(cellerbedrock.Block{State: air})
	local := layer_storage.At(1, 15, 1)
	base.Blocks.Set(local, cellerbedrock.Block{State: ir.IRBlock(`minecraft:bed ["direction"=2]`).Tidy()})
	water.Blocks.Set(local, cellerbedrock.Block{State: ir.IRBlock(`minecraft:water ["liquid_depth"=0]`).Tidy()})
	section := &cellerbedrock.Section{Y: -2, Version: 9, Layers: []*cellerbedrock.Layer{base, water}}
	entities := []nbt.Compound{
		{"id": "Bed", "color": int8(11), "x": int32(-31), "y": int32(-17), "z": int32(49)},
		{"id": "Sign", "Text": "世界坐标", "x": int32(-30), "y": int32(-17), "z": int32(49)},
		{"id": "Bed", "color": int8(11), "x": int32(-31), "y": int32(0), "z": int32(49)},
		{"id": "Bed", "color": int8(11), "x": int32(0), "y": int32(-17), "z": int32(49)},
		{"id": "Bed", "x": int8(-31), "y": int32(-17), "z": int32(49)},
	}
	source := cellerbedrock.NewChunk(pos, 1)
	if err := source.SetSection(section); err != nil {
		t.Fatal(err)
	}
	var documents []nbt.Document
	for _, entity := range entities {
		documents = append(documents, nbt.Document{Value: entity})
	}
	if err := source.SetBlockEntities(documents); err != nil {
		t.Fatal(err)
	}
	chunk, err := ConvertChunk(source)
	if err != nil {
		t.Fatal(err)
	}
	// 已解码入口无需可编码的版本号，避免暗中依赖临时柱子往返编码。
	section.Version = 0
	got := ConvertSection(
		pos,
		1,
		section,
		entities,
	)
	if got.Pos != pos || got.Dimension != 1 || len(got.Sections) != 1 {
		t.Fatalf("section metadata: %+v", got)
	}
	for index := range 4096 {
		at := layer_storage.At(index>>8, index&15, index>>4&15)
		assertBlock(t, got.Sections[-2].Blocks.Get(at), chunk.Sections[-2].Blocks.Get(at))
	}
	assertBlock(t, got.Sections[-2].Blocks.Get(local), `minecraft:blue_bed ["facing"="north","occupied"="false","part"="foot","waterlogged"="true"]`)
	if !reflect.DeepEqual(got.BlockEntities, chunk.BlockEntities) || len(got.BlockEntities) != 1 || got.BlockEntities[0].Pos != (celler.BlockPos{X: -30, Y: -17, Z: 49}) {
		t.Fatalf("block entities: %+v", got.BlockEntities)
	}
	if len(got.Diagnostics) != 3 || len(chunk.Diagnostics) != 3 {
		t.Fatalf("diagnostics: %+v / %+v", got.Diagnostics, chunk.Diagnostics)
	}
	for i, diagnostic := range got.Diagnostics {
		if diagnostic.Kind != converter.RuleError || diagnostic.Pos != nil || diagnostic.Err.Error() != chunk.Diagnostics[i].Err.Error() {
			t.Fatalf("outside section: %+v", diagnostic)
		}
	}
	assertBlock(t, base.Blocks.Get(local).State, `minecraft:bed ["direction"=2]`)
}

func TestUnsupportedBlockEntityCounts(t *testing.T) {
	// 按 id 跨 section 合并；已有未知方块诊断的格子不再重复报告实体缺规则。
	source := cellerbedrock.NewChunk(celler.ChunkPos{}, 0)
	for _, y := range []int32{-1, 0} {
		base := cellerbedrock.NewLayer(cellerbedrock.Block{State: air})
		base.Blocks.Set(layer_storage.At(1, 0, 0), cellerbedrock.Block{State: "example:missing []"})
		if err := source.SetSection(&cellerbedrock.Section{Y: y, Version: 9, Layers: []*cellerbedrock.Layer{base}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := source.SetBlockEntities([]nbt.Document{
		{Value: nbt.Compound{"id": "Furnace", "x": int32(0), "y": int32(-16), "z": int32(0)}},
		{Value: nbt.Compound{"id": "Furnace", "x": int32(0), "y": int32(0), "z": int32(0)}},
		{Value: nbt.Compound{"id": "Hopper", "x": int32(2), "y": int32(0), "z": int32(0)}},
		{Value: nbt.Compound{"id": "Furnace", "x": int32(1), "y": int32(0), "z": int32(0)}},
		{Value: nbt.Compound{"id": "OnlyOnUnknown", "x": int32(1), "y": int32(-16), "z": int32(0)}},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := ConvertChunk(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Diagnostics) != 3 || got.Diagnostics[0].Kind != converter.UnknownBlock || got.Diagnostics[0].Count != 2 {
		t.Fatalf("diagnostics: %+v", got.Diagnostics)
	}
	for i, id := range []string{"Furnace", "Hopper"} {
		diagnostic := got.Diagnostics[i+1]
		if diagnostic.Kind != converter.UnsupportedBlockEntity || diagnostic.BlockEntityID != id || diagnostic.Count != uint64(2-i) || diagnostic.Pos != nil || diagnostic.State != "" || diagnostic.Err == nil {
			t.Fatalf("unsupported entity: %+v", diagnostic)
		}
	}
}
