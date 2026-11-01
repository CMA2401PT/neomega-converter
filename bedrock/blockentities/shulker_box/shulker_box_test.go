package shulker_box

import (
	"testing"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	const base ir.IRBlock = `minecraft:purple_shulker_box ["facing"="up"]`
	for _, test := range []struct {
		name    string
		source  nbt.Compound
		want    ir.IRBlock
		wantErr bool
	}{
		{
			name:   "down",
			source: nbt.Compound{"facing": int8(0)},
			want:   `minecraft:purple_shulker_box ["facing"="down"]`,
		},
		{
			name:   "up",
			source: nbt.Compound{"facing": int8(1)},
			want:   `minecraft:purple_shulker_box ["facing"="up"]`,
		},
		{
			name:   "north",
			source: nbt.Compound{"facing": int8(2)},
			want:   `minecraft:purple_shulker_box ["facing"="north"]`,
		},
		{
			name:   "south",
			source: nbt.Compound{"facing": int8(3)},
			want:   `minecraft:purple_shulker_box ["facing"="south"]`,
		},
		{
			name:   "west",
			source: nbt.Compound{"facing": int8(4)},
			want:   `minecraft:purple_shulker_box ["facing"="west"]`,
		},
		{
			name:   "east",
			source: nbt.Compound{"facing": int8(5)},
			want:   `minecraft:purple_shulker_box ["facing"="east"]`,
		},
		{
			name:    "missing",
			source:  nbt.Compound{},
			wantErr: true,
		},
		{
			name:    "wrong type",
			source:  nbt.Compound{"facing": int32(1)},
			wantErr: true,
		},
		{
			name:    "negative",
			source:  nbt.Compound{"facing": int8(-1)},
			wantErr: true,
		},
		{
			name:    "above range",
			source:  nbt.Compound{"facing": int8(6)},
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Convert(blockentities.Input{Base: base, Source: test.source})
			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.State.Tidy() != test.want.Tidy() {
				t.Fatalf("got %s, want %s", got.State, test.want)
			}
			if got.BlockEntity != nil {
				t.Fatalf("unexpected block entity: %v", got.BlockEntity)
			}
		})
	}
}
