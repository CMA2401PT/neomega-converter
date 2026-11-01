package jukebox

import (
	"strconv"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
	nbt "neomega-nbt/presets/v1"
)

const ID string = "Jukebox"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	// 唱片是否存在由物品 compound 表达，基础状态只有默认值。
	_, hasRecord := input.Source["RecordItem"].(nbt.Compound)
	states := input.Base.States().Set("has_record", strconv.FormatBool(hasRecord))
	state := ir.IRBlock(input.Base.NameWithNameSpace() + " " + string(states)).Tidy()
	return blockentities.Result{State: state}, nil
}
