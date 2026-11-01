package skull

import (
	"testing"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	for _, test := range []struct {
		name    string
		base    ir.IRBlock
		source  nbt.Compound
		want    ir.IRBlock
		wantErr bool
	}{
		{
			name:   "zero",
			base:   `minecraft:skeleton_skull ["rotation"="5"]`,
			source: nbt.Compound{"Rotation": float32(0)},
			want:   `minecraft:skeleton_skull ["rotation"="0"]`,
		},
		{
			name:   "quarter turn",
			base:   `minecraft:player_head ["rotation"="0"]`,
			source: nbt.Compound{"Rotation": float32(90)},
			want:   `minecraft:player_head ["rotation"="4"]`,
		},
		{
			name:   "below half step",
			base:   `minecraft:skeleton_skull ["rotation"="0"]`,
			source: nbt.Compound{"Rotation": float32(11)},
			want:   `minecraft:skeleton_skull ["rotation"="0"]`,
		},
		{
			name:   "half step rounds up",
			base:   `minecraft:skeleton_skull ["rotation"="0"]`,
			source: nbt.Compound{"Rotation": float32(11.25)},
			want:   `minecraft:skeleton_skull ["rotation"="1"]`,
		},
		{
			name:   "last step",
			base:   `minecraft:skeleton_skull ["rotation"="0"]`,
			source: nbt.Compound{"Rotation": float32(337.5)},
			want:   `minecraft:skeleton_skull ["rotation"="15"]`,
		},
		{
			name:   "rounding wraps",
			base:   `minecraft:skeleton_skull ["rotation"="5"]`,
			source: nbt.Compound{"Rotation": float32(348.75)},
			want:   `minecraft:skeleton_skull ["rotation"="0"]`,
		},
		{
			name:   "full turn",
			base:   `minecraft:skeleton_skull ["rotation"="5"]`,
			source: nbt.Compound{"Rotation": float32(360)},
			want:   `minecraft:skeleton_skull ["rotation"="0"]`,
		},
		{
			name:   "wall skull missing rotation",
			base:   `minecraft:skeleton_wall_skull ["facing"="north"]`,
			source: nbt.Compound{},
			want:   `minecraft:skeleton_wall_skull ["facing"="north"]`,
		},
		{
			name:   "wall head wrong rotation type",
			base:   `minecraft:player_wall_head ["facing"="south"]`,
			source: nbt.Compound{"Rotation": "unused"},
			want:   `minecraft:player_wall_head ["facing"="south"]`,
		},
		{
			name:    "missing",
			base:    `minecraft:skeleton_skull ["rotation"="0"]`,
			source:  nbt.Compound{},
			wantErr: true,
		},
		{
			name:    "wrong type",
			base:    `minecraft:skeleton_skull ["rotation"="0"]`,
			source:  nbt.Compound{"Rotation": float64(90)},
			wantErr: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Convert(blockentities.Input{Base: test.base, Source: test.source})
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
