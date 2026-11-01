package jukebox

import (
	"testing"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	for _, test := range []struct {
		name   string
		base   ir.IRBlock
		source nbt.Compound
		want   ir.IRBlock
	}{
		{
			name:   "compound",
			base:   `minecraft:jukebox ["has_record"="false"]`,
			source: nbt.Compound{"RecordItem": nbt.Compound{}},
			want:   `minecraft:jukebox ["has_record"="true"]`,
		},
		{
			name:   "missing",
			base:   `minecraft:jukebox ["has_record"="true"]`,
			source: nbt.Compound{},
			want:   `minecraft:jukebox ["has_record"="false"]`,
		},
		{
			name:   "wrong type",
			base:   `minecraft:jukebox ["has_record"="true"]`,
			source: nbt.Compound{"RecordItem": "record"},
			want:   `minecraft:jukebox ["has_record"="false"]`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Convert(blockentities.Input{Base: test.base, Source: test.source})
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
