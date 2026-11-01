package bedrock

import (
	"encoding/json"
	"os"
	"testing"

	"celler"
	cellerbedrock "celler/bedrock"
	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter"
	nbt "neomega-nbt/presets/v1"
)

func TestSavedSubchunk(t *testing.T) {
	data, err := os.ReadFile("testdata/subchunk.json")
	if err != nil {
		t.Fatal(err)
	}
	var cells []struct {
		Position [3]int       `json:"position"`
		Layers   []ir.IRBlock `json:"layers"`
		SNBT     string       `json:"snbt"`
	}
	if err := json.Unmarshal(data, &cells); err != nil {
		t.Fatal(err)
	}
	layers := [][]ir.IRBlock{airLayer(), airLayer()}
	entities := map[[3]uint8]string{}
	for _, cell := range cells {
		position := [3]uint8{uint8(cell.Position[0]), uint8(cell.Position[1] + 64), uint8(cell.Position[2])}
		index := (int(position[0]) << 8) | (int(position[2]) << 4) | int(position[1])
		for layer, block := range cell.Layers {
			layers[layer][index] = block
		}
		if cell.SNBT != "" {
			entities[position] = cell.SNBT
		}
	}
	converted := convertFixture(t, layers, entities, -4)
	blocks, javaNBT := converted.Sections[-4].Blocks, converted.BlockEntities
	if len(javaNBT) != 0 {
		t.Fatalf("unexpected Java NBT: %v", javaNBT)
	}
	for _, test := range []struct {
		position [3]int
		want     ir.IRBlock
	}{
		{[3]int{2, -60, 6}, `minecraft:blue_bed ["facing"="east","occupied"="false","part"="foot"]`},
		{[3]int{3, -60, 6}, `minecraft:blue_bed ["facing"="east","occupied"="false","part"="head"]`},
		{[3]int{4, -60, 5}, `minecraft:light_gray_bed ["facing"="north","occupied"="false","part"="head"]`},
		{[3]int{4, -60, 6}, `minecraft:light_gray_bed ["facing"="north","occupied"="false","part"="foot"]`},
		{[3]int{5, -60, 4}, `minecraft:potted_dandelion []`},
		{[3]int{6, -60, 4}, `minecraft:potted_orange_tulip []`},
		{[3]int{2, -60, 2}, `minecraft:smooth_quartz_stairs ["facing"="north","half"="bottom","shape"="straight","waterlogged"="true"]`},
		{[3]int{2, -60, 3}, `minecraft:smooth_quartz_stairs ["facing"="west","half"="bottom","shape"="straight","waterlogged"="true"]`},
		{[3]int{3, -60, 2}, `minecraft:smooth_quartz_stairs ["facing"="north","half"="bottom","shape"="straight","waterlogged"="true"]`},
	} {
		index := (test.position[0] << 8) | (test.position[2] << 4) | (test.position[1] + 64)
		assertBlock(t, blocks.Get(layer_storage.At(index>>8, index&15, index>>4&15)), test.want)
		t.Logf("%v -> %s", test.position, test.want.Tidy())
	}
}

func TestChunkConversionAndWater(t *testing.T) {
	layers := [][]ir.IRBlock{airLayer(), airLayer()}
	for index := range 4 {
		layers[0][index] = `minecraft:cobblestone []`
	}
	layers[1][0] = `minecraft:water ["liquid_depth"=7]`
	layers[1][1] = `minecraft:flowing_water ["liquid_depth"=3]`
	layers[1][2] = `minecraft:lava ["liquid_depth"=0]`
	layers[0][4] = `minecraft:missing_block []`
	layers[1][4] = layers[1][0]
	layers[0][5] = `minecraft:bed ["direction"=2,"head_piece_bit"=true,"occupied_bit"=false]`
	entities := map[[3]uint8]string{{0, 5, 0}: `{id:"Bed",color:8b}`, {0, 3, 0}: `{id:"UnknownEntity"}`}
	want := []ir.IRBlock{
		`minecraft:cobblestone ["waterlogged"="true"]`,
		`minecraft:cobblestone ["waterlogged"="true"]`,
		`minecraft:cobblestone []`,
		`minecraft:cobblestone []`,
		`minecraft:air []`,
		`minecraft:light_gray_bed ["facing"="north","occupied"="false","part"="head"]`,
	}
	converted := convertFixture(t, layers, entities, 0)
	for index, block := range want {
		assertBlock(t, converted.Sections[0].Blocks.Get(layer_storage.At(0, index, 0)), block)
	}
	if len(converted.BlockEntities) != 0 {
		t.Fatalf("unexpected Java NBT: %v", converted.BlockEntities)
	}
	if len(converted.Diagnostics) != 1 || converted.Diagnostics[0].Kind != converter.UnknownBlock || converted.Diagnostics[0].Count != 1 {
		t.Fatalf("unexpected diagnostics: %+v", converted.Diagnostics)
	}
	dry := convertFixture(t, layers[:1], nil, 0)
	assertBlock(t, dry.Sections[0].Blocks.Get(layer_storage.At(0, 0, 0)), "minecraft:cobblestone []")

	// 源状态保留新版属性，转换表负责按目标版本规范化。
	layers[0][0] = ir.IRBlock(`minecraft:smooth_quartz_stairs ["minecraft:corner"="inner_left","upside_down_bit"=false,"weirdo_direction"=3]`)
	newer := convertFixture(t, layers[:1], nil, 0)
	assertBlock(t, newer.Sections[0].Blocks.Get(layer_storage.At(0, 0, 0)), `minecraft:smooth_quartz_stairs ["facing"="north","half"="bottom","shape"="straight"]`)
}

