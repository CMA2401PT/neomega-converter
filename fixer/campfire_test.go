package fixer

import "testing"

func TestCampfireSignal(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"campfire", "soul_campfire"} {
		for _, test := range []struct{ below, signal string }{
			{below: "hay_block", signal: "true"},
			{below: "air", signal: "false"},
			{below: "stone", signal: "false"},
		} {
			t.Run(name+"/"+test.below, func(t *testing.T) {
				chunk := airChunk(t)
				input := "true"
				if test.signal == "true" {
					input = "false"
				}
				chunk[cell(8, 8, 8)] = stateBlock(t, name, map[string]string{
					"signal_fire": input, "facing": "west", "lit": "false", "waterlogged": "true",
				})
				chunk[cell(8, 7, 8)] = stateBlock(t, test.below, nil)
				got := fixSection(f, chunk, sectionNeighbours{})
				assertBlock(t, got[cell(8, 8, 8)], stateBlock(t, name, map[string]string{
					"signal_fire": test.signal, "facing": "west", "lit": "false", "waterlogged": "true",
				}))
				if len(runPass(f.campfireSignal, got, sectionNeighbours{})) != 0 {
					t.Fatal("unchanged campfires were reported as changes")
				}
			})
		}
	}

	chunk, down := airChunk(t), airChunk(t)
	chunk[cell(8, 0, 8)] = stateBlock(t, "campfire", map[string]string{"signal_fire": "true"})
	got := fixSection(f, chunk, sectionNeighbours{})
	assertBlock(t, got[cell(8, 0, 8)], stateBlock(t, "campfire", map[string]string{"signal_fire": "false"}))
	down[cell(8, 15, 8)] = stateBlock(t, "hay_block", nil)
	got = fixSection(f, chunk, sectionNeighbours{2: down})
	assertBlock(t, got[cell(8, 0, 8)], chunk[cell(8, 0, 8)])
}
