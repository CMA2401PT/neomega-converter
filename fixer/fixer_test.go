package fixer

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"neomega-blocks/geometry/solidtable"
	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
)

func stateBlock(t *testing.T, name string, overrides map[string]string) ir.IRBlock {
	t.Helper()
	properties, ok := java.BlockProperties(name)
	if !ok {
		t.Fatalf("unknown block %s", name)
	}
	fields := make([]string, 0, len(properties))
	for key, property := range properties {
		fields = append(fields, fmt.Sprintf("%q=%s", key, property.Default))
	}
	states := ir.IRStates("[" + strings.Join(fields, ",") + "]").Tidy()
	for key, value := range overrides {
		states = states.Set(key, value)
	}
	block := ir.IRBlock(name + " " + string(states)).Tidy()
	hash := java.BlockToHash(block)
	if _, ok := java.HashToBlock(hash); !ok {
		t.Fatalf("state absent from preset: %s", block)
	}
	return block
}

func airChunk(t *testing.T) []ir.IRBlock {
	t.Helper()
	chunk := make([]ir.IRBlock, 4096)
	air := stateBlock(t, "air", nil)
	for index := range chunk {
		chunk[index] = air
	}
	return chunk
}

func cell(x, y, z int) int { return (x << 8) | (z << 4) | y }

func assertBlock(t *testing.T, got, want ir.IRBlock) {
	t.Helper()
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestFenceConnections(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	chunk := airChunk(t)
	fence := stateBlock(t, "oak_fence", nil)
	chunk[cell(2, 3, 4)], chunk[cell(3, 3, 4)] = fence, fence
	chunk[cell(7, 8, 9)] = stateBlock(t, "oak_fence", map[string]string{
		"north": "true", "east": "true", "south": "true", "west": "true",
	})
	chunk[cell(11, 12, 13)] = fence
	chunk[cell(11, 12, 12)] = stateBlock(t, "stone", nil)
	original := slices.Clone(chunk)
	got := fixSection(f, chunk, sectionNeighbours{})
	assertBlock(t, got[cell(2, 3, 4)], stateBlock(t, "oak_fence", map[string]string{"east": "true"}))
	assertBlock(t, got[cell(3, 3, 4)], stateBlock(t, "oak_fence", map[string]string{"west": "true"}))
	assertBlock(t, got[cell(7, 8, 9)], fence)
	assertBlock(t, got[cell(11, 12, 13)], stateBlock(t, "oak_fence", map[string]string{"north": "true"}))
	if !slices.Equal(chunk, original) {
		t.Fatal("Fix changed its input")
	}
	if !slices.Equal(fixSection(f, got, sectionNeighbours{}), got) {
		t.Fatal("connection repair is not idempotent")
	}
	if len(runPass(connectionTarget, f.connections, got, sectionNeighbours{})) != 0 {
		t.Fatal("unchanged hashes were reported as changes")
	}
}

func TestConnectionFamilies(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		block     string
		other     string
		facing    string
		connected bool
	}{
		{name: "wood types", block: "oak_fence", other: "spruce_fence", connected: true},
		{name: "nether bricks", block: "nether_brick_fence", other: "nether_brick_fence", connected: true},
		{name: "wood to nether", block: "oak_fence", other: "nether_brick_fence"},
		{name: "nether to wood", block: "nether_brick_fence", other: "oak_fence"},
		{name: "gate north", block: "oak_fence", other: "oak_fence_gate", facing: "north", connected: true},
		{name: "gate south", block: "oak_fence", other: "oak_fence_gate", facing: "south", connected: true},
		{name: "gate east", block: "oak_fence", other: "oak_fence_gate", facing: "east"},
		{name: "gate west", block: "oak_fence", other: "oak_fence_gate", facing: "west"},
		{name: "pane", block: "glass_pane", other: "red_stained_glass_pane", connected: true},
		{name: "bars", block: "red_stained_glass_pane", other: "iron_bars", connected: true},
		{name: "glass", block: "iron_bars", other: "glass", connected: true},
		{name: "tinted glass", block: "glass_pane", other: "tinted_glass", connected: true},
		{name: "stained glass", block: "glass_pane", other: "blue_stained_glass", connected: true},
		{name: "wall", block: "glass_pane", other: "cobblestone_wall", connected: true},
		{name: "solid", block: "glass_pane", other: "stone", connected: true},
		{name: "pane to fence", block: "glass_pane", other: "oak_fence"},
		{name: "fence to glass", block: "oak_fence", other: "glass"},
		{name: "partial face", block: "oak_fence", other: "stone_slab"},
	} {
		t.Run(test.name, func(t *testing.T) {
			chunk := airChunk(t)
			chunk[cell(4, 7, 9)] = stateBlock(t, test.block, nil)
			overrides := map[string]string{}
			if test.facing != "" {
				overrides["facing"] = test.facing
			}
			chunk[cell(5, 7, 9)] = stateBlock(t, test.other, overrides)
			value := "false"
			if test.connected {
				value = "true"
			}
			got := fixSection(f, chunk, sectionNeighbours{})
			assertBlock(t, got[cell(4, 7, 9)], stateBlock(t, test.block, map[string]string{"east": value}))
		})
	}
}

