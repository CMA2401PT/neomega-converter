package bedrock

import (
	"encoding/json"
	"os"
	"testing"

	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
)

func TestSavedSubchunk(t *testing.T) {
	data, err := os.ReadFile("testdata/subchunk.json")
	if err != nil {
		t.Fatal(err)
	}
	var cells []struct {
		Position [3]int       `json:"position"`
		Layers   []ir.IRBlock `json:"layers"`
		SNBT     string       `json:"snbt"`
	}
	if err := json.Unmarshal(data, &cells); err != nil {
		t.Fatal(err)
	}
	layers := [][]ir.IRBlock{airLayer(), airLayer()}
	entities := map[[3]uint8]string{}
	for _, cell := range cells {
		position := [3]uint8{uint8(cell.Position[0]), uint8(cell.Position[1] + 64), uint8(cell.Position[2])}
		index := (int(position[0]) << 8) | (int(position[2]) << 4) | int(position[1])
		for layer, block := range cell.Layers {
			layers[layer][index] = block
		}
		if cell.SNBT != "" {
			entities[position] = cell.SNBT
		}
	}
	blocks, javaNBT := Convert(layers, entities)
	if len(javaNBT) != 0 {
		t.Fatalf("unexpected Java NBT: %v", javaNBT)
	}
	for _, test := range []struct {
		position [3]int
		want     ir.IRBlock
	}{
		{[3]int{2, -60, 6}, `minecraft:blue_bed ["facing"="east","occupied"="false","part"="foot"]`},
		{[3]int{3, -60, 6}, `minecraft:blue_bed ["facing"="east","occupied"="false","part"="head"]`},
		{[3]int{4, -60, 5}, `minecraft:light_gray_bed ["facing"="north","occupied"="false","part"="head"]`},
		{[3]int{4, -60, 6}, `minecraft:light_gray_bed ["facing"="north","occupied"="false","part"="foot"]`},
		{[3]int{5, -60, 4}, `minecraft:potted_dandelion []`},
		{[3]int{6, -60, 4}, `minecraft:potted_orange_tulip []`},
		{[3]int{2, -60, 2}, `minecraft:smooth_quartz_stairs ["facing"="north","half"="bottom","shape"="straight","waterlogged"="true"]`},
		{[3]int{2, -60, 3}, `minecraft:smooth_quartz_stairs ["facing"="west","half"="bottom","shape"="straight","waterlogged"="true"]`},
		{[3]int{3, -60, 2}, `minecraft:smooth_quartz_stairs ["facing"="north","half"="bottom","shape"="straight","waterlogged"="true"]`},
	} {
		index := (test.position[0] << 8) | (test.position[2] << 4) | (test.position[1] + 64)
		assertBlock(t, blocks[index], test.want)
		t.Logf("%v -> %s", test.position, test.want.Tidy())
	}
}

func TestEntityRules(t *testing.T) {
	for _, test := range []struct {
		name   string
		source ir.IRBlock
		snbt   string
		want   ir.IRBlock
	}{
		{
			name:   "occupied black bed",
			source: `minecraft:bed ["direction"=3,"head_piece_bit"=false,"occupied_bit"=true]`,
			snbt:   `{id:"Bed",color:15b}`,
			want:   `minecraft:black_bed ["facing"="east","occupied"="true","part"="foot"]`,
		},
		{
			name:   "plant name translation",
			source: `minecraft:flower_pot ["update_bit"=true]`,
			snbt:   `{id:"FlowerPot",PlantBlock:{name:"minecraft:deadbush",states:{},version:18168865}}`,
			want:   `minecraft:potted_dead_bush []`,
		},
		{
			name:   "no Java potted block",
			source: `minecraft:flower_pot ["update_bit"=true]`,
			snbt:   `{id:"FlowerPot",PlantBlock:{name:"minecraft:stone",states:{},version:18168865}}`,
			want:   `minecraft:flower_pot []`,
		},
		{
			name:   "empty pot",
			source: `minecraft:flower_pot ["update_bit"=false]`,
			snbt:   `{id:"FlowerPot"}`,
			want:   `minecraft:flower_pot []`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			layer := airLayer()
			layer[0] = test.source
			got, javaNBT := Convert([][]ir.IRBlock{layer, nil}, map[[3]uint8]string{{0, 0, 0}: test.snbt})
			assertBlock(t, got[0], test.want)
			if len(javaNBT) != 0 {
				t.Fatalf("unexpected Java NBT: %v", javaNBT)
			}
		})
	}
}

func TestHashConversionAndWater(t *testing.T) {
	layers := [][]ir.IRBlock{airLayer(), airLayer()}
	for index := range 4 {
		layers[0][index] = `minecraft:cobblestone []`
	}
	layers[1][0] = `minecraft:water ["liquid_depth"=7]`
	layers[1][1] = `minecraft:flowing_water ["liquid_depth"=3]`
	layers[1][2] = `minecraft:lava ["liquid_depth"=0]`
	layers[0][4] = `minecraft:missing_block []`
	layers[1][4] = layers[1][0]
	layers[0][5] = `minecraft:bed ["direction"=2,"head_piece_bit"=true,"occupied_bit"=false]`
	hashes := make([][]uint32, len(layers))
	for layer, blocks := range layers {
		hashes[layer] = make([]uint32, len(blocks))
		for index, block := range blocks {
			hashes[layer][index] = block.Tidy().Hash()
		}
	}
	entities := map[[3]uint8]string{{0, 5, 0}: `{id:"Bed",color:8b}`, {0, 3, 0}: `{id:"Chest",Items:[]}`}
	want := []ir.IRBlock{
		`minecraft:cobblestone ["waterlogged"="true"]`,
		`minecraft:cobblestone ["waterlogged"="true"]`,
		`minecraft:cobblestone []`,
		`minecraft:cobblestone []`,
		`minecraft:air []`,
		`minecraft:light_gray_bed ["facing"="north","occupied"="false","part"="head"]`,
	}
	fromBlocks, _ := Convert(layers, entities)
	fromHashes, javaNBT := ConvertHashes(hashes, entities)
	for index, block := range want {
		assertBlock(t, fromBlocks[index], block)
		assertBlock(t, fromHashes[index], block)
	}
	if len(javaNBT) != 0 {
		t.Fatalf("unexpected Java NBT: %v", javaNBT)
	}
	dry, _ := ConvertHashes([][]uint32{hashes[0], nil}, nil)
	assertBlock(t, dry[0], "minecraft:cobblestone []")

	// 新版属性只有源状态能保留；hash 入口无法从表外值恢复它。
	newer := ir.IRBlock(`minecraft:smooth_quartz_stairs ["minecraft:corner"="inner_left","upside_down_bit"=false,"weirdo_direction"=3]`)
	hashes[0][0] = newer.Hash()
	unknown, _ := ConvertHashes(hashes, nil)
	assertBlock(t, unknown[0], "minecraft:air []")
}

func airLayer() []ir.IRBlock {
	layer := make([]ir.IRBlock, 4096)
	for index := range layer {
		layer[index] = "minecraft:air []"
	}
	return layer
}

func assertBlock(t *testing.T, hash uint32, want ir.IRBlock) {
	t.Helper()
	got, ok := java.HashToBlock(hash)
	if !ok || got != want.Tidy() {
		t.Fatalf("hash %d -> (%s, %v), want %s", hash, got, ok, want.Tidy())
	}
}
