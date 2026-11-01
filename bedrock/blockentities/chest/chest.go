package chest

import (
	"fmt"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
	"neomega-converter/bedrock/internal/textcomponent"
	"neomega-converter/bedrock/items"
	nbt "neomega-nbt/presets/v1"
)

const ID = "Chest"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	state, err := pairedState(input)
	if err != nil {
		return blockentities.Result{}, err
	}
	converted := []nbt.Compound{}
	if raw, exists := input.Source["Items"]; exists {
		source, ok := nbt.Compounds(raw)
		if !ok {
			return blockentities.Result{}, fmt.Errorf("chest Items must be a compound list")
		}
		for _, item := range source {
			if out := items.ConvertItem(item); out != nil {
				converted = append(converted, out)
			}
		}
	}
	id := "minecraft:chest"
	if input.Base.NameWithNameSpace() == "minecraft:trapped_chest" {
		id = "minecraft:trapped_chest"
	}
	entity := nbt.Compound{"id": id, "Items": converted}
	if name, ok := input.Source["CustomName"].(string); ok {
		entity["CustomName"] = textcomponent.Section(name)
	}
	if loot, ok := input.Source["LootTable"].(string); ok {
		entity["LootTable"] = loot
	}
	return blockentities.Result{State: state, BlockEntity: entity}, nil
}

func pairedState(input blockentities.Input) (ir.IRBlock, error) {
	px, hasX := input.Source["pairx"]
	pz, hasZ := input.Source["pairz"]
	if !hasX || !hasZ {
		return input.Base, nil
	}
	x, xOK := px.(int32)
	z, zOK := pz.(int32)
	if !xOK || !zOK {
		return "", fmt.Errorf("chest pairx/pairz must be Ints")
	}
	facing, _ := input.Base.States().Get("facing")
	var cw [2]int32
	switch facing {
	case "north":
		cw = [2]int32{1, 0}
	case "east":
		cw = [2]int32{0, 1}
	case "south":
		cw = [2]int32{-1, 0}
	case "west":
		cw = [2]int32{0, -1}
	default:
		return "", fmt.Errorf("paired chest requires a horizontal facing")
	}
	// 配对坐标是世界坐标，局部坐标在区块边界处无法相减。
	dx, dz := x-input.WorldPos.X, z-input.WorldPos.Z
	var side string
	switch {
	case dx == cw[0] && dz == cw[1]:
		side = "left"
	case dx == -cw[0] && dz == -cw[1]:
		side = "right"
	default:
		return input.Base, nil
	}
	states := input.Base.States().Set("type", side)
	return ir.IRBlock(input.Base.NameWithNameSpace() + " " + string(states)).Tidy(), nil
}