func TestChestAndSignEntities(t *testing.T) {
	layers := [][]ir.IRBlock{airLayer()}
	layers[0][0] = `minecraft:chest ["minecraft:cardinal_direction"="north"]`
	layers[0][1] = `minecraft:birch_standing_sign ["ground_sign_direction"=0]`
	layers[0][2] = `minecraft:birch_hanging_sign ["hanging"=true,"ground_sign_direction"=0]`
	layers[0][3] = layers[0][2]
	converted := convertFixture(t, layers, map[[3]uint8]string{
		{0, 0, 0}: `{id:"Chest",pairx:1,pairz:0,Items:[{Name:"minecraft:oak_log",Count:4b,Slot:2b}]}`,
		{0, 1, 0}: `{id:"Sign",Text:"旧版"}`,
		{0, 2, 0}: `{id:"HangingSign",FrontText:{Text:"悬挂"},IsWaxed:1b}`,
		{0, 3, 0}: `{id:"Sign",FrontText:{Text:"共用 ID"}}`,
	}, -4)
	if len(converted.Diagnostics) != 0 || len(converted.BlockEntities) != 4 {
		t.Fatalf("entities: %v, diagnostics: %v", converted.BlockEntities, converted.Diagnostics)
	}
	chest := converted.Sections[-4].Blocks.Get(layer_storage.At(0, 0, 0))
	if !chest.States().CheckKey("facing", "north") || !chest.States().CheckKey("type", "left") {
		t.Fatalf("unexpected paired chest: %s", chest)
	}
	for _, entity := range converted.BlockEntities {
		if entity.NBT["x"] != entity.Pos.X || entity.NBT["y"] != entity.Pos.Y || entity.NBT["z"] != entity.Pos.Z {
			t.Fatalf("entity coordinates differ: %+v", entity)
		}
		switch entity.Pos.Y {
		case -64:
			items, ok := nbt.Compounds(entity.NBT["Items"])
			if entity.NBT["id"] != "minecraft:chest" || !ok || len(items) != 1 || items[0]["id"] != "minecraft:oak_log" {
				t.Fatalf("unexpected chest: %v", entity.NBT)
			}
		case -63:
			if entity.NBT["id"] != "minecraft:sign" {
				t.Fatalf("unexpected sign: %v", entity.NBT)
			}
		case -62, -61:
			if entity.NBT["id"] != "minecraft:hanging_sign" {
				t.Fatalf("unexpected hanging sign: %v", entity.NBT)
			}
		}
	}
}

func TestLegacyBlockStates(t *testing.T) {
	// 旧状态需先对齐目标 Bedrock 表，成功转换的格子不能计入 unknown_block。
	tests := []struct {
		source ir.IRBlock
		want   ir.IRBlock
	}{
		{`minecraft:wool ["color"="red"]`, `minecraft:red_wool []`},
		{`minecraft:skull ["facing_direction"=1]`, `minecraft:skeleton_skull ["powered"="false","rotation"="0"]`},
		{`minecraft:shulker_box ["color"="red"]`, `minecraft:red_shulker_box ["facing"="up"]`},
		{`minecraft:planks ["wood_type"="spruce"]`, `minecraft:spruce_planks []`},
	}
	layers := [][]ir.IRBlock{airLayer()}
	for index, test := range tests {
		layers[0][index] = test.source
	}
	converted := convertFixture(t, layers, nil, 0)
	for index, test := range tests {
		assertBlock(t, converted.Sections[0].Blocks.Get(layer_storage.At(0, index, 0)), test.want)
	}
	if len(converted.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", converted.Diagnostics)
	}
}

func airLayer() []ir.IRBlock {
	layer := make([]ir.IRBlock, 4096)
	for index := range layer {
		layer[index] = "minecraft:air []"
	}
	return layer
}

func assertBlock(t *testing.T, got, want ir.IRBlock) {
	t.Helper()
	if got != want.Tidy() {
		t.Fatalf("got %s, want %s", got, want.Tidy())
	}
}

func fixtureChunk(t *testing.T, layers [][]ir.IRBlock, entities map[[3]uint8]string, y int32) *cellerbedrock.Chunk {
	// fixture 的位置对应真实 section 高度，实体坐标写入原生 NBT 后随区块读取。
	t.Helper()
	chunk := cellerbedrock.NewChunk(celler.ChunkPos{}, 0)
	section := &cellerbedrock.Section{Y: y, Version: 9}
	for _, states := range layers {
		layer := cellerbedrock.NewLayer(cellerbedrock.Block{State: air})
		for index, state := range states {
			layer.Blocks.Set(layer_storage.At(index>>8, index&15, index>>4&15), cellerbedrock.Block{State: state.Tidy(), Version: 18168865})
		}
		section.Layers = append(section.Layers, layer)
	}
	if err := chunk.SetSection(section); err != nil {
		t.Fatal(err)
	}
	var documents []nbt.Document
	for pos, text := range entities {
		var value nbt.Compound
		if err := nbt.DecodeFromSNBT(text, &value, nbt.OptDecodeFromSNBT{}); err != nil {
			t.Fatal(err)
		}
		value["x"], value["y"], value["z"] = int32(pos[0]), y*16+int32(pos[1]), int32(pos[2])
		documents = append(documents, nbt.Document{Value: value})
	}
	if err := chunk.SetBlockEntities(documents); err != nil {
		t.Fatal(err)
	}
	return chunk
}

func convertFixture(t *testing.T, layers [][]ir.IRBlock, entities map[[3]uint8]string, y int32) *converter.ConvertedChunk {
	t.Helper()
	converted, err := ConvertChunk(fixtureChunk(t, layers, entities, y))
	if err != nil {
		t.Fatal(err)
	}
	return converted
}
