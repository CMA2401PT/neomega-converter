package lectern

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
			name:   "book",
			base:   `minecraft:lectern ["facing"="west","has_book"="false","powered"="true"]`,
			source: nbt.Compound{"hasBook": int8(1)},
			want:   `minecraft:lectern ["facing"="west","has_book"="true","powered"="true"]`,
		},
		{
			name:   "negative nonzero",
			base:   `minecraft:lectern ["facing"="west","has_book"="false","powered"="true"]`,
			source: nbt.Compound{"hasBook": int8(-1)},
			want:   `minecraft:lectern ["facing"="west","has_book"="true","powered"="true"]`,
		},
		{
			name:   "zero",
			base:   `minecraft:lectern ["facing"="west","has_book"="true","powered"="true"]`,
			source: nbt.Compound{"hasBook": int8(0)},
			want:   `minecraft:lectern ["facing"="west","has_book"="false","powered"="true"]`,
		},
		{
			name:   "missing",
			base:   `minecraft:lectern ["facing"="west","has_book"="true","powered"="true"]`,
			source: nbt.Compound{},
			want:   `minecraft:lectern ["facing"="west","has_book"="false","powered"="true"]`,
		},
		{
			name:    "wrong type",
			base:    `minecraft:lectern ["facing"="west","has_book"="true","powered"="true"]`,
			source:  nbt.Compound{"hasBook": int32(1)},
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
