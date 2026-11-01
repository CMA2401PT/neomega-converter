package fixer

import "testing"

func TestWaterloggedRepairs(t *testing.T) {
	// 含水只改变完整状态的 hash，邻接修复仍应识别方块并保留这项属性。
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	chunk := airChunk(t)
	chunk[cell(8, 8, 8)] = stateBlock(t, "oak_stairs", map[string]string{
		"facing": "north", "shape": "straight", "waterlogged": "true",
	})
	chunk[cell(8, 8, 7)] = stateBlock(t, "oak_stairs", map[string]string{
		"facing": "west", "waterlogged": "true",
	})
	chunk[cell(2, 3, 4)] = stateBlock(t, "oak_fence", map[string]string{"waterlogged": "true"})
	chunk[cell(3, 3, 4)] = stateBlock(t, "oak_fence", map[string]string{"waterlogged": "true"})
	got := fixSection(f, chunk, sectionNeighbours{})
	assertBlock(t, got[cell(8, 8, 8)], stateBlock(t, "oak_stairs", map[string]string{
		"facing": "north", "shape": "outer_left", "waterlogged": "true",
	}))
	assertBlock(t, got[cell(2, 3, 4)], stateBlock(t, "oak_fence", map[string]string{
		"east": "true", "waterlogged": "true",
	}))
	assertBlock(t, got[cell(3, 3, 4)], stateBlock(t, "oak_fence", map[string]string{
		"west": "true", "waterlogged": "true",
	}))
}
