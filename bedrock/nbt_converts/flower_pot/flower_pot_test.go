package flower_pot

import (
	"testing"

	"neomega-blocks/ir"
	nbt "neomega-nbt/presets/v1"
)

func TestConvert(t *testing.T) {
	for _, test := range []struct {
		name string
		snbt string
		want ir.IRBlock
	}{
		{
			name: "plant name translation",
			snbt: `{id:"FlowerPot",PlantBlock:{name:"minecraft:deadbush",states:{},version:18168865}}`,
			want: `minecraft:potted_dead_bush []`,
		},
		{
			name: "no Java potted block",
			snbt: `{id:"FlowerPot",PlantBlock:{name:"minecraft:stone",states:{},version:18168865}}`,
			want: `minecraft:flower_pot []`,
		},
		{
			name: "empty pot",
			snbt: `{id:"FlowerPot"}`,
			want: `minecraft:flower_pot []`,
		},
		{
			name: "azalea",
			snbt: `{id:"FlowerPot",PlantBlock:{name:"minecraft:azalea",states:{},version:18168865}}`,
			want: `minecraft:potted_azalea_bush []`,
		},
		{
			name: "flowering azalea",
			snbt: `{id:"FlowerPot",PlantBlock:{name:"minecraft:flowering_azalea",states:{},version:18168865}}`,
			want: `minecraft:potted_flowering_azalea_bush []`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var entity map[string]any
			if err := nbt.DecodeFromSNBT(test.snbt, &entity, nbt.OptDecodeFromSNBT{}); err != nil {
				t.Fatal(err)
			}
			got := Convert("minecraft:flower_pot []", entity)
			if got.Tidy() != test.want.Tidy() {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}
