package banner

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
			name:   "standing black",
			base:   `minecraft:white_banner ["rotation"="7"]`,
			source: nbt.Compound{"Base": int32(0)},
			want:   `minecraft:black_banner ["rotation"="7"]`,
		},
		{
			name:   "wall white",
			base:   `minecraft:red_wall_banner ["facing"="east"]`,
			source: nbt.Compound{"Base": int32(15)},
			want:   `minecraft:white_wall_banner ["facing"="east"]`,
		},
		{
			name:   "light gray",
			base:   `minecraft:white_banner ["rotation"="2"]`,
			source: nbt.Compound{"Base": int32(7)},
			want:   `minecraft:light_gray_banner ["rotation"="2"]`,
		},
		{
			name:    "missing",
			base:    `minecraft:white_banner ["rotation"="0"]`,
			source:  nbt.Compound{},
			wantErr: true,
		},
		{
			name:    "wrong type",
			base:    `minecraft:white_banner ["rotation"="0"]`,
			source:  nbt.Compound{"Base": int8(0)},
			wantErr: true,
		},
		{
			name:    "negative",
			base:    `minecraft:white_banner ["rotation"="0"]`,
			source:  nbt.Compound{"Base": int32(-1)},
			wantErr: true,
		},
		{
			name:    "above range",
			base:    `minecraft:white_banner ["rotation"="0"]`,
			source:  nbt.Compound{"Base": int32(16)},
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
