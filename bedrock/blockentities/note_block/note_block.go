package note_block

import (
	"fmt"
	"strconv"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
)

const ID string = "Music"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	note, ok := input.Source["note"].(int8)
	if !ok || note < 0 || note > 24 {
		return blockentities.Result{}, fmt.Errorf("note block note must be a Byte in 0..24")
	}
	// 乐器依赖下方的 Java 方块，留给能访问邻居的 fixer 处理。
	states := input.Base.States().Set("note", strconv.Itoa(int(note)))
	state := ir.IRBlock(input.Base.NameWithNameSpace() + " " + string(states)).Tidy()
	return blockentities.Result{State: state}, nil
}
