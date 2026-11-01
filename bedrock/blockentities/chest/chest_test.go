package chest

import (
	"reflect"
	"testing"

	"celler"
	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestPairing(t *testing.T) {
	for _, test := range []struct {
		name   string
		facing string
		source nbt.Compound
		want   string
	}{
		{"north left across chunk", "north", nbt.Compound{"pairx": int32(16), "pairz": int32(-32)}, "left"},
		{"north right", "north", nbt.Compound{"pairx": int32(14), "pairz": int32(-32)}, "right"},
		{"east left", "east", nbt.Compound{"pairx": int32(15), "pairz": int32(-31)}, "left"},
		{"south left", "south", nbt.Compound{"pairx": int32(14), "pairz": int32(-32)}, "left"},
		{"west left", "west", nbt.Compound{"pairx": int32(15), "pairz": int32(-33)}, "left"},
		{"single", "north", nbt.Compound{}, "single"},
		{"missing pairx", "north", nbt.Compound{"pairz": int32(-32)}, "single"},
		{"nonadjacent", "north", nbt.Compound{"pairx": int32(17), "pairz": int32(-32)}, "single"},
		{"wrong axis", "north", nbt.Compound{"pairx": int32(15), "pairz": int32(-31)}, "single"},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := ir.IRBlock(`minecraft:chest ["facing"="` + test.facing + `","type"="single","waterlogged"="false"]`).Tidy()
			got, err := Convert(blockentities.Input{
				Base: base, Source: test.source,
				WorldPos: celler.BlockPos{X: 15, Y: 64, Z: -32},
			})
			if err != nil {
				t.Fatal(err)
			}
			want := ir.IRBlock(base.NameWithNameSpace() + " " + string(base.States().Set("type", test.want))).Tidy()
			if got.State != want {
				t.Fatalf("got %s, want %s", got.State, want)
			}
			if !reflect.DeepEqual(got.BlockEntity, nbt.Compound{"id": "minecraft:chest", "Items": []nbt.Compound{}}) {
				t.Fatalf("empty chest entity lost: %v", got.BlockEntity)
			}
		})
	}
}

func TestItemsAndMetadata(t *testing.T) {
	for _, test := range []struct {
		block string
		id    string
	}{
		{"minecraft:chest", "minecraft:chest"},
		{"minecraft:trapped_chest", "minecraft:trapped_chest"},
		{"minecraft:copper_chest", "minecraft:chest"},
		{"minecraft:waxed_oxidized_copper_chest", "minecraft:chest"},
	} {
		t.Run(test.block, func(t *testing.T) {
			base := ir.IRBlock(test.block + ` ["facing"="north","type"="single"]`).Tidy()
			got, err := Convert(blockentities.Input{
				Base: base,
				Source: nbt.Compound{
					"Items": []nbt.Compound{
						{"Name": "minecraft:oak_log", "Count": int8(4), "Slot": int8(2)},
						{"Name": "minecraft:diamond_sword", "Count": int8(1), "Slot": int8(3)},
					},
					"CustomName": "§a\"储物箱\"", "LootTable": "minecraft:chests/simple_dungeon",
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			want := nbt.Compound{
				"id":         test.id,
				"Items":      []nbt.Compound{{"id": "minecraft:oak_log", "count": int32(4), "Slot": int8(2)}},
				"CustomName": `{"text":"§a\"储物箱\""}`, "LootTable": "minecraft:chests/simple_dungeon",
			}
			if got.State != base || !reflect.DeepEqual(got.BlockEntity, want) {
				t.Fatalf("unexpected result: %+v", got)
			}
		})
	}
}

func TestSkippedItemsKeepChest(t *testing.T) {
	got, err := Convert(blockentities.Input{
		Base: `minecraft:chest ["facing"="north","type"="single"]`,
		Source: nbt.Compound{"Items": []nbt.Compound{
			{"Name": "minecraft:diamond_sword", "Count": int8(1)},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.BlockEntity, nbt.Compound{"id": "minecraft:chest", "Items": []nbt.Compound{}}) {
		t.Fatalf("unexpected empty chest: %v", got.BlockEntity)
	}
}
