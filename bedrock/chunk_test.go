package bedrock

import (
	"errors"
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"
	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
	"neomega-converter"
	"neomega-converter/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestUnknownBlockCounts(t *testing.T) {
	// 同一未知状态跨版本、跨 section 合并格数，调色板缓存访问不增加计数。
	source := cellerbedrock.NewChunk(celler.ChunkPos{X: -2, Z: 3}, 1)
	unknown := ir.IRBlock("example:unknown []")
	for _, y := range []int32{-2, 0} {
		layer := cellerbedrock.NewLayer(cellerbedrock.Block{State: unknown, Version: y + 10})
		if y == 0 {
			layer.Blocks.Set(layer_storage.At(1, 2, 3), cellerbedrock.Block{State: air})
		}
		if err := source.SetSection(&cellerbedrock.Section{Y: y, Version: 9, Layers: []*cellerbedrock.Layer{layer}}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ConvertChunk(source)
	if err != nil {
		t.Fatal(err)
	}
	if got.Pos != source.Pos || got.Dimension != source.Dimension || len(got.Sections) != 2 {
		t.Fatalf("lost chunk metadata: %+v", got)
	}
	if len(got.Diagnostics) != 1 {
		t.Fatalf("diagnostics: %+v", got.Diagnostics)
	}
	issue := got.Diagnostics[0]
	if issue.Kind != converter.UnknownBlock || issue.State != unknown || issue.Count != 8191 || issue.Pos != nil {
		t.Fatalf("unknown count: %+v", issue)
	}
	for _, section := range got.Sections {
		if section.Biomes != nil {
			t.Fatal("unexpected biome mapping")
		}
		for index := range 4096 {
			assertBlock(t, section.Blocks.Get(layer_storage.At(index>>8, index&15, index>>4&15)), air)
		}
	}
}

func TestChunkDecodeFailures(t *testing.T) {
	// section 无法解码会阻止转换，实体列表损坏只跳过实体规则。
	broken := cellerbedrock.NewChunk(celler.ChunkPos{}, 0)
	broken.Records[string([]byte{cellerbedrock.TagSubChunk, 0})] = []byte{255}
	if _, err := ConvertChunk(broken); err == nil {
		t.Fatal("broken section accepted")
	}
	layers := [][]ir.IRBlock{airLayer(), airLayer()}
	layers[0][1] = `minecraft:bed ["direction"=2,"head_piece_bit"=false,"occupied_bit"=false]`
	layers[1][1] = `minecraft:water ["liquid_depth"=0]`
	source := fixtureChunk(t, layers, map[[3]uint8]string{{0, 1, 0}: `{id:"Bed",color:11b}`}, 0)
	key := string([]byte{cellerbedrock.TagBlockEntities})
	source.Records[key] = append(source.Records[key], 10)
	got, err := ConvertChunk(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != converter.NBTError || got.Diagnostics[0].Pos != nil {
		t.Fatalf("diagnostics: %+v", got.Diagnostics)
	}
	assertBlock(t, got.Sections[0].Blocks.Get(layer_storage.At(0, 1, 0)), `minecraft:red_bed ["facing"="north","occupied"="false","part"="foot","waterlogged"="true"]`)
}

func TestEntityRulesBeforeWater(t *testing.T) {
	// 共用调色板条目的实体仍各自执行规则，规则输出的位置由区块入口统一填写。
	const id = "TestEntity"
	failure := errors.New("rule failed")
	entityRules[id] = func(input blockentities.Input) (blockentities.Result, error) {
		if _, wet := input.Base.States().Get("waterlogged"); wet {
			t.Fatal("entity rule received water before conversion")
		}
		if input.WorldPos != (celler.BlockPos{X: -31, Y: -17, Z: 49}) || input.LocalPos != ([3]uint8{1, 15, 1}) {
			t.Fatalf("rule coordinates: %+v", input)
		}
		return blockentities.Result{
			State:       `minecraft:oak_slab ["type"="bottom"]`,
			BlockEntity: nbt.Compound{"id": "minecraft:test", "x": int32(99), "y": int32(99), "z": int32(99)},
		}, nil
	}
	entityRules["TestFailure"] = func(input blockentities.Input) (blockentities.Result, error) {
		return blockentities.Result{State: "minecraft:gold_block []", BlockEntity: nbt.Compound{"id": "discard"}}, failure
	}
	t.Cleanup(func() { delete(entityRules, id); delete(entityRules, "TestFailure") })
	bed := ir.IRBlock(`minecraft:bed ["direction"=2,"head_piece_bit"=false,"occupied_bit"=false]`).Tidy()
	source := cellerbedrock.NewChunk(celler.ChunkPos{X: -2, Z: 3}, 1)
	base := cellerbedrock.NewLayer(cellerbedrock.Block{State: bed, Version: 1})
	water := cellerbedrock.NewLayer(cellerbedrock.Block{State: ir.IRBlock(`minecraft:flowing_water ["liquid_depth"=3]`)})
	if err := source.SetSection(&cellerbedrock.Section{Y: -2, Version: 9, Layers: []*cellerbedrock.Layer{base, water}}); err != nil {
		t.Fatal(err)
	}
	entities := []nbt.Document{
		{Value: nbt.Compound{"id": id, "x": int32(-31), "y": int32(-17), "z": int32(49)}},
		{Value: nbt.Compound{"id": "TestFailure", "x": int32(-30), "y": int32(-17), "z": int32(49)}},
		{Value: nbt.Compound{"id": "Bed", "color": int8(11), "x": int32(-29), "y": int32(-17), "z": int32(49)}},
	}
	if err := source.SetBlockEntities(entities); err != nil {
		t.Fatal(err)
	}
	got, err := ConvertChunk(source)
	if err != nil {
		t.Fatal(err)
	}
	blocks := got.Sections[-2].Blocks
	assertBlock(t, blocks.Get(layer_storage.At(1, 15, 1)), `minecraft:oak_slab ["type"="bottom","waterlogged"="true"]`)
	assertBlock(t, blocks.Get(layer_storage.At(2, 15, 1)), `minecraft:red_bed ["facing"="north","occupied"="false","part"="foot","waterlogged"="true"]`)
	assertBlock(t, blocks.Get(layer_storage.At(3, 15, 1)), `minecraft:blue_bed ["facing"="north","occupied"="false","part"="foot","waterlogged"="true"]`)
	assertBlock(t, blocks.Get(layer_storage.At(4, 15, 1)), `minecraft:red_bed ["facing"="north","occupied"="false","part"="foot","waterlogged"="true"]`)
	if len(got.BlockEntities) != 1 {
		t.Fatalf("entities: %+v", got.BlockEntities)
	}
	entity := got.BlockEntities[0]
	if entity.Pos != (celler.BlockPos{X: -31, Y: -17, Z: 49}) || entity.NBT["x"] != entity.Pos.X || entity.NBT["y"] != entity.Pos.Y || entity.NBT["z"] != entity.Pos.Z {
		t.Fatalf("entity coordinates: %+v", entity)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != converter.RuleError || !errors.Is(got.Diagnostics[0].Err, failure) || *got.Diagnostics[0].Pos != (celler.BlockPos{X: -30, Y: -17, Z: 49}) {
		t.Fatalf("diagnostics: %+v", got.Diagnostics)
	}
}

func TestInvalidEntities(t *testing.T) {
	// 错误单项不影响同一区块的其它实体，未注册的 id 可以直接跳过。
	bed := ir.IRBlock(`minecraft:bed ["direction"=2,"head_piece_bit"=false,"occupied_bit"=false]`).Tidy()
	for _, test := range []struct {
		name      string
		state     ir.IRBlock
		entity    nbt.Compound
		wantError bool
	}{
		{"missing coordinate", bed, nbt.Compound{"id": "Bed", "color": int8(11), "x": nil}, true},
		{"wrong coordinate type", bed, nbt.Compound{"id": "Bed", "color": int8(11), "x": int8(1)}, true},
		{"outside chunk", bed, nbt.Compound{"id": "Bed", "color": int8(11), "x": int32(16)}, true},
		{"absent section", bed, nbt.Compound{"id": "Bed", "color": int8(11), "y": int32(16)}, true},
		{"missing id", bed, nbt.Compound{}, true},
		{"color type", bed, nbt.Compound{"id": "Bed", "color": int32(11)}, true},
		{"negative color", bed, nbt.Compound{"id": "Bed", "color": int8(-1)}, true},
		{"color range", bed, nbt.Compound{"id": "Bed", "color": int8(16)}, true},
		{"plant type", `minecraft:flower_pot ["update_bit"=false]`, nbt.Compound{"id": "FlowerPot", "PlantBlock": int32(1)}, true},
		{"plant name", `minecraft:flower_pot ["update_bit"=false]`, nbt.Compound{"id": "FlowerPot", "PlantBlock": nbt.Compound{"states": nbt.Compound{}}}, true},
		{"plant states", `minecraft:flower_pot ["update_bit"=false]`, nbt.Compound{"id": "FlowerPot", "PlantBlock": nbt.Compound{"name": "minecraft:stone", "states": int32(0)}}, true},
		{"unregistered id", bed, nbt.Compound{"id": "UnknownEntity"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			layers := [][]ir.IRBlock{airLayer()}
			layers[0][0], layers[0][1] = test.state, bed
			source := fixtureChunk(t, layers, nil, 0)
			value := nbt.Compound{"x": int32(0), "y": int32(0), "z": int32(0)}
			for key, item := range test.entity {
				if item == nil {
					delete(value, key)
				} else {
					value[key] = item
				}
			}
			if err := source.SetBlockEntities([]nbt.Document{
				{Value: value},
				{Value: nbt.Compound{"id": "Bed", "color": int8(11), "x": int32(0), "y": int32(1), "z": int32(0)}},
			}); err != nil {
				t.Fatal(err)
			}
			got, err := ConvertChunk(source)
			if err != nil {
				t.Fatal(err)
			}
			if (len(got.Diagnostics) == 1) != test.wantError || (len(got.Diagnostics) > 0 && got.Diagnostics[0].Kind != converter.RuleError) {
				t.Fatalf("diagnostics: %+v", got.Diagnostics)
			}
			base, _ := converts.ConvertToJava(test.state)
			assertBlock(t, got.Sections[0].Blocks.Get(layer_storage.At(0, 0, 0)), base)
			assertBlock(t, got.Sections[0].Blocks.Get(layer_storage.At(0, 1, 0)), `minecraft:blue_bed ["facing"="north","occupied"="false","part"="foot"]`)
		})
	}
}
