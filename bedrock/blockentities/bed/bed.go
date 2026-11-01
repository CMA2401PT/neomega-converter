package bed

import (
	"fmt"

	"neomega-blocks/ir"
	"neomega-converter/bedrock/blockentities"
)

const ID = "Bed"

func Convert(input blockentities.Input) (blockentities.Result, error) {
	// 颜色只存在于 Bedrock NBT，改名时保留基础转换得到的朝向、床头尾和占用状态。
	colors := [...]string{
		"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray",
		"light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black",
	}
	color, ok := input.Source["color"].(int8)
	if !ok || color < 0 || int(color) >= len(colors) {
		return blockentities.Result{}, fmt.Errorf("bed color must be a Byte in 0..15")
	}
	state := ir.IRBlock("minecraft:" + colors[color] + "_bed " + string(input.Base.States())).Tidy()
	return blockentities.Result{State: state}, nil
}
