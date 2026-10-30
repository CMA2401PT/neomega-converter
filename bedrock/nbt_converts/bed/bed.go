package bed

import "neomega-blocks/ir"

const ID = "Bed"

func Convert(block ir.IRBlock, entity map[string]any) ir.IRBlock {
	// 颜色只存在于 Bedrock NBT，改名时保留基础转换得到的朝向、床头尾和占用状态。
	colors := [...]string{
		"white", "orange", "magenta", "light_blue", "yellow", "lime", "pink", "gray",
		"light_gray", "cyan", "purple", "blue", "brown", "green", "red", "black",
	}
	color := colors[entity["color"].(int8)]
	return ir.IRBlock("minecraft:" + color + "_bed " + string(block.States()))
}
