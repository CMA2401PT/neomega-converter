package fixer

import (
	java "neomega-blocks/presets/java/v1"
	"testing"

	"neomega-blocks/geometry/solidtable"
)

func TestFireConnections(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	stone := stateBlock(t, "stone", nil)
	for _, test := range []struct {
		name     string
		side     int
		adjacent int
	}{
		{name: "north", side: 4, adjacent: cell(8, 8, 7)},
		{name: "east", side: 1, adjacent: cell(9, 8, 8)},
		{name: "south", side: 5, adjacent: cell(8, 8, 9)},
		{name: "west", side: 0, adjacent: cell(7, 8, 8)},
		{name: "up", side: 3, adjacent: cell(8, 9, 8)},
		{name: "air"},
	} {
		t.Run(test.name, func(t *testing.T) {
			chunk := airChunk(t)
			chunk[cell(8, 8, 8)] = stateBlock(t, "fire", map[string]string{
				"age": "7", "north": "true", "east": "true", "south": "true", "west": "true", "up": "true",
			})
			states := map[string]string{
				"age": "7", "north": "false", "east": "false", "south": "false", "west": "false", "up": "false",
			}
			if test.name != "air" {
				chunk[test.adjacent] = stone
				states[test.name] = "true"
			}
			want := stateBlock(t, "fire", states)
			got := fixSection(f, chunk, sectionNeighbours{})
			assertBlock(t, got[cell(8, 8, 8)], want)
			if len(runPass(fireTarget, f.fireConnections, got, sectionNeighbours{})) != 0 {
				t.Fatal("unchanged fire was reported as a change")
			}
			if test.name != "air" {
				// 单面实心才能分辨是否取了邻居朝回火的面。
				oneFace := &Fixer{solid: solidtable.Table{java.BlockToHash(stone): 1 << (test.side ^ 1)}}
				assertBlock(t, runPass(fireTarget, oneFace.fireConnections, chunk, sectionNeighbours{})[cell(8, 8, 8)], want)
				oneFace.solid[java.BlockToHash(stone)] = 1 << test.side
				states[test.name] = "false"
				assertBlock(t, runPass(fireTarget, oneFace.fireConnections, chunk, sectionNeighbours{})[cell(8, 8, 8)], stateBlock(t, "fire", states))
			}
		})
	}
}
