package shulker_box

import (
	"fmt"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
)

const ID string = "ShulkerBox"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	// Bedrock 把朝向放在实体 NBT 中，基础方块状态无法提供它。
	directions := [...]string{"down", "up", "north", "south", "west", "east"}
	facing, ok := input.Source["facing"].(int8)
	if !ok || facing < 0 || int(facing) >= len(directions) {
		return blockentities.Result{}, fmt.Errorf("shulker box facing must be a Byte in 0..5")
	}
	states := input.Base.States().Set("facing", directions[facing])
	state := ir.IRBlock(input.Base.NameWithNameSpace() + " " + string(states)).Tidy()
	return blockentities.Result{State: state}, nil
}
