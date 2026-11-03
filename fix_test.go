package converter

import (
	"reflect"
	"testing"

	"celler"
	"celler/java"
	"celler/layer_storage"
	"neomega-blocks/ir"
	core "neomega-converter-core"
	"neomega-converter/fixpalette"
	nbt "neomega-nbt/presets/v1"
	fixer "neomega-viewer-core/fixer/java-postprocess"
)

func TestFixChunkPreservesMetadataAndOwnership(t *testing.T) {
	// 修复会复制有变化的调色板；转换层持有的实体、诊断和生物群系必须保留。
	f, err := fixer.New()
	if err != nil {
		t.Fatal(err)
	}
	blocks := layer_storage.MakePaletteStorage(ir.IRBlock("minecraft:air []"))
	at := layer_storage.At(15, 2, 8)
	original := ir.IRBlock("minecraft:oak_fence []")
	blocks.Set(at, original)
	east := layer_storage.MakePaletteStorage(ir.IRBlock("minecraft:air []"))
	east.Set(layer_storage.At(0, 2, 8), "minecraft:stone []")
	untouched := &core.ConvertedSection{Blocks: layer_storage.MakePaletteStorage(ir.IRBlock("minecraft:stone []"))}
	biomes := &[64]string{"minecraft:plains"}
	center := &core.ConvertedChunk{
		Pos:           celler.ChunkPos{X: -7, Z: 4},
		Dimension:     2,
		Sections:      map[int32]*core.ConvertedSection{0: {Blocks: blocks, Biomes: biomes}, 1: untouched},
		BlockEntities: []java.BlockEntity{{NBT: nbt.Compound{"id": "test"}}},
		Entities:      []nbt.Compound{{"id": "minecraft:item_frame"}},
		Diagnostics:   []core.Diagnostic{{Kind: core.UnknownBlock, Count: 2}},
	}
	neighbour := &core.ConvertedChunk{Sections: map[int32]*core.ConvertedSection{0: {Blocks: east}}}
	fixed := FixChunk(f, center, [8]*core.ConvertedChunk{fixpalette.E: neighbour})
	if fixed.Sections[0].Blocks.Get(at).States().GetDefault("east", "false") != "true" {
		t.Fatal("未使用邻居修复栅栏连接")
	}
	if fixed == center || fixed.Sections[0] == center.Sections[0] || fixed.Sections[1] != untouched || fixed.Sections[0].Biomes != biomes {
		t.Fatal("修改与共享的边界错误")
	}
	if blocks.Get(at) != original || east.Get(layer_storage.At(0, 2, 8)) != "minecraft:stone []" {
		t.Fatal("修复修改了输入")
	}
	if fixed.Pos != center.Pos || fixed.Dimension != center.Dimension || !reflect.DeepEqual(fixed.BlockEntities, center.BlockEntities) || !reflect.DeepEqual(fixed.Entities, center.Entities) || !reflect.DeepEqual(fixed.Diagnostics, center.Diagnostics) {
		t.Fatal("修复丢失转换结果中的元数据")
	}
}
