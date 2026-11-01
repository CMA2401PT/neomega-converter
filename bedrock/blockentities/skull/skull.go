package skull

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
)

const ID string = "Skull"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	// 墙上头颅的朝向已在方块状态中，无需读取落地旋转角。
	if strings.Contains(input.Base.NameWithNameSpace(), "_wall_") {
		return blockentities.Result{State: input.Base}, nil
	}
	rotation, ok := input.Source["Rotation"].(float32)
	if !ok {
		return blockentities.Result{}, fmt.Errorf("skull Rotation must be a Float")
	}
	step := int(math.Round(float64(rotation)/22.5)) & 15
	states := input.Base.States().Set("rotation", strconv.Itoa(step))
	state := ir.IRBlock(input.Base.NameWithNameSpace() + " " + string(states)).Tidy()
	return blockentities.Result{State: state}, nil
}
