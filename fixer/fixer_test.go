package fixer

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"neomega-assets/solidtable"
	"neomega-blocks/ir"
	java "neomega-blocks/presets/java/v1"
)

func stateHash(t *testing.T, name string, overrides map[string]string) JavaBlockHash {
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
	return hash
}

func airChunk(t *testing.T) []JavaBlockHash {
	t.Helper()
	chunk := make([]JavaBlockHash, 4096)
	air := stateHash(t, "air", nil)
	for index := range chunk {
		chunk[index] = air
	}
	return chunk
}

func cell(x, y, z int) int { return (x << 8) | (z << 4) | y }

func assertHash(t *testing.T, got, want JavaBlockHash) {
	t.Helper()
	if got != want {
		gotBlock, _ := java.HashToBlock(got)
		wantBlock, _ := java.HashToBlock(want)
		t.Fatalf("got %d (%s), want %d (%s)", got, gotBlock, want, wantBlock)
	}
}

func TestFenceConnections(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	chunk := airChunk(t)
	fence := stateHash(t, "oak_fence", nil)
	chunk[cell(2, 3, 4)], chunk[cell(3, 3, 4)] = fence, fence
	chunk[cell(7, 8, 9)] = stateHash(t, "oak_fence", map[string]string{
		"north": "true", "east": "true", "south": "true", "west": "true",
	})
	chunk[cell(11, 12, 13)] = fence
	chunk[cell(11, 12, 12)] = stateHash(t, "stone", nil)
	original := slices.Clone(chunk)
	got := f.Fix(chunk, Neighbours{})
	assertHash(t, got[cell(2, 3, 4)], stateHash(t, "oak_fence", map[string]string{"east": "true"}))
	assertHash(t, got[cell(3, 3, 4)], stateHash(t, "oak_fence", map[string]string{"west": "true"}))
	assertHash(t, got[cell(7, 8, 9)], fence)
	assertHash(t, got[cell(11, 12, 13)], stateHash(t, "oak_fence", map[string]string{"north": "true"}))
	if !slices.Equal(chunk, original) {
		t.Fatal("Fix changed its input")
	}
	if !slices.Equal(f.Fix(got, Neighbours{}), got) {
		t.Fatal("connection repair is not idempotent")
	}
	if len(f.connections(got, Neighbours{})) != 0 {
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
			chunk[cell(4, 7, 9)] = stateHash(t, test.block, nil)
			overrides := map[string]string{}
			if test.facing != "" {
				overrides["facing"] = test.facing
			}
			chunk[cell(5, 7, 9)] = stateHash(t, test.other, overrides)
			value := "false"
			if test.connected {
				value = "true"
			}
			got := f.Fix(chunk, Neighbours{})
			assertHash(t, got[cell(4, 7, 9)], stateHash(t, test.block, map[string]string{"east": value}))
		})
	}
}

func TestBoundaryConnections(t *testing.T) {
	fence := stateHash(t, "oak_fence", nil)
	stone := stateHash(t, "stone", nil)
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
			var neighbours Neighbours
			neighbours[test.side] = adjacent
			want := stateHash(t, "oak_fence", map[string]string{test.name: "true"})
			assertHash(t, f.Fix(chunk, neighbours)[test.current], want)
			assertHash(t, adjacent[test.adjacent], fence)
			assertHash(t, f.Fix(chunk, Neighbours{})[test.current], fence)

			// 只给一面实心，区分朝向当前格与背向当前格的查询。
			adjacent[test.adjacent] = stone
			f.solid = solidtable.Table{stone: 1 << (test.side ^ 1)}
			assertHash(t, f.Fix(chunk, neighbours)[test.current], want)
			f.solid = solidtable.Table{stone: 1 << test.side}
			assertHash(t, f.Fix(chunk, neighbours)[test.current], fence)
		})
	}
}

func TestPassesAccumulate(t *testing.T) {
	chunk := airChunk(t)
	stone := stateHash(t, "stone", nil)
	fence := stateHash(t, "oak_fence", nil)
	f := &Fixer{passes: []pass{
		func(current []JavaBlockHash, neighbours Neighbours) map[int]JavaBlockHash {
			return map[int]JavaBlockHash{3: stone}
		},
		func(current []JavaBlockHash, neighbours Neighbours) map[int]JavaBlockHash {
			assertHash(t, current[3], stone)
			return map[int]JavaBlockHash{3: fence, 4: current[3]}
		},
	}}
	got := f.Fix(chunk, Neighbours{})
	assertHash(t, got[3], fence)
	assertHash(t, got[4], stone)
	assertHash(t, chunk[3], stateHash(t, "air", nil))
}
