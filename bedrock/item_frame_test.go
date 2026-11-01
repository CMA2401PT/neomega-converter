package bedrock

import (
	"reflect"
	"testing"

	"celler/layer_storage"
	"neomega-blocks/ir"
	"neomega-converter"
	"neomega-converter/bedrock/blockentities"
	"neomega-converter/bedrock/blockentities/item_frame"

	nbt "neomega-nbt/presets/v1"
)

func TestItemFrameChunk(t *testing.T) {
	// 两个 id 都接入整根入口；缺省朝向由版本对齐补齐后提供给规则。
	for _, test := range []struct {
		name   string
		state  ir.IRBlock
		nbt    string
		id     string
		facing int8
	}{
		{"normal", `minecraft:frame ["facing_direction"=3]`, `{id:"ItemFrame",Item:{Name:"minecraft:stone",Count:1b}}`, "minecraft:item_frame", 3},
		{"glowing", `minecraft:glow_frame ["facing_direction"=5]`, `{id:"GlowItemFrame",Item:{Name:"minecraft:stone",Count:1b}}`, "minecraft:glow_item_frame", 5},
		{"aligned defaults", `minecraft:frame []`, `{id:"ItemFrame",Item:{Name:"minecraft:stone",Count:1b}}`, "minecraft:item_frame", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			layers := [][]ir.IRBlock{airLayer()}
			layers[0][0] = test.state
			got := convertFixture(t, layers, map[[3]uint8]string{{0, 0, 0}: test.nbt}, -4)
			assertBlock(t, got.Sections[-4].Blocks.Get(layer_storage.At(0, 0, 0)), air)
			if len(got.Entities) != 1 || len(got.BlockEntities) != 0 || len(got.Diagnostics) != 0 {
				t.Fatalf("frame output: %+v", got)
			}
			entity := got.Entities[0]
			if entity["id"] != test.id || entity["Facing"] != test.facing || entity["TileX"] != int32(0) || entity["TileY"] != int32(-64) || entity["TileZ"] != int32(0) || entity["Item"] == nil {
				t.Fatalf("frame entity: %+v", entity)
			}
		})
	}
}

func TestItemFrameDimensions(t *testing.T) {
	// 两个入口都必须把维度传给规则，确保同坐标的跨维度展示框不会重号。
	layers := [][]ir.IRBlock{airLayer()}
	layers[0][0] = `minecraft:frame ["facing_direction"=3]`
	source := fixtureChunk(t, layers, map[[3]uint8]string{{0, 0, 0}: `{id:"ItemFrame"}`}, 0)
	section, err := source.Section(0)
	if err != nil {
		t.Fatal(err)
	}
	documents, err := source.BlockEntities()
	if err != nil {
		t.Fatal(err)
	}
	var previousUUID any
	for _, dimension := range []int32{0, 1} {
		source.Dimension = dimension
		chunk, err := ConvertChunk(source)
		if err != nil {
			t.Fatal(err)
		}
		single := ConvertSection(
			source.Pos,
			dimension,
			section,
			[]nbt.Compound{documents[0].Value},
		)
		if len(chunk.Entities) != 1 || len(single.Entities) != 1 {
			t.Fatalf("missing frame in dimension %d: %+v / %+v", dimension, chunk, single)
		}
		uuid := chunk.Entities[0]["UUID"]
		if !reflect.DeepEqual(uuid, single.Entities[0]["UUID"]) {
			t.Fatalf("entrypoints disagree in dimension %d: %+v / %+v", dimension, chunk.Entities, single.Entities)
		}
		if reflect.DeepEqual(uuid, previousUUID) {
			t.Fatalf("UUID collision across dimensions: %v", uuid)
		}
		previousUUID = uuid
	}
}

func TestItemFrameRuleError(t *testing.T) {
	// 已注册的展示框规则遇到错误方块时按 rule_error 处理，基础状态仍保留。
	layers := [][]ir.IRBlock{airLayer()}
	layers[0][0] = "minecraft:stone []"
	got := convertFixture(t, layers, map[[3]uint8]string{{0, 0, 0}: `{id:"ItemFrame"}`}, 0)
	assertBlock(t, got.Sections[0].Blocks.Get(layer_storage.At(0, 0, 0)), "minecraft:stone []")
	if len(got.Entities) != 0 || len(got.Diagnostics) != 1 || got.Diagnostics[0].Kind != converter.RuleError || got.Diagnostics[0].Pos == nil {
		t.Fatalf("invalid frame: %+v", got)
	}
}

func TestItemFrameRegistry(t *testing.T) {
	// 单格公开入口也能通过两个 Bedrock id 找到同一个展示框规则。
	for _, id := range []string{item_frame.ID, item_frame.GlowID} {
		output, ok, err := ConvertBlockEntity(id, blockentities.Input{
			Base:        air,
			SourceState: `minecraft:glow_frame ["facing_direction"=2]`,
		})
		if !ok || err != nil || output.Entity["id"] != "minecraft:glow_item_frame" {
			t.Fatalf("registry %s: %+v, %v, %v", id, output, ok, err)
		}
	}
}
