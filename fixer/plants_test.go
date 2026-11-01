package fixer

import "testing"

func TestPlantHeads(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		dy   int
	}{
		{name: "kelp", dy: 1},
		{name: "twisting_vines", dy: 1},
		{name: "weeping_vines", dy: -1},
		{name: "cave_vines", dy: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			chunk := airChunk(t)
			properties := map[string]string{}
			if test.name == "cave_vines" {
				properties["berries"] = "true"
			}
			chunk[cell(8, 8, 8)] = stateBlock(t, test.name, properties)
			chunk[cell(8, 8+test.dy, 8)] = stateBlock(t, test.name+"_plant", properties)
			// 其他植物类别不能延长当前植物。
			other := "kelp"
			if test.name == "kelp" {
				other = "twisting_vines"
			}
			chunk[cell(8, 8+2*test.dy, 8)] = stateBlock(t, other, nil)
			got := fixSection(f, chunk, sectionNeighbours{})
			assertBlock(t, got[cell(8, 8, 8)], stateBlock(t, test.name+"_plant", properties))
			assertBlock(t, got[cell(8, 8+test.dy, 8)], stateBlock(t, test.name, properties))
			if len(runPass(f.plantHeads, got, sectionNeighbours{})) != 0 {
				t.Fatal("unchanged plants were reported as changes")
			}

			y, side, adjacentY := 15, 3, 0
			if test.dy == -1 {
				y, side, adjacentY = 0, 2, 15
			}
			chunk = airChunk(t)
			chunk[cell(8, y, 8)] = stateBlock(t, test.name+"_plant", properties)
			got = fixSection(f, chunk, sectionNeighbours{})
			assertBlock(t, got[cell(8, y, 8)], stateBlock(t, test.name, properties))
			adjacent := airChunk(t)
			adjacent[cell(8, adjacentY, 8)] = stateBlock(t, test.name, properties)
			var neighbours sectionNeighbours
			neighbours[side] = adjacent
			got = fixSection(f, chunk, neighbours)
			assertBlock(t, got[cell(8, y, 8)], stateBlock(t, test.name+"_plant", properties))

			chunk[cell(8, y, 8)] = stateBlock(t, test.name, map[string]string{"age": "18"})
			got = fixSection(f, chunk, sectionNeighbours{})
			assertBlock(t, got[cell(8, y, 8)], chunk[cell(8, y, 8)])
		})
	}
}
