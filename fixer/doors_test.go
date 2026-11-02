package fixer

import (
	"slices"
	"strings"
	"testing"

	"neomega-blocks/ir"
	converts "neomega-blocks/presets/block-converts/v1"
)

func TestDoorRoundtripRepair(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"oak_door", "spruce_door", "birch_door", "iron_door"} {
		for _, facing := range []string{"north", "east", "south", "west"} {
			for _, hinge := range []string{"left", "right"} {
				for _, open := range []string{"false", "true"} {
					t.Run(strings.Join([]string{name, facing, hinge, open}, "/"), func(t *testing.T) {
						chunk := airChunk(t)
						positions := [2]int{cell(4, 7, 9), cell(4, 8, 9)}
						var original [2]ir.IRBlock
						for i, half := range []string{"lower", "upper"} {
							original[i] = stateBlock(t, name, map[string]string{
								"half": half, "facing": facing, "hinge": hinge, "open": open, "powered": "true",
							})
							chunk[positions[i]] = roundtripDoorBlock(t, original[i])
						}
						before := slices.Clone(chunk)
						got := fixSection(f, chunk, sectionNeighbours{})
						for i, position := range positions {
							assertDoorRestored(t, got[position], original[i], before[position])
						}
						if !slices.Equal(chunk, before) {
							t.Fatal("door repair mutated input snapshot")
						}
						if len(runPass(doorTarget, f.doorHalves, got, sectionNeighbours{})) != 0 {
							t.Fatal("unchanged doors reported as changes")
						}
					})
				}
			}
		}
	}
}

func TestDoorAcrossSubchunkBoundary(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	lower, upper := airChunk(t), airChunk(t)
	positions := [2]int{cell(4, 15, 9), cell(4, 0, 9)}
	var original [2]ir.IRBlock
	for i, half := range []string{"lower", "upper"} {
		original[i] = stateBlock(t, "spruce_door", map[string]string{
			"half": half, "facing": "east", "hinge": "right", "open": "true", "powered": "true",
		})
	}
	lower[positions[0]], upper[positions[1]] = roundtripDoorBlock(t, original[0]), roundtripDoorBlock(t, original[1])
	lowerBefore, upperBefore := slices.Clone(lower), slices.Clone(upper)
	lowerFixed := fixSection(f, lower, sectionNeighbours{3: upper})
	upperFixed := fixSection(f, upper, sectionNeighbours{2: lower})
	assertDoorRestored(t, lowerFixed[positions[0]], original[0], lowerBefore[positions[0]])
	assertDoorRestored(t, upperFixed[positions[1]], original[1], upperBefore[positions[1]])
	if !slices.Equal(lower, lowerBefore) || !slices.Equal(upper, upperBefore) {
		t.Fatal("door repair mutated current or neighbor snapshot")
	}
	if !slices.Equal(fixSection(f, lower, sectionNeighbours{}), lower) || !slices.Equal(fixSection(f, upper, sectionNeighbours{}), upper) {
		t.Fatal("missing neighbor changed door state")
	}
}

func TestDoorRequiresMatchingPartner(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, half := range []string{"lower", "upper"} {
		partnerHalf, partnerY := "upper", 8
		if half == "upper" {
			partnerHalf, partnerY = "lower", 6
		}
		for _, partner := range []struct {
			name   string
			states map[string]string
		}{
			{"air", nil},
			{"stone", nil},
			{"birch_door", map[string]string{"half": partnerHalf}},
			{"oak_door", map[string]string{"half": half}},
		} {
			t.Run(half+"/"+partner.name, func(t *testing.T) {
				chunk := airChunk(t)
				chunk[cell(4, 7, 9)] = stateBlock(t, "oak_door", map[string]string{
					"half": half, "facing": "east", "hinge": "right", "open": "true",
				})
				chunk[cell(4, partnerY, 9)] = stateBlock(t, partner.name, partner.states)
				if len(runPass(doorTarget, f.doorHalves, chunk, sectionNeighbours{})) != 0 {
					t.Fatal("mismatched partner changed door state")
				}
			})
		}
	}
}

func TestDoorKeepsEachHalfPowered(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Fatal(err)
	}
	chunk := airChunk(t)
	lower := cell(4, 7, 9)
	upper := cell(4, 8, 9)
	chunk[lower] = stateBlock(t, "oak_door", map[string]string{
		"half": "lower", "facing": "east", "open": "true", "hinge": "left", "powered": "true",
	})
	chunk[upper] = stateBlock(t, "oak_door", map[string]string{
		"half": "upper", "facing": "north", "open": "false", "hinge": "right", "powered": "false",
	})
	before := slices.Clone(chunk)
	changes := runPass(doorTarget, f.doorHalves, chunk, sectionNeighbours{})
	if !slices.Equal(chunk, before) || len(changes) != 2 {
		t.Fatal("door pass must return both changes without modifying its snapshot")
	}
	assertBlock(t, changes[lower], stateBlock(t, "oak_door", map[string]string{
		"half": "lower", "facing": "east", "open": "true", "hinge": "right", "powered": "true",
	}))
	assertBlock(t, changes[upper], stateBlock(t, "oak_door", map[string]string{
		"half": "upper", "facing": "east", "open": "true", "hinge": "right", "powered": "false",
	}))
}

func roundtripDoorBlock(t *testing.T, block ir.IRBlock) ir.IRBlock {
	t.Helper()
	bedrock, ok := converts.ConvertFromJava(block)
	if !ok {
		t.Fatalf("Java conversion failed: %s", block)
	}
	converted, ok := converts.ConvertToJava(bedrock)
	if !ok {
		t.Fatalf("Bedrock conversion failed: %s", bedrock)
	}
	return converted.Tidy()
}

func assertDoorRestored(t *testing.T, got, original, input ir.IRBlock) {
	t.Helper()
	want := original
	before := input
	powered, _ := before.States().Get("powered")
	states := want.States().Set("powered", powered)
	assertBlock(t, got, ir.IRBlock(want.NameWithNameSpace()+" "+string(states)).Tidy())
}
