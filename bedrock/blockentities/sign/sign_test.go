package sign

import (
	"reflect"
	"testing"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestModernSign(t *testing.T) {
	base := ir.IRBlock(`minecraft:oak_sign ["rotation"="0"]`)
	got, err := Convert(blockentities.Input{
		Base: base,
		Source: nbt.Compound{
			"Text": "旧文字",
			"FrontText": nbt.Compound{
				"Text": "§a首行\n\"第二行\"", "SignTextColor": int32(-0x1000000 + 0xB02E27), "IgnoreLighting": int8(1),
			},
			"BackText": nbt.Compound{
				"Text": "一\n二\n三\n四\n五", "SignTextColor": int32(0x3C44AA), "IgnoreLighting": int8(0),
			},
			"IsWaxed": int8(1),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := nbt.Compound{
		"id": "minecraft:sign",
		"front_text": nbt.Compound{
			"messages": []string{`{"text":"§a首行"}`, `{"text":"\"第二行\""}`, `{"text":""}`, `{"text":""}`},
			"color":    "red", "has_glowing_text": int8(1),
		},
		"back_text": nbt.Compound{
			"messages": []string{`{"text":"一"}`, `{"text":"二"}`, `{"text":"三"}`, `{"text":"四"}`},
			"color":    "blue", "has_glowing_text": int8(0),
		},
		"is_waxed": int8(1),
	}
	if got.State != base || !reflect.DeepEqual(got.BlockEntity, want) {
		t.Fatalf("got %+v, want %v", got, want)
	}
}

func TestLegacyAndHangingSign(t *testing.T) {
	for _, test := range []struct {
		block string
		id    string
	}{
		{"minecraft:oak_wall_sign", "minecraft:sign"},
		{"minecraft:oak_hanging_sign", "minecraft:hanging_sign"},
		{"minecraft:oak_wall_hanging_sign", "minecraft:hanging_sign"},
	} {
		t.Run(test.block, func(t *testing.T) {
			base := ir.IRBlock(test.block + " []")
			got, err := Convert(blockentities.Input{
				Base: base,
				Source: nbt.Compound{
					"Text": "旧版", "SignTextColor": int32(0xF9FFFE), "IgnoreLighting": int8(1),
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			want := nbt.Compound{
				"id": test.id,
				"front_text": nbt.Compound{
					"messages": []string{`{"text":"旧版"}`, `{"text":""}`, `{"text":""}`, `{"text":""}`},
					"color":    "white", "has_glowing_text": int8(1),
				},
				"back_text": nbt.Compound{
					"messages": []string{`{"text":""}`, `{"text":""}`, `{"text":""}`, `{"text":""}`},
					"color":    "black", "has_glowing_text": int8(0),
				},
				"is_waxed": int8(0),
			}
			if got.State != base || !reflect.DeepEqual(got.BlockEntity, want) {
				t.Fatalf("got %+v, want %v", got, want)
			}
		})
	}
}
