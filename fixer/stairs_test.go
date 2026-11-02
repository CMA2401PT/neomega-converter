package fixer

import (
	"testing"

	"neomega-blocks/ir"
)

func TestStairCorners(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, orientation := range []struct {
		facing, left, right string
		dx, dz              int
	}{
		{facing: "north", left: "west", right: "east", dx: 0, dz: -1},
		{facing: "east", left: "north", right: "south", dx: 1, dz: 0},
		{facing: "south", left: "east", right: "west", dx: 0, dz: 1},
		{facing: "west", left: "south", right: "north", dx: -1, dz: 0},
	} {
		for _, half := range []string{"bottom", "top"} {
			for _, corner := range []struct {
				shape, otherFacing string
				distance           int
			}{
				{shape: "outer_left", otherFacing: orientation.left, distance: 1},
				{shape: "outer_right", otherFacing: orientation.right, distance: 1},
				{shape: "inner_left", otherFacing: orientation.left, distance: -1},
				{shape: "inner_right", otherFacing: orientation.right, distance: -1},
			} {
				t.Run(orientation.facing+"/"+half+"/"+corner.shape, func(t *testing.T) {
					chunk := airChunk(t)
					states := map[string]string{
						"facing": orientation.facing, "half": half, "shape": "straight",
					}
					chunk[cell(8, 8, 8)] = stateBlock(t, "oak_stairs", states)
					chunk[cell(8+corner.distance*orientation.dx, 8, 8+corner.distance*orientation.dz)] = stateBlock(t, "stone_stairs", map[string]string{
						"facing": corner.otherFacing, "half": half,
					})
					states["shape"] = corner.shape
					got := fixSection(f, chunk, sectionNeighbours{})
					assertBlock(t, got[cell(8, 8, 8)], stateBlock(t, "oak_stairs", states))
					if len(runPass(stairTarget, f.stairShapes, got, sectionNeighbours{})) != 0 {
						t.Fatal("unchanged stairs were reported as changes")
					}
				})
			}
		}
	}
}

func TestStairShapeSelection(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, frontFacing, frontHalf, backFacing, backHalf, want string
	}{
		{name: "isolated", want: "straight"},
		{name: "different halves", frontFacing: "west", frontHalf: "top", backFacing: "east", backHalf: "top", want: "straight"},
		{name: "parallel", frontFacing: "north", frontHalf: "bottom", backFacing: "south", backHalf: "bottom", want: "straight"},
		{name: "outer takes priority", frontFacing: "west", frontHalf: "bottom", backFacing: "east", backHalf: "bottom", want: "outer_left"},
		{name: "skip front with different half", frontFacing: "west", frontHalf: "top", backFacing: "east", backHalf: "bottom", want: "inner_right"},
	} {
		t.Run(test.name, func(t *testing.T) {
			chunk := airChunk(t)
			states := map[string]string{"facing": "north", "half": "bottom", "shape": "inner_right"}
			chunk[cell(8, 8, 8)] = stateBlock(t, "oak_stairs", states)
			if test.frontFacing != "" {
				chunk[cell(8, 8, 7)] = stateBlock(t, "oak_stairs", map[string]string{"facing": test.frontFacing, "half": test.frontHalf})
			}
			if test.backFacing != "" {
				chunk[cell(8, 8, 9)] = stateBlock(t, "oak_stairs", map[string]string{"facing": test.backFacing, "half": test.backHalf})
			}
			states["shape"] = test.want
			assertBlock(t, fixSection(f, chunk, sectionNeighbours{})[cell(8, 8, 8)], stateBlock(t, "oak_stairs", states))
		})
	}
}

func TestStairUnknownFacing(t *testing.T) {
	f := &Fixer{}
	for _, source := range []ir.IRBlock{
		`minecraft:oak_stairs ["half"="bottom"]`,
		`minecraft:oak_stairs ["facing"=1,"half"="bottom"]`,
		`minecraft:oak_stairs ["facing"="up","half"="bottom"]`,
	} {
		chunk := airChunk(t)
		pos := cell(8, 8, 8)
		chunk[pos] = source.Tidy()
		if changes := runPass(stairTarget, f.stairShapes, chunk, sectionNeighbours{}); len(changes) != 0 {
			t.Fatalf("unknown facing was changed: %v", changes)
		}
		chunk[pos] = stateBlock(t, "oak_stairs", nil)
		chunk[cell(8, 8, 7)] = source.Tidy()
		if changes := runPass(stairTarget, f.stairShapes, chunk, sectionNeighbours{}); len(changes) != 0 {
			t.Fatalf("unknown facing used as a corner partner: %v", changes)
		}
	}
}
