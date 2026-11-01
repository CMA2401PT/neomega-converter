package note_block

import (
	"testing"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	const base ir.IRBlock = `minecraft:note_block ["instrument"="bass","note"="12","powered"="true"]`
	for _, test := range []struct {
		name    string
		source  nbt.Compound
		want    ir.IRBlock
		wantErr bool
	}{
		{
			name:   "lowest note",
			source: nbt.Compound{"note": int8(0)},
			want:   `minecraft:note_block ["instrument"="bass","note"="0","powered"="true"]`,
		},
		{
			name:   "highest note",
			source: nbt.Compound{"note": int8(24)},
			want:   `minecraft:note_block ["instrument"="bass","note"="24","powered"="true"]`,
		},
		{
			name:    "missing",
			source:  nbt.Compound{},
			wantErr: true,
		},
		{
			name:    "wrong type",
			source:  nbt.Compound{"note": int32(12)},
			wantErr: true,
		},
		{
			name:    "negative",
			source:  nbt.Compound{"note": int8(-1)},
			wantErr: true,
		},
		{
			name:    "above range",
			source:  nbt.Compound{"note": int8(25)},
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
