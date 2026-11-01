package bed

import (
	"testing"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
)

func TestConvert(t *testing.T) {
	block := ir.IRBlock(`minecraft:red_bed ["facing"="east","occupied"="true","part"="foot"]`)
	entity := map[string]any{"id": ID, "color": int8(15)}
	want := ir.IRBlock(`minecraft:black_bed ["facing"="east","occupied"="true","part"="foot"]`)
	got, err := Convert(blockentities.Input{Base: block, Source: entity})
	if err != nil {
		t.Fatal(err)
	}
	if got.State.Tidy() != want.Tidy() {
		t.Fatalf("got %s, want %s", got.State, want)
	}
}
