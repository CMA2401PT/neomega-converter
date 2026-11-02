package fixer

import "testing"

func TestNoteBlockInstrument(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ below, instrument string }{
		{below: "gold_block", instrument: "bell"},
		{below: "clay", instrument: "flute"},
		{below: "packed_ice", instrument: "chime"},
		{below: "red_wool", instrument: "guitar"},
		{below: "bone_block", instrument: "xylophone"},
		{below: "iron_block", instrument: "iron_xylophone"},
		{below: "soul_sand", instrument: "cow_bell"},
		{below: "pumpkin", instrument: "didgeridoo"},
		{below: "emerald_block", instrument: "bit"},
		{below: "hay_block", instrument: "banjo"},
		{below: "glowstone", instrument: "pling"},
		{below: "sand", instrument: "snare"},
		{below: "red_sand", instrument: "snare"},
		{below: "gravel", instrument: "snare"},
		{below: "white_concrete_powder", instrument: "snare"},
		{below: "suspicious_gravel", instrument: "snare"},
		{below: "glass", instrument: "hat"},
		{below: "sea_lantern", instrument: "hat"},
		{below: "beacon", instrument: "hat"},
		{below: "oak_planks", instrument: "bass"},
		{below: "oak_log", instrument: "bass"},
		{below: "bamboo_block", instrument: "bass"},
		{below: "ender_chest", instrument: "bass"},
		{below: "stone_pressure_plate", instrument: "bass"},
		{below: "iron_door", instrument: "harp"},
		{below: "copper_trapdoor", instrument: "harp"},
		{below: "stone", instrument: "basedrum"},
		{below: "white_concrete", instrument: "basedrum"},
		{below: "respawn_anchor", instrument: "basedrum"},
		{below: "air", instrument: "harp"},
	} {
		t.Run(test.below, func(t *testing.T) {
			chunk := airChunk(t)
			chunk[cell(8, 8, 8)] = stateBlock(t, "note_block", map[string]string{
				"instrument": "zombie", "note": "13", "powered": "true",
			})
			chunk[cell(8, 7, 8)] = stateBlock(t, test.below, nil)
			got := fixSection(f, chunk, sectionNeighbours{})
			assertBlock(t, got[cell(8, 8, 8)], stateBlock(t, "note_block", map[string]string{
				"instrument": test.instrument, "note": "13", "powered": "true",
			}))
			if len(runPass(noteBlockTarget, f.noteBlockInstrument, got, sectionNeighbours{})) != 0 {
				t.Fatal("unchanged instruments were reported as changes")
			}
		})
	}
}

func TestNoteBlockHeadInstrument(t *testing.T) {
	f := &Fixer{}
	for _, test := range []struct{ head, instrument string }{
		{head: "zombie_head", instrument: "zombie"},
		{head: "skeleton_skull", instrument: "skeleton"},
		{head: "creeper_head", instrument: "creeper"},
		{head: "dragon_head", instrument: "dragon"},
		{head: "wither_skeleton_skull", instrument: "wither_skeleton"},
		{head: "piglin_head", instrument: "piglin"},
		{head: "player_head", instrument: "custom_head"},
		{head: "zombie_wall_head", instrument: "bell"},
	} {
		t.Run(test.head, func(t *testing.T) {
			chunk := airChunk(t)
			chunk[cell(8, 8, 8)] = stateBlock(t, "note_block", nil)
			chunk[cell(8, 7, 8)] = stateBlock(t, "gold_block", nil)
			chunk[cell(8, 9, 8)] = stateBlock(t, test.head, nil)
			changes := runPass(noteBlockTarget, f.noteBlockInstrument, chunk, sectionNeighbours{})
			assertBlock(t, changes[cell(8, 8, 8)], stateBlock(t, "note_block", map[string]string{
				"instrument": test.instrument,
			}))
		})
	}
}

func TestNoteBlockMissingNeighbours(t *testing.T) {
	f := &Fixer{}
	chunk, adjacent := airChunk(t), airChunk(t)
	chunk[cell(8, 0, 8)] = stateBlock(t, "note_block", map[string]string{"instrument": "bell"})
	changes := runPass(noteBlockTarget, f.noteBlockInstrument, chunk, sectionNeighbours{})
	assertBlock(t, changes[cell(8, 0, 8)], stateBlock(t, "note_block", map[string]string{"instrument": "harp"}))
	adjacent[cell(8, 15, 8)] = stateBlock(t, "gold_block", nil)
	if len(runPass(noteBlockTarget, f.noteBlockInstrument, chunk, sectionNeighbours{2: adjacent})) != 0 {
		t.Fatal("gold block in lower section must retain bell")
	}

	chunk = airChunk(t)
	chunk[cell(8, 15, 8)] = stateBlock(t, "note_block", nil)
	chunk[cell(8, 14, 8)] = stateBlock(t, "gold_block", nil)
	changes = runPass(noteBlockTarget, f.noteBlockInstrument, chunk, sectionNeighbours{})
	assertBlock(t, changes[cell(8, 15, 8)], stateBlock(t, "note_block", map[string]string{"instrument": "bell"}))
	adjacent[cell(8, 0, 8)] = stateBlock(t, "zombie_head", nil)
	changes = runPass(noteBlockTarget, f.noteBlockInstrument, chunk, sectionNeighbours{3: adjacent})
	assertBlock(t, changes[cell(8, 15, 8)], stateBlock(t, "note_block", map[string]string{"instrument": "zombie"}))
}
