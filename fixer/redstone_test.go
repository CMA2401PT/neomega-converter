package fixer

import "testing"

func TestRedstoneConnections(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	wire := stateBlock(t, "redstone_wire", map[string]string{
		"power": "7", "north": "up", "east": "up", "south": "up", "west": "up",
	})
	type placement struct {
		x, y, z int
		name    string
	}
	for _, test := range []struct {
		name   string
		blocks []placement
		sides  []string
	}{
		{
			name: "three wire straight",
			blocks: []placement{
				{x: 7, y: 8, z: 8, name: "redstone_wire"},
				{x: 9, y: 8, z: 8, name: "redstone_wire"},
			},
			sides: []string{"west", "east"},
		},
		{
			name:   "endpoint east",
			blocks: []placement{{x: 9, y: 8, z: 8, name: "redstone_wire"}},
			sides:  []string{"west", "east"},
		},
		{
			name:   "endpoint north",
			blocks: []placement{{x: 8, y: 8, z: 7, name: "redstone_wire"}},
			sides:  []string{"north", "south"},
		},
		{
			name: "corner",
			blocks: []placement{
				{x: 9, y: 8, z: 8, name: "redstone_wire"},
				{x: 8, y: 8, z: 7, name: "redstone_wire"},
			},
			sides: []string{"east", "north"},
		},
		{
			name: "junction",
			blocks: []placement{
				{x: 7, y: 8, z: 8, name: "redstone_wire"},
				{x: 9, y: 8, z: 8, name: "redstone_wire"},
				{x: 8, y: 8, z: 9, name: "redstone_wire"},
			},
			sides: []string{"west", "east", "south"},
		},
		{name: "isolated preserves existing shape"},
		{
			name: "repeater skips whole wire",
			blocks: []placement{
				{x: 7, y: 8, z: 8, name: "redstone_wire"},
				{x: 9, y: 8, z: 8, name: "repeater"},
			},
		},
		{
			name: "unlisted ordinary block skips whole wire",
			blocks: []placement{
				{x: 7, y: 8, z: 8, name: "redstone_wire"},
				{x: 9, y: 8, z: 8, name: "glass"},
			},
		},
		{
			name: "upward slope",
			blocks: []placement{
				{x: 7, y: 8, z: 8, name: "redstone_wire"},
				{x: 9, y: 8, z: 8, name: "stone"},
				{x: 9, y: 9, z: 8, name: "redstone_wire"},
			},
		},
		{
			name: "downward slope",
			blocks: []placement{
				{x: 7, y: 8, z: 8, name: "redstone_wire"},
				{x: 9, y: 7, z: 8, name: "redstone_wire"},
			},
		},
		{
			name: "ordinary whitelist",
			blocks: []placement{
				{x: 7, y: 8, z: 8, name: "redstone_wire"},
				{x: 9, y: 8, z: 8, name: "stone"},
				{x: 8, y: 8, z: 7, name: "cobblestone"},
				{x: 8, y: 8, z: 9, name: "dirt"},
			},
			sides: []string{"west", "east"},
		},
		{
			name: "air whitelist",
			blocks: []placement{
				{x: 7, y: 8, z: 8, name: "redstone_wire"},
				{x: 9, y: 8, z: 8, name: "cave_air"},
				{x: 8, y: 8, z: 7, name: "void_air"},
			},
			sides: []string{"west", "east"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			chunk := airChunk(t)
			index := cell(8, 8, 8)
			chunk[index] = wire
			for _, block := range test.blocks {
				// 邻线保持默认连接，验证判定只看名称。
				chunk[cell(block.x, block.y, block.z)] = stateBlock(t, block.name, nil)
			}
			want := wire
			if len(test.sides) != 0 {
				states := map[string]string{
					"power": "7", "north": "none", "east": "none", "south": "none", "west": "none",
				}
				for _, side := range test.sides {
					states[side] = "side"
				}
				want = stateBlock(t, "redstone_wire", states)
			}
			got := fixSection(f, chunk, sectionNeighbours{})
			assertBlock(t, got[index], want)
			if _, changed := runPass(redstoneTarget, f.redstoneConnections, got, sectionNeighbours{})[index]; changed {
				t.Fatal("unchanged wire was reported as a change")
			}
		})
	}
}

func TestRedstoneBoundaries(t *testing.T) {
	f := &Fixer{}
	wire := stateBlock(t, "redstone_wire", map[string]string{
		"power": "11", "north": "up", "east": "up", "south": "up", "west": "up",
	})
	for _, test := range []struct {
		name                     string
		side, current, connected int
		probe                    int
		sides                    [2]string
	}{
		{name: "west", side: 0, current: cell(0, 8, 8), connected: cell(1, 8, 8), probe: cell(15, 9, 8), sides: [2]string{"west", "east"}},
		{name: "east", side: 1, current: cell(15, 8, 8), connected: cell(14, 8, 8), probe: cell(0, 7, 8), sides: [2]string{"west", "east"}},
		{name: "down", side: 2, current: cell(8, 0, 8), connected: cell(7, 0, 8), probe: cell(9, 15, 8), sides: [2]string{"west", "east"}},
		{name: "up", side: 3, current: cell(8, 15, 8), connected: cell(7, 15, 8), probe: cell(9, 0, 8), sides: [2]string{"west", "east"}},
		{name: "north", side: 4, current: cell(8, 8, 0), connected: cell(8, 8, 1), probe: cell(8, 9, 15), sides: [2]string{"north", "south"}},
		{name: "south", side: 5, current: cell(8, 8, 15), connected: cell(8, 8, 14), probe: cell(8, 7, 0), sides: [2]string{"north", "south"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			chunk := airChunk(t)
			chunk[test.current], chunk[test.connected] = wire, wire
			if _, changed := runPass(redstoneTarget, f.redstoneConnections, chunk, sectionNeighbours{})[test.current]; changed {
				t.Fatal("wire changed with missing neighbour data")
			}
			var neighbours sectionNeighbours
			neighbours[test.side] = airChunk(t)
			states := map[string]string{
				"power": "11", "north": "none", "east": "none", "south": "none", "west": "none",
			}
			for _, side := range test.sides {
				states[side] = "side"
			}
			assertBlock(t, runPass(redstoneTarget, f.redstoneConnections, chunk, neighbours)[test.current], stateBlock(t, "redstone_wire", states))
			neighbours[test.side][test.probe] = wire
			if _, changed := runPass(redstoneTarget, f.redstoneConnections, chunk, neighbours)[test.current]; changed {
				t.Fatal("wire changed with a slope in an adjacent subchunk")
			}
		})
	}
}

func TestRedstoneMissingDiagonal(t *testing.T) {
	chunk := airChunk(t)
	wire := stateBlock(t, "redstone_wire", map[string]string{
		"north": "up", "east": "up", "south": "up", "west": "up",
	})
	chunk[cell(0, 15, 8)], chunk[cell(1, 15, 8)] = wire, wire
	var neighbours sectionNeighbours
	for side := range neighbours {
		neighbours[side] = airChunk(t)
	}
	f := &Fixer{}
	if _, changed := runPass(redstoneTarget, f.redstoneConnections, chunk, neighbours)[cell(0, 15, 8)]; changed {
		t.Fatal("wire changed without the diagonal subchunk needed above the west neighbour")
	}
}
