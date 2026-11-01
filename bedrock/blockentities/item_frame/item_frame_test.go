package item_frame

import (
	"reflect"
	"testing"

	"celler"
	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"

	nbt "neomega-nbt/presets/v1"
)

func TestFrameKinds(t *testing.T) {
	// 实体种类由源方块决定，不能从已变成空气的 Base 或 NBT id 猜测。
	for _, test := range []struct {
		source ir.IRBlock
		id     string
	}{
		{`minecraft:frame ["facing_direction"=3]`, "minecraft:item_frame"},
		{`minecraft:glow_frame ["facing_direction"=3]`, "minecraft:glow_item_frame"},
	} {
		output, err := Convert(blockentities.Input{
			Base:        "minecraft:air []",
			Source:      nbt.Compound{"id": "ItemFrame"},
			SourceState: test.source,
			WorldPos:    celler.BlockPos{X: -31, Y: -17, Z: 49},
		})
		if err != nil {
			t.Fatal(err)
		}
		entity := output.Entity
		if output.State != "minecraft:air []" || output.BlockEntity != nil || entity["id"] != test.id {
			t.Fatalf("frame output: %+v", output)
		}
		if entity["TileX"] != int32(-31) || entity["TileY"] != int32(-17) || entity["TileZ"] != int32(49) {
			t.Fatalf("frame block coordinates: %+v", entity)
		}
		if _, exists := entity["block_pos"]; exists {
			t.Fatal("frame uses block_pos instead of Java 1.21.4 coordinates")
		}
		if !reflect.DeepEqual(entity["Motion"], []float64{0, 0, 0}) || !reflect.DeepEqual(entity["Rotation"], []float32{0, 0}) {
			t.Fatalf("entity vectors: %+v", entity)
		}
		// 编解码往返确认坐标是 Int、UUID 是 Int 数组，向量保留 Double/Float 列表。
		encoded, err := nbt.EncodeDocument(nbt.Document{Value: entity}, nbt.EncodingBigEndian)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := nbt.DecodeDocument(encoded, nbt.EncodingBigEndian)
		if err != nil || !reflect.DeepEqual(decoded.Value, entity) {
			t.Fatalf("NBT types changed: %+v, %v", decoded.Value, err)
		}
		if _, exists := entity["Item"]; exists {
			t.Fatal("empty frame has Item")
		}
	}
}

func TestFrameItem(t *testing.T) {
	// 复用物品转换，但展示框的物品不带容器槽位；角度按最近的八分之一圈保存。
	for _, test := range []struct {
		name         string
		item         string
		angle        float32
		drop         any
		wantRotation int8
		wantDrop     float32
	}{
		{"default drop", "minecraft:stone", 67.5, nil, 2, 1},
		{"explicit drop and wrap", "minecraft:stone", 382.5, float32(0.25), 1, 0.25},
		{"negative half step", "minecraft:stone", -22.5, nil, 7, 1},
		{"unsupported item", "minecraft:diamond_sword", 90, nil, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := nbt.Compound{
				"Item":         nbt.Compound{"Name": test.item, "Count": int8(1), "Slot": int8(3)},
				"ItemRotation": test.angle,
			}
			if test.drop != nil {
				source["ItemDropChance"] = test.drop
			}
			output, err := Convert(blockentities.Input{
				Source:      source,
				SourceState: `minecraft:frame ["facing_direction"=2]`,
			})
			if err != nil || output.Entity == nil {
				t.Fatalf("frame: %+v, %v", output, err)
			}
			entity := output.Entity
			if test.item == "minecraft:diamond_sword" {
				for _, key := range []string{"Item", "ItemRotation", "ItemDropChance"} {
					if _, exists := entity[key]; exists {
						t.Fatalf("unsupported item retained %s", key)
					}
				}
				return
			}
			wantItem := nbt.Compound{"id": "minecraft:stone", "count": int32(1)}
			if !reflect.DeepEqual(entity["Item"], wantItem) || entity["ItemRotation"] != test.wantRotation || entity["ItemDropChance"] != test.wantDrop {
				t.Fatalf("item fields: %+v", entity)
			}
			if source["Item"].(nbt.Compound)["Slot"] != int8(3) {
				t.Fatal("source item mutated")
			}
		})
	}
}

func TestFrameFacingPositions(t *testing.T) {
	// 六个挂载面都从世界坐标中心反向偏移，不把世界坐标截成 section 内坐标。
	for facing, want := range [][]float64{
		{-30.5, -16.03125, 49.5},
		{-30.5, -16.96875, 49.5},
		{-30.5, -16.5, 49.96875},
		{-30.5, -16.5, 49.03125},
		{-30.03125, -16.5, 49.5},
		{-30.96875, -16.5, 49.5},
	} {
		state, err := ir.NewIRBlockFromGo("minecraft:frame", nbt.Compound{"facing_direction": int32(facing)})
		if err != nil {
			t.Fatal(err)
		}
		output, err := Convert(blockentities.Input{
			SourceState: state,
			WorldPos:    celler.BlockPos{X: -31, Y: -17, Z: 49},
		})
		if err != nil {
			t.Fatal(err)
		}
		if output.Entity["Facing"] != int8(facing) || !reflect.DeepEqual(output.Entity["Pos"], want) {
			t.Fatalf("facing %d: %+v, want %v", facing, output.Entity, want)
		}
	}
}

func TestFrameUUID(t *testing.T) {
	// 重复转换产生同一 UUID，改变任一坐标或维度仍有独立身份。
	input := blockentities.Input{
		SourceState: `minecraft:frame ["facing_direction"=2]`,
		WorldPos:    celler.BlockPos{X: -30000000, Y: -64, Z: 30000000},
	}
	first, err := Convert(input)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := Convert(input)
	if err != nil || !reflect.DeepEqual(first.Entity["UUID"], repeated.Entity["UUID"]) {
		t.Fatalf("UUID not stable: %+v / %+v, %v", first, repeated, err)
	}
	input.Dimension = 1
	otherDimension, err := Convert(input)
	if err != nil || reflect.DeepEqual(first.Entity["UUID"], otherDimension.Entity["UUID"]) {
		t.Fatalf("UUID collision across dimensions: %+v / %+v, %v", first, otherDimension, err)
	}
	input.Dimension = 0
	for _, pos := range []celler.BlockPos{
		{X: -29999999, Y: -64, Z: 30000000},
		{X: -30000000, Y: -63, Z: 30000000},
		{X: -30000000, Y: -64, Z: 29999999},
	} {
		input.WorldPos = pos
		other, err := Convert(input)
		if err != nil || reflect.DeepEqual(first.Entity["UUID"], other.Entity["UUID"]) {
			t.Fatalf("UUID collision at %v: %+v, %v", pos, other, err)
		}
	}
}

func TestInvalidFrameSource(t *testing.T) {
	// 没有可用源朝向时不能把错误的实体位置作为成功输出。
	for _, source := range []ir.IRBlock{
		`minecraft:stone ["facing_direction"=2]`,
		`minecraft:frame []`,
		`minecraft:frame ["facing_direction"=-1]`,
		`minecraft:glow_frame ["facing_direction"=6]`,
	} {
		output, err := Convert(blockentities.Input{SourceState: source})
		if err == nil || !reflect.DeepEqual(output, blockentities.Result{}) {
			t.Fatalf("invalid source %s: %+v, %v", source, output, err)
		}
	}
}