func TestBoundaryConnections(t *testing.T) {
	fence := stateBlock(t, "oak_fence", nil)
	stone := stateBlock(t, "stone", nil)
	for _, test := range []struct {
		name     string
		side     int
		current  int
		adjacent int
	}{
		{name: "west", side: 0, current: cell(0, 7, 9), adjacent: cell(15, 7, 9)},
		{name: "east", side: 1, current: cell(15, 7, 9), adjacent: cell(0, 7, 9)},
		{name: "north", side: 4, current: cell(4, 7, 0), adjacent: cell(4, 7, 15)},
		{name: "south", side: 5, current: cell(4, 7, 15), adjacent: cell(4, 7, 0)},
	} {
		t.Run(test.name, func(t *testing.T) {
			f, err := New()
			if err != nil {
				t.Fatal(err)
			}
			chunk, adjacent := airChunk(t), airChunk(t)
			chunk[test.current], adjacent[test.adjacent] = fence, fence
			var neighbours sectionNeighbours
			neighbours[test.side] = adjacent
			want := stateBlock(t, "oak_fence", map[string]string{test.name: "true"})
			assertBlock(t, fixSection(f, chunk, neighbours)[test.current], want)
			assertBlock(t, adjacent[test.adjacent], fence)
			assertBlock(t, fixSection(f, chunk, sectionNeighbours{})[test.current], fence)

			// 只给一面实心，区分朝向当前格与背向当前格的查询。
			adjacent[test.adjacent] = stone
			f.solid = solidtable.Table{java.BlockToHash(stone): 1 << (test.side ^ 1)}
			assertBlock(t, fixSection(f, chunk, neighbours)[test.current], want)
			f.solid = solidtable.Table{java.BlockToHash(stone): 1 << test.side}
			assertBlock(t, fixSection(f, chunk, neighbours)[test.current], fence)
		})
	}
}

func TestPassesAccumulate(t *testing.T) {
	chunk := airChunk(t)
	stone := stateBlock(t, "stone", nil)
	fence := stateBlock(t, "oak_fence", nil)
	third, fourth := position{y: 3}, position{y: 4}
	f := &Fixer{passes: []pass{
		{
			cares: func(ir.IRBlock) bool { return true },
			run: func(view snapshot) map[position]ir.IRBlock {
				return map[position]ir.IRBlock{third: stone}
			},
		},
		{
			cares: func(ir.IRBlock) bool { return true },
			run: func(view snapshot) map[position]ir.IRBlock {
				current, _ := view.get(0, 3, 0)
				assertBlock(t, current, stone)
				return map[position]ir.IRBlock{third: fence, fourth: current}
			},
		},
	}}
	got := fixSection(f, chunk, sectionNeighbours{})
	assertBlock(t, got[3], fence)
	assertBlock(t, got[4], stone)
	assertBlock(t, chunk[3], stateBlock(t, "air", nil))
}
