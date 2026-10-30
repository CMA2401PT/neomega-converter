package bed

import (
	"testing"

	"neomega-blocks/ir"
)

func TestConvert(t *testing.T) {
	block := ir.IRBlock(`minecraft:red_bed ["facing"="east","occupied"="true","part"="foot"]`)
	entity := map[string]any{"id": ID, "color": int8(15)}
	want := ir.IRBlock(`minecraft:black_bed ["facing"="east","occupied"="true","part"="foot"]`)
	if got := Convert(block, entity); got.Tidy() != want.Tidy() {
		t.Fatalf("got %s, want %s", got, want)
	}
}
