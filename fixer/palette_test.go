package fixer

import (
	"slices"
	"testing"

	"celler/layer_storage"
	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
	"neomega-converter"
)

func TestFixSectionIncompleteStairs(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		source ir.IRBlock
		states map[string]string
	}{
		{name: "missing properties", source: "minecraft:oak_stairs []"},
		{
			name:   "preserve waterlogged",
			source: `minecraft:oak_stairs ["waterlogged"="true"]`,
			states: map[string]string{"waterlogged": "true"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			blocks := palette(airChunk(t))
			pos := layer_storage.At(8, 8, 8)
			blocks.Set(pos, test.source)
			fixed := f.FixSection(blocks, [6]*layer_storage.Palette[ir.IRBlock]{})
			got := fixed.Get(pos)
			assertBlock(t, got, stateBlock(t, "oak_stairs", test.states))
			if _, ok := java.HashToBlock(java.BlockToHash(got)); !ok {
				t.Fatalf("incomplete output: %s", got)
			}
			assertBlock(t, blocks.Get(pos), test.source)
			if fixed == blocks {
				t.Fatal("completed section must use a new palette")
			}
		})
	}
}

func TestFixCompletesPalettesWithoutChangingInputs(t *testing.T) {
	// 邻居补全后的朝向参与拐角判断，但调用方仍持有原始调色板。
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	centerBlocks, northBlocks := palette(airChunk(t)), palette(airChunk(t))
	pos, northPos := layer_storage.At(8, 8, 0), layer_storage.At(8, 8, 15)
	centerBlocks.Set(pos, "minecraft:oak_stairs []")
	northBlocks.Set(northPos, `minecraft:oak_stairs ["facing"="west"]`)
	centerBefore := slices.Clone(centerBlocks.Palette)
	northBefore := slices.Clone(northBlocks.Palette)
	center := &converter.ConvertedChunk{Sections: map[int32]*converter.ConvertedSection{0: {Blocks: centerBlocks}}}
	north := &converter.ConvertedChunk{Sections: map[int32]*converter.ConvertedSection{0: {Blocks: northBlocks}}}
	fixed := f.Fix(center, ChunkNeighbours{N: north})
	assertBlock(t, fixed.Sections[0].Blocks.Get(pos), stateBlock(t, "oak_stairs", map[string]string{"shape": "outer_left"}))
	if !slices.Equal(centerBlocks.Palette, centerBefore) || !slices.Equal(northBlocks.Palette, northBefore) {
		t.Fatal("Fix changed an input palette")
	}
	if center.Sections[0].Blocks != centerBlocks || north.Sections[0].Blocks != northBlocks || fixed.Sections[0] == center.Sections[0] {
		t.Fatal("only the output section should be replaced")
	}

	// 没有规则修改时，入口补全产生的新调色板也要出现在结果中。
	completed := (&Fixer{}).Fix(center, ChunkNeighbours{N: north})
	assertBlock(t, completed.Sections[0].Blocks.Get(pos), stateBlock(t, "oak_stairs", nil))
	if completed.Sections[0].Blocks == centerBlocks {
		t.Fatal("completion was lost without rule changes")
	}
}

func TestFixSectionCompletesNeighbourPalette(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	blocks, north := palette(airChunk(t)), palette(airChunk(t))
	pos, northPos := layer_storage.At(8, 8, 0), layer_storage.At(8, 8, 15)
	blocks.Set(pos, stateBlock(t, "oak_stairs", nil))
	source := ir.IRBlock(`minecraft:oak_stairs ["facing"="west"]`)
	north.Set(northPos, source)
	fixed := f.FixSection(blocks, [6]*layer_storage.Palette[ir.IRBlock]{4: north})
	assertBlock(t, fixed.Get(pos), stateBlock(t, "oak_stairs", map[string]string{"shape": "outer_left"}))
	assertBlock(t, north.Get(northPos), source)
}

func TestFixSectionPreservesUnchangedPalette(t *testing.T) {
	// 完整状态和无法识别的自定义方块都无需复制或改写。
	f := &Fixer{}
	for _, source := range []ir.IRBlock{
		stateBlock(t, "oak_stairs", nil),
		`minecraft:oak_stairs ["shape"="straight","half"="bottom","facing"="north"]`,
		"minecraft:micro_block []",
	} {
		blocks := layer_storage.MakePaletteStorage(source)
		if fixed := f.FixSection(blocks, [6]*layer_storage.Palette[ir.IRBlock]{}); fixed != blocks {
			t.Fatalf("unchanged palette copied: %s", source)
		}
		assertBlock(t, blocks.Default(), source)
	}
}
